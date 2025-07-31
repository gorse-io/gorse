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
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/google/uuid"
	"github.com/gorse-io/gorse/base"
	"github.com/gorse-io/gorse/base/log"
	"github.com/gorse-io/gorse/cmd/version"
	"github.com/gorse-io/gorse/common/util"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/protocol"
	"github.com/gorse-io/gorse/storage"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Server manages states of a server node.
type Server struct {
	RestServer
	traceConfig  config.TracingConfig
	cachePath    string
	cachePrefix  string
	dataPath     string
	dataPrefix   string
	conn         *grpc.ClientConn
	masterClient protocol.MasterClient
	serverName   string
	masterHost   string
	masterPort   int
	tlsConfig    *util.TLSConfig
	testMode     bool
	cacheFile    string
}

// NewServer creates a server node.
func NewServer(
	masterHost string,
	masterPort int,
	serverHost string,
	serverPort int,
	cacheFile string,
	tlsConfig *util.TLSConfig,
) *Server {
	s := &Server{
		masterHost: masterHost,
		masterPort: masterPort,
		tlsConfig:  tlsConfig,
		cacheFile:  cacheFile,
		RestServer: RestServer{
			Settings:   config.NewSettings(),
			HttpHost:   serverHost,
			HttpPort:   serverPort,
			WebService: new(restful.WebService),
		},
	}
	return s
}

// Serve starts a server node.
func (s *Server) Serve() {
	rand.Seed(time.Now().UTC().UnixNano())
	// open local store
	state, err := LoadLocalCache(s.cacheFile)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			log.Logger().Info("no cache file found, create a new one", zap.String("path", s.cacheFile))
		} else {
			log.Logger().Error("failed to connect local store", zap.Error(err),
				zap.String("path", s.cacheFile))
		}
	}
	if state.ServerName == "" {
		state.ServerName = uuid.New().String()
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
	var opts []grpc.DialOption
	if s.tlsConfig != nil {
		c, err := util.NewClientCreds(s.tlsConfig)
		if err != nil {
			log.Logger().Fatal("failed to create credentials", zap.Error(err))
		}
		opts = append(opts, grpc.WithTransportCredentials(c))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	s.conn, err = grpc.Dial(fmt.Sprintf("%v:%v", s.masterHost, s.masterPort), opts...)
	if err != nil {
		log.Logger().Fatal("failed to connect master", zap.Error(err))
	}
	s.masterClient = protocol.NewMasterClient(s.conn)

	go s.Sync()
	container := restful.NewContainer()
	s.StartHttpServer(container)
}

func (s *Server) Shutdown() {
	err := s.HttpServer.Shutdown(context.TODO())
	if err != nil {
		log.Logger().Fatal("failed to shutdown http server", zap.Error(err))
	}
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
				NodeType:      protocol.NodeType_Server,
				Uuid:          s.serverName,
				BinaryVersion: version.Version,
				Hostname:      lo.Must(os.Hostname()),
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
		if s.dataPath != s.Config.Database.DataStore || s.dataPrefix != s.Config.Database.DataTablePrefix {
			if strings.HasPrefix(s.Config.Database.DataStore, storage.SQLitePrefix) {
				log.Logger().Info("connect cache store via master")
				s.DataClient = data.NewProxyClient(s.conn)
			} else {
				log.Logger().Info("connect data store",
					zap.String("database", log.RedactDBURL(s.Config.Database.DataStore)))
				if s.DataClient, err = data.Open(s.Config.Database.DataStore, s.Config.Database.DataTablePrefix); err != nil {
					log.Logger().Error("failed to connect data store", zap.Error(err))
					goto sleep
				}
			}
			s.dataPath = s.Config.Database.DataStore
			s.dataPrefix = s.Config.Database.DataTablePrefix
		}

		// connect to cache store
		if s.cachePath != s.Config.Database.CacheStore || s.cachePrefix != s.Config.Database.CacheTablePrefix {
			if strings.HasPrefix(s.Config.Database.CacheStore, storage.SQLitePrefix) {
				log.Logger().Info("connect cache store via master")
				s.CacheClient = cache.NewProxyClient(s.conn)
			} else {
				log.Logger().Info("connect cache store",
					zap.String("database", log.RedactDBURL(s.Config.Database.CacheStore)))
				if s.CacheClient, err = cache.Open(s.Config.Database.CacheStore, s.Config.Database.CacheTablePrefix); err != nil {
					log.Logger().Error("failed to connect cache store", zap.Error(err))
					goto sleep
				}
			}
			s.cachePath = s.Config.Database.CacheStore
			s.cachePrefix = s.Config.Database.CacheTablePrefix
		}

		// create trace provider
		if !s.traceConfig.Equal(s.Config.Tracing) {
			log.Logger().Info("create trace provider", zap.Any("tracing_config", s.Config.Tracing))
			tp, err := s.Config.Tracing.NewTracerProvider()
			if err != nil {
				log.Logger().Fatal("failed to create trace provider", zap.Error(err))
			}
			otel.SetTracerProvider(tp)
			otel.SetErrorHandler(log.GetErrorHandler())
			otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
			s.traceConfig = s.Config.Tracing
		}

	sleep:
		if s.testMode {
			return
		}
		time.Sleep(s.Config.Master.MetaTimeout)
	}
}
