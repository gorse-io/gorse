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
	"github.com/gorse-io/gorse/storage/blob"
	"github.com/gorse-io/gorse/storage/meta"
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
	m.ConfigMutex.RLock()
	oldConfig := *m.Config
	m.ConfigMutex.RUnlock()

	if !reflect.DeepEqual(oldConfig.Database, newConfig.Database) {
		log.Logger().Warn("database changes require restart")
		newConfig.Database = oldConfig.Database
	}

	var nextBlobStore blob.Store
	if !reflect.DeepEqual(oldConfig.Blob, newConfig.Blob) {
		if !strings.Contains(oldConfig.Blob.URI, "://") || !strings.Contains(newConfig.Blob.URI, "://") {
			log.Logger().Warn("local blob store changes require restart", zap.String("uri", newConfig.Blob.URI))
		} else {
			var err error
			nextBlobStore, err = blob.NewStore(newConfig.Blob, nil)
			if err != nil {
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

	m.ConfigMutex.Lock()
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
	m.ConfigMutex.Unlock()

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
