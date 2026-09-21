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

package vectors

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/gorse-io/gorse/protocol"
	"github.com/gorse-io/gorse/storage"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProxyServer struct {
	protocol.UnimplementedVectorStoreServer
	database Database
	server   *grpc.Server
}

func NewProxyServer(database Database) *ProxyServer {
	return &ProxyServer{database: database}
}

func (p *ProxyServer) Serve(lis net.Listener) error {
	p.server = grpc.NewServer()
	protocol.RegisterVectorStoreServer(p.server, p)
	return p.server.Serve(lis)
}

func (p *ProxyServer) Stop() {
	p.server.Stop()
}

func (p *ProxyServer) ListCollections(ctx context.Context, _ *protocol.ListCollectionsRequest) (*protocol.ListCollectionsResponse, error) {
	collections, err := p.database.ListCollections(ctx)
	if err != nil {
		return nil, err
	}
	return &protocol.ListCollectionsResponse{Collections: collections}, nil
}

func (p *ProxyServer) DescribeCollection(ctx context.Context, request *protocol.DescribeCollectionRequest) (*protocol.DescribeCollectionResponse, error) {
	info, err := p.database.DescribeCollection(ctx, request.GetName())
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, err
	}
	distance, err := distanceToProtoDistance(info.Distance)
	if err != nil {
		return nil, err
	}
	return &protocol.DescribeCollectionResponse{
		Name:       info.Name,
		Dimensions: int32(info.Dimension),
		Distance:   distance,
		Config: &protocol.VectorConfig{
			QuantizationType: string(info.Type),
			QuantizationBits: int32(info.Bits),
		},
	}, nil
}

func (p *ProxyServer) AddCollection(ctx context.Context, request *protocol.AddCollectionRequest) (*protocol.AddCollectionResponse, error) {
	distance, err := protoDistanceToDistance(request.GetDistance())
	if err != nil {
		return nil, err
	}
	config := protoVectorConfigToVectorConfig(request.GetConfig())
	err = p.database.AddCollection(ctx, request.GetName(), int(request.GetDimensions()), distance, config)
	if err != nil {
		return nil, err
	}
	return &protocol.AddCollectionResponse{}, nil
}

func (p *ProxyServer) DeleteCollection(ctx context.Context, request *protocol.DeleteCollectionRequest) (*protocol.DeleteCollectionResponse, error) {
	err := p.database.DeleteCollection(ctx, request.GetName())
	if err != nil {
		return nil, err
	}
	return &protocol.DeleteCollectionResponse{}, nil
}

func (p *ProxyServer) CountVectors(ctx context.Context, request *protocol.CountVectorsRequest) (*protocol.CountVectorsResponse, error) {
	count, err := p.database.CountVectors(ctx, request.GetCollection())
	if err != nil {
		return nil, err
	}
	return &protocol.CountVectorsResponse{Count: count}, nil
}

func (p *ProxyServer) AddVectors(ctx context.Context, request *protocol.AddVectorsRequest) (*protocol.AddVectorsResponse, error) {
	vectors := make([]Vector, len(request.Vectors))
	for i, vector := range request.Vectors {
		timestamp := time.Time{}
		if vector.GetTimestamp() != nil {
			timestamp = vector.GetTimestamp().AsTime()
		}
		vectors[i] = Vector{
			Id:         vector.GetId(),
			Values:     vector.GetValues(),
			Indices:    vector.GetIndices(),
			IsHidden:   vector.GetIsHidden(),
			Categories: vector.GetCategories(),
			Timestamp:  timestamp,
		}
	}
	err := p.database.AddVectors(ctx, request.GetCollection(), vectors)
	if err != nil {
		return nil, err
	}
	return &protocol.AddVectorsResponse{}, nil
}

func (p *ProxyServer) GetVectors(ctx context.Context, request *protocol.GetVectorsRequest) (*protocol.GetVectorsResponse, error) {
	vectors, err := p.database.GetVectors(ctx, request.GetCollection(), request.GetIds())
	if err != nil {
		return nil, err
	}
	pbVectors := make([]*protocol.Vector, len(vectors))
	for i, vector := range vectors {
		pbVectors[i] = &protocol.Vector{
			Id:         vector.Id,
			Values:     vector.Values,
			Indices:    vector.Indices,
			IsHidden:   vector.IsHidden,
			Categories: vector.Categories,
			Timestamp:  timestamppb.New(vector.Timestamp),
		}
	}
	return &protocol.GetVectorsResponse{Vectors: pbVectors}, nil
}

func (p *ProxyServer) DeleteVectors(ctx context.Context, request *protocol.DeleteVectorsRequest) (*protocol.DeleteVectorsResponse, error) {
	timestamp := time.Time{}
	if request.GetTimestamp() != nil {
		timestamp = request.GetTimestamp().AsTime()
	}
	err := p.database.DeleteVectors(ctx, request.GetCollection(), timestamp)
	if err != nil {
		return nil, err
	}
	return &protocol.DeleteVectorsResponse{}, nil
}

func (p *ProxyServer) QueryVectors(ctx context.Context, request *protocol.QueryVectorsRequest) (*protocol.QueryVectorsResponse, error) {
	query := request.GetQuery()
	results, err := p.database.QueryVectors(ctx, request.GetCollection(), Vector{
		Values:  query.GetValues(),
		Indices: query.GetIndices(),
	}, request.GetCategories(), int(request.GetTopK()))
	if err != nil {
		return nil, err
	}
	pbVectors := make([]*protocol.ScoredVector, len(results))
	for i, result := range results {
		pbVectors[i] = &protocol.ScoredVector{
			Vector: &protocol.Vector{
				Id:         result.Id,
				Values:     result.Vector.Values,
				Indices:    result.Vector.Indices,
				IsHidden:   result.IsHidden,
				Categories: result.Categories,
			},
			Score: result.Score,
		}
	}
	return &protocol.QueryVectorsResponse{Vectors: pbVectors}, nil
}

type ProxyClient struct {
	protocol.VectorStoreClient
}

func NewProxyClient(conn *grpc.ClientConn) *ProxyClient {
	return &ProxyClient{
		VectorStoreClient: protocol.NewVectorStoreClient(conn),
	}
}

func (p ProxyClient) Init() error {
	return nil
}

func (p ProxyClient) Optimize(_ context.Context, _ string) error {
	return nil
}

func (p ProxyClient) Close() error {
	return nil
}

func (p ProxyClient) ListCollections(ctx context.Context) ([]string, error) {
	resp, err := p.VectorStoreClient.ListCollections(ctx, &protocol.ListCollectionsRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Collections, nil
}

func (p ProxyClient) DescribeCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	resp, err := p.VectorStoreClient.DescribeCollection(ctx, &protocol.DescribeCollectionRequest{Name: name})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, fmt.Errorf("collection %s: %w", name, storage.ErrNotFound)
		}
		return nil, err
	}
	distance, err := protoDistanceToDistance(resp.GetDistance())
	if err != nil {
		return nil, err
	}
	config := VectorConfig{}
	if resp.GetConfig() != nil {
		config.Type = QuantizationType(resp.GetConfig().GetQuantizationType())
		config.Bits = int(resp.GetConfig().GetQuantizationBits())
	}
	return &CollectionInfo{
		Name:         resp.GetName(),
		Dimension:    int(resp.GetDimensions()),
		Distance:     distance,
		VectorConfig: config,
	}, nil
}

func (p ProxyClient) AddCollection(ctx context.Context, name string, dimensions int, distance Distance, config VectorConfig) error {
	pbDistance, err := distanceToProtoDistance(distance)
	if err != nil {
		return err
	}
	_, err = p.VectorStoreClient.AddCollection(ctx, &protocol.AddCollectionRequest{
		Name:       name,
		Dimensions: int32(dimensions),
		Distance:   pbDistance,
		Config:     vectorConfigToProtoVectorConfig(config),
	})
	return err
}

func (p ProxyClient) DeleteCollection(ctx context.Context, name string) error {
	_, err := p.VectorStoreClient.DeleteCollection(ctx, &protocol.DeleteCollectionRequest{Name: name})
	return err
}

func (p ProxyClient) CountVectors(ctx context.Context, collection string) (int64, error) {
	resp, err := p.VectorStoreClient.CountVectors(ctx, &protocol.CountVectorsRequest{Collection: collection})
	if err != nil {
		return 0, err
	}
	return resp.GetCount(), nil
}

func (p ProxyClient) AddVectors(ctx context.Context, collection string, vectors []Vector) error {
	pbVectors := make([]*protocol.Vector, len(vectors))
	for i, vector := range vectors {
		pbVectors[i] = &protocol.Vector{
			Id:         vector.Id,
			Values:     vector.Values,
			Indices:    vector.Indices,
			IsHidden:   vector.IsHidden,
			Categories: vector.Categories,
			Timestamp:  timestamppb.New(vector.Timestamp),
		}
	}
	_, err := p.VectorStoreClient.AddVectors(ctx, &protocol.AddVectorsRequest{
		Collection: collection,
		Vectors:    pbVectors,
	})
	return err
}

func (p ProxyClient) GetVectors(ctx context.Context, collection string, ids []string) ([]Vector, error) {
	resp, err := p.VectorStoreClient.GetVectors(ctx, &protocol.GetVectorsRequest{
		Collection: collection,
		Ids:        ids,
	})
	if err != nil {
		return nil, err
	}
	vectors := make([]Vector, len(resp.GetVectors()))
	for i, vector := range resp.GetVectors() {
		timestamp := time.Time{}
		if vector.GetTimestamp() != nil {
			timestamp = vector.GetTimestamp().AsTime()
		}
		vectors[i] = Vector{
			Id:         vector.GetId(),
			Values:     vector.GetValues(),
			Indices:    vector.GetIndices(),
			IsHidden:   vector.GetIsHidden(),
			Categories: vector.GetCategories(),
			Timestamp:  timestamp,
		}
	}
	return vectors, nil
}

func (p ProxyClient) DeleteVectors(ctx context.Context, collection string, timestamp time.Time) error {
	_, err := p.VectorStoreClient.DeleteVectors(ctx, &protocol.DeleteVectorsRequest{
		Collection: collection,
		Timestamp:  timestamppb.New(timestamp),
	})
	return err
}

func (p ProxyClient) QueryVectors(ctx context.Context, collection string, q Vector, categories []string, topK int) ([]ScoredVector, error) {
	resp, err := p.VectorStoreClient.QueryVectors(ctx, &protocol.QueryVectorsRequest{
		Collection: collection,
		Query: &protocol.Vector{
			Values:  q.Values,
			Indices: q.Indices,
		},
		Categories: categories,
		TopK:       int32(topK),
	})
	if err != nil {
		return nil, err
	}
	results := make([]ScoredVector, len(resp.Vectors))
	for i, scored := range resp.Vectors {
		vector := scored.GetVector()
		results[i] = ScoredVector{
			Vector: Vector{
				Id:         vector.GetId(),
				Values:     vector.GetValues(),
				Indices:    vector.GetIndices(),
				IsHidden:   vector.GetIsHidden(),
				Categories: vector.GetCategories(),
			},
			Score: scored.GetScore(),
		}
	}
	return results, nil
}

func distanceToProtoDistance(distance Distance) (protocol.Distance, error) {
	switch distance {
	case Cosine:
		return protocol.Distance_Cosine, nil
	case Euclidean:
		return protocol.Distance_Euclidean, nil
	case Dot:
		return protocol.Distance_Dot, nil
	default:
		return protocol.Distance_Unknown, fmt.Errorf("distance method %w", storage.ErrNotSupported)
	}
}

func protoDistanceToDistance(distance protocol.Distance) (Distance, error) {
	switch distance {
	case protocol.Distance_Cosine:
		return Cosine, nil
	case protocol.Distance_Euclidean:
		return Euclidean, nil
	case protocol.Distance_Dot:
		return Dot, nil
	default:
		return Cosine, fmt.Errorf("distance method %w", storage.ErrNotSupported)
	}
}

func vectorConfigToProtoVectorConfig(config VectorConfig) *protocol.VectorConfig {
	return &protocol.VectorConfig{
		QuantizationType: string(config.Type),
		QuantizationBits: int32(config.Bits),
	}
}

func protoVectorConfigToVectorConfig(config *protocol.VectorConfig) VectorConfig {
	vectorConfig := VectorConfig{}
	if config == nil {
		return vectorConfig
	}
	vectorConfig.Type = QuantizationType(config.GetQuantizationType())
	vectorConfig.Bits = int(config.GetQuantizationBits())
	return vectorConfig
}
