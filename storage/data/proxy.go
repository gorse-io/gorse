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

package data

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"time"

	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/protocol"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProxyServer struct {
	protocol.UnimplementedDataStoreServer
	database Database
	server   *grpc.Server
}

func NewProxyServer(database Database) *ProxyServer {
	return &ProxyServer{database: database}
}

func (p *ProxyServer) Serve(lis net.Listener) error {
	p.server = grpc.NewServer()
	protocol.RegisterDataStoreServer(p.server, p)
	return p.server.Serve(lis)
}

func (p *ProxyServer) Stop() {
	p.server.Stop()
}

func (p *ProxyServer) Ping(_ context.Context, _ *protocol.PingRequest) (*protocol.PingResponse, error) {
	return &protocol.PingResponse{}, p.database.Ping()
}

func (p *ProxyServer) BatchInsertItems(ctx context.Context, in *protocol.BatchInsertItemsRequest) (*protocol.BatchInsertItemsResponse, error) {
	items := make([]Item, len(in.Items))
	for i, item := range in.Items {
		var labels any
		err := json.Unmarshal(item.Labels, &labels)
		if err != nil {
			return nil, err
		}
		items[i] = Item{
			ItemId:     item.ItemId,
			IsHidden:   item.IsHidden,
			Categories: item.Categories,
			Timestamp:  item.Timestamp.AsTime(),
			Labels:     labels,
			Comment:    item.Comment,
		}
	}
	err := p.database.BatchInsertItems(ctx, items)
	return &protocol.BatchInsertItemsResponse{}, err
}

func (p *ProxyServer) BatchGetItems(ctx context.Context, in *protocol.BatchGetItemsRequest) (*protocol.BatchGetItemsResponse, error) {
	items, err := p.database.BatchGetItems(ctx, in.ItemIds)
	if err != nil {
		return nil, err
	}
	pbItems := make([]*protocol.Item, len(items))
	for i, item := range items {
		labels, err := json.Marshal(item.Labels)
		if err != nil {
			return nil, err
		}
		pbItems[i] = &protocol.Item{
			ItemId:     item.ItemId,
			IsHidden:   item.IsHidden,
			Categories: item.Categories,
			Timestamp:  timestamppb.New(item.Timestamp),
			Labels:     labels,
			Comment:    item.Comment,
		}
	}
	return &protocol.BatchGetItemsResponse{Items: pbItems}, nil
}

func (p *ProxyServer) DeleteItem(ctx context.Context, in *protocol.DeleteItemRequest) (*protocol.DeleteItemResponse, error) {
	err := p.database.DeleteItem(ctx, in.ItemId)
	return &protocol.DeleteItemResponse{}, err
}

func (p *ProxyServer) GetItem(ctx context.Context, in *protocol.GetItemRequest) (*protocol.GetItemResponse, error) {
	item, err := p.database.GetItem(ctx, in.ItemId)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			return &protocol.GetItemResponse{}, nil
		}
		return nil, err
	}
	labels, err := json.Marshal(item.Labels)
	if err != nil {
		return nil, err
	}
	return &protocol.GetItemResponse{
		Item: &protocol.Item{
			ItemId:     item.ItemId,
			IsHidden:   item.IsHidden,
			Categories: item.Categories,
			Timestamp:  timestamppb.New(item.Timestamp),
			Labels:     labels,
			Comment:    item.Comment,
		},
	}, nil
}

func (p *ProxyServer) ModifyItem(ctx context.Context, in *protocol.ModifyItemRequest) (*protocol.ModifyItemResponse, error) {
	var labels any
	if in.Patch.Labels != nil {
		err := json.Unmarshal(in.Patch.Labels, &labels)
		if err != nil {
			return nil, err
		}
	}
	var timestamp *time.Time
	if in.Patch.Timestamp != nil {
		timestamp = lo.ToPtr(in.Patch.Timestamp.AsTime())
	}
	err := p.database.ModifyItem(ctx, in.ItemId, ItemPatch{
		IsHidden:   in.Patch.IsHidden,
		Categories: in.Patch.Categories,
		Labels:     labels,
		Comment:    in.Patch.Comment,
		Timestamp:  timestamp,
	})
	return &protocol.ModifyItemResponse{}, err
}

func (p *ProxyServer) GetItems(ctx context.Context, in *protocol.GetItemsRequest) (*protocol.GetItemsResponse, error) {
	var beginTime *time.Time
	if in.BeginTime != nil {
		beginTime = lo.ToPtr(in.BeginTime.AsTime())
	}
	cursor, items, err := p.database.GetItems(ctx, in.Cursor, int(in.N), beginTime)
	if err != nil {
		return nil, err
	}
	pbItems := make([]*protocol.Item, len(items))
	for i, item := range items {
		labels, err := json.Marshal(item.Labels)
		if err != nil {
			return nil, err
		}
		pbItems[i] = &protocol.Item{
			ItemId:     item.ItemId,
			IsHidden:   item.IsHidden,
			Categories: item.Categories,
			Timestamp:  timestamppb.New(item.Timestamp),
			Labels:     labels,
			Comment:    item.Comment,
		}
	}
	return &protocol.GetItemsResponse{Cursor: cursor, Items: pbItems}, nil
}

func (p *ProxyServer) GetItemFeedback(ctx context.Context, in *protocol.GetItemFeedbackRequest) (*protocol.GetFeedbackResponse, error) {
	feedback, err := p.database.GetItemFeedback(ctx, in.ItemId, in.FeedbackTypes...)
	if err != nil {
		return nil, err
	}
	pbFeedback := make([]*protocol.Feedback, len(feedback))
	for i, f := range feedback {
		pbFeedback[i] = &protocol.Feedback{
			FeedbackType: f.FeedbackType,
			UserId:       f.UserId,
			ItemId:       f.ItemId,
			Value:        f.Value,
			Timestamp:    timestamppb.New(f.Timestamp),
			Comment:      f.Comment,
		}
	}
	return &protocol.GetFeedbackResponse{Feedback: pbFeedback}, nil
}

func (p *ProxyServer) BatchInsertUsers(ctx context.Context, in *protocol.BatchInsertUsersRequest) (*protocol.BatchInsertUsersResponse, error) {
	users := make([]User, len(in.Users))
	for i, user := range in.Users {
		var labels any
		err := json.Unmarshal(user.Labels, &labels)
		if err != nil {
			return nil, err
		}
		users[i] = User{
			UserId:    user.UserId,
			Labels:    labels,
			Comment:   user.Comment,
			Subscribe: user.Subscribe,
		}
	}
	err := p.database.BatchInsertUsers(ctx, users)
	return &protocol.BatchInsertUsersResponse{}, err
}

func (p *ProxyServer) DeleteUser(ctx context.Context, in *protocol.DeleteUserRequest) (*protocol.DeleteUserResponse, error) {
	err := p.database.DeleteUser(ctx, in.UserId)
	return &protocol.DeleteUserResponse{}, err
}

func (p *ProxyServer) GetUser(ctx context.Context, in *protocol.GetUserRequest) (*protocol.GetUserResponse, error) {
	user, err := p.database.GetUser(ctx, in.UserId)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			return &protocol.GetUserResponse{}, nil
		}
		return nil, err
	}
	labels, err := json.Marshal(user.Labels)
	if err != nil {
		return nil, err
	}
	return &protocol.GetUserResponse{
		User: &protocol.User{
			UserId:    user.UserId,
			Labels:    labels,
			Comment:   user.Comment,
			Subscribe: user.Subscribe,
		},
	}, nil
}

func (p *ProxyServer) ModifyUser(ctx context.Context, in *protocol.ModifyUserRequest) (*protocol.ModifyUserResponse, error) {
	var labels any
	if in.Patch.Labels != nil {
		err := json.Unmarshal(in.Patch.Labels, &labels)
		if err != nil {
			return nil, err
		}
	}
	err := p.database.ModifyUser(ctx, in.UserId, UserPatch{
		Labels:    labels,
		Comment:   in.Patch.Comment,
		Subscribe: in.Patch.Subscribe,
	})
	return &protocol.ModifyUserResponse{}, err
}

func (p *ProxyServer) GetUsers(ctx context.Context, in *protocol.GetUsersRequest) (*protocol.GetUsersResponse, error) {
	cursor, users, err := p.database.GetUsers(ctx, in.Cursor, int(in.N))
	if err != nil {
		return nil, err
	}
	pbUsers := make([]*protocol.User, len(users))
	for i, user := range users {
		labels, err := json.Marshal(user.Labels)
		if err != nil {
			return nil, err
		}
		pbUsers[i] = &protocol.User{
			UserId:    user.UserId,
			Labels:    labels,
			Comment:   user.Comment,
			Subscribe: user.Subscribe,
		}
	}
	return &protocol.GetUsersResponse{Cursor: cursor, Users: pbUsers}, nil
}

func (p *ProxyServer) GetUserFeedback(ctx context.Context, in *protocol.GetUserFeedbackRequest) (*protocol.GetFeedbackResponse, error) {
	var endTime *time.Time
	if in.EndTime != nil {
		endTime = lo.ToPtr(in.EndTime.AsTime())
	}
	feedback, err := p.database.GetUserFeedback(ctx, in.UserId, endTime, in.FeedbackTypes...)
	if err != nil {
		return nil, err
	}
	pbFeedback := make([]*protocol.Feedback, len(feedback))
	for i, f := range feedback {
		pbFeedback[i] = &protocol.Feedback{
			FeedbackType: f.FeedbackType,
			UserId:       f.UserId,
			ItemId:       f.ItemId,
			Value:        f.Value,
			Timestamp:    timestamppb.New(f.Timestamp),
			Comment:      f.Comment,
		}
	}
	return &protocol.GetFeedbackResponse{Feedback: pbFeedback}, nil
}

func (p *ProxyServer) GetUserItemFeedback(ctx context.Context, in *protocol.GetUserItemFeedbackRequest) (*protocol.GetFeedbackResponse, error) {
	feedback, err := p.database.GetUserItemFeedback(ctx, in.UserId, in.ItemId, in.FeedbackTypes...)
	if err != nil {
		return nil, err
	}
	pbFeedback := make([]*protocol.Feedback, len(feedback))
	for i, f := range feedback {
		pbFeedback[i] = &protocol.Feedback{
			FeedbackType: f.FeedbackType,
			UserId:       f.UserId,
			ItemId:       f.ItemId,
			Value:        f.Value,
			Timestamp:    timestamppb.New(f.Timestamp),
			Comment:      f.Comment,
		}
	}
	return &protocol.GetFeedbackResponse{Feedback: pbFeedback}, nil
}

func (p *ProxyServer) DeleteUserItemFeedback(ctx context.Context, in *protocol.DeleteUserItemFeedbackRequest) (*protocol.DeleteUserItemFeedbackResponse, error) {
	count, err := p.database.DeleteUserItemFeedback(ctx, in.UserId, in.ItemId, in.FeedbackTypes...)
	return &protocol.DeleteUserItemFeedbackResponse{Count: int32(count)}, err
}

func (p *ProxyServer) BatchInsertFeedback(ctx context.Context, in *protocol.BatchInsertFeedbackRequest) (*protocol.BatchInsertFeedbackResponse, error) {
	feedback := make([]Feedback, len(in.Feedback))
	for i, f := range in.Feedback {
		feedback[i] = Feedback{
			FeedbackKey: FeedbackKey{
				FeedbackType: f.FeedbackType,
				UserId:       f.UserId,
				ItemId:       f.ItemId,
			},
			Value:     f.Value,
			Timestamp: f.Timestamp.AsTime(),
			Comment:   f.Comment,
		}
	}
	err := p.database.BatchInsertFeedback(ctx, feedback, in.InsertUser, in.InsertItem, in.Overwrite)
	return &protocol.BatchInsertFeedbackResponse{}, err
}

func (p *ProxyServer) GetFeedback(ctx context.Context, in *protocol.GetFeedbackRequest) (*protocol.GetFeedbackResponse, error) {
	var beginTime, endTime *time.Time
	if in.BeginTime != nil {
		beginTime = lo.ToPtr(in.BeginTime.AsTime())
	}
	if in.EndTime != nil {
		endTime = lo.ToPtr(in.EndTime.AsTime())
	}
	cursor, feedback, err := p.database.GetFeedback(ctx, in.Cursor, int(in.N), beginTime, endTime, in.FeedbackTypes...)
	if err != nil {
		return nil, err
	}
	pbFeedback := make([]*protocol.Feedback, len(feedback))
	for i, f := range feedback {
		pbFeedback[i] = &protocol.Feedback{
			FeedbackType: f.FeedbackType,
			UserId:       f.UserId,
			ItemId:       f.ItemId,
			Value:        f.Value,
			Timestamp:    timestamppb.New(f.Timestamp),
			Comment:      f.Comment,
		}
	}
	return &protocol.GetFeedbackResponse{Cursor: cursor, Feedback: pbFeedback}, nil
}

func (p *ProxyServer) GetUserStream(in *protocol.GetUserStreamRequest, stream grpc.ServerStreamingServer[protocol.GetUserStreamResponse]) error {
	usersChan, errChan := p.database.GetUserStream(stream.Context(), int(in.BatchSize))
	for users := range usersChan {
		pbUsers := make([]*protocol.User, len(users))
		for i, user := range users {
			labels, err := json.Marshal(user.Labels)
			if err != nil {
				return err
			}
			pbUsers[i] = &protocol.User{
				UserId:    user.UserId,
				Labels:    labels,
				Comment:   user.Comment,
				Subscribe: user.Subscribe,
			}
		}
		err := stream.Send(&protocol.GetUserStreamResponse{Users: pbUsers})
		if err != nil {
			return err
		}
	}
	return <-errChan
}

func (p *ProxyServer) GetItemStream(in *protocol.GetItemStreamRequest, stream grpc.ServerStreamingServer[protocol.GetItemStreamResponse]) error {
	var timeLimit *time.Time
	if in.TimeLimit != nil {
		timeLimit = lo.ToPtr(in.TimeLimit.AsTime())
	}
	itemsChan, errChan := p.database.GetItemStream(stream.Context(), int(in.BatchSize), timeLimit)
	for items := range itemsChan {
		pbItems := make([]*protocol.Item, len(items))
		for i, item := range items {
			labels, err := json.Marshal(item.Labels)
			if err != nil {
				return err
			}
			pbItems[i] = &protocol.Item{
				ItemId:     item.ItemId,
				IsHidden:   item.IsHidden,
				Categories: item.Categories,
				Timestamp:  timestamppb.New(item.Timestamp),
				Labels:     labels,
				Comment:    item.Comment,
			}
		}
		err := stream.Send(&protocol.GetItemStreamResponse{Items: pbItems})
		if err != nil {
			return err
		}
	}
	return <-errChan
}

func (p *ProxyServer) GetFeedbackStream(in *protocol.GetFeedbackStreamRequest, stream grpc.ServerStreamingServer[protocol.GetFeedbackStreamResponse]) error {
	var opts []ScanOption
	if in.ScanOptions.BeginTime != nil {
		opts = append(opts, WithBeginTime(in.ScanOptions.BeginTime.AsTime()))
	}
	if in.ScanOptions.EndTime != nil {
		opts = append(opts, WithEndTime(in.ScanOptions.EndTime.AsTime()))
	}
	if in.ScanOptions.FeedbackTypes != nil {
		opts = append(opts, WithFeedbackTypes(in.ScanOptions.FeedbackTypes...))
	}
	if in.ScanOptions.BeginUserId != nil {
		opts = append(opts, WithBeginUserId(*in.ScanOptions.BeginUserId))
	}
	if in.ScanOptions.EndUserId != nil {
		opts = append(opts, WithEndUserId(*in.ScanOptions.EndUserId))
	}
	if in.ScanOptions.BeginItemId != nil {
		opts = append(opts, WithBeginItemId(*in.ScanOptions.BeginItemId))
	}
	if in.ScanOptions.EndItemId != nil {
		opts = append(opts, WithEndItemId(*in.ScanOptions.EndItemId))
	}
	if in.ScanOptions.OrderByItemId {
		opts = append(opts, WithOrderByItemId())
	}
	feedbackChan, errChan := p.database.GetFeedbackStream(stream.Context(), int(in.BatchSize), opts...)
	for feedback := range feedbackChan {
		pbFeedback := make([]*protocol.Feedback, len(feedback))
		for i, f := range feedback {
			pbFeedback[i] = &protocol.Feedback{
				FeedbackType: f.FeedbackType,
				UserId:       f.UserId,
				ItemId:       f.ItemId,
				Value:        f.Value,
				Timestamp:    timestamppb.New(f.Timestamp),
				Comment:      f.Comment,
			}
		}
		err := stream.Send(&protocol.GetFeedbackStreamResponse{Feedback: pbFeedback})
		if err != nil {
			return err
		}
	}
	return <-errChan
}

func (p *ProxyServer) CountUsers(ctx context.Context, in *protocol.CountUsersRequest) (*protocol.CountUsersResponse, error) {
	count, err := p.database.CountUsers(ctx)
	return &protocol.CountUsersResponse{Count: int32(count)}, err
}

func (p *ProxyServer) CountItems(ctx context.Context, in *protocol.CountItemsRequest) (*protocol.CountItemsResponse, error) {
	count, err := p.database.CountItems(ctx)
	return &protocol.CountItemsResponse{Count: int32(count)}, err
}

func (p *ProxyServer) CountFeedback(ctx context.Context, in *protocol.CountFeedbackRequest) (*protocol.CountFeedbackResponse, error) {
	count, err := p.database.CountFeedback(ctx)
	return &protocol.CountFeedbackResponse{Count: int32(count)}, err
}

type ProxyClient struct {
	protocol.DataStoreClient
}

func NewProxyClient(conn *grpc.ClientConn) *ProxyClient {
	return &ProxyClient{
		DataStoreClient: protocol.NewDataStoreClient(conn),
	}
}

func (p ProxyClient) Init() error {
	return errors.MethodNotAllowedf("method Init is not allowed in ProxyClient")
}

func (p ProxyClient) Ping() error {
	_, err := p.DataStoreClient.Ping(context.Background(), &protocol.PingRequest{})
	return err
}

func (p ProxyClient) Close() error {
	return nil
}

func (p ProxyClient) Optimize() error {
	return nil
}

func (p ProxyClient) Purge() error {
	return errors.MethodNotAllowedf("method Purge is not allowed in ProxyClient")
}

func (p ProxyClient) BatchInsertItems(ctx context.Context, items []Item) error {
	pbItems := make([]*protocol.Item, len(items))
	for i, item := range items {
		labels, err := json.Marshal(item.Labels)
		if err != nil {
			return err
		}
		pbItems[i] = &protocol.Item{
			ItemId:     item.ItemId,
			IsHidden:   item.IsHidden,
			Categories: item.Categories,
			Timestamp:  timestamppb.New(item.Timestamp),
			Labels:     labels,
			Comment:    item.Comment,
		}
	}
	_, err := p.DataStoreClient.BatchInsertItems(ctx, &protocol.BatchInsertItemsRequest{Items: pbItems})
	return err
}

func (p ProxyClient) BatchGetItems(ctx context.Context, itemIds []string) ([]Item, error) {
	resp, err := p.DataStoreClient.BatchGetItems(ctx, &protocol.BatchGetItemsRequest{ItemIds: itemIds})
	if err != nil {
		return nil, err
	}
	items := make([]Item, len(resp.Items))
	for i, item := range resp.Items {
		var labels any
		err = json.Unmarshal(item.Labels, &labels)
		if err != nil {
			return nil, err
		}
		items[i] = Item{
			ItemId:     item.ItemId,
			IsHidden:   item.IsHidden,
			Categories: item.Categories,
			Timestamp:  item.Timestamp.AsTime(),
			Labels:     labels,
			Comment:    item.Comment,
		}
	}
	return items, nil
}

func (p ProxyClient) DeleteItem(ctx context.Context, itemId string) error {
	_, err := p.DataStoreClient.DeleteItem(ctx, &protocol.DeleteItemRequest{ItemId: itemId})
	return err
}

func (p ProxyClient) GetItem(ctx context.Context, itemId string) (Item, error) {
	resp, err := p.DataStoreClient.GetItem(ctx, &protocol.GetItemRequest{ItemId: itemId})
	if err != nil {
		return Item{}, err
	}
	if resp.Item == nil {
		return Item{}, errors.Annotate(ErrItemNotExist, itemId)
	}
	var labels any
	if err = json.Unmarshal(resp.Item.Labels, &labels); err != nil {
		return Item{}, err
	}
	return Item{
		ItemId:     resp.Item.ItemId,
		IsHidden:   resp.Item.IsHidden,
		Categories: resp.Item.Categories,
		Timestamp:  resp.Item.Timestamp.AsTime(),
		Labels:     labels,
		Comment:    resp.Item.Comment,
	}, nil
}

func (p ProxyClient) ModifyItem(ctx context.Context, itemId string, patch ItemPatch) error {
	var labels []byte
	if patch.Labels != nil {
		var err error
		labels, err = json.Marshal(patch.Labels)
		if err != nil {
			return err
		}
	}
	var timestamp *timestamppb.Timestamp
	if patch.Timestamp != nil {
		timestamp = timestamppb.New(*patch.Timestamp)
	}
	_, err := p.DataStoreClient.ModifyItem(ctx, &protocol.ModifyItemRequest{
		ItemId: itemId,
		Patch: &protocol.ItemPatch{
			IsHidden:   patch.IsHidden,
			Categories: patch.Categories,
			Labels:     labels,
			Comment:    patch.Comment,
			Timestamp:  timestamp,
		},
	})
	return err
}

func (p ProxyClient) GetItems(ctx context.Context, cursor string, n int, beginTime *time.Time) (string, []Item, error) {
	var beginTimeProto *timestamppb.Timestamp
	if beginTime != nil {
		beginTimeProto = timestamppb.New(*beginTime)
	}
	resp, err := p.DataStoreClient.GetItems(ctx, &protocol.GetItemsRequest{Cursor: cursor, N: int32(n), BeginTime: beginTimeProto})
	if err != nil {
		return "", nil, err
	}
	items := make([]Item, len(resp.Items))
	for i, item := range resp.Items {
		var labels any
		err = json.Unmarshal(item.Labels, &labels)
		if err != nil {
			return "", nil, err
		}
		items[i] = Item{
			ItemId:     item.ItemId,
			IsHidden:   item.IsHidden,
			Categories: item.Categories,
			Timestamp:  item.Timestamp.AsTime(),
			Labels:     labels,
			Comment:    item.Comment,
		}
	}
	return resp.Cursor, items, nil
}

func (p ProxyClient) GetItemFeedback(ctx context.Context, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	resp, err := p.DataStoreClient.GetItemFeedback(ctx, &protocol.GetItemFeedbackRequest{
		ItemId:        itemId,
		FeedbackTypes: feedbackTypes,
	})
	if err != nil {
		return nil, err
	}
	feedback := make([]Feedback, len(resp.Feedback))
	for i, f := range resp.Feedback {
		feedback[i] = Feedback{
			FeedbackKey: FeedbackKey{
				FeedbackType: f.FeedbackType,
				UserId:       f.UserId,
				ItemId:       f.ItemId,
			},
			Value:     f.Value,
			Timestamp: f.Timestamp.AsTime(),
			Comment:   f.Comment,
		}
	}
	return feedback, nil
}

func (p ProxyClient) BatchInsertUsers(ctx context.Context, users []User) error {
	pbUsers := make([]*protocol.User, len(users))
	for i, user := range users {
		labels, err := json.Marshal(user.Labels)
		if err != nil {
			return err
		}
		pbUsers[i] = &protocol.User{
			UserId:    user.UserId,
			Labels:    labels,
			Comment:   user.Comment,
			Subscribe: user.Subscribe,
		}
	}
	_, err := p.DataStoreClient.BatchInsertUsers(ctx, &protocol.BatchInsertUsersRequest{Users: pbUsers})
	return err
}

func (p ProxyClient) DeleteUser(ctx context.Context, userId string) error {
	_, err := p.DataStoreClient.DeleteUser(ctx, &protocol.DeleteUserRequest{UserId: userId})
	return err
}

func (p ProxyClient) GetUser(ctx context.Context, userId string) (User, error) {
	resp, err := p.DataStoreClient.GetUser(ctx, &protocol.GetUserRequest{UserId: userId})
	if err != nil {
		return User{}, err
	}
	if resp.User == nil {
		return User{}, errors.Annotate(ErrUserNotExist, userId)
	}
	var labels any
	if err = json.Unmarshal(resp.User.Labels, &labels); err != nil {
		return User{}, err
	}
	return User{
		UserId:    resp.User.UserId,
		Labels:    labels,
		Comment:   resp.User.Comment,
		Subscribe: resp.User.Subscribe,
	}, nil
}

func (p ProxyClient) ModifyUser(ctx context.Context, userId string, patch UserPatch) error {
	var labels []byte
	if patch.Labels != nil {
		var err error
		labels, err = json.Marshal(patch.Labels)
		if err != nil {
			return err
		}
	}
	_, err := p.DataStoreClient.ModifyUser(ctx, &protocol.ModifyUserRequest{
		UserId: userId,
		Patch: &protocol.UserPatch{
			Labels:    labels,
			Comment:   patch.Comment,
			Subscribe: patch.Subscribe,
		},
	})
	return err
}

func (p ProxyClient) GetUsers(ctx context.Context, cursor string, n int) (string, []User, error) {
	resp, err := p.DataStoreClient.GetUsers(ctx, &protocol.GetUsersRequest{Cursor: cursor, N: int32(n)})
	if err != nil {
		return "", nil, err
	}
	users := make([]User, len(resp.Users))
	for i, user := range resp.Users {
		var labels any
		err = json.Unmarshal(user.Labels, &labels)
		if err != nil {
			return "", nil, err
		}
		users[i] = User{
			UserId:    user.UserId,
			Labels:    labels,
			Comment:   user.Comment,
			Subscribe: user.Subscribe,
		}
	}
	return resp.Cursor, users, nil
}

func (p ProxyClient) GetUserFeedback(ctx context.Context, userId string, endTime *time.Time, feedbackTypes ...string) ([]Feedback, error) {
	req := &protocol.GetUserFeedbackRequest{UserId: userId}
	if endTime != nil {
		req.EndTime = timestamppb.New(*endTime)
	}
	if len(feedbackTypes) > 0 {
		req.FeedbackTypes = feedbackTypes
	}
	resp, err := p.DataStoreClient.GetUserFeedback(ctx, req)
	if err != nil {
		return nil, err
	}
	feedback := make([]Feedback, len(resp.Feedback))
	for i, f := range resp.Feedback {
		feedback[i] = Feedback{
			FeedbackKey: FeedbackKey{
				FeedbackType: f.FeedbackType,
				UserId:       f.UserId,
				ItemId:       f.ItemId,
			},
			Value:     f.Value,
			Timestamp: f.Timestamp.AsTime(),
			Comment:   f.Comment,
		}
	}
	return feedback, nil
}

func (p ProxyClient) GetUserItemFeedback(ctx context.Context, userId, itemId string, feedbackTypes ...string) ([]Feedback, error) {
	resp, err := p.DataStoreClient.GetUserItemFeedback(ctx, &protocol.GetUserItemFeedbackRequest{
		UserId:        userId,
		ItemId:        itemId,
		FeedbackTypes: feedbackTypes,
	})
	if err != nil {
		return nil, err
	}
	feedback := make([]Feedback, len(resp.Feedback))
	for i, f := range resp.Feedback {
		feedback[i] = Feedback{
			FeedbackKey: FeedbackKey{
				FeedbackType: f.FeedbackType,
				UserId:       f.UserId,
				ItemId:       f.ItemId,
			},
			Value:     f.Value,
			Timestamp: f.Timestamp.AsTime(),
			Comment:   f.Comment,
		}
	}
	return feedback, nil
}

func (p ProxyClient) DeleteUserItemFeedback(ctx context.Context, userId, itemId string, feedbackTypes ...string) (int, error) {
	resp, err := p.DataStoreClient.DeleteUserItemFeedback(ctx, &protocol.DeleteUserItemFeedbackRequest{
		UserId:        userId,
		ItemId:        itemId,
		FeedbackTypes: feedbackTypes,
	})
	if err != nil {
		return 0, err
	}
	return int(resp.Count), nil
}

func (p ProxyClient) BatchInsertFeedback(ctx context.Context, feedback []Feedback, insertUser, insertItem, overwrite bool) error {
	reqFeedback := make([]*protocol.Feedback, len(feedback))
	for i, f := range feedback {
		reqFeedback[i] = &protocol.Feedback{
			FeedbackType: f.FeedbackType,
			UserId:       f.UserId,
			ItemId:       f.ItemId,
			Value:        f.Value,
			Timestamp:    timestamppb.New(f.Timestamp),
			Comment:      f.Comment,
		}
	}
	_, err := p.DataStoreClient.BatchInsertFeedback(ctx, &protocol.BatchInsertFeedbackRequest{
		Feedback:   reqFeedback,
		InsertUser: insertUser,
		InsertItem: insertItem,
		Overwrite:  overwrite,
	})
	return err
}

func (p ProxyClient) GetFeedback(ctx context.Context, cursor string, n int, beginTime, endTime *time.Time, feedbackTypes ...string) (string, []Feedback, error) {
	req := &protocol.GetFeedbackRequest{
		Cursor: cursor,
		N:      int32(n),
	}
	if beginTime != nil {
		req.BeginTime = timestamppb.New(*beginTime)
	}
	if endTime != nil {
		req.EndTime = timestamppb.New(*endTime)
	}
	if len(feedbackTypes) > 0 {
		req.FeedbackTypes = feedbackTypes
	}
	resp, err := p.DataStoreClient.GetFeedback(ctx, req)
	if err != nil {
		return "", nil, err
	}
	feedback := make([]Feedback, len(resp.Feedback))
	for i, f := range resp.Feedback {
		feedback[i] = Feedback{
			FeedbackKey: FeedbackKey{
				FeedbackType: f.FeedbackType,
				UserId:       f.UserId,
				ItemId:       f.ItemId,
			},
			Value:     f.Value,
			Timestamp: f.Timestamp.AsTime(),
			Comment:   f.Comment,
		}
	}
	return resp.Cursor, feedback, nil
}

func (p ProxyClient) GetUserStream(ctx context.Context, batchSize int) (chan []User, chan error) {
	usersChan := make(chan []User, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(usersChan)
		defer close(errChan)
		stream, err := p.DataStoreClient.GetUserStream(ctx, &protocol.GetUserStreamRequest{BatchSize: int32(batchSize)})
		if err != nil {
			errChan <- err
			return
		}
		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				errChan <- err
				return
			}
			users := make([]User, len(resp.Users))
			for i, user := range resp.Users {
				var labels any
				if err = json.Unmarshal(user.Labels, &labels); err != nil {
					errChan <- err
					return
				}
				users[i] = User{
					UserId:    user.UserId,
					Labels:    labels,
					Comment:   user.Comment,
					Subscribe: user.Subscribe,
				}
			}
			usersChan <- users
		}
	}()
	return usersChan, errChan
}

func (p ProxyClient) GetItemStream(ctx context.Context, batchSize int, timeLimit *time.Time) (chan []Item, chan error) {
	itemsChan := make(chan []Item, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(itemsChan)
		defer close(errChan)
		stream, err := p.DataStoreClient.GetItemStream(ctx, &protocol.GetItemStreamRequest{BatchSize: int32(batchSize)})
		if err != nil {
			errChan <- err
			return
		}
		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				errChan <- err
				return
			}
			items := make([]Item, len(resp.Items))
			for i, item := range resp.Items {
				var labels any
				if err = json.Unmarshal(item.Labels, &labels); err != nil {
					errChan <- err
					return
				}
				items[i] = Item{
					ItemId:     item.ItemId,
					IsHidden:   item.IsHidden,
					Categories: item.Categories,
					Timestamp:  item.Timestamp.AsTime(),
					Labels:     labels,
					Comment:    item.Comment,
				}
			}
			itemsChan <- items
		}
	}()
	return itemsChan, errChan
}

func (p ProxyClient) GetFeedbackStream(ctx context.Context, batchSize int, options ...ScanOption) (chan []Feedback, chan error) {
	var o ScanOptions
	for _, opt := range options {
		opt(&o)
	}
	pbOptions := &protocol.ScanOptions{
		BeginUserId:   o.BeginUserId,
		EndUserId:     o.EndUserId,
		BeginItemId:   o.BeginItemId,
		EndItemId:     o.EndItemId,
		FeedbackTypes: o.FeedbackTypes,
		OrderByItemId: o.OrderByItemId,
	}
	if o.BeginTime != nil {
		pbOptions.BeginTime = timestamppb.New(*o.BeginTime)
	}
	if o.EndTime != nil {
		pbOptions.EndTime = timestamppb.New(*o.EndTime)
	}

	feedbackChan := make(chan []Feedback, bufSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(feedbackChan)
		defer close(errChan)
		req := &protocol.GetFeedbackStreamRequest{
			BatchSize:   int32(batchSize),
			ScanOptions: pbOptions,
		}

		stream, err := p.DataStoreClient.GetFeedbackStream(ctx, req)
		if err != nil {
			errChan <- err
			return
		}
		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				errChan <- err
				return
			}
			feedback := make([]Feedback, len(resp.Feedback))
			for i, f := range resp.Feedback {
				feedback[i] = Feedback{
					FeedbackKey: FeedbackKey{
						FeedbackType: f.FeedbackType,
						UserId:       f.UserId,
						ItemId:       f.ItemId,
					},
					Value:     f.Value,
					Timestamp: f.Timestamp.AsTime(),
					Comment:   f.Comment,
				}
			}
			feedbackChan <- feedback
		}
	}()
	return feedbackChan, errChan
}

func (p ProxyClient) CountUsers(ctx context.Context) (int, error) {
	resp, err := p.DataStoreClient.CountUsers(ctx, &protocol.CountUsersRequest{})
	if err != nil {
		return 0, err
	}
	return int(resp.Count), nil
}

func (p ProxyClient) CountItems(ctx context.Context) (int, error) {
	resp, err := p.DataStoreClient.CountItems(ctx, &protocol.CountItemsRequest{})
	if err != nil {
		return 0, err
	}
	return int(resp.Count), nil
}

func (p ProxyClient) CountFeedback(ctx context.Context) (int, error) {
	resp, err := p.DataStoreClient.CountFeedback(ctx, &protocol.CountFeedbackRequest{})
	if err != nil {
		return 0, err
	}
	return int(resp.Count), nil
}
