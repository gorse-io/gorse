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

package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/gorse-io/gorse/protocol"
	"github.com/gorse-io/gorse/storage"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func geoServerError(err error) error {
	if err == nil {
		return nil
	}
	var code codes.Code
	switch {
	case errors.Is(err, context.Canceled):
		code = codes.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		code = codes.DeadlineExceeded
	case errors.Is(err, storage.ErrInvalidArgument):
		code = codes.InvalidArgument
	case errors.Is(err, storage.ErrAlreadyExists):
		code = codes.AlreadyExists
	case errors.Is(err, storage.ErrNotFound):
		code = codes.NotFound
	case errors.Is(err, storage.ErrNoDatabase):
		s, detailErr := status.New(codes.FailedPrecondition, err.Error()).WithDetails(&errdetails.ErrorInfo{
			Reason: "CACHE_NO_DATABASE", Domain: "gorse",
		})
		if detailErr == nil {
			return s.Err()
		}
		code = codes.FailedPrecondition
	default:
		code = codes.Internal
	}
	return status.Error(code, err.Error())
}

func geoClientError(err error) error {
	if err == nil {
		return nil
	}
	s := status.Convert(err)
	var target error
	switch s.Code() {
	case codes.InvalidArgument:
		target = storage.ErrInvalidArgument
	case codes.AlreadyExists:
		target = storage.ErrAlreadyExists
	case codes.NotFound:
		target = storage.ErrNotFound
	case codes.Canceled:
		target = context.Canceled
	case codes.DeadlineExceeded:
		target = context.DeadlineExceeded
	case codes.FailedPrecondition:
		for _, detail := range s.Details() {
			if info, ok := detail.(*errdetails.ErrorInfo); ok && info.GetDomain() == "gorse" && info.GetReason() == "CACHE_NO_DATABASE" {
				target = storage.ErrNoDatabase
				break
			}
		}
	}
	if target == nil {
		return err
	}
	return fmt.Errorf("%s %w", s.Message(), target)
}

func geoPointToProto(point GeoPoint) *protocol.GeoPoint {
	return &protocol.GeoPoint{Id: point.Id, Longitude: point.Longitude, Latitude: point.Latitude}
}

func geoPointFromProto(point *protocol.GeoPoint) GeoPoint {
	return GeoPoint{Id: point.GetId(), Longitude: point.GetLongitude(), Latitude: point.GetLatitude()}
}

func (p *ProxyServer) AddGeoIndex(ctx context.Context, request *protocol.AddGeoIndexRequest) (*protocol.AddGeoIndexResponse, error) {
	return &protocol.AddGeoIndexResponse{}, geoServerError(p.database.AddGeoIndex(ctx, request.GetName()))
}

func (p *ProxyServer) AddGeoPoints(ctx context.Context, request *protocol.AddGeoPointsRequest) (*protocol.AddGeoPointsResponse, error) {
	points := make([]GeoPoint, len(request.GetPoints()))
	for i, point := range request.GetPoints() {
		points[i] = geoPointFromProto(point)
	}
	return &protocol.AddGeoPointsResponse{}, geoServerError(p.database.AddGeoPoints(ctx, request.GetName(), points))
}

func (p *ProxyServer) SearchGeoPoints(ctx context.Context, request *protocol.SearchGeoPointsRequest) (*protocol.SearchGeoPointsResponse, error) {
	q := request.GetQuery()
	if q == nil || q.GetN() < 0 || int64(int(q.GetN())) != q.GetN() {
		return nil, geoServerError(fmt.Errorf("invalid geographic query %w", storage.ErrInvalidArgument))
	}
	points, err := p.database.SearchGeoPoints(ctx, request.GetName(), GeoQuery{
		Longitude: q.GetLongitude(), Latitude: q.GetLatitude(), N: int(q.GetN()), MaxDistance: q.GetMaxDistance(),
	})
	if err != nil {
		return nil, geoServerError(err)
	}
	response := &protocol.SearchGeoPointsResponse{Points: make([]*protocol.GeoResult, len(points))}
	for i, point := range points {
		response.Points[i] = &protocol.GeoResult{Point: geoPointToProto(point.GeoPoint), Distance: point.Distance}
	}
	return response, nil
}

func (p *ProxyServer) DeleteGeoPoints(ctx context.Context, request *protocol.DeleteGeoPointsRequest) (*protocol.DeleteGeoPointsResponse, error) {
	return &protocol.DeleteGeoPointsResponse{}, geoServerError(p.database.DeleteGeoPoints(ctx, request.GetName(), request.GetIds()...))
}

func (p *ProxyServer) DeleteGeoIndex(ctx context.Context, request *protocol.DeleteGeoIndexRequest) (*protocol.DeleteGeoIndexResponse, error) {
	return &protocol.DeleteGeoIndexResponse{}, geoServerError(p.database.DeleteGeoIndex(ctx, request.GetName()))
}

func (p ProxyClient) AddGeoIndex(ctx context.Context, name string) error {
	if err := validateGeoName(name); err != nil {
		return err
	}
	_, err := p.CacheStoreClient.AddGeoIndex(ctx, &protocol.AddGeoIndexRequest{Name: name})
	return geoClientError(err)
}

func (p ProxyClient) AddGeoPoints(ctx context.Context, name string, points []GeoPoint) error {
	records, err := prepareGeoPoints(name, points)
	if err != nil {
		return err
	}
	request := &protocol.AddGeoPointsRequest{Name: name, Points: make([]*protocol.GeoPoint, len(records))}
	for i, record := range records {
		request.Points[i] = geoPointToProto(record.GeoPoint)
	}
	_, err = p.CacheStoreClient.AddGeoPoints(ctx, request)
	return geoClientError(err)
}

func (p ProxyClient) SearchGeoPoints(ctx context.Context, name string, query GeoQuery) ([]GeoResult, error) {
	query, err := normalizeGeoQuery(name, query)
	if err != nil {
		return nil, err
	}
	response, err := p.CacheStoreClient.SearchGeoPoints(ctx, &protocol.SearchGeoPointsRequest{
		Name: name, Query: &protocol.GeoQuery{Longitude: query.Longitude, Latitude: query.Latitude, N: int64(query.N), MaxDistance: query.MaxDistance},
	})
	if err != nil {
		return nil, geoClientError(err)
	}
	points := make([]GeoResult, len(response.GetPoints()))
	for i, point := range response.GetPoints() {
		if point == nil || point.GetPoint() == nil {
			return nil, errors.New("invalid geographic point in proxy response")
		}
		points[i] = GeoResult{GeoPoint: geoPointFromProto(point.GetPoint()), Distance: point.GetDistance()}
	}
	return points, nil
}

func (p ProxyClient) DeleteGeoPoints(ctx context.Context, name string, ids ...string) error {
	ids, err := prepareGeoIDs(name, ids)
	if err != nil {
		return err
	}
	_, err = p.CacheStoreClient.DeleteGeoPoints(ctx, &protocol.DeleteGeoPointsRequest{Name: name, Ids: ids})
	return geoClientError(err)
}

func (p ProxyClient) DeleteGeoIndex(ctx context.Context, name string) error {
	if err := validateGeoName(name); err != nil {
		return err
	}
	_, err := p.CacheStoreClient.DeleteGeoIndex(ctx, &protocol.DeleteGeoIndexRequest{Name: name})
	return geoClientError(err)
}
