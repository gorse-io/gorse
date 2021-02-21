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
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/memberlist"
	log "github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/protocol"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"google.golang.org/grpc"
	"time"
)

type Worker struct {
	cfg        *config.Config
	members    *memberlist.Memberlist
	cacheStore cache.Database
	dataStore  data.Database

	// master
	MasterHost   string
	MasterPort   int
	MasterClient protocol.MasterClient
}

func NewWorker(masterHost string, masterPort int) *Worker {
	return &Worker{
		MasterPort: masterPort,
		MasterHost: masterHost,
	}
}

func (w *Worker) Register() {
	for {
		if _, err := w.MasterClient.RegisterWorker(context.Background(), &protocol.Void{}); err != nil {
			log.Fatal("worker:", err)
		}
		time.Sleep(time.Duration(w.cfg.Common.ClusterMetaTimeout/2) * time.Second)
	}
}

func (w *Worker) Serve() {

	// connect to master
	conn, err := grpc.Dial(fmt.Sprintf("%v:%v", w.MasterHost, w.MasterPort), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("worker: failed to connect master (%v)", err)
	}
	w.MasterClient = protocol.NewMasterClient(conn)

	// load master config
	masterCfgJson, err := w.MasterClient.GetConfig(context.Background(), &protocol.Void{})
	if err != nil {
		log.Fatalf("worker: failed to load master config (%v)", err)
	}
	err = json.Unmarshal([]byte(masterCfgJson.Json), &w.cfg)
	if err != nil {
		log.Fatalf("worker: failed to parse master config (%v)", err)
	}

	// register to master
	w.Register()

	//// start gossip protocol
	//cfg := memberlist.DefaultLocalConfig()
	//cfg.Name = protocol.NewName(protocol.WorkerNodePrefix, w.cfg.Worker.RPCPort, cfg.Name)
	//cfg.BindPort = w.cfg.Worker.GossipPort
	//var err error
	//w.members, err = memberlist.Create(cfg)
	//if err != nil {
	//	log.Fatal("Worker: failed to start gossip, " + err.Error())
	//}
	//
	//go w.GossipServe()
	//go w.RPCServe()
	//
	//for {
	//
	//	// check runnable
	//	w.modelMutex.Lock()
	//	if w.model == nil {
	//		w.modelMutex.Unlock()
	//		log.Infof("Worker: no model found, next round starts after %v minutes", w.cfg.Worker.PredictInterval)
	//		time.Sleep(time.Minute * time.Duration(w.cfg.Worker.PredictInterval))
	//		continue
	//	}
	//	w.modelMutex.Unlock()
	//
	//	w.userMutex.Lock()
	//	if w.userPrefixes == nil {
	//		w.userMutex.Unlock()
	//		log.Infof("Worker: no user partition found, next round starts after %v minutes", w.cfg.Worker.PredictInterval)
	//		time.Sleep(time.Minute * time.Duration(w.cfg.Worker.PredictInterval))
	//		continue
	//	}
	//	w.userMutex.Unlock()
	//
	//	// download users
	//	w.userMutex.Lock()
	//	prefixes := w.userPrefixes
	//	w.userMutex.Unlock()
	//	users, err := w.GetUsers(prefixes)
	//	if err != nil {
	//		log.Error("Worker:", err)
	//		time.Sleep(time.Minute * time.Duration(w.cfg.Common.RetryInterval))
	//		continue
	//	}
	//
	//	// Generate recommends
	//	log.Infof("Worker: start to generate recommendations (%v)", len(users))
	//	w.modelMutex.Lock()
	//	m := w.model
	//	w.modelMutex.Unlock()
	//	if err := w.SetRecommends(m, users); err != nil {
	//		log.Error("Worker:", err)
	//		time.Sleep(time.Minute * time.Duration(w.cfg.Common.RetryInterval))
	//		continue
	//	}
	//
	//	log.Infof("Worker: complete predict, next round starts after %v minutes", w.cfg.Worker.PredictInterval)
	//	time.Sleep(time.Minute * time.Duration(w.cfg.Worker.PredictInterval))
	//}
}

//
//func (w *Worker) GossipServe() {
//	for {
//		_, err := w.members.Join([]string{w.cfg.Worker.LeaderAddr})
//		if err != nil {
//			log.Errorf("Worker: failed to join cluster %v", w.cfg.Worker.LeaderAddr)
//		}
//		time.Sleep(time.Minute * time.Duration(w.cfg.Worker.GossipInterval))
//	}
//}
//
//func (w *Worker) RPCServe() {
//	log.Infof("Worker: start RPC server %v:%v", w.cfg.Worker.Host, w.cfg.Worker.RPCPort)
//	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", w.cfg.Worker.Host, w.cfg.Worker.RPCPort))
//	if err != nil {
//		log.Fatalf("Worker: failed to listen: %v", err)
//	}
//	var opts []grpc.ServerOption
//	grpcServer := grpc.NewServer(opts...)
//	protocol.RegisterWorkerServer(grpcServer, w)
//	if err = grpcServer.Serve(lis); err != nil {
//		log.Fatal(err)
//	}
//}
//
//func (w *Worker) BroadcastModel(ctx context.Context, data *protocol.Model) (*protocol.Void, error) {
//	log.Infof("Worker: receive new model (%v,%v)", data.Version.Tag, data.Version.Term)
//	// decode model
//	m, err := model.DecodeModel(data.Name, data.Model)
//	if err != nil {
//		log.Error("Worker: ", err)
//		return nil, err
//	}
//	// replace model
//	w.modelMutex.Lock()
//	defer w.modelMutex.Unlock()
//	w.model = m
//	w.modelVersion = data.Version
//	return &protocol.Void{}, nil
//}
//
//func (w *Worker) BroadcastUserPartition(ctx context.Context, data *protocol.Partition) (*protocol.Void, error) {
//	log.Infof("Worker: receive new user partition (%v,%v,%v)", data.Version.Tag, data.Version.Term, data.Prefixes)
//	// update partition
//	w.userMutex.Lock()
//	defer w.userMutex.Unlock()
//	w.userPrefixes = data.Prefixes
//	w.userVersion = data.Version
//	return &protocol.Void{}, nil
//}
//
//func (w *Worker) GetModelVersion(context.Context, *protocol.Void) (*protocol.Version, error) {
//	log.Infof("Worker: request model version from leader")
//	w.modelMutex.Lock()
//	defer w.modelMutex.Unlock()
//	return w.modelVersion, nil
//}
//func (w *Worker) GetUserPartitionVersion(context.Context, *protocol.Void) (*protocol.Version, error) {
//	log.Infof("Worker: request user partition version from leader")
//	w.userMutex.Lock()
//	defer w.userMutex.Unlock()
//	return w.userVersion, nil
//}
//
//func (w *Worker) GetUsers(prefixes []string) ([]string, error) {
//	log.Infof("Worker: pull users within partition %v", prefixes)
//	users := make([]string, 0)
//	for _, prefix := range prefixes {
//		cursor := ""
//		var batch []storage.User
//		var err error
//		for {
//			cursor, batch, err = w.db.GetUsers(prefix, cursor, batchSize)
//			if err != nil {
//				return nil, err
//			}
//			for _, user := range batch {
//				users = append(users, user.UserId)
//			}
//			if cursor == "" {
//				break
//			}
//		}
//	}
//	return users, nil
//}
//
//// SetRecommends updates personalized recommendations for the database.
//func (w *Worker) SetRecommends(m model.Model, users []string) error {
//	collector, err := NewCollector(w.db, w.cfg.Common.CacheSize)
//	if err != nil {
//		return err
//	}
//	for i, user := range users {
//		// Collect candidates
//		itemSet, err := collector.Collect(user, []string{"all"})
//		if err != nil {
//			return err
//		}
//		recItems := base.NewTopKStringFilter(w.cfg.Common.CacheSize)
//		for item := range itemSet {
//			var score float32
//			switch m.(type) {
//			case model.MatrixFactorization:
//				score = m.(model.MatrixFactorization).Predict(user, item)
//			default:
//				return fmt.Errorf("not implement yet")
//			}
//			recItems.Push(item, score)
//		}
//		elems, scores := recItems.PopAll()
//		recommends := make([]storage.RecommendedItem, len(elems))
//		for i := range elems {
//			recommends[i].ItemId = elems[i]
//			recommends[i].Score = float64(scores[i])
//		}
//		if err := w.db.SetRecommend(user, recommends); err != nil {
//			return err
//		}
//		// verbose
//		if (i+1)%1000 == 0 {
//			log.Infof("Worker: generate recommendations (%v/%v)", i+1, len(users))
//		}
//	}
//	return nil
//}
