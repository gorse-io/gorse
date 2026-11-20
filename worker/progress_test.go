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

package worker

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gorse-io/gorse/common/expression"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/protocol"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type progressDataStore struct {
	data.NoDatabase
	feedback func()
}

func (s progressDataStore) GetUserFeedback(context.Context, string, *time.Time, ...expression.FeedbackTypeExpression) ([]data.Feedback, error) {
	if s.feedback != nil {
		s.feedback()
	}
	return nil, nil
}

type progressCacheStore struct{ cache.NoDatabase }

func (progressCacheStore) SearchScores(context.Context, string, string, []string, int, int) ([]cache.Score, error) {
	return nil, nil
}

func newProgressPipeline(t *testing.T) *Pipeline {
	log.SetTestLogger(t)
	// Cold-start users exercise job completion without external stores.
	return &Pipeline{
		Config:      config.GetDefaultConfig(),
		DataClient:  progressDataStore{},
		CacheClient: progressCacheStore{},
		Tracer:      monitor.NewTracer(t.Name()),
		Jobs:        1,
	}
}

func TestPipelineFinalProgress(t *testing.T) {
	for _, count := range []int{0, 1, 1001} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				p := newProgressPipeline(t)
				users := make([]data.User, count)
				calls := 0
				p.Recommend(t.Context(), users, func(completed, throughput int) {
					calls++
					assert.Equal(t, count, completed)
					assert.Equal(t, count, throughput)
					progress := p.Tracer.List()
					if assert.Len(t, progress, 1) {
						assert.Equal(t, monitor.StatusComplete, progress[0].Status)
						assert.Equal(t, count, progress[0].Count)
						assert.False(t, progress[0].FinishTime.IsZero())
					}
				})
				assert.Equal(t, 1, calls, "final progress must be reported before Recommend returns")
			})
		})
	}
}

func TestPipelineProgressAfterTick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := newProgressPipeline(t)
		p.DataClient = progressDataStore{feedback: func() { time.Sleep(6 * time.Second) }}
		var completedCounts, throughputs []int
		var statuses []monitor.Status
		p.Recommend(t.Context(), make([]data.User, 2), func(completed, throughput int) {
			completedCounts = append(completedCounts, completed)
			throughputs = append(throughputs, throughput)
			statuses = append(statuses, p.Tracer.List()[0].Status)
		})
		assert.Equal(t, []int{1, 2}, completedCounts)
		assert.Equal(t, []int{1, 1}, throughputs)
		assert.Equal(t, []monitor.Status{monitor.StatusRunning, monitor.StatusComplete}, statuses)
	})
}

func TestPipelineProgressCancellation(t *testing.T) {
	for _, jobs := range []int{1, 1001} {
		t.Run(fmt.Sprint(jobs), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				p := newProgressPipeline(t)
				p.Jobs = jobs
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				var started atomic.Int32
				p.DataClient = progressDataStore{feedback: func() {
					if started.Add(1) == int32(jobs) {
						cancel()
					}
					// Finish after cancellation, with more senders than the buffer can hold.
					time.Sleep(time.Second)
				}}
				calls := 0
				p.Recommend(ctx, make([]data.User, jobs+1), func(completed, throughput int) {
					calls++
					assert.Equal(t, jobs, completed, "only jobs that actually ran should be counted")
					assert.Equal(t, jobs, throughput)
				})
				assert.Equal(t, 1, calls)
			})
		})
	}
}

func TestPipelineProgressAlreadyCanceled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := newProgressPipeline(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		calls := 0
		p.Recommend(ctx, []data.User{{UserId: "user"}}, func(completed, throughput int) {
			calls++
			assert.Zero(t, completed)
			assert.Zero(t, throughput)
		})
		assert.Equal(t, 1, calls)
	})
}

func TestPipelineProgressNilCallback(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := newProgressPipeline(t)
		p.Recommend(t.Context(), []data.User{{UserId: "user"}}, nil)
		progress := p.Tracer.List()
		require.Len(t, progress, 1)
		assert.Equal(t, monitor.StatusComplete, progress[0].Status)
		assert.Equal(t, 1, progress[0].Count)
	})
}

type progressMaster struct {
	protocol.UnimplementedMasterServer
	reports chan []monitor.Progress
}

func (m *progressMaster) PushProgress(_ context.Context, request *protocol.PushProgressRequest) (*protocol.PushProgressResponse, error) {
	m.reports <- monitor.DecodeProgress(request)
	return &protocol.PushProgressResponse{}, nil
}

func TestPipelineFinalProgressRPC(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	t.Cleanup(server.Stop)
	m := &progressMaster{reports: make(chan []monitor.Progress, 10)}
	protocol.RegisterMasterServer(server, m)
	go func() { _ = server.Serve(listener) }()
	connection, err := grpc.NewClient("passthrough:///progress",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, connection.Close()) })
	client := protocol.NewMasterClient(connection)
	p := newProgressPipeline(t)
	p.Recommend(t.Context(), []data.User{{UserId: "user"}}, func(int, int) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		_, err := client.PushProgress(ctx, monitor.EncodeProgress(p.Tracer.List()))
		assert.NoError(t, err)
	})
	select {
	case report := <-m.reports:
		require.Len(t, report, 1)
		assert.Equal(t, monitor.StatusComplete, report[0].Status)
		assert.Equal(t, 1, report[0].Count)
	default:
		t.Fatal("master received no final progress report")
	}
}
