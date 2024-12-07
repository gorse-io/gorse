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

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.1
// 	protoc        v5.28.3
// source: protocol.proto

package protocol

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type NodeType int32

const (
	NodeType_ServerNode NodeType = 0
	NodeType_WorkerNode NodeType = 1
	NodeType_ClientNode NodeType = 2
)

// Enum value maps for NodeType.
var (
	NodeType_name = map[int32]string{
		0: "ServerNode",
		1: "WorkerNode",
		2: "ClientNode",
	}
	NodeType_value = map[string]int32{
		"ServerNode": 0,
		"WorkerNode": 1,
		"ClientNode": 2,
	}
)

func (x NodeType) Enum() *NodeType {
	p := new(NodeType)
	*p = x
	return p
}

func (x NodeType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (NodeType) Descriptor() protoreflect.EnumDescriptor {
	return file_protocol_proto_enumTypes[0].Descriptor()
}

func (NodeType) Type() protoreflect.EnumType {
	return &file_protocol_proto_enumTypes[0]
}

func (x NodeType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use NodeType.Descriptor instead.
func (NodeType) EnumDescriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{0}
}

type User struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	UserId    string   `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	Labels    []byte   `protobuf:"bytes,2,opt,name=labels,proto3" json:"labels,omitempty"`
	Comment   string   `protobuf:"bytes,3,opt,name=comment,proto3" json:"comment,omitempty"`
	Subscribe []string `protobuf:"bytes,4,rep,name=subscribe,proto3" json:"subscribe,omitempty"`
}

func (x *User) Reset() {
	*x = User{}
	mi := &file_protocol_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *User) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*User) ProtoMessage() {}

func (x *User) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use User.ProtoReflect.Descriptor instead.
func (*User) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{0}
}

func (x *User) GetUserId() string {
	if x != nil {
		return x.UserId
	}
	return ""
}

func (x *User) GetLabels() []byte {
	if x != nil {
		return x.Labels
	}
	return nil
}

func (x *User) GetComment() string {
	if x != nil {
		return x.Comment
	}
	return ""
}

func (x *User) GetSubscribe() []string {
	if x != nil {
		return x.Subscribe
	}
	return nil
}

type Item struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Namespace  string                 `protobuf:"bytes,1,opt,name=namespace,proto3" json:"namespace,omitempty"`
	ItemId     string                 `protobuf:"bytes,2,opt,name=item_id,json=itemId,proto3" json:"item_id,omitempty"`
	IsHidden   bool                   `protobuf:"varint,3,opt,name=is_hidden,json=isHidden,proto3" json:"is_hidden,omitempty"`
	Categories []string               `protobuf:"bytes,4,rep,name=categories,proto3" json:"categories,omitempty"`
	Timestamp  *timestamppb.Timestamp `protobuf:"bytes,5,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	Labels     []byte                 `protobuf:"bytes,6,opt,name=labels,proto3" json:"labels,omitempty"`
	Comment    string                 `protobuf:"bytes,7,opt,name=comment,proto3" json:"comment,omitempty"`
}

func (x *Item) Reset() {
	*x = Item{}
	mi := &file_protocol_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Item) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Item) ProtoMessage() {}

func (x *Item) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Item.ProtoReflect.Descriptor instead.
func (*Item) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{1}
}

func (x *Item) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *Item) GetItemId() string {
	if x != nil {
		return x.ItemId
	}
	return ""
}

func (x *Item) GetIsHidden() bool {
	if x != nil {
		return x.IsHidden
	}
	return false
}

func (x *Item) GetCategories() []string {
	if x != nil {
		return x.Categories
	}
	return nil
}

func (x *Item) GetTimestamp() *timestamppb.Timestamp {
	if x != nil {
		return x.Timestamp
	}
	return nil
}

func (x *Item) GetLabels() []byte {
	if x != nil {
		return x.Labels
	}
	return nil
}

func (x *Item) GetComment() string {
	if x != nil {
		return x.Comment
	}
	return ""
}

type Feedback struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Namespace    string                 `protobuf:"bytes,1,opt,name=namespace,proto3" json:"namespace,omitempty"`
	FeedbackType string                 `protobuf:"bytes,2,opt,name=feedback_type,json=feedbackType,proto3" json:"feedback_type,omitempty"`
	UserId       string                 `protobuf:"bytes,3,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	ItemId       string                 `protobuf:"bytes,4,opt,name=item_id,json=itemId,proto3" json:"item_id,omitempty"`
	Timestamp    *timestamppb.Timestamp `protobuf:"bytes,5,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	Comment      string                 `protobuf:"bytes,6,opt,name=comment,proto3" json:"comment,omitempty"`
}

func (x *Feedback) Reset() {
	*x = Feedback{}
	mi := &file_protocol_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Feedback) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Feedback) ProtoMessage() {}

func (x *Feedback) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Feedback.ProtoReflect.Descriptor instead.
func (*Feedback) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{2}
}

func (x *Feedback) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *Feedback) GetFeedbackType() string {
	if x != nil {
		return x.FeedbackType
	}
	return ""
}

func (x *Feedback) GetUserId() string {
	if x != nil {
		return x.UserId
	}
	return ""
}

func (x *Feedback) GetItemId() string {
	if x != nil {
		return x.ItemId
	}
	return ""
}

func (x *Feedback) GetTimestamp() *timestamppb.Timestamp {
	if x != nil {
		return x.Timestamp
	}
	return nil
}

func (x *Feedback) GetComment() string {
	if x != nil {
		return x.Comment
	}
	return ""
}

type Meta struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Config              string   `protobuf:"bytes,1,opt,name=config,proto3" json:"config,omitempty"`
	RankingModelVersion int64    `protobuf:"varint,3,opt,name=ranking_model_version,json=rankingModelVersion,proto3" json:"ranking_model_version,omitempty"`
	ClickModelVersion   int64    `protobuf:"varint,4,opt,name=click_model_version,json=clickModelVersion,proto3" json:"click_model_version,omitempty"`
	Me                  string   `protobuf:"bytes,5,opt,name=me,proto3" json:"me,omitempty"`
	Servers             []string `protobuf:"bytes,6,rep,name=servers,proto3" json:"servers,omitempty"`
	Workers             []string `protobuf:"bytes,7,rep,name=workers,proto3" json:"workers,omitempty"`
}

func (x *Meta) Reset() {
	*x = Meta{}
	mi := &file_protocol_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Meta) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Meta) ProtoMessage() {}

func (x *Meta) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Meta.ProtoReflect.Descriptor instead.
func (*Meta) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{3}
}

func (x *Meta) GetConfig() string {
	if x != nil {
		return x.Config
	}
	return ""
}

func (x *Meta) GetRankingModelVersion() int64 {
	if x != nil {
		return x.RankingModelVersion
	}
	return 0
}

func (x *Meta) GetClickModelVersion() int64 {
	if x != nil {
		return x.ClickModelVersion
	}
	return 0
}

func (x *Meta) GetMe() string {
	if x != nil {
		return x.Me
	}
	return ""
}

func (x *Meta) GetServers() []string {
	if x != nil {
		return x.Servers
	}
	return nil
}

func (x *Meta) GetWorkers() []string {
	if x != nil {
		return x.Workers
	}
	return nil
}

type Fragment struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Data []byte `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *Fragment) Reset() {
	*x = Fragment{}
	mi := &file_protocol_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Fragment) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Fragment) ProtoMessage() {}

func (x *Fragment) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Fragment.ProtoReflect.Descriptor instead.
func (*Fragment) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{4}
}

func (x *Fragment) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type VersionInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Version int64 `protobuf:"varint,1,opt,name=version,proto3" json:"version,omitempty"`
}

func (x *VersionInfo) Reset() {
	*x = VersionInfo{}
	mi := &file_protocol_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *VersionInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VersionInfo) ProtoMessage() {}

func (x *VersionInfo) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VersionInfo.ProtoReflect.Descriptor instead.
func (*VersionInfo) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{5}
}

func (x *VersionInfo) GetVersion() int64 {
	if x != nil {
		return x.Version
	}
	return 0
}

type NodeInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	NodeType      NodeType `protobuf:"varint,1,opt,name=node_type,json=nodeType,proto3,enum=protocol.NodeType" json:"node_type,omitempty"`
	Uuid          string   `protobuf:"bytes,2,opt,name=uuid,proto3" json:"uuid,omitempty"`
	BinaryVersion string   `protobuf:"bytes,4,opt,name=binary_version,json=binaryVersion,proto3" json:"binary_version,omitempty"`
	Hostname      string   `protobuf:"bytes,5,opt,name=hostname,proto3" json:"hostname,omitempty"`
}

func (x *NodeInfo) Reset() {
	*x = NodeInfo{}
	mi := &file_protocol_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *NodeInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NodeInfo) ProtoMessage() {}

func (x *NodeInfo) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NodeInfo.ProtoReflect.Descriptor instead.
func (*NodeInfo) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{6}
}

func (x *NodeInfo) GetNodeType() NodeType {
	if x != nil {
		return x.NodeType
	}
	return NodeType_ServerNode
}

func (x *NodeInfo) GetUuid() string {
	if x != nil {
		return x.Uuid
	}
	return ""
}

func (x *NodeInfo) GetBinaryVersion() string {
	if x != nil {
		return x.BinaryVersion
	}
	return ""
}

func (x *NodeInfo) GetHostname() string {
	if x != nil {
		return x.Hostname
	}
	return ""
}

type Progress struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Tracer     string `protobuf:"bytes,1,opt,name=tracer,proto3" json:"tracer,omitempty"`
	Name       string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Status     string `protobuf:"bytes,3,opt,name=status,proto3" json:"status,omitempty"`
	Error      string `protobuf:"bytes,4,opt,name=error,proto3" json:"error,omitempty"`
	Count      int64  `protobuf:"varint,5,opt,name=count,proto3" json:"count,omitempty"`
	Total      int64  `protobuf:"varint,6,opt,name=total,proto3" json:"total,omitempty"`
	StartTime  int64  `protobuf:"varint,7,opt,name=start_time,json=startTime,proto3" json:"start_time,omitempty"`
	FinishTime int64  `protobuf:"varint,8,opt,name=finish_time,json=finishTime,proto3" json:"finish_time,omitempty"`
}

func (x *Progress) Reset() {
	*x = Progress{}
	mi := &file_protocol_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Progress) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Progress) ProtoMessage() {}

func (x *Progress) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Progress.ProtoReflect.Descriptor instead.
func (*Progress) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{7}
}

func (x *Progress) GetTracer() string {
	if x != nil {
		return x.Tracer
	}
	return ""
}

func (x *Progress) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Progress) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

func (x *Progress) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

func (x *Progress) GetCount() int64 {
	if x != nil {
		return x.Count
	}
	return 0
}

func (x *Progress) GetTotal() int64 {
	if x != nil {
		return x.Total
	}
	return 0
}

func (x *Progress) GetStartTime() int64 {
	if x != nil {
		return x.StartTime
	}
	return 0
}

func (x *Progress) GetFinishTime() int64 {
	if x != nil {
		return x.FinishTime
	}
	return 0
}

type PushProgressRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Progress []*Progress `protobuf:"bytes,1,rep,name=progress,proto3" json:"progress,omitempty"`
}

func (x *PushProgressRequest) Reset() {
	*x = PushProgressRequest{}
	mi := &file_protocol_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PushProgressRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PushProgressRequest) ProtoMessage() {}

func (x *PushProgressRequest) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PushProgressRequest.ProtoReflect.Descriptor instead.
func (*PushProgressRequest) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{8}
}

func (x *PushProgressRequest) GetProgress() []*Progress {
	if x != nil {
		return x.Progress
	}
	return nil
}

type PushProgressResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *PushProgressResponse) Reset() {
	*x = PushProgressResponse{}
	mi := &file_protocol_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PushProgressResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PushProgressResponse) ProtoMessage() {}

func (x *PushProgressResponse) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PushProgressResponse.ProtoReflect.Descriptor instead.
func (*PushProgressResponse) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{9}
}

type PingRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *PingRequest) Reset() {
	*x = PingRequest{}
	mi := &file_protocol_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PingRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PingRequest) ProtoMessage() {}

func (x *PingRequest) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PingRequest.ProtoReflect.Descriptor instead.
func (*PingRequest) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{10}
}

type PingResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *PingResponse) Reset() {
	*x = PingResponse{}
	mi := &file_protocol_proto_msgTypes[11]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PingResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PingResponse) ProtoMessage() {}

func (x *PingResponse) ProtoReflect() protoreflect.Message {
	mi := &file_protocol_proto_msgTypes[11]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PingResponse.ProtoReflect.Descriptor instead.
func (*PingResponse) Descriptor() ([]byte, []int) {
	return file_protocol_proto_rawDescGZIP(), []int{11}
}

var File_protocol_proto protoreflect.FileDescriptor

var file_protocol_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x08, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65,
	0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x6f, 0x0a, 0x04, 0x55,
	0x73, 0x65, 0x72, 0x12, 0x17, 0x0a, 0x07, 0x75, 0x73, 0x65, 0x72, 0x5f, 0x69, 0x64, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x75, 0x73, 0x65, 0x72, 0x49, 0x64, 0x12, 0x16, 0x0a, 0x06,
	0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x06, 0x6c, 0x61,
	0x62, 0x65, 0x6c, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x74, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x74, 0x12, 0x1c,
	0x0a, 0x09, 0x73, 0x75, 0x62, 0x73, 0x63, 0x72, 0x69, 0x62, 0x65, 0x18, 0x04, 0x20, 0x03, 0x28,
	0x09, 0x52, 0x09, 0x73, 0x75, 0x62, 0x73, 0x63, 0x72, 0x69, 0x62, 0x65, 0x22, 0xe6, 0x01, 0x0a,
	0x04, 0x49, 0x74, 0x65, 0x6d, 0x12, 0x1c, 0x0a, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61,
	0x63, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70,
	0x61, 0x63, 0x65, 0x12, 0x17, 0x0a, 0x07, 0x69, 0x74, 0x65, 0x6d, 0x5f, 0x69, 0x64, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x69, 0x74, 0x65, 0x6d, 0x49, 0x64, 0x12, 0x1b, 0x0a, 0x09,
	0x69, 0x73, 0x5f, 0x68, 0x69, 0x64, 0x64, 0x65, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52,
	0x08, 0x69, 0x73, 0x48, 0x69, 0x64, 0x64, 0x65, 0x6e, 0x12, 0x1e, 0x0a, 0x0a, 0x63, 0x61, 0x74,
	0x65, 0x67, 0x6f, 0x72, 0x69, 0x65, 0x73, 0x18, 0x04, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0a, 0x63,
	0x61, 0x74, 0x65, 0x67, 0x6f, 0x72, 0x69, 0x65, 0x73, 0x12, 0x38, 0x0a, 0x09, 0x74, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54,
	0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74,
	0x61, 0x6d, 0x70, 0x12, 0x16, 0x0a, 0x06, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x18, 0x06, 0x20,
	0x01, 0x28, 0x0c, 0x52, 0x06, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x63,
	0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x74, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x63, 0x6f,
	0x6d, 0x6d, 0x65, 0x6e, 0x74, 0x22, 0xd3, 0x01, 0x0a, 0x08, 0x46, 0x65, 0x65, 0x64, 0x62, 0x61,
	0x63, 0x6b, 0x12, 0x1c, 0x0a, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65,
	0x12, 0x23, 0x0a, 0x0d, 0x66, 0x65, 0x65, 0x64, 0x62, 0x61, 0x63, 0x6b, 0x5f, 0x74, 0x79, 0x70,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x66, 0x65, 0x65, 0x64, 0x62, 0x61, 0x63,
	0x6b, 0x54, 0x79, 0x70, 0x65, 0x12, 0x17, 0x0a, 0x07, 0x75, 0x73, 0x65, 0x72, 0x5f, 0x69, 0x64,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x75, 0x73, 0x65, 0x72, 0x49, 0x64, 0x12, 0x17,
	0x0a, 0x07, 0x69, 0x74, 0x65, 0x6d, 0x5f, 0x69, 0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x06, 0x69, 0x74, 0x65, 0x6d, 0x49, 0x64, 0x12, 0x38, 0x0a, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73,
	0x74, 0x61, 0x6d, 0x70, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d,
	0x70, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x74, 0x18, 0x06, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x07, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x74, 0x22, 0xc6, 0x01, 0x0a, 0x04,
	0x4d, 0x65, 0x74, 0x61, 0x12, 0x16, 0x0a, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x32, 0x0a, 0x15,
	0x72, 0x61, 0x6e, 0x6b, 0x69, 0x6e, 0x67, 0x5f, 0x6d, 0x6f, 0x64, 0x65, 0x6c, 0x5f, 0x76, 0x65,
	0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x13, 0x72, 0x61, 0x6e,
	0x6b, 0x69, 0x6e, 0x67, 0x4d, 0x6f, 0x64, 0x65, 0x6c, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e,
	0x12, 0x2e, 0x0a, 0x13, 0x63, 0x6c, 0x69, 0x63, 0x6b, 0x5f, 0x6d, 0x6f, 0x64, 0x65, 0x6c, 0x5f,
	0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x03, 0x52, 0x11, 0x63,
	0x6c, 0x69, 0x63, 0x6b, 0x4d, 0x6f, 0x64, 0x65, 0x6c, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e,
	0x12, 0x0e, 0x0a, 0x02, 0x6d, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x6d, 0x65,
	0x12, 0x18, 0x0a, 0x07, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x73, 0x18, 0x06, 0x20, 0x03, 0x28,
	0x09, 0x52, 0x07, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x77, 0x6f,
	0x72, 0x6b, 0x65, 0x72, 0x73, 0x18, 0x07, 0x20, 0x03, 0x28, 0x09, 0x52, 0x07, 0x77, 0x6f, 0x72,
	0x6b, 0x65, 0x72, 0x73, 0x22, 0x1e, 0x0a, 0x08, 0x46, 0x72, 0x61, 0x67, 0x6d, 0x65, 0x6e, 0x74,
	0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04,
	0x64, 0x61, 0x74, 0x61, 0x22, 0x27, 0x0a, 0x0b, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x49,
	0x6e, 0x66, 0x6f, 0x12, 0x18, 0x0a, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x22, 0x92, 0x01,
	0x0a, 0x08, 0x4e, 0x6f, 0x64, 0x65, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x2f, 0x0a, 0x09, 0x6e, 0x6f,
	0x64, 0x65, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x12, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x54, 0x79, 0x70,
	0x65, 0x52, 0x08, 0x6e, 0x6f, 0x64, 0x65, 0x54, 0x79, 0x70, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x75,
	0x75, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x75, 0x75, 0x69, 0x64, 0x12,
	0x25, 0x0a, 0x0e, 0x62, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x5f, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f,
	0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x62, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x56,
	0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x1a, 0x0a, 0x08, 0x68, 0x6f, 0x73, 0x74, 0x6e, 0x61,
	0x6d, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x68, 0x6f, 0x73, 0x74, 0x6e, 0x61,
	0x6d, 0x65, 0x22, 0xd0, 0x01, 0x0a, 0x08, 0x50, 0x72, 0x6f, 0x67, 0x72, 0x65, 0x73, 0x73, 0x12,
	0x16, 0x0a, 0x06, 0x74, 0x72, 0x61, 0x63, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x06, 0x74, 0x72, 0x61, 0x63, 0x65, 0x72, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x73,
	0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x74, 0x61,
	0x74, 0x75, 0x73, 0x12, 0x14, 0x0a, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x63, 0x6f, 0x75,
	0x6e, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x12,
	0x14, 0x0a, 0x05, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x18, 0x06, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05,
	0x74, 0x6f, 0x74, 0x61, 0x6c, 0x12, 0x1d, 0x0a, 0x0a, 0x73, 0x74, 0x61, 0x72, 0x74, 0x5f, 0x74,
	0x69, 0x6d, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x73, 0x74, 0x61, 0x72, 0x74,
	0x54, 0x69, 0x6d, 0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x66, 0x69, 0x6e, 0x69, 0x73, 0x68, 0x5f, 0x74,
	0x69, 0x6d, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0a, 0x66, 0x69, 0x6e, 0x69, 0x73,
	0x68, 0x54, 0x69, 0x6d, 0x65, 0x22, 0x45, 0x0a, 0x13, 0x50, 0x75, 0x73, 0x68, 0x50, 0x72, 0x6f,
	0x67, 0x72, 0x65, 0x73, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x2e, 0x0a, 0x08,
	0x70, 0x72, 0x6f, 0x67, 0x72, 0x65, 0x73, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x12,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x50, 0x72, 0x6f, 0x67, 0x72, 0x65,
	0x73, 0x73, 0x52, 0x08, 0x70, 0x72, 0x6f, 0x67, 0x72, 0x65, 0x73, 0x73, 0x22, 0x16, 0x0a, 0x14,
	0x50, 0x75, 0x73, 0x68, 0x50, 0x72, 0x6f, 0x67, 0x72, 0x65, 0x73, 0x73, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x22, 0x0d, 0x0a, 0x0b, 0x50, 0x69, 0x6e, 0x67, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x22, 0x0e, 0x0a, 0x0c, 0x50, 0x69, 0x6e, 0x67, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x2a, 0x3a, 0x0a, 0x08, 0x4e, 0x6f, 0x64, 0x65, 0x54, 0x79, 0x70, 0x65, 0x12,
	0x0e, 0x0a, 0x0a, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x4e, 0x6f, 0x64, 0x65, 0x10, 0x00, 0x12,
	0x0e, 0x0a, 0x0a, 0x57, 0x6f, 0x72, 0x6b, 0x65, 0x72, 0x4e, 0x6f, 0x64, 0x65, 0x10, 0x01, 0x12,
	0x0e, 0x0a, 0x0a, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x4e, 0x6f, 0x64, 0x65, 0x10, 0x02, 0x32,
	0x8c, 0x02, 0x0a, 0x06, 0x4d, 0x61, 0x73, 0x74, 0x65, 0x72, 0x12, 0x2f, 0x0a, 0x07, 0x47, 0x65,
	0x74, 0x4d, 0x65, 0x74, 0x61, 0x12, 0x12, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c,
	0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x49, 0x6e, 0x66, 0x6f, 0x1a, 0x0e, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x4d, 0x65, 0x74, 0x61, 0x22, 0x00, 0x12, 0x40, 0x0a, 0x0f, 0x47,
	0x65, 0x74, 0x52, 0x61, 0x6e, 0x6b, 0x69, 0x6e, 0x67, 0x4d, 0x6f, 0x64, 0x65, 0x6c, 0x12, 0x15,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f,
	0x6e, 0x49, 0x6e, 0x66, 0x6f, 0x1a, 0x12, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c,
	0x2e, 0x46, 0x72, 0x61, 0x67, 0x6d, 0x65, 0x6e, 0x74, 0x22, 0x00, 0x30, 0x01, 0x12, 0x3e, 0x0a,
	0x0d, 0x47, 0x65, 0x74, 0x43, 0x6c, 0x69, 0x63, 0x6b, 0x4d, 0x6f, 0x64, 0x65, 0x6c, 0x12, 0x15,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f,
	0x6e, 0x49, 0x6e, 0x66, 0x6f, 0x1a, 0x12, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c,
	0x2e, 0x46, 0x72, 0x61, 0x67, 0x6d, 0x65, 0x6e, 0x74, 0x22, 0x00, 0x30, 0x01, 0x12, 0x4f, 0x0a,
	0x0c, 0x50, 0x75, 0x73, 0x68, 0x50, 0x72, 0x6f, 0x67, 0x72, 0x65, 0x73, 0x73, 0x12, 0x1d, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x50, 0x75, 0x73, 0x68, 0x50, 0x72, 0x6f,
	0x67, 0x72, 0x65, 0x73, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1e, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x50, 0x75, 0x73, 0x68, 0x50, 0x72, 0x6f, 0x67,
	0x72, 0x65, 0x73, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x25,
	0x5a, 0x23, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x7a, 0x68, 0x65,
	0x6e, 0x67, 0x68, 0x61, 0x6f, 0x7a, 0x2f, 0x67, 0x6f, 0x72, 0x73, 0x65, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_protocol_proto_rawDescOnce sync.Once
	file_protocol_proto_rawDescData = file_protocol_proto_rawDesc
)

func file_protocol_proto_rawDescGZIP() []byte {
	file_protocol_proto_rawDescOnce.Do(func() {
		file_protocol_proto_rawDescData = protoimpl.X.CompressGZIP(file_protocol_proto_rawDescData)
	})
	return file_protocol_proto_rawDescData
}

var file_protocol_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_protocol_proto_msgTypes = make([]protoimpl.MessageInfo, 12)
var file_protocol_proto_goTypes = []any{
	(NodeType)(0),                 // 0: protocol.NodeType
	(*User)(nil),                  // 1: protocol.User
	(*Item)(nil),                  // 2: protocol.Item
	(*Feedback)(nil),              // 3: protocol.Feedback
	(*Meta)(nil),                  // 4: protocol.Meta
	(*Fragment)(nil),              // 5: protocol.Fragment
	(*VersionInfo)(nil),           // 6: protocol.VersionInfo
	(*NodeInfo)(nil),              // 7: protocol.NodeInfo
	(*Progress)(nil),              // 8: protocol.Progress
	(*PushProgressRequest)(nil),   // 9: protocol.PushProgressRequest
	(*PushProgressResponse)(nil),  // 10: protocol.PushProgressResponse
	(*PingRequest)(nil),           // 11: protocol.PingRequest
	(*PingResponse)(nil),          // 12: protocol.PingResponse
	(*timestamppb.Timestamp)(nil), // 13: google.protobuf.Timestamp
}
var file_protocol_proto_depIdxs = []int32{
	13, // 0: protocol.Item.timestamp:type_name -> google.protobuf.Timestamp
	13, // 1: protocol.Feedback.timestamp:type_name -> google.protobuf.Timestamp
	0,  // 2: protocol.NodeInfo.node_type:type_name -> protocol.NodeType
	8,  // 3: protocol.PushProgressRequest.progress:type_name -> protocol.Progress
	7,  // 4: protocol.Master.GetMeta:input_type -> protocol.NodeInfo
	6,  // 5: protocol.Master.GetRankingModel:input_type -> protocol.VersionInfo
	6,  // 6: protocol.Master.GetClickModel:input_type -> protocol.VersionInfo
	9,  // 7: protocol.Master.PushProgress:input_type -> protocol.PushProgressRequest
	4,  // 8: protocol.Master.GetMeta:output_type -> protocol.Meta
	5,  // 9: protocol.Master.GetRankingModel:output_type -> protocol.Fragment
	5,  // 10: protocol.Master.GetClickModel:output_type -> protocol.Fragment
	10, // 11: protocol.Master.PushProgress:output_type -> protocol.PushProgressResponse
	8,  // [8:12] is the sub-list for method output_type
	4,  // [4:8] is the sub-list for method input_type
	4,  // [4:4] is the sub-list for extension type_name
	4,  // [4:4] is the sub-list for extension extendee
	0,  // [0:4] is the sub-list for field type_name
}

func init() { file_protocol_proto_init() }
func file_protocol_proto_init() {
	if File_protocol_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_protocol_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   12,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_protocol_proto_goTypes,
		DependencyIndexes: file_protocol_proto_depIdxs,
		EnumInfos:         file_protocol_proto_enumTypes,
		MessageInfos:      file_protocol_proto_msgTypes,
	}.Build()
	File_protocol_proto = out.File
	file_protocol_proto_rawDesc = nil
	file_protocol_proto_goTypes = nil
	file_protocol_proto_depIdxs = nil
}
