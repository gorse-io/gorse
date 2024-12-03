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
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/protocol"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"net"
	"time"
)

type ProxyServer struct {
	protocol.UnimplementedCacheStoreServer
	database Database
	server   *grpc.Server
}

func NewProxyServer(database Database) *ProxyServer {
	return &ProxyServer{database: database}
}

func (p *ProxyServer) Serve(lis net.Listener) error {
	p.server = grpc.NewServer()
	protocol.RegisterCacheStoreServer(p.server, p)
	return p.server.Serve(lis)
}

func (p *ProxyServer) Stop() {
	p.server.Stop()
}

func (p *ProxyServer) Get(ctx context.Context, request *protocol.GetRequest) (*protocol.GetResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (p *ProxyServer) Set(ctx context.Context, request *protocol.SetRequest) (*protocol.SetResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (p *ProxyServer) Delete(ctx context.Context, request *protocol.DeleteRequest) (*protocol.DeleteResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (p *ProxyServer) GetSet(ctx context.Context, request *protocol.GetSetRequest) (*protocol.GetSetResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (p *ProxyServer) SetSet(ctx context.Context, request *protocol.SetSetRequest) (*protocol.SetSetResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (p *ProxyServer) AddSet(ctx context.Context, request *protocol.AddSetRequest) (*protocol.AddSetResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (p *ProxyServer) RemSet(ctx context.Context, request *protocol.RemSetRequest) (*protocol.RemSetResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (p *ProxyServer) Push(ctx context.Context, request *protocol.PushRequest) (*protocol.PushResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (p *ProxyServer) Pop(ctx context.Context, request *protocol.PopRequest) (*protocol.PopResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (p *ProxyServer) Remain(ctx context.Context, request *protocol.RemainRequest) (*protocol.RemainResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (p *ProxyServer) AddScores(ctx context.Context, request *protocol.AddScoresRequest) (*protocol.AddScoresResponse, error) {
	scores := make([]Score, len(request.Documents))
	for i, doc := range request.Documents {
		scores[i] = Score{
			Id:         doc.GetId(),
			Score:      doc.GetScore(),
			IsHidden:   doc.GetIsHidden(),
			Categories: doc.GetCategories(),
			Timestamp:  doc.GetTimestamp().AsTime(),
		}
	}
	return &protocol.AddScoresResponse{}, p.database.AddScores(ctx, request.GetCollection(), request.GetSubset(), scores)
}

func (p *ProxyServer) SearchScores(ctx context.Context, request *protocol.SearchScoresRequest) (*protocol.SearchScoresResponse, error) {
	resp, err := p.database.SearchScores(ctx, request.GetCollection(), request.GetSubset(), request.GetQuery(), int(request.GetBegin()), int(request.GetEnd()))
	if err != nil {
		return nil, err
	}
	scores := make([]*protocol.Score, len(resp))
	for i, score := range resp {
		scores[i] = &protocol.Score{
			Id:         score.Id,
			Score:      score.Score,
			IsHidden:   score.IsHidden,
			Categories: score.Categories,
			Timestamp:  timestamppb.New(score.Timestamp),
		}
	}
	return &protocol.SearchScoresResponse{Documents: scores}, nil
}

func (p *ProxyServer) DeleteScores(ctx context.Context, request *protocol.DeleteScoresRequest) (*protocol.DeleteScoresResponse, error) {
	var before *time.Time
	if request.Condition.Before != nil {
		before = lo.ToPtr(request.Condition.Before.AsTime())
	}
	return &protocol.DeleteScoresResponse{}, p.database.DeleteScores(ctx, request.GetCollection(), ScoreCondition{
		Subset: request.Condition.Subset,
		Id:     request.Condition.Id,
		Before: before,
	})
}

func (p *ProxyServer) UpdateScores(ctx context.Context, request *protocol.UpdateScoresRequest) (*protocol.UpdateScoresResponse, error) {
	return &protocol.UpdateScoresResponse{}, p.database.UpdateScores(ctx, request.GetCollection(), request.GetId(), ScorePatch{
		IsHidden:   request.GetPatch().IsHidden,
		Categories: request.GetPatch().Categories,
		Score:      request.GetPatch().Score,
	})
}

func (p *ProxyServer) AddTimeSeriesPoints(ctx context.Context, request *protocol.AddTimeSeriesPointsRequest) (*protocol.AddTimeSeriesPointsResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (p *ProxyServer) GetTimeSeriesPoints(ctx context.Context, request *protocol.GetTimeSeriesPointsRequest) (*protocol.GetTimeSeriesPointsResponse, error) {
	//TODO implement me
	panic("implement me")
}

type ProxyClient struct {
	*grpc.ClientConn
	protocol.CacheStoreClient
}

func (p ProxyClient) Ping() error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyClient) Init() error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyClient) Scan(work func(string) error) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyClient) Purge() error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyClient) Set(ctx context.Context, values ...Value) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyClient) Get(ctx context.Context, name string) *ReturnValue {
	//TODO implement me
	panic("implement me")
}

func (p ProxyClient) Delete(ctx context.Context, name string) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyClient) GetSet(ctx context.Context, key string) ([]string, error) {
	//TODO implement me
	panic("implement me")
}

func (p ProxyClient) SetSet(ctx context.Context, key string, members ...string) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyClient) AddSet(ctx context.Context, key string, members ...string) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyClient) RemSet(ctx context.Context, key string, members ...string) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyClient) Push(ctx context.Context, name, value string) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyClient) Pop(ctx context.Context, name string) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (p ProxyClient) Remain(ctx context.Context, name string) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (p ProxyClient) AddScores(ctx context.Context, collection, subset string, documents []Score) error {
	scores := make([]*protocol.Score, len(documents))
	for i, doc := range documents {
		scores[i] = &protocol.Score{
			Id:         doc.Id,
			Score:      doc.Score,
			IsHidden:   doc.IsHidden,
			Categories: doc.Categories,
			Timestamp:  timestamppb.New(doc.Timestamp),
		}
	}
	_, err := p.CacheStoreClient.AddScores(ctx, &protocol.AddScoresRequest{
		Collection: collection,
		Subset:     subset,
		Documents:  scores,
	})
	return err
}

func (p ProxyClient) SearchScores(ctx context.Context, collection, subset string, query []string, begin, end int) ([]Score, error) {
	resp, err := p.CacheStoreClient.SearchScores(ctx, &protocol.SearchScoresRequest{
		Collection: collection,
		Subset:     subset,
		Query:      query,
		Begin:      int32(begin),
		End:        int32(end),
	})
	if err != nil {
		return nil, err
	}
	scores := make([]Score, len(resp.Documents))
	for i, score := range resp.Documents {
		scores[i] = Score{
			Id:         score.Id,
			Score:      score.Score,
			IsHidden:   score.IsHidden,
			Categories: score.Categories,
			Timestamp:  score.Timestamp.AsTime(),
		}
	}
	return scores, nil
}

func (p ProxyClient) DeleteScores(ctx context.Context, collection []string, condition ScoreCondition) error {
	if err := condition.Check(); err != nil {
		return errors.Trace(err)
	}
	var before *timestamppb.Timestamp
	if condition.Before != nil {
		before = timestamppb.New(*condition.Before)
	}
	_, err := p.CacheStoreClient.DeleteScores(ctx, &protocol.DeleteScoresRequest{
		Collection: collection,
		Condition: &protocol.ScoreCondition{
			Subset: condition.Subset,
			Id:     condition.Id,
			Before: before,
		},
	})
	return err
}

func (p ProxyClient) UpdateScores(ctx context.Context, collection []string, id string, patch ScorePatch) error {
	_, err := p.CacheStoreClient.UpdateScores(ctx, &protocol.UpdateScoresRequest{
		Collection: collection,
		Id:         id,
		Patch: &protocol.ScorePatch{
			Score:      patch.Score,
			IsHidden:   patch.IsHidden,
			Categories: patch.Categories,
		},
	})
	return err
}

func (p ProxyClient) AddTimeSeriesPoints(ctx context.Context, points []TimeSeriesPoint) error {
	//TODO implement me
	panic("implement me")
}

func (p ProxyClient) GetTimeSeriesPoints(ctx context.Context, name string, begin, end time.Time) ([]TimeSeriesPoint, error) {
	//TODO implement me
	panic("implement me")
}

func OpenProxyClient(address string) (*ProxyClient, error) {
	// Create gRPC connection
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	// Create client
	return &ProxyClient{
		ClientConn:       conn,
		CacheStoreClient: protocol.NewCacheStoreClient(conn),
	}, nil
}
