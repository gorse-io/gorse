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

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.28.3
// source: data_store.proto

package protocol

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	DataStore_Ping_FullMethodName                   = "/protocol.DataStore/Ping"
	DataStore_BatchInsertItems_FullMethodName       = "/protocol.DataStore/BatchInsertItems"
	DataStore_BatchGetItems_FullMethodName          = "/protocol.DataStore/BatchGetItems"
	DataStore_DeleteItem_FullMethodName             = "/protocol.DataStore/DeleteItem"
	DataStore_GetItem_FullMethodName                = "/protocol.DataStore/GetItem"
	DataStore_ModifyItem_FullMethodName             = "/protocol.DataStore/ModifyItem"
	DataStore_GetItems_FullMethodName               = "/protocol.DataStore/GetItems"
	DataStore_GetItemFeedback_FullMethodName        = "/protocol.DataStore/GetItemFeedback"
	DataStore_BatchInsertUsers_FullMethodName       = "/protocol.DataStore/BatchInsertUsers"
	DataStore_DeleteUser_FullMethodName             = "/protocol.DataStore/DeleteUser"
	DataStore_GetUser_FullMethodName                = "/protocol.DataStore/GetUser"
	DataStore_ModifyUser_FullMethodName             = "/protocol.DataStore/ModifyUser"
	DataStore_GetUsers_FullMethodName               = "/protocol.DataStore/GetUsers"
	DataStore_GetUserFeedback_FullMethodName        = "/protocol.DataStore/GetUserFeedback"
	DataStore_GetUserItemFeedback_FullMethodName    = "/protocol.DataStore/GetUserItemFeedback"
	DataStore_DeleteUserItemFeedback_FullMethodName = "/protocol.DataStore/DeleteUserItemFeedback"
	DataStore_BatchInsertFeedback_FullMethodName    = "/protocol.DataStore/BatchInsertFeedback"
	DataStore_GetFeedback_FullMethodName            = "/protocol.DataStore/GetFeedback"
	DataStore_GetUserStream_FullMethodName          = "/protocol.DataStore/GetUserStream"
	DataStore_GetItemStream_FullMethodName          = "/protocol.DataStore/GetItemStream"
	DataStore_GetFeedbackStream_FullMethodName      = "/protocol.DataStore/GetFeedbackStream"
)

// DataStoreClient is the client API for DataStore service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type DataStoreClient interface {
	Ping(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error)
	BatchInsertItems(ctx context.Context, in *BatchInsertItemsRequest, opts ...grpc.CallOption) (*BatchInsertItemsResponse, error)
	BatchGetItems(ctx context.Context, in *BatchGetItemsRequest, opts ...grpc.CallOption) (*BatchGetItemsResponse, error)
	DeleteItem(ctx context.Context, in *DeleteItemRequest, opts ...grpc.CallOption) (*DeleteItemResponse, error)
	GetItem(ctx context.Context, in *GetItemRequest, opts ...grpc.CallOption) (*GetItemResponse, error)
	ModifyItem(ctx context.Context, in *ModifyItemRequest, opts ...grpc.CallOption) (*ModifyItemResponse, error)
	GetItems(ctx context.Context, in *GetItemsRequest, opts ...grpc.CallOption) (*GetItemsResponse, error)
	GetItemFeedback(ctx context.Context, in *GetItemFeedbackRequest, opts ...grpc.CallOption) (*GetFeedbackResponse, error)
	BatchInsertUsers(ctx context.Context, in *BatchInsertUsersRequest, opts ...grpc.CallOption) (*BatchInsertUsersResponse, error)
	DeleteUser(ctx context.Context, in *DeleteUserRequest, opts ...grpc.CallOption) (*DeleteUserResponse, error)
	GetUser(ctx context.Context, in *GetUserRequest, opts ...grpc.CallOption) (*GetUserResponse, error)
	ModifyUser(ctx context.Context, in *ModifyUserRequest, opts ...grpc.CallOption) (*ModifyUserResponse, error)
	GetUsers(ctx context.Context, in *GetUsersRequest, opts ...grpc.CallOption) (*GetUsersResponse, error)
	GetUserFeedback(ctx context.Context, in *GetUserFeedbackRequest, opts ...grpc.CallOption) (*GetFeedbackResponse, error)
	GetUserItemFeedback(ctx context.Context, in *GetUserItemFeedbackRequest, opts ...grpc.CallOption) (*GetFeedbackResponse, error)
	DeleteUserItemFeedback(ctx context.Context, in *DeleteUserItemFeedbackRequest, opts ...grpc.CallOption) (*DeleteUserItemFeedbackResponse, error)
	BatchInsertFeedback(ctx context.Context, in *BatchInsertFeedbackRequest, opts ...grpc.CallOption) (*BatchInsertFeedbackResponse, error)
	GetFeedback(ctx context.Context, in *GetFeedbackRequest, opts ...grpc.CallOption) (*GetFeedbackResponse, error)
	GetUserStream(ctx context.Context, in *GetUserStreamRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[GetUserStreamResponse], error)
	GetItemStream(ctx context.Context, in *GetItemStreamRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[GetItemStreamResponse], error)
	GetFeedbackStream(ctx context.Context, in *GetFeedbackStreamRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[GetFeedbackStreamResponse], error)
}

type dataStoreClient struct {
	cc grpc.ClientConnInterface
}

func NewDataStoreClient(cc grpc.ClientConnInterface) DataStoreClient {
	return &dataStoreClient{cc}
}

func (c *dataStoreClient) Ping(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(PingResponse)
	err := c.cc.Invoke(ctx, DataStore_Ping_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) BatchInsertItems(ctx context.Context, in *BatchInsertItemsRequest, opts ...grpc.CallOption) (*BatchInsertItemsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(BatchInsertItemsResponse)
	err := c.cc.Invoke(ctx, DataStore_BatchInsertItems_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) BatchGetItems(ctx context.Context, in *BatchGetItemsRequest, opts ...grpc.CallOption) (*BatchGetItemsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(BatchGetItemsResponse)
	err := c.cc.Invoke(ctx, DataStore_BatchGetItems_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) DeleteItem(ctx context.Context, in *DeleteItemRequest, opts ...grpc.CallOption) (*DeleteItemResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteItemResponse)
	err := c.cc.Invoke(ctx, DataStore_DeleteItem_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) GetItem(ctx context.Context, in *GetItemRequest, opts ...grpc.CallOption) (*GetItemResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetItemResponse)
	err := c.cc.Invoke(ctx, DataStore_GetItem_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) ModifyItem(ctx context.Context, in *ModifyItemRequest, opts ...grpc.CallOption) (*ModifyItemResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ModifyItemResponse)
	err := c.cc.Invoke(ctx, DataStore_ModifyItem_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) GetItems(ctx context.Context, in *GetItemsRequest, opts ...grpc.CallOption) (*GetItemsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetItemsResponse)
	err := c.cc.Invoke(ctx, DataStore_GetItems_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) GetItemFeedback(ctx context.Context, in *GetItemFeedbackRequest, opts ...grpc.CallOption) (*GetFeedbackResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetFeedbackResponse)
	err := c.cc.Invoke(ctx, DataStore_GetItemFeedback_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) BatchInsertUsers(ctx context.Context, in *BatchInsertUsersRequest, opts ...grpc.CallOption) (*BatchInsertUsersResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(BatchInsertUsersResponse)
	err := c.cc.Invoke(ctx, DataStore_BatchInsertUsers_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) DeleteUser(ctx context.Context, in *DeleteUserRequest, opts ...grpc.CallOption) (*DeleteUserResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteUserResponse)
	err := c.cc.Invoke(ctx, DataStore_DeleteUser_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) GetUser(ctx context.Context, in *GetUserRequest, opts ...grpc.CallOption) (*GetUserResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetUserResponse)
	err := c.cc.Invoke(ctx, DataStore_GetUser_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) ModifyUser(ctx context.Context, in *ModifyUserRequest, opts ...grpc.CallOption) (*ModifyUserResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ModifyUserResponse)
	err := c.cc.Invoke(ctx, DataStore_ModifyUser_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) GetUsers(ctx context.Context, in *GetUsersRequest, opts ...grpc.CallOption) (*GetUsersResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetUsersResponse)
	err := c.cc.Invoke(ctx, DataStore_GetUsers_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) GetUserFeedback(ctx context.Context, in *GetUserFeedbackRequest, opts ...grpc.CallOption) (*GetFeedbackResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetFeedbackResponse)
	err := c.cc.Invoke(ctx, DataStore_GetUserFeedback_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) GetUserItemFeedback(ctx context.Context, in *GetUserItemFeedbackRequest, opts ...grpc.CallOption) (*GetFeedbackResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetFeedbackResponse)
	err := c.cc.Invoke(ctx, DataStore_GetUserItemFeedback_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) DeleteUserItemFeedback(ctx context.Context, in *DeleteUserItemFeedbackRequest, opts ...grpc.CallOption) (*DeleteUserItemFeedbackResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteUserItemFeedbackResponse)
	err := c.cc.Invoke(ctx, DataStore_DeleteUserItemFeedback_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) BatchInsertFeedback(ctx context.Context, in *BatchInsertFeedbackRequest, opts ...grpc.CallOption) (*BatchInsertFeedbackResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(BatchInsertFeedbackResponse)
	err := c.cc.Invoke(ctx, DataStore_BatchInsertFeedback_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) GetFeedback(ctx context.Context, in *GetFeedbackRequest, opts ...grpc.CallOption) (*GetFeedbackResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetFeedbackResponse)
	err := c.cc.Invoke(ctx, DataStore_GetFeedback_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataStoreClient) GetUserStream(ctx context.Context, in *GetUserStreamRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[GetUserStreamResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &DataStore_ServiceDesc.Streams[0], DataStore_GetUserStream_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[GetUserStreamRequest, GetUserStreamResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type DataStore_GetUserStreamClient = grpc.ServerStreamingClient[GetUserStreamResponse]

func (c *dataStoreClient) GetItemStream(ctx context.Context, in *GetItemStreamRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[GetItemStreamResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &DataStore_ServiceDesc.Streams[1], DataStore_GetItemStream_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[GetItemStreamRequest, GetItemStreamResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type DataStore_GetItemStreamClient = grpc.ServerStreamingClient[GetItemStreamResponse]

func (c *dataStoreClient) GetFeedbackStream(ctx context.Context, in *GetFeedbackStreamRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[GetFeedbackStreamResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &DataStore_ServiceDesc.Streams[2], DataStore_GetFeedbackStream_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[GetFeedbackStreamRequest, GetFeedbackStreamResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type DataStore_GetFeedbackStreamClient = grpc.ServerStreamingClient[GetFeedbackStreamResponse]

// DataStoreServer is the server API for DataStore service.
// All implementations must embed UnimplementedDataStoreServer
// for forward compatibility.
type DataStoreServer interface {
	Ping(context.Context, *PingRequest) (*PingResponse, error)
	BatchInsertItems(context.Context, *BatchInsertItemsRequest) (*BatchInsertItemsResponse, error)
	BatchGetItems(context.Context, *BatchGetItemsRequest) (*BatchGetItemsResponse, error)
	DeleteItem(context.Context, *DeleteItemRequest) (*DeleteItemResponse, error)
	GetItem(context.Context, *GetItemRequest) (*GetItemResponse, error)
	ModifyItem(context.Context, *ModifyItemRequest) (*ModifyItemResponse, error)
	GetItems(context.Context, *GetItemsRequest) (*GetItemsResponse, error)
	GetItemFeedback(context.Context, *GetItemFeedbackRequest) (*GetFeedbackResponse, error)
	BatchInsertUsers(context.Context, *BatchInsertUsersRequest) (*BatchInsertUsersResponse, error)
	DeleteUser(context.Context, *DeleteUserRequest) (*DeleteUserResponse, error)
	GetUser(context.Context, *GetUserRequest) (*GetUserResponse, error)
	ModifyUser(context.Context, *ModifyUserRequest) (*ModifyUserResponse, error)
	GetUsers(context.Context, *GetUsersRequest) (*GetUsersResponse, error)
	GetUserFeedback(context.Context, *GetUserFeedbackRequest) (*GetFeedbackResponse, error)
	GetUserItemFeedback(context.Context, *GetUserItemFeedbackRequest) (*GetFeedbackResponse, error)
	DeleteUserItemFeedback(context.Context, *DeleteUserItemFeedbackRequest) (*DeleteUserItemFeedbackResponse, error)
	BatchInsertFeedback(context.Context, *BatchInsertFeedbackRequest) (*BatchInsertFeedbackResponse, error)
	GetFeedback(context.Context, *GetFeedbackRequest) (*GetFeedbackResponse, error)
	GetUserStream(*GetUserStreamRequest, grpc.ServerStreamingServer[GetUserStreamResponse]) error
	GetItemStream(*GetItemStreamRequest, grpc.ServerStreamingServer[GetItemStreamResponse]) error
	GetFeedbackStream(*GetFeedbackStreamRequest, grpc.ServerStreamingServer[GetFeedbackStreamResponse]) error
	mustEmbedUnimplementedDataStoreServer()
}

// UnimplementedDataStoreServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedDataStoreServer struct{}

func (UnimplementedDataStoreServer) Ping(context.Context, *PingRequest) (*PingResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Ping not implemented")
}
func (UnimplementedDataStoreServer) BatchInsertItems(context.Context, *BatchInsertItemsRequest) (*BatchInsertItemsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method BatchInsertItems not implemented")
}
func (UnimplementedDataStoreServer) BatchGetItems(context.Context, *BatchGetItemsRequest) (*BatchGetItemsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method BatchGetItems not implemented")
}
func (UnimplementedDataStoreServer) DeleteItem(context.Context, *DeleteItemRequest) (*DeleteItemResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteItem not implemented")
}
func (UnimplementedDataStoreServer) GetItem(context.Context, *GetItemRequest) (*GetItemResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetItem not implemented")
}
func (UnimplementedDataStoreServer) ModifyItem(context.Context, *ModifyItemRequest) (*ModifyItemResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ModifyItem not implemented")
}
func (UnimplementedDataStoreServer) GetItems(context.Context, *GetItemsRequest) (*GetItemsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetItems not implemented")
}
func (UnimplementedDataStoreServer) GetItemFeedback(context.Context, *GetItemFeedbackRequest) (*GetFeedbackResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetItemFeedback not implemented")
}
func (UnimplementedDataStoreServer) BatchInsertUsers(context.Context, *BatchInsertUsersRequest) (*BatchInsertUsersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method BatchInsertUsers not implemented")
}
func (UnimplementedDataStoreServer) DeleteUser(context.Context, *DeleteUserRequest) (*DeleteUserResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteUser not implemented")
}
func (UnimplementedDataStoreServer) GetUser(context.Context, *GetUserRequest) (*GetUserResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetUser not implemented")
}
func (UnimplementedDataStoreServer) ModifyUser(context.Context, *ModifyUserRequest) (*ModifyUserResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ModifyUser not implemented")
}
func (UnimplementedDataStoreServer) GetUsers(context.Context, *GetUsersRequest) (*GetUsersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetUsers not implemented")
}
func (UnimplementedDataStoreServer) GetUserFeedback(context.Context, *GetUserFeedbackRequest) (*GetFeedbackResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetUserFeedback not implemented")
}
func (UnimplementedDataStoreServer) GetUserItemFeedback(context.Context, *GetUserItemFeedbackRequest) (*GetFeedbackResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetUserItemFeedback not implemented")
}
func (UnimplementedDataStoreServer) DeleteUserItemFeedback(context.Context, *DeleteUserItemFeedbackRequest) (*DeleteUserItemFeedbackResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteUserItemFeedback not implemented")
}
func (UnimplementedDataStoreServer) BatchInsertFeedback(context.Context, *BatchInsertFeedbackRequest) (*BatchInsertFeedbackResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method BatchInsertFeedback not implemented")
}
func (UnimplementedDataStoreServer) GetFeedback(context.Context, *GetFeedbackRequest) (*GetFeedbackResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetFeedback not implemented")
}
func (UnimplementedDataStoreServer) GetUserStream(*GetUserStreamRequest, grpc.ServerStreamingServer[GetUserStreamResponse]) error {
	return status.Errorf(codes.Unimplemented, "method GetUserStream not implemented")
}
func (UnimplementedDataStoreServer) GetItemStream(*GetItemStreamRequest, grpc.ServerStreamingServer[GetItemStreamResponse]) error {
	return status.Errorf(codes.Unimplemented, "method GetItemStream not implemented")
}
func (UnimplementedDataStoreServer) GetFeedbackStream(*GetFeedbackStreamRequest, grpc.ServerStreamingServer[GetFeedbackStreamResponse]) error {
	return status.Errorf(codes.Unimplemented, "method GetFeedbackStream not implemented")
}
func (UnimplementedDataStoreServer) mustEmbedUnimplementedDataStoreServer() {}
func (UnimplementedDataStoreServer) testEmbeddedByValue()                   {}

// UnsafeDataStoreServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to DataStoreServer will
// result in compilation errors.
type UnsafeDataStoreServer interface {
	mustEmbedUnimplementedDataStoreServer()
}

func RegisterDataStoreServer(s grpc.ServiceRegistrar, srv DataStoreServer) {
	// If the following call pancis, it indicates UnimplementedDataStoreServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&DataStore_ServiceDesc, srv)
}

func _DataStore_Ping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).Ping(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_Ping_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).Ping(ctx, req.(*PingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_BatchInsertItems_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BatchInsertItemsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).BatchInsertItems(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_BatchInsertItems_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).BatchInsertItems(ctx, req.(*BatchInsertItemsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_BatchGetItems_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BatchGetItemsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).BatchGetItems(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_BatchGetItems_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).BatchGetItems(ctx, req.(*BatchGetItemsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_DeleteItem_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteItemRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).DeleteItem(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_DeleteItem_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).DeleteItem(ctx, req.(*DeleteItemRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_GetItem_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetItemRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).GetItem(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_GetItem_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).GetItem(ctx, req.(*GetItemRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_ModifyItem_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ModifyItemRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).ModifyItem(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_ModifyItem_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).ModifyItem(ctx, req.(*ModifyItemRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_GetItems_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetItemsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).GetItems(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_GetItems_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).GetItems(ctx, req.(*GetItemsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_GetItemFeedback_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetItemFeedbackRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).GetItemFeedback(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_GetItemFeedback_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).GetItemFeedback(ctx, req.(*GetItemFeedbackRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_BatchInsertUsers_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BatchInsertUsersRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).BatchInsertUsers(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_BatchInsertUsers_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).BatchInsertUsers(ctx, req.(*BatchInsertUsersRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_DeleteUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).DeleteUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_DeleteUser_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).DeleteUser(ctx, req.(*DeleteUserRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_GetUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).GetUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_GetUser_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).GetUser(ctx, req.(*GetUserRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_ModifyUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ModifyUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).ModifyUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_ModifyUser_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).ModifyUser(ctx, req.(*ModifyUserRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_GetUsers_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetUsersRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).GetUsers(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_GetUsers_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).GetUsers(ctx, req.(*GetUsersRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_GetUserFeedback_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetUserFeedbackRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).GetUserFeedback(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_GetUserFeedback_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).GetUserFeedback(ctx, req.(*GetUserFeedbackRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_GetUserItemFeedback_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetUserItemFeedbackRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).GetUserItemFeedback(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_GetUserItemFeedback_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).GetUserItemFeedback(ctx, req.(*GetUserItemFeedbackRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_DeleteUserItemFeedback_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteUserItemFeedbackRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).DeleteUserItemFeedback(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_DeleteUserItemFeedback_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).DeleteUserItemFeedback(ctx, req.(*DeleteUserItemFeedbackRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_BatchInsertFeedback_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BatchInsertFeedbackRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).BatchInsertFeedback(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_BatchInsertFeedback_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).BatchInsertFeedback(ctx, req.(*BatchInsertFeedbackRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_GetFeedback_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetFeedbackRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataStoreServer).GetFeedback(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DataStore_GetFeedback_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataStoreServer).GetFeedback(ctx, req.(*GetFeedbackRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataStore_GetUserStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(GetUserStreamRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(DataStoreServer).GetUserStream(m, &grpc.GenericServerStream[GetUserStreamRequest, GetUserStreamResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type DataStore_GetUserStreamServer = grpc.ServerStreamingServer[GetUserStreamResponse]

func _DataStore_GetItemStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(GetItemStreamRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(DataStoreServer).GetItemStream(m, &grpc.GenericServerStream[GetItemStreamRequest, GetItemStreamResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type DataStore_GetItemStreamServer = grpc.ServerStreamingServer[GetItemStreamResponse]

func _DataStore_GetFeedbackStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(GetFeedbackStreamRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(DataStoreServer).GetFeedbackStream(m, &grpc.GenericServerStream[GetFeedbackStreamRequest, GetFeedbackStreamResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type DataStore_GetFeedbackStreamServer = grpc.ServerStreamingServer[GetFeedbackStreamResponse]

// DataStore_ServiceDesc is the grpc.ServiceDesc for DataStore service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var DataStore_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "protocol.DataStore",
	HandlerType: (*DataStoreServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler:    _DataStore_Ping_Handler,
		},
		{
			MethodName: "BatchInsertItems",
			Handler:    _DataStore_BatchInsertItems_Handler,
		},
		{
			MethodName: "BatchGetItems",
			Handler:    _DataStore_BatchGetItems_Handler,
		},
		{
			MethodName: "DeleteItem",
			Handler:    _DataStore_DeleteItem_Handler,
		},
		{
			MethodName: "GetItem",
			Handler:    _DataStore_GetItem_Handler,
		},
		{
			MethodName: "ModifyItem",
			Handler:    _DataStore_ModifyItem_Handler,
		},
		{
			MethodName: "GetItems",
			Handler:    _DataStore_GetItems_Handler,
		},
		{
			MethodName: "GetItemFeedback",
			Handler:    _DataStore_GetItemFeedback_Handler,
		},
		{
			MethodName: "BatchInsertUsers",
			Handler:    _DataStore_BatchInsertUsers_Handler,
		},
		{
			MethodName: "DeleteUser",
			Handler:    _DataStore_DeleteUser_Handler,
		},
		{
			MethodName: "GetUser",
			Handler:    _DataStore_GetUser_Handler,
		},
		{
			MethodName: "ModifyUser",
			Handler:    _DataStore_ModifyUser_Handler,
		},
		{
			MethodName: "GetUsers",
			Handler:    _DataStore_GetUsers_Handler,
		},
		{
			MethodName: "GetUserFeedback",
			Handler:    _DataStore_GetUserFeedback_Handler,
		},
		{
			MethodName: "GetUserItemFeedback",
			Handler:    _DataStore_GetUserItemFeedback_Handler,
		},
		{
			MethodName: "DeleteUserItemFeedback",
			Handler:    _DataStore_DeleteUserItemFeedback_Handler,
		},
		{
			MethodName: "BatchInsertFeedback",
			Handler:    _DataStore_BatchInsertFeedback_Handler,
		},
		{
			MethodName: "GetFeedback",
			Handler:    _DataStore_GetFeedback_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "GetUserStream",
			Handler:       _DataStore_GetUserStream_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "GetItemStream",
			Handler:       _DataStore_GetItemStream_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "GetFeedbackStream",
			Handler:       _DataStore_GetFeedbackStream_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "data_store.proto",
}
