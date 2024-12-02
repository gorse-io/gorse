// Copyright 2024 gorse Project Authors
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

package cache

import (
	"context"
	"github.com/zhenghaoz/gorse/protocol"
	"time"
)

type ProxyServer struct {
	protocol.UnimplementedCacheStoreServer
}

func (p ProxyServer) Close() error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) Ping() error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) Init() error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) Scan(work func(string) error) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) Purge() error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) Set(ctx context.Context, values ...Value) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) Get(ctx context.Context, name string) *ReturnValue {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) Delete(ctx context.Context, name string) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) GetSet(ctx context.Context, key string) ([]string, error) {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) SetSet(ctx context.Context, key string, members ...string) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) AddSet(ctx context.Context, key string, members ...string) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) RemSet(ctx context.Context, key string, members ...string) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) Push(ctx context.Context, name, value string) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) Pop(ctx context.Context, name string) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) Remain(ctx context.Context, name string) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) AddScores(ctx context.Context, collection, subset string, documents []Score) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) SearchScores(ctx context.Context, collection, subset string, query []string, begin, end int) ([]Score, error) {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) DeleteScores(ctx context.Context, collection []string, condition ScoreCondition) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) UpdateScores(ctx context.Context, collection []string, id string, patch ScorePatch) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) AddTimeSeriesPoints(ctx context.Context, points []TimeSeriesPoint) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyServer) GetTimeSeriesPoints(ctx context.Context, name string, begin, end time.Time) ([]TimeSeriesPoint, error) {
	//TODO implement me
	panic("implement me")
}

type ProxyClient struct {
}

var _ Database = (*ProxyServer)(nil)
