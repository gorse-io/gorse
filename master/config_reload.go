// Copyright 2026 gorse Project Authors
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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/parallel"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/model"
	"github.com/gorse-io/gorse/storage/blob"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/meta"
	"github.com/gorse-io/gorse/storage/vectors"
	"github.com/juju/errors"
	"github.com/sashabaranov/go-openai"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const configReloadDebounce = time.Second

func (m *Master) loadConfigWithOverrides() (*config.Config, error) {
	newConfig, err := config.LoadConfig(m.configPath)
	if err != nil {
		return nil, err
	}
	if err = m.applyRecommendOverride(newConfig); err != nil {
		return nil, err
	}
	if err = newConfig.Validate(); err != nil {
		return nil, err
	}
	return newConfig, nil
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

func (m *Master) reloadConfigFromFile(reason string) {
	newConfig, err := m.loadConfigWithOverrides()
	if err != nil {
		log.Logger().Error("failed to reload config", zap.String("reason", reason), zap.Error(err))
		return
	}
	if err = m.applyConfig(newConfig); err != nil {
		log.Logger().Error("failed to apply reloaded config", zap.String("reason", reason), zap.Error(err))
		return
	}
	log.Logger().Info("reloaded config", zap.String("reason", reason), zap.String("path", m.configPath))
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
	resetTimer := func() {
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
				resetTimer()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Logger().Error("failed to watch config file", zap.Error(err))
		case <-timerC:
			timerC = nil
			m.reloadConfigFromFile("file changed")
		}
	}
}

func (m *Master) applyConfig(newConfig *config.Config) error {
	m.configMutex.RLock()
	oldConfig := *m.Config
	m.configMutex.RUnlock()

	var nextDataClient data.Database
	if !reflect.DeepEqual(oldConfig.Database, newConfig.Database) &&
		(oldConfig.Database.DataStore != newConfig.Database.DataStore ||
			oldConfig.Database.DataTablePrefix != newConfig.Database.DataTablePrefix ||
			!reflect.DeepEqual(oldConfig.Database.MySQL, newConfig.Database.MySQL) ||
			!reflect.DeepEqual(oldConfig.Database.Postgres, newConfig.Database.Postgres)) {
		dataOpts := newConfig.Database.StorageOptions(newConfig.Database.DataStore)
		var err error
		nextDataClient, err = data.Open(newConfig.Database.DataStore, newConfig.Database.DataTablePrefix, dataOpts...)
		if err != nil {
			return errors.Trace(err)
		}
		if err = nextDataClient.Init(); err != nil {
			_ = nextDataClient.Close()
			return errors.Trace(err)
		}
	}

	var nextCacheClient cache.Database
	if !reflect.DeepEqual(oldConfig.Database, newConfig.Database) &&
		(oldConfig.Database.CacheStore != newConfig.Database.CacheStore ||
			oldConfig.Database.CacheTablePrefix != newConfig.Database.CacheTablePrefix ||
			oldConfig.Database.CacheClientName != newConfig.Database.CacheClientName ||
			!reflect.DeepEqual(oldConfig.Database.Redis, newConfig.Database.Redis) ||
			!reflect.DeepEqual(oldConfig.Database.MySQL, newConfig.Database.MySQL) ||
			!reflect.DeepEqual(oldConfig.Database.Postgres, newConfig.Database.Postgres)) {
		cacheOpts := newConfig.Database.StorageOptions(newConfig.Database.CacheStore)
		var err error
		nextCacheClient, err = cache.Open(newConfig.Database.CacheStore, newConfig.Database.CacheTablePrefix, cacheOpts...)
		if err != nil {
			if nextDataClient != nil {
				_ = nextDataClient.Close()
			}
			return errors.Trace(err)
		}
		if err = nextCacheClient.Init(); err != nil {
			if nextDataClient != nil {
				_ = nextDataClient.Close()
			}
			_ = nextCacheClient.Close()
			return errors.Trace(err)
		}
	}

	var nextVectorClient vectors.Database
	if oldConfig.Database.VectorStore != newConfig.Database.VectorStore ||
		oldConfig.Database.VectorTablePrefix != newConfig.Database.VectorTablePrefix {
		var err error
		if newConfig.Database.VectorStore == "" {
			nextVectorClient = vectors.NoDatabase{}
		} else {
			nextVectorClient, err = vectors.Open(newConfig.Database.VectorStore, newConfig.Database.VectorTablePrefix)
			if err != nil {
				closeReloadClients(nextDataClient, nextCacheClient, nil)
				return errors.Trace(err)
			}
			if err = nextVectorClient.Init(); err != nil {
				closeReloadClients(nextDataClient, nextCacheClient, nextVectorClient)
				return errors.Trace(err)
			}
			if err = initCollaborativeFilteringVectorCollection(nextVectorClient, newConfig); err != nil {
				closeReloadClients(nextDataClient, nextCacheClient, nextVectorClient)
				return errors.Trace(err)
			}
		}
	} else if !reflect.DeepEqual(oldConfig.Database.Vector, newConfig.Database.Vector) {
		log.Logger().Warn("vector collection config changes require restart",
			zap.String("quantization_type", newConfig.Database.Vector.QuantizationType),
			zap.Int("quantization_bits", newConfig.Database.Vector.QuantizationBits))
	}

	var nextBlobStore blob.Store
	if !reflect.DeepEqual(oldConfig.Blob, newConfig.Blob) {
		if !strings.Contains(oldConfig.Blob.URI, "://") || !strings.Contains(newConfig.Blob.URI, "://") {
			log.Logger().Warn("local blob store changes require restart", zap.String("uri", newConfig.Blob.URI))
		} else {
			var err error
			nextBlobStore, err = blob.NewStore(newConfig.Blob, nil)
			if err != nil {
				closeReloadClients(nextDataClient, nextCacheClient, nextVectorClient)
				return errors.Trace(err)
			}
		}
	}

	var nextOpenAIClient *openai.Client
	if !reflect.DeepEqual(oldConfig.OpenAI, newConfig.OpenAI) {
		clientConfig := openai.DefaultConfig(newConfig.OpenAI.AuthToken)
		clientConfig.BaseURL = newConfig.OpenAI.BaseURL
		nextOpenAIClient = openai.NewClientWithConfig(clientConfig)
	}
	var nextTracerProvider trace.TracerProvider
	if !oldConfig.Tracing.Equal(newConfig.Tracing) {
		var err error
		nextTracerProvider, err = newConfig.Tracing.NewTracerProvider()
		if err != nil {
			closeReloadClients(nextDataClient, nextCacheClient, nextVectorClient)
			return errors.Trace(err)
		}
	}

	if oldConfig.Master.Host != newConfig.Master.Host ||
		oldConfig.Master.Port != newConfig.Master.Port ||
		oldConfig.Master.SSLMode != newConfig.Master.SSLMode ||
		oldConfig.Master.SSLCA != newConfig.Master.SSLCA ||
		oldConfig.Master.SSLCert != newConfig.Master.SSLCert ||
		oldConfig.Master.SSLKey != newConfig.Master.SSLKey ||
		oldConfig.Master.HttpHost != newConfig.Master.HttpHost ||
		oldConfig.Master.HttpPort != newConfig.Master.HttpPort ||
		!reflect.DeepEqual(oldConfig.Master.HttpCorsDomains, newConfig.Master.HttpCorsDomains) ||
		!reflect.DeepEqual(oldConfig.Master.HttpCorsMethods, newConfig.Master.HttpCorsMethods) {
		log.Logger().Warn("master listener changes require restart")
	}
	if !reflect.DeepEqual(oldConfig.OIDC, newConfig.OIDC) {
		log.Logger().Warn("OIDC changes require restart")
	}

	m.configMutex.Lock()
	if nextDataClient != nil {
		oldDataClient := m.DataClient
		m.DataClient = nextDataClient
		nextDataClient = oldDataClient
	}
	if nextCacheClient != nil {
		oldCacheClient := m.CacheClient
		m.CacheClient = nextCacheClient
		nextCacheClient = oldCacheClient
	}
	if nextVectorClient != nil {
		oldVectorClient := m.VectorClient
		m.VectorClient = nextVectorClient
		nextVectorClient = oldVectorClient
	}
	if nextBlobStore != nil {
		m.blobStore = nextBlobStore
	}
	if nextOpenAIClient != nil {
		m.openAIClient = nextOpenAIClient
		log.InitOpenAILogger(newConfig.OpenAI.LogFile)
		parallel.InitChatCompletionLimiters(newConfig.OpenAI.ChatCompletionRPM, newConfig.OpenAI.ChatCompletionTPM)
		parallel.InitEmbeddingLimiters(newConfig.OpenAI.EmbeddingRPM, newConfig.OpenAI.EmbeddingTPM)
	}
	if nextTracerProvider != nil {
		otel.SetTracerProvider(nextTracerProvider)
		otel.SetErrorHandler(log.GetErrorHandler())
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	}
	m.Config = newConfig
	m.RestServer.Config = newConfig
	m.configMutex.Unlock()

	closeReloadClients(nextDataClient, nextCacheClient, nextVectorClient)

	if !reflect.DeepEqual(oldConfig.Recommend, newConfig.Recommend) ||
		oldConfig.Master.NumJobs != newConfig.Master.NumJobs {
		m.cancel()
		select {
		case m.scheduled <- struct{}{}:
		default:
		}
	}
	return nil
}

func closeReloadClients(dataClient data.Database, cacheClient cache.Database, vectorClient vectors.Database) {
	if dataClient != nil {
		_ = dataClient.Close()
	}
	if cacheClient != nil {
		_ = cacheClient.Close()
	}
	if vectorClient != nil {
		_ = vectorClient.Close()
	}
}

func initCollaborativeFilteringVectorCollection(vectorClient vectors.Database, cfg *config.Config) error {
	vectorConfig := vectors.VectorConfig{
		Type: vectors.QuantizationType(cfg.Database.Vector.QuantizationType),
		Bits: cfg.Database.Vector.QuantizationBits,
	}
	dimension := model.Params(nil).GetInt(model.NFactors, 16)

	collections, err := vectorClient.ListCollections(context.Background())
	if err != nil {
		return errors.Trace(err)
	}
	if !contains(collections, vectors.CollaborativeFiltering) {
		if err = vectorClient.AddCollection(context.Background(), vectors.CollaborativeFiltering, dimension, vectors.Dot, vectorConfig); err != nil {
			return errors.Trace(err)
		}
	}
	info, err := vectorClient.DescribeCollection(context.Background(), vectors.CollaborativeFiltering)
	if err != nil {
		return errors.Trace(err)
	}
	if info.Dimension != 0 && info.Dimension != dimension {
		return errors.Errorf("collection %s dimension mismatch: expected %d, got %d", info.Name, dimension, info.Dimension)
	}
	if info.Distance != vectors.Dot {
		return errors.Errorf("collection %s distance mismatch: expected %v, got %v", info.Name, vectors.Dot, info.Distance)
	}
	if info.Type != vectorConfig.Type {
		return errors.Errorf("collection %s quantization type mismatch: expected %s, got %s", info.Name, vectorConfig.Type, info.Type)
	}
	if vectorConfig.Bits > 0 && info.Bits != vectorConfig.Bits {
		return errors.Errorf("collection %s quantization bits mismatch: expected %d, got %d", info.Name, vectorConfig.Bits, info.Bits)
	}
	return nil
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
