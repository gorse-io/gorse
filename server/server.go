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
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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
}

// NewServer creates a server node.
func NewServer(masterHost string, masterPort int, serverHost string, serverPort int) *Server {
	return &Server{
		masterHost: masterHost,
		masterPort: masterPort,
		RestServer: RestServer{
			DataClient:  &data.NoDatabase{},
			CacheClient: &cache.NoDatabase{},
			GorseConfig: (*config.Config)(nil).LoadDefaultIfNil(),
			HttpHost:    serverHost,
			HttpPort:    serverPort,
			EnableAuth:  true,
			WebService:  new(restful.WebService),
		},
	}
}

// Serve starts a server node.
func (s *Server) Serve() {
	rand.Seed(time.Now().UTC().UnixNano())
	// open local store
	state, err := LoadLocalCache(filepath.Join(os.TempDir(), "gorse-server"))
	if err != nil {
		base.Logger().Error("failed to connect local store", zap.Error(err),
			zap.String("path", filepath.Join(os.TempDir(), "gorse-server")))
	}
	if state.ServerName == "" {
		state.ServerName = base.GetRandomName(0)
		err = state.WriteLocalCache()
		if err != nil {
			base.Logger().Fatal("failed to write meta", zap.Error(err))
		}
	}
	s.serverName = state.ServerName
	base.Logger().Info("start server",
		zap.String("server_name", s.serverName),
		zap.String("server_host", s.HttpHost),
		zap.Int("server_port", s.HttpPort),
		zap.String("master_host", s.masterHost),
		zap.Int("master_port", s.masterPort))

	// connect to master
	conn, err := grpc.Dial(fmt.Sprintf("%v:%v", s.masterHost, s.masterPort), grpc.WithInsecure())
	if err != nil {
		base.Logger().Fatal("failed to connect master", zap.Error(err))
	}
	s.masterClient = protocol.NewMasterClient(conn)

	go s.Sync()
	s.StartHttpServer()
}

// Sync this server to the master.
func (s *Server) Sync() {
	defer base.CheckPanic()
	base.Logger().Info("start meta sync", zap.Int("meta_timeout", s.GorseConfig.Master.MetaTimeout))
	for {
		var meta *protocol.Meta
		var err error
		if meta, err = s.masterClient.GetMeta(context.Background(),
			&protocol.NodeInfo{
				NodeType: protocol.NodeType_ServerNode,
				NodeName: s.serverName,
				HttpPort: int64(s.HttpPort),
			}); err != nil {
			base.Logger().Error("failed to get meta", zap.Error(err))
			goto sleep
		}

		// load master config
		err = json.Unmarshal([]byte(meta.Config), &s.GorseConfig)
		if err != nil {
			base.Logger().Error("failed to parse master config", zap.Error(err))
			goto sleep
		}

		// connect to data store
		if s.dataPath != s.GorseConfig.Database.DataStore {
			base.Logger().Info("connect data store", zap.String("database", s.GorseConfig.Database.DataStore))
			if s.DataClient, err = data.Open(s.GorseConfig.Database.DataStore); err != nil {
				base.Logger().Error("failed to connect data store", zap.Error(err))
				goto sleep
			}
			s.dataPath = s.GorseConfig.Database.DataStore
		}

		// connect to cache store
		if s.cachePath != s.GorseConfig.Database.CacheStore {
			base.Logger().Info("connect cache store", zap.String("database", s.GorseConfig.Database.CacheStore))
			if s.CacheClient, err = cache.Open(s.GorseConfig.Database.CacheStore); err != nil {
				base.Logger().Error("failed to connect cache store", zap.Error(err))
				goto sleep
			}
			s.cachePath = s.GorseConfig.Database.CacheStore
		}

	sleep:
		if s.testMode {
			return
		}
		time.Sleep(time.Duration(s.GorseConfig.Master.MetaTimeout) * time.Second)
	}
}
