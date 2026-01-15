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
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/emicklei/go-restful/v3"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/common/util"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/model/cf"
	"github.com/gorse-io/gorse/model/ctr"
	"github.com/gorse-io/gorse/protocol"
	"github.com/gorse-io/gorse/server"
	"github.com/gorse-io/gorse/storage"
	"github.com/gorse-io/gorse/storage/blob"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/meta"
	"github.com/jellydator/ttlcache/v3"
	"github.com/juju/errors"
	"github.com/sashabaranov/go-openai"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
)

type Datasets struct {
	rankingDataset  *dataset.Dataset
	rankingTrainSet dataset.CFSplit
	rankingTestSet  dataset.CFSplit
	clickDataset    *ctr.Dataset
	clickTrainSet   *ctr.Dataset
	clickTestSet    *ctr.Dataset
}

// Master is the master node.
type Master struct {
	protocol.UnimplementedMasterServer
	server.RestServer
	grpcServer *grpc.Server

	tracer         *monitor.Monitor
	remoteProgress sync.Map
	cachePath      string
	standalone     bool
	openAIClient   *openai.Client

	// cluster meta cache
	metaStore  meta.Database
	blobStore  blob.Store
	blobServer *blob.MasterStoreServer

	// collaborative filtering
	collaborativeFilteringModelMutex   sync.RWMutex
	collaborativeFilteringTrainSetSize int
	collaborativeFilteringMeta         meta.Model[cf.Score]
	collaborativeFilteringTarget       meta.Model[cf.Score]

	// click model
	clickThroughRateModelMutex   sync.RWMutex
	clickThroughRateTrainSetSize int
	clickThroughRateMeta         meta.Model[ctr.Score]
	clickThroughRateTarget       meta.Model[ctr.Score]

	// oauth2
	oauth2Config oauth2.Config
	verifier     *oidc.IDTokenVerifier
	tokenCache   *ttlcache.Cache[string, UserInfo]

	// events
	ticker    *time.Ticker
	scheduled chan struct{}
	cancel    context.CancelFunc
}

// NewMaster creates a master node.
func NewMaster(cfg *config.Config, cacheFolder string, standalone bool) *Master {
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
	// setup OpenAI logger
	log.InitOpenAILogger(cfg.OpenAI.LogFile)
	// setup OpenAI rate limiter
	parallel.InitChatCompletionLimiters(cfg.OpenAI.ChatCompletionRPM, cfg.OpenAI.ChatCompletionTPM)
	parallel.InitEmbeddingLimiters(cfg.OpenAI.EmbeddingRPM, cfg.OpenAI.EmbeddingTPM)

	duration := min(cfg.Recommend.Collaborative.FitPeriod, cfg.Recommend.Ranker.FitPeriod)
	m := &Master{
		// create task monitor
		cachePath:    cacheFolder,
		standalone:   standalone,
		tracer:       monitor.NewTracer("master"),
		openAIClient: openai.NewClientWithConfig(clientConfig),
		RestServer: server.RestServer{
			Config:      cfg,
			CacheClient: cache.NoDatabase{},
			DataClient:  data.NoDatabase{},
			HttpHost:    cfg.Master.HttpHost,
			HttpPort:    cfg.Master.HttpPort,
			WebService:  new(restful.WebService),
		},
		ticker:    time.NewTicker(duration),
		scheduled: make(chan struct{}, 1),
		cancel:    func() {},
	}
	return m
}

// Serve starts the master node.
func (m *Master) Serve() {
	// connect blob store
	var err error
	m.blobServer = blob.NewMasterStoreServer(m.cachePath)
	if m.Config.S3.Endpoint != "" {
		m.blobStore, err = blob.NewS3(m.Config.S3)
		if err != nil {
			log.Logger().Fatal("failed to create S3 blob store", zap.Error(err))
		}
	} else if m.Config.GCS.Bucket != "" {
		m.blobStore, err = blob.NewGCS(m.Config.GCS)
		if err != nil {
			log.Logger().Fatal("failed to create GCS blob store", zap.Error(err))
		}
	} else {
		m.blobStore = blob.NewPOSIX(m.cachePath)
	}

	// connect meta database
	m.metaStore, err = meta.Open(fmt.Sprintf("sqlite://%s/meta.sqlite3", m.cachePath), m.Config.Master.MetaTimeout)
	if err != nil {
		log.Logger().Fatal("failed to connect meta database", zap.Error(err))
	}
	if err = m.metaStore.Init(); err != nil {
		log.Logger().Fatal("failed to init meta database", zap.Error(err))
	}

	// connect data database
	m.DataClient, err = data.Open(m.Config.Database.DataStore, m.Config.Database.DataTablePrefix,
		storage.WithIsolationLevel(m.Config.Database.MySQL.IsolationLevel),
		storage.WithMaxOpenConns(m.Config.Database.MySQL.MaxOpenConns),
		storage.WithMaxIdleConns(m.Config.Database.MySQL.MaxIdleConns),
		storage.WithConnMaxLifetime(m.Config.Database.MySQL.ConnMaxLifetime))
	if err != nil {
		log.Logger().Fatal("failed to connect data database", zap.Error(err),
			zap.String("database", log.RedactDBURL(m.Config.Database.DataStore)))
	}
	if err = m.DataClient.Init(); err != nil {
		log.Logger().Fatal("failed to init database", zap.Error(err))
	}

	// connect cache database
	m.CacheClient, err = cache.Open(m.Config.Database.CacheStore, m.Config.Database.CacheTablePrefix,
		storage.WithIsolationLevel(m.Config.Database.MySQL.IsolationLevel),
		storage.WithMaxOpenConns(m.Config.Database.MySQL.MaxOpenConns),
		storage.WithMaxIdleConns(m.Config.Database.MySQL.MaxIdleConns),
		storage.WithConnMaxLifetime(m.Config.Database.MySQL.ConnMaxLifetime))
	if err != nil {
		log.Logger().Fatal("failed to connect cache database", zap.Error(err),
			zap.String("database", log.RedactDBURL(m.Config.Database.CacheStore)))
	}
	if err = m.CacheClient.Init(); err != nil {
		log.Logger().Fatal("failed to init database", zap.Error(err))
	}

	// load recommend config
	metaStr, err := m.metaStore.Get(meta.RECOMMEND_CONFIG)
	if err != nil && !errors.Is(err, errors.NotFound) {
		log.Logger().Error("failed to load recommend config", zap.Error(err))
	} else if metaStr != nil {
		err = json.Unmarshal([]byte(*metaStr), &m.Config.Recommend)
		if err != nil {
			log.Logger().Error("failed to unmarshal recommend config", zap.Error(err))
		}
	}

	// load collective filtering model meta
	metaStr, err = m.metaStore.Get(meta.COLLABORATIVE_FILTERING_MODEL)
	if err != nil && !errors.Is(err, errors.NotFound) {
		log.Logger().Error("failed to load collaborative filtering meta", zap.Error(err))
	} else if metaStr != nil {
		if err = m.collaborativeFilteringMeta.FromJSON(*metaStr); err != nil {
			log.Logger().Error("failed to unmarshal collaborative filtering meta", zap.Error(err))
		} else {
			log.Logger().Info("loaded collaborative filtering model",
				zap.String("type", m.collaborativeFilteringMeta.Type),
				zap.Any("params", m.collaborativeFilteringMeta.Params),
				zap.Any("score", m.collaborativeFilteringMeta.Score))
		}
	}

	// load click-through rate model
	metaStr, err = m.metaStore.Get(meta.CLICK_THROUGH_RATE_MODEL)
	if err != nil && !errors.Is(err, errors.NotFound) {
		log.Logger().Error("failed to load click-through rate meta", zap.Error(err))
	} else if metaStr != nil {
		if err = m.clickThroughRateMeta.FromJSON(*metaStr); err != nil {
			log.Logger().Error("failed to unmarshal click-through rate meta", zap.Error(err))
		} else {
			log.Logger().Info("loaded click-through rate model",
				zap.String("type", m.clickThroughRateMeta.Type),
				zap.Any("params", m.clickThroughRateMeta.Params),
				zap.Any("score", m.clickThroughRateMeta.Score))
		}
	}

	go m.RunTasksLoop()

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
		protocol.RegisterBlobStoreServer(m.grpcServer, m.blobServer)
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

func (m *Master) RunTasksLoop() {
	defer util.CheckPanic()
	select {
	case m.scheduled <- struct{}{}:
	default:
	}
	for {
		select {
		case <-m.ticker.C:
		case <-m.scheduled:
		}

		// download dataset
		var ctx context.Context
		ctx, m.cancel = context.WithCancel(context.Background())
		err := m.runLoadDatasetTask(ctx)
		if err != nil {
			log.Logger().Error("failed to load ranking dataset", zap.Error(err))
			continue
		}
	}
}
