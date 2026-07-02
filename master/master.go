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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/emicklei/go-restful/v3"
	"github.com/fsnotify/fsnotify"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/common/util"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/model"
	"github.com/gorse-io/gorse/model/cf"
	"github.com/gorse-io/gorse/model/ctr"
	"github.com/gorse-io/gorse/protocol"
	"github.com/gorse-io/gorse/server"
	"github.com/gorse-io/gorse/storage/blob"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/meta"
	"github.com/gorse-io/gorse/storage/vectors"
	"github.com/jellydator/ttlcache/v3"
	"github.com/juju/errors"
	"github.com/sashabaranov/go-openai"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
)

const configReloadDebounce = time.Second

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
	configPath     string
	standalone     bool
	openAIClient   *openai.Client
	ConfigMutex    sync.RWMutex

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
	ticker      *time.Ticker
	scheduled   chan struct{}
	cancel      context.CancelFunc
	reconciling atomic.Bool
}

// NewMaster creates a master node.
func NewMaster(cfg *config.Config, cacheFolder string, standalone bool, configPath string) *Master {
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
		configPath:   configPath,
		standalone:   standalone,
		tracer:       monitor.NewTracer("master"),
		openAIClient: openai.NewClientWithConfig(clientConfig),
		RestServer: server.RestServer{
			Config:       cfg,
			CacheClient:  cache.NoDatabase{},
			DataClient:   data.NoDatabase{},
			VectorClient: vectors.NoDatabase{},
			HttpHost:     cfg.Master.HttpHost,
			HttpPort:     cfg.Master.HttpPort,
			WebService:   new(restful.WebService),
		},
		ticker:    time.NewTicker(duration),
		scheduled: make(chan struct{}, 1),
		cancel:    func() {},
	}
	return m
}

func (m *Master) applyRecommendOverride(cfg *config.Config) error {
	metaStr, err := m.metaStore.Get(meta.RECOMMEND_CONFIG)
	if err != nil && !errors.Is(err, errors.NotFound) {
		return errors.Trace(err)
	}
	if metaStr == nil {
		return nil
	}
	if err = json.Unmarshal([]byte(*metaStr), &cfg.Recommend); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (m *Master) reloadConfigFromFile() error {
	newConfig, err := config.LoadConfig(m.configPath)
	if err != nil {
		return errors.Trace(err)
	}
	if err = m.applyRecommendOverride(newConfig); err != nil {
		return errors.Trace(err)
	}
	if err = newConfig.Validate(); err != nil {
		return errors.Trace(err)
	}

	m.ConfigMutex.Lock()
	m.Config = newConfig
	m.RestServer.Config = newConfig
	m.ConfigMutex.Unlock()

	select {
	case m.scheduled <- struct{}{}:
	default:
	}
	return nil
}

func (m *Master) watchConfigFile(ctx context.Context) {
	if m.configPath == "" {
		return
	}
	absPath, err := filepath.Abs(m.configPath)
	if err != nil {
		log.Logger().Error("failed to resolve config path", zap.String("path", m.configPath), zap.Error(err))
		return
	}
	if _, err = os.Stat(absPath); err != nil {
		log.Logger().Warn("skip watching config file", zap.String("path", absPath), zap.Error(err))
		return
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Logger().Error("failed to create config watcher", zap.Error(err))
		return
	}
	defer watcher.Close()
	if err = watcher.Add(filepath.Dir(absPath)); err != nil {
		log.Logger().Error("failed to watch config directory", zap.String("path", absPath), zap.Error(err))
		return
	}
	log.Logger().Info("watch config file", zap.String("path", absPath))

	var timer *time.Timer
	var timerC <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			eventPath, err := filepath.Abs(event.Name)
			if err != nil || eventPath != absPath {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Chmod) != 0 {
				if timer == nil {
					timer = time.NewTimer(configReloadDebounce)
				} else {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(configReloadDebounce)
				}
				timerC = timer.C
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Logger().Error("failed to watch config file", zap.Error(err))
		case <-timerC:
			timerC = nil
			if err = m.reloadConfigFromFile(); err != nil {
				log.Logger().Error("failed to reload config file", zap.String("path", absPath), zap.Error(err))
			} else {
				log.Logger().Info("reloaded config file", zap.String("path", absPath))
			}
		}
	}
}

// Serve starts the master node.
func (m *Master) Serve() {
	// connect blob store
	var err error
	if !strings.Contains(m.Config.Blob.URI, "://") {
		m.blobServer = blob.NewMasterStoreServer(m.Config.Blob.URI)
	}
	m.blobStore, err = blob.NewStore(m.Config.Blob, nil)
	if err != nil {
		log.Logger().Fatal("failed to create blob store", zap.Error(err))
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
	dataOpts := m.Config.Database.StorageOptions(m.Config.Database.DataStore)
	m.DataClient, err = data.Open(m.Config.Database.DataStore, m.Config.Database.DataTablePrefix, dataOpts...)
	if err != nil {
		log.Logger().Fatal("failed to connect data database", zap.Error(err),
			zap.String("database", log.RedactDBURL(m.Config.Database.DataStore)))
	}
	if err = m.DataClient.Init(); err != nil {
		log.Logger().Fatal("failed to init database", zap.Error(err))
	}

	// connect cache database
	cacheOpts := m.Config.Database.StorageOptions(m.Config.Database.CacheStore)
	m.CacheClient, err = cache.Open(m.Config.Database.CacheStore, m.Config.Database.CacheTablePrefix, cacheOpts...)
	if err != nil {
		log.Logger().Fatal("failed to connect cache database", zap.Error(err),
			zap.String("database", log.RedactDBURL(m.Config.Database.CacheStore)))
	}
	if err = m.CacheClient.Init(); err != nil {
		log.Logger().Fatal("failed to init database", zap.Error(err))
	}

	// open vector store
	log.Logger().Info("opening vector store", zap.String("path", m.Config.Database.VectorStore))
	m.VectorClient, err = vectors.Open(m.Config.Database.VectorStore, m.Config.Database.VectorTablePrefix)
	if err != nil {
		log.Logger().Fatal("failed to connect vector store", zap.Error(err))
	}
	if err = m.VectorClient.Init(); err != nil {
		log.Logger().Fatal("failed to init vector store", zap.Error(err))
	}
	if err = m.initCollaborativeFilteringVectorCollection(context.Background()); err != nil {
		log.Logger().Fatal("failed to init collaborative filtering vector collection", zap.Error(err))
	}

	// load config overrides
	if err = m.applyRecommendOverride(m.Config); err != nil {
		log.Logger().Error("failed to apply config overrides", zap.Error(err))
	}

	// load collective filtering model meta
	metaStr, err := m.metaStore.Get(meta.COLLABORATIVE_FILTERING_MODEL)
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

	go m.watchConfigFile(context.Background())
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
		protocol.RegisterVectorStoreServer(m.grpcServer, vectors.NewProxyServer(m.VectorClient))
		if m.blobServer != nil {
			protocol.RegisterBlobStoreServer(m.grpcServer, m.blobServer)
		}
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

func (m *Master) initCollaborativeFilteringVectorCollection(ctx context.Context) error {
	vectorConfig := vectors.VectorConfig{
		Type: vectors.QuantizationType(m.Config.Database.Vector.QuantizationType),
		Bits: m.Config.Database.Vector.QuantizationBits,
	}
	dimension := model.Params(nil).GetInt(model.NFactors, 16)

	info, err := m.VectorClient.DescribeCollection(ctx, vectors.CollaborativeFiltering)
	if errors.Is(err, errors.NotFound) {
		info = nil
	} else if err != nil {
		return errors.Trace(err)
	} else if err = m.checkCollaborativeFilteringVectorCollection(info, dimension, vectorConfig); err != nil {
		log.Logger().Warn("recreating collaborative filtering vector collection",
			zap.String("collection", vectors.CollaborativeFiltering),
			zap.Error(err))
		if err = m.VectorClient.DeleteCollection(ctx, vectors.CollaborativeFiltering); err != nil && !errors.Is(err, errors.NotFound) {
			return errors.Trace(err)
		}
		info = nil
	}

	if info == nil {
		if err = m.VectorClient.AddCollection(ctx, vectors.CollaborativeFiltering, dimension, vectors.Dot, vectorConfig); err != nil {
			return errors.Trace(err)
		}
		info, err = m.VectorClient.DescribeCollection(ctx, vectors.CollaborativeFiltering)
		if err != nil {
			return errors.Trace(err)
		}
		if err = m.checkCollaborativeFilteringVectorCollection(info, dimension, vectorConfig); err != nil {
			return errors.Trace(err)
		}
	}
	log.Logger().Info("initialized collaborative filtering vector collection",
		zap.String("collection", vectors.CollaborativeFiltering),
		zap.Int("dimension", info.Dimension),
		zap.Int("distance", int(info.Distance)),
		zap.String("quantization_type", string(info.Type)),
		zap.Int("quantization_bits", info.Bits))
	return nil
}

func (m *Master) checkCollaborativeFilteringVectorCollection(info *vectors.CollectionInfo, dimension int, config vectors.VectorConfig) error {
	if info.Dimension != 0 && info.Dimension != dimension {
		return errors.Errorf("collection %s dimension mismatch: expected %d, got %d", info.Name, dimension, info.Dimension)
	}
	if info.Type != config.Type {
		return errors.Errorf("collection %s quantization type mismatch: expected %s, got %s", info.Name, config.Type, info.Type)
	}
	if config.Bits > 0 && info.Bits != config.Bits {
		return errors.Errorf("collection %s quantization bits mismatch: expected %d, got %d", info.Name, config.Bits, info.Bits)
	}
	return nil
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
