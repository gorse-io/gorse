// Copyright 2020 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/scylladb/go-set/strset"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/cmd/version"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"math"
	"math/rand"
	"sync"
	"time"
)

// Server manages states of a server node.
type Server struct {
	RestServer
	cachePath    string
	dataPath     string
	masterClient protocol.MasterClient
	serverName   string
	masterHost   string
	masterPort   int
	testMode     bool
	cacheFile    string
}

// NewServer creates a server node.
func NewServer(masterHost string, masterPort int, serverHost string, serverPort int, cacheFile string) *Server {
	s := &Server{
		masterHost: masterHost,
		masterPort: masterPort,
		cacheFile:  cacheFile,
		RestServer: RestServer{
			Settings:   config.NewSettings(),
			HttpHost:   serverHost,
			HttpPort:   serverPort,
			WebService: new(restful.WebService),
		},
	}
	s.RestServer.PopularItemsCache = NewPopularItemsCache(&s.RestServer)
	s.RestServer.HiddenItemsManager = NewHiddenItemsManager(&s.RestServer)
	return s
}

func (s *Server) SetOneMode(settings *config.Settings) {
	s.OneMode = true
	s.Settings = settings
}

// Serve starts a server node.
func (s *Server) Serve() {
	rand.Seed(time.Now().UTC().UnixNano())
	// open local store
	state, err := LoadLocalCache(s.cacheFile)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Logger().Info("no cache file found, create a new one", zap.String("path", s.cacheFile))
		} else {
			log.Logger().Error("failed to connect local store", zap.Error(err),
				zap.String("path", state.path))
		}
	}
	if state.ServerName == "" {
		state.ServerName = base.GetRandomName(0)
		err = state.WriteLocalCache()
		if err != nil {
			log.Logger().Fatal("failed to write meta", zap.Error(err))
		}
	}
	s.serverName = state.ServerName
	log.Logger().Info("start server",
		zap.String("server_name", s.serverName),
		zap.String("server_host", s.HttpHost),
		zap.Int("server_port", s.HttpPort),
		zap.String("master_host", s.masterHost),
		zap.Int("master_port", s.masterPort))

	// connect to master
	conn, err := grpc.Dial(fmt.Sprintf("%v:%v", s.masterHost, s.masterPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Logger().Fatal("failed to connect master", zap.Error(err))
	}
	s.masterClient = protocol.NewMasterClient(conn)

	if !s.OneMode {
		go s.Sync()
	}
	container := restful.NewContainer()
	s.StartHttpServer(container)
}

// Sync this server to the master.
func (s *Server) Sync() {
	defer base.CheckPanic()
	log.Logger().Info("start meta sync", zap.Duration("meta_timeout", s.Config.Master.MetaTimeout))
	for {
		var meta *protocol.Meta
		var err error
		if meta, err = s.masterClient.GetMeta(context.Background(),
			&protocol.NodeInfo{
				NodeType:      protocol.NodeType_ServerNode,
				NodeName:      s.serverName,
				HttpPort:      int64(s.HttpPort),
				BinaryVersion: version.Version,
			}); err != nil {
			log.Logger().Error("failed to get meta", zap.Error(err))
			goto sleep
		}

		// load master config
		err = json.Unmarshal([]byte(meta.Config), &s.Config)
		if err != nil {
			log.Logger().Error("failed to parse master config", zap.Error(err))
			goto sleep
		}

		// connect to data store
		if s.dataPath != s.Config.Database.DataStore {
			log.Logger().Info("connect data store",
				zap.String("database", log.RedactDBURL(s.Config.Database.DataStore)))
			if s.DataClient, err = data.Open(s.Config.Database.DataStore); err != nil {
				log.Logger().Error("failed to connect data store", zap.Error(err))
				goto sleep
			}
			s.dataPath = s.Config.Database.DataStore
		}

		// connect to cache store
		if s.cachePath != s.Config.Database.CacheStore {
			log.Logger().Info("connect cache store",
				zap.String("database", log.RedactDBURL(s.Config.Database.CacheStore)))
			if s.CacheClient, err = cache.Open(s.Config.Database.CacheStore); err != nil {
				log.Logger().Error("failed to connect cache store", zap.Error(err))
				goto sleep
			}
			s.cachePath = s.Config.Database.CacheStore
		}

	sleep:
		if s.testMode {
			return
		}
		time.Sleep(s.Config.Master.MetaTimeout)
	}
}

type PopularItemsCache struct {
	mu     sync.RWMutex
	scores map[string]float64
	server *RestServer
	test   bool
}

func NewPopularItemsCache(s *RestServer) *PopularItemsCache {
	sc := &PopularItemsCache{
		server: s,
		scores: make(map[string]float64),
	}
	go func() {
		for {
			sc.sync()
			log.Logger().Debug("refresh server side popular items cache", zap.String("cache_expire", s.Config.Server.CacheExpire.String()))
			time.Sleep(s.Config.Server.CacheExpire)
		}
	}()
	return sc
}

func newPopularItemsCacheForTest(s *RestServer) *PopularItemsCache {
	sc := &PopularItemsCache{
		server: s,
		scores: make(map[string]float64),
		test:   true,
	}
	return sc
}

func (sc *PopularItemsCache) sync() {
	// load popular items
	items, err := sc.server.CacheClient.GetSorted(cache.Key(cache.PopularItems), 0, -1)
	if err != nil {
		if !errors.IsNotAssigned(err) {
			log.Logger().Error("failed to get popular items", zap.Error(err))
		}
		return
	}
	scores := make(map[string]float64)
	for _, item := range items {
		scores[item.Id] = item.Score
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.scores = scores
}

func (sc *PopularItemsCache) GetSortedScore(member string) float64 {
	if sc.test {
		sc.sync()
	}
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.scores[member]
}

type HiddenItemsManager struct {
	server                  *RestServer
	mu                      sync.RWMutex
	hiddenItems             *strset.Set // global hidden items
	hiddenItemsInCategories sync.Map    // categorized hidden items
	updateTime              time.Time
	test                    bool
}

func NewHiddenItemsManager(s *RestServer) *HiddenItemsManager {
	hc := &HiddenItemsManager{
		server:      s,
		hiddenItems: strset.New(),
	}
	go func() {
		for {
			hc.sync()
			log.Logger().Debug("refresh server side hidden items cache", zap.String("cache_expire", s.Config.Server.CacheExpire.String()))
			time.Sleep(hc.server.Config.Server.CacheExpire)
		}
	}()
	return hc
}

func newHiddenItemsManagerForTest(s *RestServer) *HiddenItemsManager {
	hc := &HiddenItemsManager{
		server:      s,
		hiddenItems: strset.New(),
		test:        true,
	}
	return hc
}

func (hc *HiddenItemsManager) sync() {
	ts := time.Now()
	// load categories
	categories, err := hc.server.CacheClient.GetSet(cache.ItemCategories)
	if err != nil {
		if !errors.IsNotAssigned(err) {
			log.Logger().Error("failed to load item categories", zap.Error(err))
		}
		return
	}
	// load hidden items
	score, err := hc.server.CacheClient.GetSortedByScore(cache.HiddenItemsV2, math.Inf(-1), float64(ts.Unix()))
	if err != nil {
		if !errors.IsNotAssigned(err) {
			log.Logger().Error("failed to load hidden items", zap.Error(err))
		}
		return
	}
	hiddenItems := strset.New(cache.RemoveScores(score)...)
	// load hidden items in categories
	for _, category := range categories {
		score, err = hc.server.CacheClient.GetSortedByScore(cache.Key(cache.HiddenItemsV2, category), math.Inf(-1), float64(ts.Unix()))
		if err != nil {
			if !errors.IsNotAssigned(err) {
				log.Logger().Error("failed to load categorized hidden items", zap.Error(err))
			}
			return
		}
		hc.hiddenItemsInCategories.Store(category, strset.New(cache.RemoveScores(score)...))
	}
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.hiddenItems = hiddenItems
	hc.updateTime = ts
}

func (hc *HiddenItemsManager) IsHidden(members []string, category string) ([]bool, error) {
	if hc.test {
		hc.sync()
	}
	// load hidden items
	hc.mu.RLock()
	hiddenItems := hc.hiddenItems
	updateTime := hc.updateTime
	hc.mu.RUnlock()
	// load hidden items in category
	hiddenItemsInCategory := strset.New()
	if category != "" {
		if temp, exist := hc.hiddenItemsInCategories.Load(category); exist {
			hiddenItemsInCategory = temp.(*strset.Set)
		}
	}
	// load delta hidden items
	score, err := hc.server.CacheClient.GetSortedByScore(cache.HiddenItemsV2, float64(updateTime.Unix()), float64(time.Now().Unix()))
	if err != nil {
		return nil, errors.Trace(err)
	}
	deltaHiddenItems := strset.New(cache.RemoveScores(score)...)
	// load delta hidden items in category
	deltaHiddenItemsInCategory := strset.New()
	if category != "" {
		score, err = hc.server.CacheClient.GetSortedByScore(cache.Key(cache.HiddenItemsV2, category), float64(updateTime.Unix()), float64(time.Now().Unix()))
		if err != nil {
			return nil, errors.Trace(err)
		}
		deltaHiddenItemsInCategory = strset.New(cache.RemoveScores(score)...)
	}
	return lo.Map(members, func(t string, i int) bool {
		return hiddenItems.Has(t) || deltaHiddenItems.Has(t) || hiddenItemsInCategory.Has(t) || deltaHiddenItemsInCategory.Has(t)
	}), nil
}

func (hc *HiddenItemsManager) IsHiddenInCache(member string, category string) bool {
	if hc.test {
		hc.sync()
	}
	// load hidden items
	hc.mu.RLock()
	hiddenItems := hc.hiddenItems
	hc.mu.RUnlock()
	// load hidden items in category
	hiddenItemsInCategory := strset.New()
	if category != "" {
		if temp, exist := hc.hiddenItemsInCategories.Load(category); exist {
			hiddenItemsInCategory = temp.(*strset.Set)
		}
	}
	return hiddenItems.Has(member) || hiddenItemsInCategory.Has(member)
}

type CacheModification struct {
	client             cache.Database
	deletion           []cache.SetMember
	insertion          []cache.SortedSet
	hiddenItemsManager *HiddenItemsManager
}

func NewCacheModification(client cache.Database, hiddenItemsManager *HiddenItemsManager) *CacheModification {
	return &CacheModification{
		client:             client,
		hiddenItemsManager: hiddenItemsManager,
	}
}

func (cm *CacheModification) deleteItemCategory(itemId, category string) *CacheModification {
	cm.insertion = append(cm.insertion, cache.Sorted(cache.Key(cache.HiddenItemsV2, category), []cache.Scored{{itemId, float64(time.Now().Unix())}}))
	return cm
}

func (cm *CacheModification) addItemCategory(itemId, category string, latest, popular float64) *CacheModification {
	// 1. insert item to latest and popular
	if latest > 0 {
		cm.insertion = append(cm.insertion, cache.Sorted(cache.Key(cache.LatestItems, category), []cache.Scored{{itemId, latest}}))
	}
	if popular > 0 {
		cm.insertion = append(cm.insertion, cache.Sorted(cache.Key(cache.PopularItems, category), []cache.Scored{{itemId, popular}}))
	}
	// 2. remove delete mark
	if cm.hiddenItemsManager.IsHiddenInCache(itemId, category) {
		cm.deletion = append(cm.deletion, cache.Member(cache.Key(cache.HiddenItemsV2, category), itemId))
	}
	return cm
}

func (cm *CacheModification) modifyItem(itemId string, prevCategories, categories []string, latest, popular float64) *CacheModification {
	prevCategoriesSet := strset.New(prevCategories...)
	categoriesSet := strset.New(categories...)
	// 2. insert item to category latest and popular
	if latest > 0 {
		cm.insertion = append(cm.insertion, cache.Sorted(cache.Key(cache.LatestItems), []cache.Scored{{itemId, latest}}))
	}
	if popular > 0 {
		cm.insertion = append(cm.insertion, cache.Sorted(cache.Key(cache.PopularItems), []cache.Scored{{itemId, popular}}))
	}
	for _, category := range categories {
		// 2. insert item to category latest and popular
		if latest > 0 {
			cm.insertion = append(cm.insertion, cache.Sorted(cache.Key(cache.LatestItems, category), []cache.Scored{{itemId, latest}}))
		}
		if popular > 0 {
			cm.insertion = append(cm.insertion, cache.Sorted(cache.Key(cache.PopularItems, category), []cache.Scored{{itemId, popular}}))
		}
		// 3. remove delete mark
		if !prevCategoriesSet.Has(category) && cm.hiddenItemsManager.IsHiddenInCache(itemId, category) {
			cm.deletion = append(cm.deletion, cache.Member(cache.Key(cache.HiddenItemsV2, category), itemId))
		}
	}
	// 4. insert category delete mark
	for _, category := range prevCategories {
		if !categoriesSet.Has(category) {
			cm.insertion = append(cm.insertion, cache.Sorted(cache.Key(cache.HiddenItemsV2, category), []cache.Scored{{itemId, float64(time.Now().Unix())}}))
		}
	}
	return cm
}

func (cm *CacheModification) HideItem(itemId string) *CacheModification {
	cm.insertion = append(cm.insertion, cache.Sorted(cache.HiddenItemsV2, []cache.Scored{{itemId, float64(time.Now().Unix())}}))
	return cm
}

func (cm *CacheModification) unHideItem(itemId string) *CacheModification {
	if cm.hiddenItemsManager.IsHiddenInCache(itemId, "") {
		cm.deletion = append(cm.deletion, cache.Member(cache.HiddenItemsV2, itemId))
	}
	return cm
}

func (cm *CacheModification) addItem(itemId string, categories []string, latest, popular float64) {
	// 1. insert item to global latest and popular
	if latest > 0 {
		cm.insertion = append(cm.insertion, cache.Sorted(cache.LatestItems, []cache.Scored{{itemId, latest}}))
	}
	if popular > 0 {
		cm.insertion = append(cm.insertion, cache.Sorted(cache.PopularItems, []cache.Scored{{itemId, popular}}))
	}
	for _, category := range categories {
		// 2. insert item to category latest and popular
		if latest > 0 {
			cm.insertion = append(cm.insertion, cache.Sorted(cache.Key(cache.LatestItems, category), []cache.Scored{{itemId, latest}}))
		}
		if popular > 0 {
			cm.insertion = append(cm.insertion, cache.Sorted(cache.Key(cache.PopularItems, category), []cache.Scored{{itemId, popular}}))
		}
	}
	// 3. remove delete mark
	if cm.hiddenItemsManager.IsHiddenInCache(itemId, "") {
		cm.deletion = append(cm.deletion, cache.Member(cache.HiddenItemsV2, itemId))
	}
}

func (cm *CacheModification) Exec() error {
	if len(cm.deletion) > 0 {
		if err := cm.client.RemSorted(cm.deletion...); err != nil {
			return errors.Trace(err)
		}
	}
	if len(cm.insertion) > 0 {
		if err := cm.client.AddSorted(cm.insertion...); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}
