// Code generated by protoc-gen-go. DO NOT EDIT.
// source: protocol.proto

package protocol

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type NodeType int32

const (
	NodeType_ServerNode NodeType = 0
	NodeType_WorkerNode NodeType = 1
	NodeType_ClientNode NodeType = 2
)

var NodeType_name = map[int32]string{
	0: "ServerNode",
	1: "WorkerNode",
	2: "ClientNode",
}

var NodeType_value = map[string]int32{
	"ServerNode": 0,
	"WorkerNode": 1,
	"ClientNode": 2,
}

func (x NodeType) String() string {
	return proto.EnumName(NodeType_name, int32(x))
}

func (NodeType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_2bc2336598a3f7e0, []int{0}
}

type Meta struct {
	Config               string   `protobuf:"bytes,1,opt,name=config,proto3" json:"config,omitempty"`
	RankingModelVersion  int64    `protobuf:"varint,3,opt,name=ranking_model_version,json=rankingModelVersion,proto3" json:"ranking_model_version,omitempty"`
	ClickModelVersion    int64    `protobuf:"varint,4,opt,name=click_model_version,json=clickModelVersion,proto3" json:"click_model_version,omitempty"`
	Me                   string   `protobuf:"bytes,5,opt,name=me,proto3" json:"me,omitempty"`
	Servers              []string `protobuf:"bytes,6,rep,name=servers,proto3" json:"servers,omitempty"`
	Workers              []string `protobuf:"bytes,7,rep,name=workers,proto3" json:"workers,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Meta) Reset()         { *m = Meta{} }
func (m *Meta) String() string { return proto.CompactTextString(m) }
func (*Meta) ProtoMessage()    {}
func (*Meta) Descriptor() ([]byte, []int) {
	return fileDescriptor_2bc2336598a3f7e0, []int{0}
}

func (m *Meta) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Meta.Unmarshal(m, b)
}
func (m *Meta) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Meta.Marshal(b, m, deterministic)
}
func (m *Meta) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Meta.Merge(m, src)
}
func (m *Meta) XXX_Size() int {
	return xxx_messageInfo_Meta.Size(m)
}
func (m *Meta) XXX_DiscardUnknown() {
	xxx_messageInfo_Meta.DiscardUnknown(m)
}

var xxx_messageInfo_Meta proto.InternalMessageInfo

func (m *Meta) GetConfig() string {
	if m != nil {
		return m.Config
	}
	return ""
}

func (m *Meta) GetRankingModelVersion() int64 {
	if m != nil {
		return m.RankingModelVersion
	}
	return 0
}

func (m *Meta) GetClickModelVersion() int64 {
	if m != nil {
		return m.ClickModelVersion
	}
	return 0
}

func (m *Meta) GetMe() string {
	if m != nil {
		return m.Me
	}
	return ""
}

func (m *Meta) GetServers() []string {
	if m != nil {
		return m.Servers
	}
	return nil
}

func (m *Meta) GetWorkers() []string {
	if m != nil {
		return m.Workers
	}
	return nil
}

type Fragment struct {
	Data                 []byte   `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Fragment) Reset()         { *m = Fragment{} }
func (m *Fragment) String() string { return proto.CompactTextString(m) }
func (*Fragment) ProtoMessage()    {}
func (*Fragment) Descriptor() ([]byte, []int) {
	return fileDescriptor_2bc2336598a3f7e0, []int{1}
}

func (m *Fragment) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Fragment.Unmarshal(m, b)
}
func (m *Fragment) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Fragment.Marshal(b, m, deterministic)
}
func (m *Fragment) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Fragment.Merge(m, src)
}
func (m *Fragment) XXX_Size() int {
	return xxx_messageInfo_Fragment.Size(m)
}
func (m *Fragment) XXX_DiscardUnknown() {
	xxx_messageInfo_Fragment.DiscardUnknown(m)
}

var xxx_messageInfo_Fragment proto.InternalMessageInfo

func (m *Fragment) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

type VersionInfo struct {
	Version              int64    `protobuf:"varint,1,opt,name=version,proto3" json:"version,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *VersionInfo) Reset()         { *m = VersionInfo{} }
func (m *VersionInfo) String() string { return proto.CompactTextString(m) }
func (*VersionInfo) ProtoMessage()    {}
func (*VersionInfo) Descriptor() ([]byte, []int) {
	return fileDescriptor_2bc2336598a3f7e0, []int{2}
}

func (m *VersionInfo) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_VersionInfo.Unmarshal(m, b)
}
func (m *VersionInfo) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_VersionInfo.Marshal(b, m, deterministic)
}
func (m *VersionInfo) XXX_Merge(src proto.Message) {
	xxx_messageInfo_VersionInfo.Merge(m, src)
}
func (m *VersionInfo) XXX_Size() int {
	return xxx_messageInfo_VersionInfo.Size(m)
}
func (m *VersionInfo) XXX_DiscardUnknown() {
	xxx_messageInfo_VersionInfo.DiscardUnknown(m)
}

var xxx_messageInfo_VersionInfo proto.InternalMessageInfo

func (m *VersionInfo) GetVersion() int64 {
	if m != nil {
		return m.Version
	}
	return 0
}

type NodeInfo struct {
	NodeType             NodeType `protobuf:"varint,1,opt,name=node_type,json=nodeType,proto3,enum=protocol.NodeType" json:"node_type,omitempty"`
	NodeName             string   `protobuf:"bytes,2,opt,name=node_name,json=nodeName,proto3" json:"node_name,omitempty"`
	HttpPort             int64    `protobuf:"varint,3,opt,name=http_port,json=httpPort,proto3" json:"http_port,omitempty"`
	BinaryVersion        string   `protobuf:"bytes,4,opt,name=binary_version,json=binaryVersion,proto3" json:"binary_version,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *NodeInfo) Reset()         { *m = NodeInfo{} }
func (m *NodeInfo) String() string { return proto.CompactTextString(m) }
func (*NodeInfo) ProtoMessage()    {}
func (*NodeInfo) Descriptor() ([]byte, []int) {
	return fileDescriptor_2bc2336598a3f7e0, []int{3}
}

func (m *NodeInfo) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodeInfo.Unmarshal(m, b)
}
func (m *NodeInfo) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodeInfo.Marshal(b, m, deterministic)
}
func (m *NodeInfo) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeInfo.Merge(m, src)
}
func (m *NodeInfo) XXX_Size() int {
	return xxx_messageInfo_NodeInfo.Size(m)
}
func (m *NodeInfo) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeInfo.DiscardUnknown(m)
}

var xxx_messageInfo_NodeInfo proto.InternalMessageInfo

func (m *NodeInfo) GetNodeType() NodeType {
	if m != nil {
		return m.NodeType
	}
	return NodeType_ServerNode
}

func (m *NodeInfo) GetNodeName() string {
	if m != nil {
		return m.NodeName
	}
	return ""
}

func (m *NodeInfo) GetHttpPort() int64 {
	if m != nil {
		return m.HttpPort
	}
	return 0
}

func (m *NodeInfo) GetBinaryVersion() string {
	if m != nil {
		return m.BinaryVersion
	}
	return ""
}

type Progress struct {
	Tracer               string   `protobuf:"bytes,1,opt,name=tracer,proto3" json:"tracer,omitempty"`
	Name                 string   `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Status               string   `protobuf:"bytes,3,opt,name=status,proto3" json:"status,omitempty"`
	Error                string   `protobuf:"bytes,4,opt,name=error,proto3" json:"error,omitempty"`
	Count                int64    `protobuf:"varint,5,opt,name=count,proto3" json:"count,omitempty"`
	Total                int64    `protobuf:"varint,6,opt,name=total,proto3" json:"total,omitempty"`
	StartTime            int64    `protobuf:"varint,7,opt,name=start_time,json=startTime,proto3" json:"start_time,omitempty"`
	FinishTime           int64    `protobuf:"varint,8,opt,name=finish_time,json=finishTime,proto3" json:"finish_time,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Progress) Reset()         { *m = Progress{} }
func (m *Progress) String() string { return proto.CompactTextString(m) }
func (*Progress) ProtoMessage()    {}
func (*Progress) Descriptor() ([]byte, []int) {
	return fileDescriptor_2bc2336598a3f7e0, []int{4}
}

func (m *Progress) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Progress.Unmarshal(m, b)
}
func (m *Progress) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Progress.Marshal(b, m, deterministic)
}
func (m *Progress) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Progress.Merge(m, src)
}
func (m *Progress) XXX_Size() int {
	return xxx_messageInfo_Progress.Size(m)
}
func (m *Progress) XXX_DiscardUnknown() {
	xxx_messageInfo_Progress.DiscardUnknown(m)
}

var xxx_messageInfo_Progress proto.InternalMessageInfo

func (m *Progress) GetTracer() string {
	if m != nil {
		return m.Tracer
	}
	return ""
}

func (m *Progress) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *Progress) GetStatus() string {
	if m != nil {
		return m.Status
	}
	return ""
}

func (m *Progress) GetError() string {
	if m != nil {
		return m.Error
	}
	return ""
}

func (m *Progress) GetCount() int64 {
	if m != nil {
		return m.Count
	}
	return 0
}

func (m *Progress) GetTotal() int64 {
	if m != nil {
		return m.Total
	}
	return 0
}

func (m *Progress) GetStartTime() int64 {
	if m != nil {
		return m.StartTime
	}
	return 0
}

func (m *Progress) GetFinishTime() int64 {
	if m != nil {
		return m.FinishTime
	}
	return 0
}

type PushProgressRequest struct {
	Progress             []*Progress `protobuf:"bytes,1,rep,name=progress,proto3" json:"progress,omitempty"`
	XXX_NoUnkeyedLiteral struct{}    `json:"-"`
	XXX_unrecognized     []byte      `json:"-"`
	XXX_sizecache        int32       `json:"-"`
}

func (m *PushProgressRequest) Reset()         { *m = PushProgressRequest{} }
func (m *PushProgressRequest) String() string { return proto.CompactTextString(m) }
func (*PushProgressRequest) ProtoMessage()    {}
func (*PushProgressRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_2bc2336598a3f7e0, []int{5}
}

func (m *PushProgressRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PushProgressRequest.Unmarshal(m, b)
}
func (m *PushProgressRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PushProgressRequest.Marshal(b, m, deterministic)
}
func (m *PushProgressRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PushProgressRequest.Merge(m, src)
}
func (m *PushProgressRequest) XXX_Size() int {
	return xxx_messageInfo_PushProgressRequest.Size(m)
}
func (m *PushProgressRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_PushProgressRequest.DiscardUnknown(m)
}

var xxx_messageInfo_PushProgressRequest proto.InternalMessageInfo

func (m *PushProgressRequest) GetProgress() []*Progress {
	if m != nil {
		return m.Progress
	}
	return nil
}

type PushProgressResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PushProgressResponse) Reset()         { *m = PushProgressResponse{} }
func (m *PushProgressResponse) String() string { return proto.CompactTextString(m) }
func (*PushProgressResponse) ProtoMessage()    {}
func (*PushProgressResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_2bc2336598a3f7e0, []int{6}
}

func (m *PushProgressResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PushProgressResponse.Unmarshal(m, b)
}
func (m *PushProgressResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PushProgressResponse.Marshal(b, m, deterministic)
}
func (m *PushProgressResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PushProgressResponse.Merge(m, src)
}
func (m *PushProgressResponse) XXX_Size() int {
	return xxx_messageInfo_PushProgressResponse.Size(m)
}
func (m *PushProgressResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_PushProgressResponse.DiscardUnknown(m)
}

var xxx_messageInfo_PushProgressResponse proto.InternalMessageInfo

func init() {
	proto.RegisterEnum("protocol.NodeType", NodeType_name, NodeType_value)
	proto.RegisterType((*Meta)(nil), "protocol.Meta")
	proto.RegisterType((*Fragment)(nil), "protocol.Fragment")
	proto.RegisterType((*VersionInfo)(nil), "protocol.VersionInfo")
	proto.RegisterType((*NodeInfo)(nil), "protocol.NodeInfo")
	proto.RegisterType((*Progress)(nil), "protocol.Progress")
	proto.RegisterType((*PushProgressRequest)(nil), "protocol.PushProgressRequest")
	proto.RegisterType((*PushProgressResponse)(nil), "protocol.PushProgressResponse")
}

func init() {
	proto.RegisterFile("protocol.proto", fileDescriptor_2bc2336598a3f7e0)
}

var fileDescriptor_2bc2336598a3f7e0 = []byte{
	// 592 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x53, 0xcd, 0x6e, 0xd3, 0x4e,
	0x10, 0x8f, 0x93, 0x36, 0x71, 0xa6, 0x6d, 0xfe, 0x7f, 0xb6, 0x1f, 0xb2, 0x8a, 0x5a, 0x22, 0xa3,
	0x8a, 0x88, 0x43, 0x82, 0xca, 0x8d, 0x03, 0x42, 0x54, 0x50, 0x71, 0x68, 0xa9, 0x4c, 0x05, 0x12,
	0x97, 0x68, 0xeb, 0x4c, 0x6d, 0xab, 0xf1, 0xae, 0xd9, 0x9d, 0x80, 0xda, 0x67, 0xe0, 0x11, 0x78,
	0x1e, 0xce, 0x3c, 0x12, 0xda, 0xb1, 0x9d, 0xba, 0x45, 0x1c, 0xb8, 0xed, 0xef, 0x63, 0xf6, 0xe3,
	0x37, 0xb3, 0x30, 0x28, 0x8c, 0x26, 0x1d, 0xeb, 0xf9, 0x98, 0x17, 0xc2, 0xaf, 0x71, 0xf8, 0xd3,
	0x83, 0x95, 0x13, 0x24, 0x29, 0x76, 0xa0, 0x1b, 0x6b, 0x75, 0x99, 0x25, 0x81, 0x37, 0xf4, 0x46,
	0xfd, 0xa8, 0x42, 0xe2, 0x10, 0xb6, 0x8d, 0x54, 0x57, 0x99, 0x4a, 0xa6, 0xb9, 0x9e, 0xe1, 0x7c,
	0xfa, 0x15, 0x8d, 0xcd, 0xb4, 0x0a, 0x3a, 0x43, 0x6f, 0xd4, 0x89, 0x36, 0x2b, 0xf1, 0xc4, 0x69,
	0x1f, 0x4b, 0x49, 0x8c, 0x61, 0x33, 0x9e, 0x67, 0xf1, 0xd5, 0xbd, 0x8a, 0x15, 0xae, 0x78, 0xc0,
	0xd2, 0x1d, 0xff, 0x00, 0xda, 0x39, 0x06, 0xab, 0x7c, 0x6e, 0x3b, 0x47, 0x11, 0x40, 0xcf, 0xa2,
	0x71, 0x65, 0x41, 0x77, 0xd8, 0x19, 0xf5, 0xa3, 0x1a, 0x3a, 0xe5, 0x9b, 0x36, 0x57, 0x4e, 0xe9,
	0x95, 0x4a, 0x05, 0xc3, 0x7d, 0xf0, 0xdf, 0x1a, 0x99, 0xe4, 0xa8, 0x48, 0x08, 0x58, 0x99, 0x49,
	0x92, 0xfc, 0x92, 0xf5, 0x88, 0xd7, 0xe1, 0x13, 0x58, 0xab, 0x8e, 0x7b, 0xa7, 0x2e, 0xb5, 0xdb,
	0xa8, 0xbe, 0x96, 0xc7, 0xd7, 0xaa, 0x61, 0xf8, 0xc3, 0x03, 0xff, 0x54, 0xcf, 0x90, 0x6d, 0x13,
	0xe8, 0x2b, 0x3d, 0xc3, 0x29, 0x5d, 0x17, 0xc8, 0xc6, 0xc1, 0xa1, 0x18, 0x2f, 0xc3, 0x74, 0xb6,
	0xf3, 0xeb, 0x02, 0x23, 0x5f, 0x55, 0x2b, 0xf1, 0xb0, 0x2a, 0x50, 0x32, 0xc7, 0xa0, 0xcd, 0x2f,
	0x62, 0xf1, 0x54, 0xe6, 0x2c, 0xa6, 0x44, 0xc5, 0xb4, 0xd0, 0x86, 0xaa, 0xfc, 0x7c, 0x47, 0x9c,
	0x69, 0x43, 0xe2, 0x00, 0x06, 0x17, 0x99, 0x92, 0xe6, 0xfa, 0x4e, 0x5e, 0xfd, 0x68, 0xa3, 0x64,
	0xab, 0xcb, 0x87, 0xbf, 0x3c, 0xf0, 0xcf, 0x8c, 0x4e, 0x0c, 0x5a, 0xeb, 0x9a, 0x46, 0x46, 0xc6,
	0x68, 0xea, 0xa6, 0x95, 0xc8, 0x05, 0xd0, 0xb8, 0x00, 0xaf, 0x9d, 0xd7, 0x92, 0xa4, 0x85, 0xe5,
	0x93, 0xfb, 0x51, 0x85, 0xc4, 0x16, 0xac, 0xa2, 0x31, 0xda, 0x54, 0xc7, 0x95, 0xc0, 0xb1, 0xb1,
	0x5e, 0x28, 0xe2, 0xae, 0x74, 0xa2, 0x12, 0x38, 0x96, 0x34, 0xc9, 0x79, 0xd0, 0x2d, 0x59, 0x06,
	0x62, 0x0f, 0xc0, 0x92, 0x34, 0x34, 0xa5, 0x2c, 0xc7, 0xa0, 0xc7, 0x52, 0x9f, 0x99, 0xf3, 0x2c,
	0x47, 0xf1, 0x08, 0xd6, 0x2e, 0x33, 0x95, 0xd9, 0xb4, 0xd4, 0x7d, 0xd6, 0xa1, 0xa4, 0x9c, 0x21,
	0x7c, 0x03, 0x9b, 0x67, 0x0b, 0x9b, 0xd6, 0xaf, 0x8a, 0xf0, 0xcb, 0x02, 0x2d, 0x89, 0x31, 0xb8,
	0x31, 0x65, 0x2a, 0xf0, 0x86, 0x9d, 0xd1, 0x5a, 0x33, 0xfa, 0xa5, 0x79, 0xe9, 0x09, 0x77, 0x60,
	0xeb, 0xee, 0x36, 0xb6, 0xd0, 0xca, 0xe2, 0xd3, 0x17, 0x65, 0x3f, 0xb9, 0x3d, 0x03, 0x80, 0x0f,
	0x3c, 0x4a, 0x8e, 0xf9, 0xbf, 0xe5, 0xf0, 0x27, 0x1e, 0x20, 0xc6, 0x9e, 0xc3, 0x47, 0xf3, 0x0c,
	0x15, 0x31, 0x6e, 0x1f, 0x7e, 0x6f, 0x43, 0xf7, 0x44, 0x5a, 0x42, 0x23, 0x26, 0xd0, 0x3b, 0x46,
	0xe2, 0xbf, 0x72, 0x6f, 0x04, 0xdc, 0xa4, 0xec, 0x0e, 0x6e, 0x39, 0xe7, 0x09, 0x5b, 0xe2, 0x15,
	0xfc, 0x77, 0x8c, 0x14, 0x35, 0xfe, 0x87, 0xd8, 0xbe, 0x35, 0x35, 0x86, 0x71, 0xb7, 0xb1, 0x5f,
	0x3d, 0xc3, 0x61, 0xeb, 0x99, 0x27, 0x5e, 0xc2, 0xc6, 0x31, 0xd2, 0xd1, 0xf2, 0xbf, 0xfc, 0x6b,
	0xfd, 0x7b, 0x58, 0x6f, 0x26, 0x22, 0xf6, 0x1a, 0xf9, 0xfd, 0x19, 0xf8, 0xee, 0xfe, 0xdf, 0xe4,
	0x32, 0xc8, 0xb0, 0xf5, 0xfa, 0xe0, 0xf3, 0xe3, 0x24, 0xa3, 0x74, 0x71, 0x31, 0x8e, 0x75, 0x3e,
	0xb9, 0x49, 0x51, 0x25, 0xa9, 0xd4, 0x37, 0x93, 0x44, 0x1b, 0x8b, 0x93, 0xba, 0xfa, 0xa2, 0xcb,
	0xab, 0xe7, 0xbf, 0x03, 0x00, 0x00, 0xff, 0xff, 0x0e, 0x9c, 0xdb, 0xcd, 0x77, 0x04, 0x00, 0x00,
}
