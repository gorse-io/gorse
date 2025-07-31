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
	"io"
	"net"
	"time"

	"github.com/gorse-io/gorse/protocol"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func (p *ProxyServer) Ping(context.Context, *protocol.PingRequest) (*protocol.PingResponse, error) {
	return &protocol.PingResponse{}, p.database.Ping()
}

func (p *ProxyServer) Get(ctx context.Context, request *protocol.GetRequest) (*protocol.GetResponse, error) {
	value := p.database.Get(ctx, request.GetName())
	if errors.Is(value.err, errors.NotFound) {
		return &protocol.GetResponse{}, nil
	}
	return &protocol.GetResponse{Value: proto.String(value.value)}, value.err
}

func (p *ProxyServer) Set(ctx context.Context, request *protocol.SetRequest) (*protocol.SetResponse, error) {
	values := make([]Value, len(request.Values))
	for i, value := range request.Values {
		values[i] = Value{
			name:  value.GetName(),
			value: value.GetValue(),
		}
	}
	return &protocol.SetResponse{}, p.database.Set(ctx, values...)
}

func (p *ProxyServer) Delete(ctx context.Context, request *protocol.DeleteRequest) (*protocol.DeleteResponse, error) {
	return &protocol.DeleteResponse{}, p.database.Delete(ctx, request.GetName())
}

func (p *ProxyServer) GetSet(ctx context.Context, request *protocol.GetSetRequest) (*protocol.GetSetResponse, error) {
	members, err := p.database.GetSet(ctx, request.GetKey())
	if err != nil {
		return nil, err
	}
	return &protocol.GetSetResponse{Members: members}, nil
}

func (p *ProxyServer) SetSet(ctx context.Context, request *protocol.SetSetRequest) (*protocol.SetSetResponse, error) {
	return &protocol.SetSetResponse{}, p.database.SetSet(ctx, request.GetKey(), request.GetMembers()...)
}

func (p *ProxyServer) AddSet(ctx context.Context, request *protocol.AddSetRequest) (*protocol.AddSetResponse, error) {
	return &protocol.AddSetResponse{}, p.database.AddSet(ctx, request.GetKey(), request.GetMembers()...)
}

func (p *ProxyServer) RemSet(ctx context.Context, request *protocol.RemSetRequest) (*protocol.RemSetResponse, error) {
	return &protocol.RemSetResponse{}, p.database.RemSet(ctx, request.GetKey(), request.GetMembers()...)
}

func (p *ProxyServer) Push(ctx context.Context, request *protocol.PushRequest) (*protocol.PushResponse, error) {
	return &protocol.PushResponse{}, p.database.Push(ctx, request.GetName(), request.GetValue())
}

func (p *ProxyServer) Pop(ctx context.Context, request *protocol.PopRequest) (*protocol.PopResponse, error) {
	value, err := p.database.Pop(ctx, request.GetName())
	if err != nil {
		if errors.Is(err, io.EOF) {
			return &protocol.PopResponse{}, nil
		}
		return nil, err
	}
	return &protocol.PopResponse{Value: proto.String(value)}, nil
}

func (p *ProxyServer) Remain(ctx context.Context, request *protocol.RemainRequest) (*protocol.RemainResponse, error) {
	count, err := p.database.Remain(ctx, request.GetName())
	if err != nil {
		return nil, err
	}
	return &protocol.RemainResponse{Count: count}, nil
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
	return &protocol.UpdateScoresResponse{}, p.database.UpdateScores(ctx, request.GetCollection(), request.Subset, request.GetId(), ScorePatch{
		IsHidden:   request.GetPatch().IsHidden,
		Categories: request.GetPatch().Categories,
		Score:      request.GetPatch().Score,
	})
}

func (p *ProxyServer) ScanScores(request *protocol.ScanScoresRequest, stream grpc.ServerStreamingServer[protocol.ScanScoresResponse]) error {
	err := p.database.ScanScores(stream.Context(), func(collection, id, subset string, timestamp time.Time) error {
		return stream.Send(&protocol.ScanScoresResponse{
			Collection: collection,
			Id:         id,
			Subset:     subset,
			Timestamp:  timestamppb.New(timestamp),
		})
	})
	if err != nil {
		return status.Errorf(codes.Internal, "failed to scan scores: %v", err)
	}
	return nil
}

func (p *ProxyServer) AddTimeSeriesPoints(ctx context.Context, request *protocol.AddTimeSeriesPointsRequest) (*protocol.AddTimeSeriesPointsResponse, error) {
	points := make([]TimeSeriesPoint, len(request.Points))
	for i, point := range request.Points {
		points[i] = TimeSeriesPoint{
			Name:      point.Name,
			Timestamp: point.Timestamp.AsTime(),
			Value:     point.Value,
		}
	}
	return &protocol.AddTimeSeriesPointsResponse{}, p.database.AddTimeSeriesPoints(ctx, points)
}

func (p *ProxyServer) GetTimeSeriesPoints(ctx context.Context, request *protocol.GetTimeSeriesPointsRequest) (*protocol.GetTimeSeriesPointsResponse, error) {
	resp, err := p.database.GetTimeSeriesPoints(ctx, request.GetName(), request.GetBegin().AsTime(), request.GetEnd().AsTime(), time.Duration(request.GetDuration()))
	if err != nil {
		return nil, err
	}
	points := make([]*protocol.TimeSeriesPoint, len(resp))
	for i, point := range resp {
		points[i] = &protocol.TimeSeriesPoint{
			Name:      point.Name,
			Timestamp: timestamppb.New(point.Timestamp),
			Value:     point.Value,
		}
	}
	return &protocol.GetTimeSeriesPointsResponse{Points: points}, nil
}

type ProxyClient struct {
	protocol.CacheStoreClient
}

func (p ProxyClient) Ping() error {
	_, err := p.CacheStoreClient.Ping(context.Background(), &protocol.PingRequest{})
	return err
}

func (p ProxyClient) Close() error {
	return nil
}

func (p ProxyClient) Init() error {
	return errors.MethodNotAllowedf("init is not allowed in proxy client")
}

func (p ProxyClient) Scan(_ func(string) error) error {
	return errors.MethodNotAllowedf("scan is not allowed in proxy client")
}

func (p ProxyClient) Purge() error {
	return errors.MethodNotAllowedf("purge is not allowed in proxy client")
}

func (p ProxyClient) Set(ctx context.Context, values ...Value) error {
	pbValues := make([]*protocol.Value, len(values))
	for i, value := range values {
		pbValues[i] = &protocol.Value{
			Name:  value.name,
			Value: value.value,
		}
	}
	_, err := p.CacheStoreClient.Set(ctx, &protocol.SetRequest{
		Values: pbValues,
	})
	return err
}

func (p ProxyClient) Get(ctx context.Context, name string) *ReturnValue {
	resp, err := p.CacheStoreClient.Get(ctx, &protocol.GetRequest{
		Name: name,
	})
	if err != nil {
		return &ReturnValue{err: err}
	}
	if resp.Value == nil {
		return &ReturnValue{err: errors.NotFound}
	}
	return &ReturnValue{value: resp.GetValue(), err: err}
}

func (p ProxyClient) Delete(ctx context.Context, name string) error {
	_, err := p.CacheStoreClient.Delete(ctx, &protocol.DeleteRequest{
		Name: name,
	})
	return err
}

func (p ProxyClient) GetSet(ctx context.Context, key string) ([]string, error) {
	resp, err := p.CacheStoreClient.GetSet(ctx, &protocol.GetSetRequest{
		Key: key,
	})
	if err != nil {
		return nil, err
	}
	return resp.Members, nil
}

func (p ProxyClient) SetSet(ctx context.Context, key string, members ...string) error {
	_, err := p.CacheStoreClient.SetSet(ctx, &protocol.SetSetRequest{
		Key:     key,
		Members: members,
	})
	return err
}

func (p ProxyClient) AddSet(ctx context.Context, key string, members ...string) error {
	_, err := p.CacheStoreClient.AddSet(ctx, &protocol.AddSetRequest{
		Key:     key,
		Members: members,
	})
	return err
}

func (p ProxyClient) RemSet(ctx context.Context, key string, members ...string) error {
	_, err := p.CacheStoreClient.RemSet(ctx, &protocol.RemSetRequest{
		Key:     key,
		Members: members,
	})
	return err
}

func (p ProxyClient) Push(ctx context.Context, name, value string) error {
	_, err := p.CacheStoreClient.Push(ctx, &protocol.PushRequest{
		Name:  name,
		Value: value,
	})
	return err
}

func (p ProxyClient) Pop(ctx context.Context, name string) (string, error) {
	resp, err := p.CacheStoreClient.Pop(ctx, &protocol.PopRequest{
		Name: name,
	})
	if err != nil {
		return "", err
	}
	if resp.Value == nil {
		return "", io.EOF
	}
	return resp.GetValue(), nil
}

func (p ProxyClient) Remain(ctx context.Context, name string) (int64, error) {
	resp, err := p.CacheStoreClient.Remain(ctx, &protocol.RemainRequest{
		Name: name,
	})
	if err != nil {
		return 0, err
	}
	return resp.Count, nil
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

func (p ProxyClient) UpdateScores(ctx context.Context, collection []string, subset *string, id string, patch ScorePatch) error {
	_, err := p.CacheStoreClient.UpdateScores(ctx, &protocol.UpdateScoresRequest{
		Collection: collection,
		Subset:     subset,
		Id:         id,
		Patch: &protocol.ScorePatch{
			Score:      patch.Score,
			IsHidden:   patch.IsHidden,
			Categories: patch.Categories,
		},
	})
	return err
}

func (p ProxyClient) ScanScores(ctx context.Context, callback func(collection string, id string, subset string, timestamp time.Time) error) error {
	stream, err := p.CacheStoreClient.ScanScores(ctx, &protocol.ScanScoresRequest{})
	if err != nil {
		return err
	}
	for {
		// check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		// receive the next message
		resp, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := callback(resp.Collection, resp.Id, resp.Subset, resp.Timestamp.AsTime()); err != nil {
			return err
		}
	}
}

func (p ProxyClient) AddTimeSeriesPoints(ctx context.Context, points []TimeSeriesPoint) error {
	pbPoints := make([]*protocol.TimeSeriesPoint, len(points))
	for i, point := range points {
		pbPoints[i] = &protocol.TimeSeriesPoint{
			Name:      point.Name,
			Timestamp: timestamppb.New(point.Timestamp),
			Value:     point.Value,
		}
	}
	_, err := p.CacheStoreClient.AddTimeSeriesPoints(ctx, &protocol.AddTimeSeriesPointsRequest{
		Points: pbPoints,
	})
	return err
}

func (p ProxyClient) GetTimeSeriesPoints(ctx context.Context, name string, begin, end time.Time, duration time.Duration) ([]TimeSeriesPoint, error) {
	resp, err := p.CacheStoreClient.GetTimeSeriesPoints(ctx, &protocol.GetTimeSeriesPointsRequest{
		Name:     name,
		Begin:    timestamppb.New(begin),
		End:      timestamppb.New(end),
		Duration: int64(duration),
	})
	if err != nil {
		return nil, err
	}
	points := make([]TimeSeriesPoint, len(resp.Points))
	for i, point := range resp.Points {
		points[i] = TimeSeriesPoint{
			Name:      point.Name,
			Timestamp: point.Timestamp.AsTime(),
			Value:     point.Value,
		}
	}
	return points, nil
}

func NewProxyClient(conn *grpc.ClientConn) *ProxyClient {
	return &ProxyClient{
		CacheStoreClient: protocol.NewCacheStoreClient(conn),
	}
}
