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
	"github.com/dgraph-io/badger/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/zhenghaoz/gorse/storage/local"
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

type Server struct {
	RestServer
	// database connections
	cacheAddress string
	dataAddress  string
	localStore   *local.Database

	// master connection
	masterClient protocol.MasterClient

	// factorization machine
	//fmModel         ctr.FactorizationMachine
	//RankModelMutex  sync.RWMutex
	//fmVersion       int64
	//latestFMVersion int64

	// config
	serverName string
	masterHost string
	masterPort int

	// events
	//syncedChan chan bool
}

func NewServer(masterHost string, masterPort int, serverHost string, serverPort int) *Server {
	return &Server{
		masterHost: masterHost,
		masterPort: masterPort,
		RestServer: RestServer{
			DataStore:   &data.NoDatabase{},
			CacheStore:  &cache.NoDatabase{},
			GorseConfig: (*config.Config)(nil).LoadDefaultIfNil(),
			HttpHost:    serverHost,
			HttpPort:    serverPort,
			EnableAuth:  true,
			WebService:  new(restful.WebService),
		},
	}
}

func (s *Server) Serve() {
	// open local store
	var err error
	s.localStore, err = local.Open(filepath.Join(os.TempDir(), "gorse-server"))
	if err != nil {
		base.Logger().Fatal("failed to connect local store", zap.Error(err),
			zap.String("path", filepath.Join(os.TempDir(), "gorse-server")))
	}
	if s.serverName, err = s.localStore.GetString(local.NodeName); err != nil {
		if err == badger.ErrKeyNotFound {
			s.serverName = base.GetRandomName(0)
			err = s.localStore.SetString(local.NodeName, s.serverName)
			if err != nil {
				base.Logger().Fatal("failed to write meta", zap.Error(err))
			}
		} else {
			base.Logger().Fatal("failed to read meta", zap.Error(err))
		}
	}

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
	//go s.Pull()
	s.StartHttpServer()
}

// Pull factorization machine.
//func (s *RestServer) Pull() {
//	defer base.CheckPanic()
//	for range s.syncedChan {
//		ctx := context.Background()
//		// pull factorization machine
//		if s.latestFMVersion != s.fmVersion {
//			base.Logger().Info("pull factorization machine")
//			if mfResponse, err := s.masterClient.GetFactorizationMachine(ctx, &protocol.RequestInfo{}, grpc.MaxCallRecvMsgSize(10e8)); err != nil {
//				base.Logger().Error("failed to pull factorization machine", zap.Error(err))
//			} else {
//				s.fmModel, err = ctr.DecodeModel(mfResponse.Model)
//				if err != nil {
//					base.Logger().Error("failed to decode factorization machine", zap.Error(err))
//				} else {
//					s.fmVersion = mfResponse.Version
//					base.Logger().Info("synced factorization machine", zap.Int64("version", s.fmVersion))
//				}
//			}
//		}
//	}
//}

// Sync this server to the master.
func (s *Server) Sync() {
	defer base.CheckPanic()
	base.Logger().Info("start meta sync", zap.Int("meta_timeout", s.GorseConfig.Master.MetaTimeout))
	for {
		var meta *protocol.Meta
		var err error
		if meta, err = s.masterClient.GetMeta(context.Background(),
			&protocol.NodeInfo{
				NodeType:    protocol.NodeType_ServerNode,
				NodeName:    s.serverName,
				HttpAddress: fmt.Sprintf("%s:%d", s.HttpHost, s.HttpPort),
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
		if s.dataAddress != s.GorseConfig.Database.DataStore {
			base.Logger().Info("connect data store", zap.String("database", s.GorseConfig.Database.DataStore))
			if s.DataStore, err = data.Open(s.GorseConfig.Database.DataStore); err != nil {
				base.Logger().Error("failed to connect data store", zap.Error(err))
				goto sleep
			}
			s.dataAddress = s.GorseConfig.Database.DataStore
		}

		// connect to cache store
		if s.cacheAddress != s.GorseConfig.Database.CacheStore {
			base.Logger().Info("connect cache store", zap.String("database", s.GorseConfig.Database.CacheStore))
			if s.CacheStore, err = cache.Open(s.GorseConfig.Database.CacheStore); err != nil {
				base.Logger().Error("failed to connect cache store", zap.Error(err))
				goto sleep
			}
			s.cacheAddress = s.GorseConfig.Database.CacheStore
		}

		// check FM version
		//s.latestFMVersion = meta.FmVersion
		//if s.latestFMVersion != s.fmVersion {
		//	base.Logger().Info("new factorization machine model found",
		//		zap.Int64("old_version", s.fmVersion),
		//		zap.Int64("new_version", s.latestFMVersion))
		//	s.syncedChan <- true
		//}
	sleep:
		time.Sleep(time.Duration(s.GorseConfig.Master.MetaTimeout) * time.Second)
	}
}
