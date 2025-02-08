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

package master

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/emicklei/go-restful/v3"
	"github.com/jellydator/ttlcache/v3"
	"github.com/juju/errors"
	"github.com/sashabaranov/go-openai"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/base/encoding"
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/base/parallel"
	"github.com/zhenghaoz/gorse/base/progress"
	"github.com/zhenghaoz/gorse/base/task"
	"github.com/zhenghaoz/gorse/common/sizeof"
	"github.com/zhenghaoz/gorse/common/util"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/server"
	"github.com/zhenghaoz/gorse/storage"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"github.com/zhenghaoz/gorse/storage/meta"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
)

type ScheduleState struct {
	IsRunning   bool      `json:"is_running"`
	SearchModel bool      `json:"search_model"`
	StartTime   time.Time `json:"start_time"`
}

// Master is the master node.
type Master struct {
	protocol.UnimplementedMasterServer
	server.RestServer
	grpcServer *grpc.Server

	tracer         *progress.Tracer
	remoteProgress sync.Map
	jobsScheduler  *task.JobsScheduler
	cacheFile      string
	managedMode    bool
	openAIClient   *openai.Client

	// cluster meta cache
	metaStore meta.Database

	// ranking dataset
	rankingTrainSet  *ranking.DataSet
	rankingTestSet   *ranking.DataSet
	rankingDataMutex sync.RWMutex

	// click dataset
	clickTrainSet  *click.Dataset
	clickTestSet   *click.Dataset
	clickDataMutex sync.RWMutex

	// ranking model
	rankingModelName     string
	rankingScore         ranking.Score
	rankingModelMutex    sync.RWMutex
	rankingModelSearcher *ranking.ModelSearcher

	// click model
	clickScore         click.Score
	clickModelMutex    sync.RWMutex
	clickModelSearcher *click.ModelSearcher

	// oauth2
	oauth2Config oauth2.Config
	verifier     *oidc.IDTokenVerifier
	tokenCache   *ttlcache.Cache[string, UserInfo]

	localCache *LocalCache

	// events
	fitTicker    *time.Ticker
	importedChan *parallel.ConditionChannel // feedback inserted events
	loadDataChan *parallel.ConditionChannel // dataset loaded events
	triggerChan  *parallel.ConditionChannel // manually trigger events

	scheduleState         ScheduleState
	workerScheduleHandler http.HandlerFunc
}

// NewMaster creates a master node.
func NewMaster(cfg *config.Config, cacheFile string, managedMode bool) *Master {
	rand.Seed(time.Now().UnixNano())

	// setup trace provider
	tp, err := cfg.Tracing.NewTracerProvider()
	if err != nil {
		log.Logger().Fatal("failed to create trace provider", zap.Error(err))
	}
	otel.SetTracerProvider(tp)
	otel.SetErrorHandler(log.GetErrorHandler())
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// setup OpenAI client
	clientConfig := openai.DefaultConfig(cfg.OpenAI.AuthToken)
	clientConfig.BaseURL = cfg.OpenAI.BaseURL

	m := &Master{
		// create task monitor
		cacheFile:     cacheFile,
		managedMode:   managedMode,
		jobsScheduler: task.NewJobsScheduler(cfg.Master.NumJobs),
		tracer:        progress.NewTracer("master"),
		openAIClient:  openai.NewClientWithConfig(clientConfig),
		// default ranking model
		rankingModelName: "bpr",
		rankingModelSearcher: ranking.NewModelSearcher(
			cfg.Recommend.Collaborative.ModelSearchEpoch,
			cfg.Recommend.Collaborative.ModelSearchTrials,
			cfg.Recommend.Collaborative.EnableModelSizeSearch,
		),
		// default click model
		clickModelSearcher: click.NewModelSearcher(
			cfg.Recommend.Collaborative.ModelSearchEpoch,
			cfg.Recommend.Collaborative.ModelSearchTrials,
			cfg.Recommend.Collaborative.EnableModelSizeSearch,
		),
		RestServer: server.RestServer{
			Settings: &config.Settings{
				Config:       cfg,
				CacheClient:  cache.NoDatabase{},
				DataClient:   data.NoDatabase{},
				RankingModel: ranking.NewBPR(nil),
				ClickModel:   click.NewFM(click.FMClassification, nil),
				// init versions
				RankingModelVersion: rand.Int63(),
				ClickModelVersion:   rand.Int63(),
			},
			HttpHost:   cfg.Master.HttpHost,
			HttpPort:   cfg.Master.HttpPort,
			WebService: new(restful.WebService),
		},
		fitTicker:    time.NewTicker(cfg.Recommend.Collaborative.ModelFitPeriod),
		importedChan: parallel.NewConditionChannel(),
		loadDataChan: parallel.NewConditionChannel(),
		triggerChan:  parallel.NewConditionChannel(),
	}

	// enable deep learning
	if cfg.Experimental.EnableDeepLearning {
		log.Logger().Debug("enable deep learning")
		m.ClickModel = click.NewDeepFM(model.Params{
			model.BatchSize: cfg.Experimental.DeepLearningBatchSize,
		})
	}

	return m
}

// Serve starts the master node.
func (m *Master) Serve() {

	// load local cached model
	var err error
	m.localCache, err = LoadLocalCache(m.cacheFile)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			log.Logger().Info("no local cache found, create a new one", zap.String("path", m.cacheFile))
		} else {
			log.Logger().Error("failed to load local cache", zap.String("path", m.cacheFile), zap.Error(err))
		}
	}
	if m.localCache.RankingModel != nil {
		log.Logger().Info("load cached ranking model",
			zap.String("model_name", m.localCache.RankingModelName),
			zap.String("model_version", encoding.Hex(m.localCache.RankingModelVersion)),
			zap.Float32("model_score", m.localCache.RankingModelScore.NDCG),
			zap.Any("params", m.localCache.RankingModel.GetParams()))
		m.RankingModel = m.localCache.RankingModel
		m.rankingModelName = m.localCache.RankingModelName
		m.RankingModelVersion = m.localCache.RankingModelVersion
		m.rankingScore = m.localCache.RankingModelScore
		CollaborativeFilteringPrecision10.Set(float64(m.rankingScore.Precision))
		CollaborativeFilteringRecall10.Set(float64(m.rankingScore.Recall))
		CollaborativeFilteringNDCG10.Set(float64(m.rankingScore.NDCG))
		MemoryInUseBytesVec.WithLabelValues("collaborative_filtering_model").Set(float64(sizeof.DeepSize(m.RankingModel)))
	}
	if m.localCache.ClickModel != nil {
		log.Logger().Info("load cached click model",
			zap.String("model_version", encoding.Hex(m.localCache.ClickModelVersion)),
			zap.Float32("model_score", m.localCache.ClickModelScore.Precision),
			zap.Any("params", m.localCache.ClickModel.GetParams()))
		m.ClickModel = m.localCache.ClickModel
		m.clickScore = m.localCache.ClickModelScore
		m.ClickModelVersion = m.localCache.ClickModelVersion
		RankingPrecision.Set(float64(m.clickScore.Precision))
		RankingRecall.Set(float64(m.clickScore.Recall))
		RankingAUC.Set(float64(m.clickScore.AUC))
		MemoryInUseBytesVec.WithLabelValues("ranking_model").Set(float64(sizeof.DeepSize(m.ClickModel)))
	}

	// connect meta database
	m.metaStore, err = meta.Open(fmt.Sprintf("sqlite://%s/gorse_meta.db", os.TempDir()), m.Config.Master.MetaTimeout)
	if err != nil {
		log.Logger().Fatal("failed to connect meta database", zap.Error(err))
	}
	if err = m.metaStore.Init(); err != nil {
		log.Logger().Fatal("failed to init meta database", zap.Error(err))
	}

	// connect data database
	m.DataClient, err = data.Open(m.Config.Database.DataStore, m.Config.Database.DataTablePrefix,
		storage.WithIsolationLevel(m.Config.Database.MySQL.IsolationLevel))
	if err != nil {
		log.Logger().Fatal("failed to connect data database", zap.Error(err),
			zap.String("database", log.RedactDBURL(m.Config.Database.DataStore)))
	}
	if err = m.DataClient.Init(); err != nil {
		log.Logger().Fatal("failed to init database", zap.Error(err))
	}

	// connect cache database
	m.CacheClient, err = cache.Open(m.Config.Database.CacheStore, m.Config.Database.CacheTablePrefix,
		storage.WithIsolationLevel(m.Config.Database.MySQL.IsolationLevel))
	if err != nil {
		log.Logger().Fatal("failed to connect cache database", zap.Error(err),
			zap.String("database", log.RedactDBURL(m.Config.Database.CacheStore)))
	}
	if err = m.CacheClient.Init(); err != nil {
		log.Logger().Fatal("failed to init database", zap.Error(err))
	}

	if m.managedMode {
		go m.RunManagedTasksLoop()
	} else {
		go m.RunPrivilegedTasksLoop()
		log.Logger().Info("start model fit", zap.Duration("period", m.Config.Recommend.Collaborative.ModelFitPeriod))
		go m.RunRagtagTasksLoop()
		log.Logger().Info("start model searcher", zap.Duration("period", m.Config.Recommend.Collaborative.ModelSearchPeriod))
	}

	// start rpc server
	go func() {
		log.Logger().Info("start rpc server",
			zap.String("host", m.Config.Master.Host),
			zap.Int("port", m.Config.Master.Port),
			zap.Bool("ssl_mode", m.Config.Master.SSLMode),
			zap.String("ssl_ca", m.Config.Master.SSLCA),
			zap.String("ssl_cert", m.Config.Master.SSLCert),
			zap.String("ssl_key", m.Config.Master.SSLKey))
		opts := []grpc.ServerOption{grpc.MaxSendMsgSize(math.MaxInt)}
		if m.Config.Master.SSLMode {
			c, err := util.NewServerCreds(&util.TLSConfig{
				SSLCA:   m.Config.Master.SSLCA,
				SSLCert: m.Config.Master.SSLCert,
				SSLKey:  m.Config.Master.SSLKey,
			})
			if err != nil {
				log.Logger().Fatal("failed to load server TLS", zap.Error(err))
			}
			opts = append(opts, grpc.Creds(c))
		}
		lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", m.Config.Master.Host, m.Config.Master.Port))
		if err != nil {
			log.Logger().Fatal("failed to listen", zap.Error(err))
		}
		m.grpcServer = grpc.NewServer(opts...)
		protocol.RegisterMasterServer(m.grpcServer, m)
		protocol.RegisterCacheStoreServer(m.grpcServer, cache.NewProxyServer(m.CacheClient))
		protocol.RegisterDataStoreServer(m.grpcServer, data.NewProxyServer(m.DataClient))
		if err = m.grpcServer.Serve(lis); err != nil {
			log.Logger().Fatal("failed to start rpc server", zap.Error(err))
		}
	}()

	if m.Config.OIDC.Enable {
		provider, err := oidc.NewProvider(context.Background(), m.Config.OIDC.Issuer)
		if err != nil {
			log.Logger().Error("failed to create oidc provider", zap.Error(err))
		} else {
			m.verifier = provider.Verifier(&oidc.Config{ClientID: m.Config.OIDC.ClientID})
			m.oauth2Config = oauth2.Config{
				ClientID:     m.Config.OIDC.ClientID,
				ClientSecret: m.Config.OIDC.ClientSecret,
				RedirectURL:  m.Config.OIDC.RedirectURL,
				Endpoint:     provider.Endpoint(),
				Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
			}
			m.tokenCache = ttlcache.New(ttlcache.WithTTL[string, UserInfo](time.Hour))
			go m.tokenCache.Start()
		}
	}

	// start http server
	m.StartHttpServer()
}

func (m *Master) Shutdown() {
	// stop http server
	err := m.HttpServer.Shutdown(context.TODO())
	if err != nil {
		log.Logger().Error("failed to shutdown http server", zap.Error(err))
	}
	// stop grpc server
	m.grpcServer.GracefulStop()
}

func (m *Master) RunPrivilegedTasksLoop() {
	defer base.CheckPanic()
	var (
		err   error
		tasks = []Task{
			NewFitClickModelTask(m),
			NewFitRankingModelTask(m),
		}
		firstLoop = true
	)
	go func() {
		m.importedChan.Signal()
		for {
			if m.checkDataImported() {
				m.importedChan.Signal()
			}
			time.Sleep(time.Second)
		}
	}()
	for {
		select {
		case <-m.fitTicker.C:
		case <-m.importedChan.C:
		}

		// download dataset
		err = m.runLoadDatasetTask()
		if err != nil {
			log.Logger().Error("failed to load ranking dataset", zap.Error(err))
			continue
		}
		if m.rankingTrainSet.UserCount() == 0 && m.rankingTrainSet.ItemCount() == 0 && m.rankingTrainSet.Count() == 0 {
			log.Logger().Warn("empty ranking dataset",
				zap.Strings("positive_feedback_type", m.Config.Recommend.DataSource.PositiveFeedbackTypes))
			continue
		}

		if firstLoop {
			m.loadDataChan.Signal()
			firstLoop = false
		}

		var registeredTask []Task
		for _, t := range tasks {
			if m.jobsScheduler.Register(t.name(), t.priority(), true) {
				registeredTask = append(registeredTask, t)
			}
		}
		for _, t := range registeredTask {
			go func(task Task) {
				j := m.jobsScheduler.GetJobsAllocator(task.name())
				defer m.jobsScheduler.Unregister(task.name())
				j.Init()
				if err := task.run(context.Background(), j); err != nil {
					log.Logger().Error("failed to run task", zap.String("task", task.name()), zap.Error(err))
					return
				}
			}(t)
		}
	}
}

// RunRagtagTasksLoop searches optimal recommendation model in background. It never modifies variables other than
// rankingModelSearcher, clickSearchedModel and clickSearchedScore.
func (m *Master) RunRagtagTasksLoop() {
	defer base.CheckPanic()
	<-m.loadDataChan.C
	var (
		err   error
		tasks = []Task{
			NewCacheGarbageCollectionTask(m),
			NewSearchRankingModelTask(m),
			NewSearchClickModelTask(m),
		}
	)
	for {
		if m.rankingTrainSet == nil || m.clickTrainSet == nil {
			time.Sleep(time.Second)
			continue
		}
		var registeredTask []Task
		for _, t := range tasks {
			if m.jobsScheduler.Register(t.name(), t.priority(), false) {
				registeredTask = append(registeredTask, t)
			}
		}
		for _, t := range registeredTask {
			go func(task Task) {
				defer m.jobsScheduler.Unregister(task.name())
				j := m.jobsScheduler.GetJobsAllocator(task.name())
				j.Init()
				if err = task.run(context.Background(), j); err != nil {
					log.Logger().Error("failed to run task", zap.String("task", task.name()), zap.Error(err))
				}
			}(t)
		}
		time.Sleep(m.Config.Recommend.Collaborative.ModelSearchPeriod)
	}
}

func (m *Master) RunManagedTasksLoop() {
	var (
		privilegedTasks = []Task{
			NewFitClickModelTask(m),
			NewFitRankingModelTask(m),
		}
		ragtagTasks = []Task{
			NewCacheGarbageCollectionTask(m),
			NewSearchRankingModelTask(m),
			NewSearchClickModelTask(m),
		}
	)

	for range m.triggerChan.C {
		func() {
			defer base.CheckPanic()

			searchModel := m.scheduleState.SearchModel
			m.scheduleState.IsRunning = true
			m.scheduleState.StartTime = time.Now()
			defer func() {
				m.scheduleState.IsRunning = false
				m.scheduleState.SearchModel = false
				m.scheduleState.StartTime = time.Time{}
			}()
			_ = searchModel

			// download dataset
			if err := m.runLoadDatasetTask(); err != nil {
				log.Logger().Error("failed to load ranking dataset", zap.Error(err))
				return
			}
			if m.rankingTrainSet.UserCount() == 0 && m.rankingTrainSet.ItemCount() == 0 && m.rankingTrainSet.Count() == 0 {
				log.Logger().Warn("empty ranking dataset",
					zap.Strings("positive_feedback_type", m.Config.Recommend.DataSource.PositiveFeedbackTypes))
				return
			}

			var registeredTask []Task
			for _, t := range privilegedTasks {
				if m.jobsScheduler.Register(t.name(), t.priority(), true) {
					registeredTask = append(registeredTask, t)
				}
			}
			if searchModel {
				for _, t := range ragtagTasks {
					if m.jobsScheduler.Register(t.name(), t.priority(), false) {
						registeredTask = append(registeredTask, t)
					}
				}
			}

			var wg sync.WaitGroup
			wg.Add(len(registeredTask))
			for _, t := range registeredTask {
				go func(task Task) {
					j := m.jobsScheduler.GetJobsAllocator(task.name())
					defer m.jobsScheduler.Unregister(task.name())
					defer wg.Done()
					j.Init()
					if err := task.run(context.Background(), j); err != nil {
						log.Logger().Error("failed to run task", zap.String("task", task.name()), zap.Error(err))
						return
					}
				}(t)
			}
			wg.Wait()
		}()
	}
}

func (m *Master) checkDataImported() bool {
	ctx := context.Background()
	isDataImported, err := m.CacheClient.Get(ctx, cache.Key(cache.GlobalMeta, cache.DataImported)).Integer()
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			log.Logger().Error("failed to read meta", zap.Error(err))
		}
		return false
	}
	if isDataImported > 0 {
		err = m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.DataImported), 0))
		if err != nil {
			log.Logger().Error("failed to write meta", zap.Error(err))
		}
		return true
	}
	return false
}

func (m *Master) notifyDataImported() {
	ctx := context.Background()
	err := m.CacheClient.Set(ctx, cache.Integer(cache.Key(cache.GlobalMeta, cache.DataImported), 1))
	if err != nil {
		log.Logger().Error("failed to write meta", zap.Error(err))
	}
}
