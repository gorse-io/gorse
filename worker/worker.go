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
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorse-io/gorse/cmd/version"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/common/util"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/logics"
	"github.com/gorse-io/gorse/model/ctr"
	"github.com/gorse-io/gorse/protocol"
	"github.com/gorse-io/gorse/storage"
	"github.com/gorse-io/gorse/storage/blob"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/juju/errors"
	"github.com/lafikl/consistent"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const batchSize = 10000

// Worker manages states of a worker node.
type Worker struct {
	Pipeline
	testMode bool

	collaborativeFilteringModelId int64
	clickThroughRateModelId       int64

	// worker config
	workerName string
	httpHost   string
	httpPort   int
	masterHost string
	masterPort int
	tlsConfig  *util.TLSConfig
	cacheFile  string

	// database connection path
	cachePath   string
	cachePrefix string
	dataPath    string
	dataPrefix  string

	blobConfig string
	blobStore  blob.Store

	// master connection
	conn         *grpc.ClientConn
	masterClient protocol.MasterClient

	latestCollaborativeFilteringModelId int64
	latestClickThroughRateModelId       int64
	randGenerator                       *rand.Rand

	// peers
	peers []string
	me    string

	// events
	tickDuration time.Duration
	ticker       *time.Ticker
	syncedChan   chan struct{} // meta synced events
	pulledChan   chan struct{} // model pulled events
}

// NewWorker creates a new worker node.
func NewWorker(
	masterHost string,
	masterPort int,
	httpHost string,
	httpPort int,
	jobs int,
	cacheFile string,
	tlsConfig *util.TLSConfig,
	interval time.Duration,
) *Worker {
	return &Worker{
		Pipeline: Pipeline{
			Config:      config.GetDefaultConfig(),
			CacheClient: new(cache.NoDatabase),
			DataClient:  new(data.NoDatabase),
			Jobs:        jobs,
		},
		randGenerator: util.NewRand(time.Now().UTC().UnixNano()),
		// config
		cacheFile:  cacheFile,
		masterHost: masterHost,
		masterPort: masterPort,
		tlsConfig:  tlsConfig,
		httpHost:   httpHost,
		httpPort:   httpPort,
		// events
		tickDuration: interval,
		ticker:       time.NewTicker(interval),
		syncedChan:   make(chan struct{}, 1),
		pulledChan:   make(chan struct{}, 1),
	}
}

// Sync this worker to the master.
func (w *Worker) Sync() {
	var nextBlobConfig string
	log.Logger().Info("start meta sync", zap.Duration("meta_timeout", w.Config.Master.MetaTimeout))
	for {
		var meta *protocol.Meta
		var err error
		if meta, err = w.masterClient.GetMeta(context.Background(),
			&protocol.NodeInfo{
				NodeType:      protocol.NodeType_Worker,
				Uuid:          w.workerName,
				BinaryVersion: version.Version,
				Hostname:      lo.Must(os.Hostname()),
			}); err != nil {
			log.Logger().Error("failed to get meta", zap.Error(err))
			goto sleep
		}

		// load master config
		err = json.Unmarshal([]byte(meta.Config), &w.Config)
		if err != nil {
			log.Logger().Error("failed to parse master config", zap.Error(err))
			goto sleep
		}

		// connect to data store
		if w.dataPath != w.Config.Database.DataStore || w.dataPrefix != w.Config.Database.DataTablePrefix {
			if strings.HasPrefix(w.Config.Database.DataStore, storage.SQLitePrefix) {
				log.Logger().Info("connect data store via master")
				w.DataClient = data.NewProxyClient(w.conn)
			} else {
				log.Logger().Info("connect data store",
					zap.String("database", log.RedactDBURL(w.Config.Database.DataStore)))
				if w.DataClient, err = data.Open(w.Config.Database.DataStore, w.Config.Database.DataTablePrefix,
					storage.WithIsolationLevel(w.Config.Database.MySQL.IsolationLevel),
					storage.WithMaxOpenConns(w.Config.Database.MySQL.MaxOpenConns),
					storage.WithMaxIdleConns(w.Config.Database.MySQL.MaxIdleConns),
					storage.WithConnMaxLifetime(w.Config.Database.MySQL.ConnMaxLifetime)); err != nil {
					log.Logger().Error("failed to connect data store", zap.Error(err))
					goto sleep
				}
			}
			w.dataPath = w.Config.Database.DataStore
			w.dataPrefix = w.Config.Database.DataTablePrefix
		}

		// connect to cache store
		if w.cachePath != w.Config.Database.CacheStore || w.cachePrefix != w.Config.Database.CacheTablePrefix {
			if strings.HasPrefix(w.Config.Database.CacheStore, storage.SQLitePrefix) {
				log.Logger().Info("connect cache store via master")
				w.CacheClient = cache.NewProxyClient(w.conn)
			} else {
				log.Logger().Info("connect cache store",
					zap.String("database", log.RedactDBURL(w.Config.Database.CacheStore)))
				if w.CacheClient, err = cache.Open(w.Config.Database.CacheStore, w.Config.Database.CacheTablePrefix,
					storage.WithIsolationLevel(w.Config.Database.MySQL.IsolationLevel),
					storage.WithMaxOpenConns(w.Config.Database.MySQL.MaxOpenConns),
					storage.WithMaxIdleConns(w.Config.Database.MySQL.MaxIdleConns),
					storage.WithConnMaxLifetime(w.Config.Database.MySQL.ConnMaxLifetime)); err != nil {
					log.Logger().Error("failed to connect cache store", zap.Error(err))
					goto sleep
				}
			}
			w.cachePath = w.Config.Database.CacheStore
			w.cachePrefix = w.Config.Database.CacheTablePrefix
		}

		// connect to blob store
		if w.Config.S3.Endpoint != "" {
			nextBlobConfig = w.Config.S3.ToJSON()
		} else if w.Config.GCS.Bucket != "" {
			nextBlobConfig = w.Config.GCS.ToJSON()
		} else {
			nextBlobConfig = "{}"
		}
		if w.blobConfig != nextBlobConfig {
			if w.Config.S3.Endpoint != "" {
				log.Logger().Info("connect s3 endpoint", zap.String("endpoint", w.Config.S3.Endpoint))
				if w.blobStore, err = blob.NewS3(w.Config.S3); err != nil {
					log.Logger().Error("failed to connect s3 endpoint", zap.Error(err))
					goto sleep
				}
			} else if w.Config.GCS.Bucket != "" {
				log.Logger().Info("connect gcs bucket", zap.String("bucket", w.Config.GCS.Bucket))
				if w.blobStore, err = blob.NewGCS(w.Config.GCS); err != nil {
					log.Logger().Error("failed to connect gcs bucket", zap.Error(err))
					goto sleep
				}
			} else {
				log.Logger().Info("connect blob store via master")
				w.blobStore = blob.NewMasterStoreClient(w.conn)
			}
			w.blobConfig = nextBlobConfig
		}

		// synchronize collaborative filtering model
		w.latestCollaborativeFilteringModelId = meta.CollaborativeFilteringModelId
		if w.latestCollaborativeFilteringModelId > w.collaborativeFilteringModelId {
			log.Logger().Info("new ranking model found",
				zap.Int64("old_version", w.collaborativeFilteringModelId),
				zap.Int64("new_version", w.latestCollaborativeFilteringModelId))

			select {
			case w.syncedChan <- struct{}{}:
			default:
			}
		}

		// synchronize click-through rate model
		w.latestClickThroughRateModelId = meta.ClickThroughRateModelId
		if w.latestClickThroughRateModelId > w.clickThroughRateModelId {
			log.Logger().Info("new click model found",
				zap.Int64("old_version", w.clickThroughRateModelId),
				zap.Int64("new_version", w.latestClickThroughRateModelId))

			select {
			case w.syncedChan <- struct{}{}:
			default:
			}
		}

		w.peers = meta.Workers
		w.me = meta.Me
	sleep:
		if w.testMode {
			return
		}
		time.Sleep(w.Config.Master.MetaTimeout)
	}
}

// Pull user index and ranking model from master.
func (w *Worker) Pull() {
	for range w.syncedChan {
		pulled := false

		// pull ranking model
		if w.latestCollaborativeFilteringModelId > w.collaborativeFilteringModelId {
			log.Logger().Info("start pull collaborative filtering model")
			r, err := w.blobStore.Open(strconv.FormatInt(w.latestCollaborativeFilteringModelId, 10))
			if err != nil {
				log.Logger().Error("failed to open collaborative filtering model", zap.Error(err))
			} else {
				items := logics.NewMatrixFactorizationItems(time.Time{})
				users := logics.NewMatrixFactorizationUsers()
				if err = items.Unmarshal(r); err != nil {
					log.Logger().Error("failed to unmarshal matrix factorization items", zap.Error(err))
				} else if err = users.Unmarshal(r); err != nil {
					log.Logger().Error("failed to unmarshal matrix factorization users", zap.Error(err))
				} else {
					w.MatrixFactorizationItems = items
					w.MatrixFactorizationUsers = users
					w.collaborativeFilteringModelId = w.latestCollaborativeFilteringModelId
					log.Logger().Info("synced collaborative filtering model",
						zap.Int64("id", w.collaborativeFilteringModelId))
					pulled = true
				}
			}
		}

		// pull click model
		if w.latestClickThroughRateModelId > w.clickThroughRateModelId {
			log.Logger().Info("start pull click model")
			r, err := w.blobStore.Open(strconv.FormatInt(w.latestClickThroughRateModelId, 10))
			if err != nil {
				log.Logger().Error("failed to open click-through rate model", zap.Error(err))
			} else {
				model, err := ctr.UnmarshalModel(r)
				if err != nil {
					log.Logger().Error("failed to unmarshal click-through rate model", zap.Error(err))
				} else {
					w.ClickThroughRateModel = model
					w.clickThroughRateModelId = w.latestClickThroughRateModelId
					log.Logger().Info("synced click-through rate model",
						zap.Int64("version", w.clickThroughRateModelId))
					pulled = true
				}
			}
		}

		if w.testMode {
			return
		}
		if pulled {
			select {
			case w.pulledChan <- struct{}{}:
			default:
			}
		}
	}
}

// ServeHTTP serves Prometheus metrics and API.
func (w *Worker) ServeHTTP() {
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/api/health/live", w.checkLive)
	http.HandleFunc("/api/health/ready", w.checkReady)
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", w.httpHost, w.httpPort), nil)
	if err != nil {
		log.Logger().Fatal("failed to start http server", zap.Error(err))
	}
}

func writeJSON(w http.ResponseWriter, content any) {
	w.WriteHeader(http.StatusOK)
	bytes, err := json.Marshal(content)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
	}
	if _, err = w.Write(bytes); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, error string, code int) {
	log.Logger().Error(strings.ToLower(http.StatusText(code)), zap.String("error", error))
	http.Error(w, error, code)
}

// Serve as a worker node.
func (w *Worker) Serve() {
	var err error
	if w.workerName, err = w.WorkerName(); err != nil {
		log.Logger().Fatal("failed to get worker name", zap.Error(err))
	}

	// create progress tracer
	w.Tracer = monitor.NewTracer(w.workerName)

	// connect to master
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(512*1024*1024)))
	if w.tlsConfig != nil {
		c, err := util.NewClientCreds(w.tlsConfig)
		if err != nil {
			log.Logger().Fatal("failed to create credentials", zap.Error(err))
		}
		opts = append(opts, grpc.WithTransportCredentials(c))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	w.conn, err = grpc.Dial(net.JoinHostPort(w.masterHost, strconv.Itoa(w.masterPort)), opts...)
	if err != nil {
		log.Logger().Fatal("failed to connect master", zap.Error(err))
	}
	w.masterClient = protocol.NewMasterClient(w.conn)

	go w.Sync()
	go w.Pull()
	go w.ServeHTTP()

	loop := func() {
		// pull users
		workingUsers, err := w.pullUsers(w.peers, w.me)
		if err != nil {
			log.Logger().Error("failed to split users", zap.Error(err),
				zap.String("me", w.me),
				zap.Strings("workers", w.peers))
			return
		}

		// recommendation
		w.Recommend(context.Background(), workingUsers, func(completed, throughput int) {
			log.Logger().Info("ranking recommendation",
				zap.Int("n_complete_users", completed),
				zap.Int("throughput", throughput))
			if w.masterClient != nil {
				if _, err := w.masterClient.PushProgress(context.Background(), monitor.EncodeProgress(w.Tracer.List())); err != nil {
					log.Logger().Error("failed to report update task", zap.Error(err))
				}
			}
		})
	}

	for {
		select {
		case tick := <-w.ticker.C:
			if time.Since(tick) <= w.tickDuration {
				loop()
			}
		case <-w.pulledChan:
			loop()
		}
	}
}

func (w *Worker) WorkerName() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	hash := md5.New()
	hash.Write([]byte(hostname))
	hash.Write([]byte(w.httpHost))
	hash.Write([]byte(strconv.Itoa(w.httpPort)))
	b := hash.Sum(nil)
	return hex.EncodeToString(b), nil
}

func (w *Worker) pullUsers(peers []string, me string) ([]data.User, error) {
	ctx := context.Background()
	// locate me
	if !lo.Contains(peers, me) {
		return nil, errors.New("current node isn't in worker nodes")
	}
	// create consistent hash ring
	c := consistent.New()
	for _, peer := range peers {
		c.Add(peer)
	}
	// pull users from database
	var users []data.User
	userChan, errChan := w.DataClient.GetUserStream(ctx, batchSize)
	for batchUsers := range userChan {
		for _, user := range batchUsers {
			p, err := c.Get(user.UserId)
			if err != nil {
				return nil, errors.Trace(err)
			}
			if p == me {
				users = append(users, user)
			}
		}
	}
	if err := <-errChan; err != nil {
		return nil, errors.Trace(err)
	}
	return users, nil
}

type HealthStatus struct {
	Ready               bool
	DataStoreError      error
	CacheStoreError     error
	DataStoreConnected  bool
	CacheStoreConnected bool
}

func (w *Worker) checkHealth() HealthStatus {
	healthStatus := HealthStatus{}
	healthStatus.DataStoreError = w.DataClient.Ping()
	healthStatus.CacheStoreError = w.CacheClient.Ping()
	healthStatus.DataStoreConnected = healthStatus.DataStoreError == nil
	healthStatus.CacheStoreConnected = healthStatus.CacheStoreError == nil
	healthStatus.Ready = healthStatus.DataStoreConnected && healthStatus.CacheStoreConnected
	return healthStatus
}

func (w *Worker) checkLive(writer http.ResponseWriter, _ *http.Request) {
	healthStatus := w.checkHealth()
	writeJSON(writer, healthStatus)
}

func (w *Worker) checkReady(writer http.ResponseWriter, _ *http.Request) {
	healthStatus := w.checkHealth()
	if healthStatus.Ready {
		writeJSON(writer, healthStatus)
	} else {
		errReason, err := json.Marshal(healthStatus)
		if err != nil {
			writeError(writer, err.Error(), http.StatusInternalServerError)
		} else {
			writeError(writer, string(errReason), http.StatusServiceUnavailable)
		}
	}
}
