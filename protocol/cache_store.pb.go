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

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2
// 	protoc        v5.29.0
// source: cache_store.proto

package protocol

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

var File_cache_store_proto protoreflect.FileDescriptor

var file_cache_store_proto_rawDesc = []byte{
	0x0a, 0x11, 0x63, 0x61, 0x63, 0x68, 0x65, 0x5f, 0x73, 0x74, 0x6f, 0x72, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x08, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x1a, 0x0e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x32, 0xe8, 0x08,
	0x0a, 0x0a, 0x43, 0x61, 0x63, 0x68, 0x65, 0x53, 0x74, 0x6f, 0x72, 0x65, 0x12, 0x34, 0x0a, 0x03,
	0x47, 0x65, 0x74, 0x12, 0x14, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x47,
	0x65, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x15, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x47, 0x65, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x22, 0x00, 0x12, 0x34, 0x0a, 0x03, 0x53, 0x65, 0x74, 0x12, 0x14, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x53, 0x65, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x15, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x53, 0x65, 0x74, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x3d, 0x0a, 0x06, 0x44, 0x65, 0x6c, 0x65,
	0x74, 0x65, 0x12, 0x17, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x44, 0x65,
	0x6c, 0x65, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x18, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x3d, 0x0a, 0x06, 0x47, 0x65, 0x74, 0x53, 0x65,
	0x74, 0x12, 0x17, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x47, 0x65, 0x74,
	0x53, 0x65, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x18, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x47, 0x65, 0x74, 0x53, 0x65, 0x74, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x3d, 0x0a, 0x06, 0x53, 0x65, 0x74, 0x53, 0x65, 0x74,
	0x12, 0x17, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x53, 0x65, 0x74, 0x53,
	0x65, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x18, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x53, 0x65, 0x74, 0x53, 0x65, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x3d, 0x0a, 0x06, 0x41, 0x64, 0x64, 0x53, 0x65, 0x74, 0x12,
	0x17, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x41, 0x64, 0x64, 0x53, 0x65,
	0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x18, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x63, 0x6f, 0x6c, 0x2e, 0x41, 0x64, 0x64, 0x53, 0x65, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x22, 0x00, 0x12, 0x3d, 0x0a, 0x06, 0x52, 0x65, 0x6d, 0x53, 0x65, 0x74, 0x12, 0x17,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x52, 0x65, 0x6d, 0x53, 0x65, 0x74,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x18, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63,
	0x6f, 0x6c, 0x2e, 0x52, 0x65, 0x6d, 0x53, 0x65, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x22, 0x00, 0x12, 0x37, 0x0a, 0x04, 0x50, 0x75, 0x73, 0x68, 0x12, 0x15, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x50, 0x75, 0x73, 0x68, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x1a, 0x16, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x50, 0x75,
	0x73, 0x68, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x34, 0x0a, 0x03,
	0x50, 0x6f, 0x70, 0x12, 0x14, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x50,
	0x6f, 0x70, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x15, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x50, 0x6f, 0x70, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x22, 0x00, 0x12, 0x3d, 0x0a, 0x06, 0x52, 0x65, 0x6d, 0x61, 0x69, 0x6e, 0x12, 0x17, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x52, 0x65, 0x6d, 0x61, 0x69, 0x6e, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x18, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c,
	0x2e, 0x52, 0x65, 0x6d, 0x61, 0x69, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x12, 0x46, 0x0a, 0x09, 0x41, 0x64, 0x64, 0x53, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x12, 0x1a,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x41, 0x64, 0x64, 0x53, 0x63, 0x6f,
	0x72, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1b, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x41, 0x64, 0x64, 0x53, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x4f, 0x0a, 0x0c, 0x53, 0x65, 0x61,
	0x72, 0x63, 0x68, 0x53, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x12, 0x1d, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x53, 0x65, 0x61, 0x72, 0x63, 0x68, 0x53, 0x63, 0x6f, 0x72, 0x65,
	0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x63, 0x6f, 0x6c, 0x2e, 0x53, 0x65, 0x61, 0x72, 0x63, 0x68, 0x53, 0x63, 0x6f, 0x72, 0x65, 0x73,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x4f, 0x0a, 0x0c, 0x44, 0x65,
	0x6c, 0x65, 0x74, 0x65, 0x53, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x12, 0x1d, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x53, 0x63, 0x6f, 0x72,
	0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1e, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x53, 0x63, 0x6f, 0x72, 0x65,
	0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x4f, 0x0a, 0x0c, 0x55,
	0x70, 0x64, 0x61, 0x74, 0x65, 0x53, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x12, 0x1d, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x53, 0x63, 0x6f,
	0x72, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1e, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x53, 0x63, 0x6f, 0x72,
	0x65, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x64, 0x0a, 0x13,
	0x41, 0x64, 0x64, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x65, 0x72, 0x69, 0x65, 0x73, 0x50, 0x6f, 0x69,
	0x6e, 0x74, 0x73, 0x12, 0x24, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x41,
	0x64, 0x64, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x65, 0x72, 0x69, 0x65, 0x73, 0x50, 0x6f, 0x69, 0x6e,
	0x74, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x25, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x41, 0x64, 0x64, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x65, 0x72, 0x69,
	0x65, 0x73, 0x50, 0x6f, 0x69, 0x6e, 0x74, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x22, 0x00, 0x12, 0x64, 0x0a, 0x13, 0x47, 0x65, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x65, 0x72,
	0x69, 0x65, 0x73, 0x50, 0x6f, 0x69, 0x6e, 0x74, 0x73, 0x12, 0x24, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x47, 0x65, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x65, 0x72, 0x69,
	0x65, 0x73, 0x50, 0x6f, 0x69, 0x6e, 0x74, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x25, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x47, 0x65, 0x74, 0x54, 0x69,
	0x6d, 0x65, 0x53, 0x65, 0x72, 0x69, 0x65, 0x73, 0x50, 0x6f, 0x69, 0x6e, 0x74, 0x73, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x25, 0x5a, 0x23, 0x67, 0x69, 0x74, 0x68,
	0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x7a, 0x68, 0x65, 0x6e, 0x67, 0x68, 0x61, 0x6f, 0x7a,
	0x2f, 0x67, 0x6f, 0x72, 0x73, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var file_cache_store_proto_goTypes = []any{
	(*GetRequest)(nil),                  // 0: protocol.GetRequest
	(*SetRequest)(nil),                  // 1: protocol.SetRequest
	(*DeleteRequest)(nil),               // 2: protocol.DeleteRequest
	(*GetSetRequest)(nil),               // 3: protocol.GetSetRequest
	(*SetSetRequest)(nil),               // 4: protocol.SetSetRequest
	(*AddSetRequest)(nil),               // 5: protocol.AddSetRequest
	(*RemSetRequest)(nil),               // 6: protocol.RemSetRequest
	(*PushRequest)(nil),                 // 7: protocol.PushRequest
	(*PopRequest)(nil),                  // 8: protocol.PopRequest
	(*RemainRequest)(nil),               // 9: protocol.RemainRequest
	(*AddScoresRequest)(nil),            // 10: protocol.AddScoresRequest
	(*SearchScoresRequest)(nil),         // 11: protocol.SearchScoresRequest
	(*DeleteScoresRequest)(nil),         // 12: protocol.DeleteScoresRequest
	(*UpdateScoresRequest)(nil),         // 13: protocol.UpdateScoresRequest
	(*AddTimeSeriesPointsRequest)(nil),  // 14: protocol.AddTimeSeriesPointsRequest
	(*GetTimeSeriesPointsRequest)(nil),  // 15: protocol.GetTimeSeriesPointsRequest
	(*GetResponse)(nil),                 // 16: protocol.GetResponse
	(*SetResponse)(nil),                 // 17: protocol.SetResponse
	(*DeleteResponse)(nil),              // 18: protocol.DeleteResponse
	(*GetSetResponse)(nil),              // 19: protocol.GetSetResponse
	(*SetSetResponse)(nil),              // 20: protocol.SetSetResponse
	(*AddSetResponse)(nil),              // 21: protocol.AddSetResponse
	(*RemSetResponse)(nil),              // 22: protocol.RemSetResponse
	(*PushResponse)(nil),                // 23: protocol.PushResponse
	(*PopResponse)(nil),                 // 24: protocol.PopResponse
	(*RemainResponse)(nil),              // 25: protocol.RemainResponse
	(*AddScoresResponse)(nil),           // 26: protocol.AddScoresResponse
	(*SearchScoresResponse)(nil),        // 27: protocol.SearchScoresResponse
	(*DeleteScoresResponse)(nil),        // 28: protocol.DeleteScoresResponse
	(*UpdateScoresResponse)(nil),        // 29: protocol.UpdateScoresResponse
	(*AddTimeSeriesPointsResponse)(nil), // 30: protocol.AddTimeSeriesPointsResponse
	(*GetTimeSeriesPointsResponse)(nil), // 31: protocol.GetTimeSeriesPointsResponse
}
var file_cache_store_proto_depIdxs = []int32{
	0,  // 0: protocol.CacheStore.Get:input_type -> protocol.GetRequest
	1,  // 1: protocol.CacheStore.Set:input_type -> protocol.SetRequest
	2,  // 2: protocol.CacheStore.Delete:input_type -> protocol.DeleteRequest
	3,  // 3: protocol.CacheStore.GetSet:input_type -> protocol.GetSetRequest
	4,  // 4: protocol.CacheStore.SetSet:input_type -> protocol.SetSetRequest
	5,  // 5: protocol.CacheStore.AddSet:input_type -> protocol.AddSetRequest
	6,  // 6: protocol.CacheStore.RemSet:input_type -> protocol.RemSetRequest
	7,  // 7: protocol.CacheStore.Push:input_type -> protocol.PushRequest
	8,  // 8: protocol.CacheStore.Pop:input_type -> protocol.PopRequest
	9,  // 9: protocol.CacheStore.Remain:input_type -> protocol.RemainRequest
	10, // 10: protocol.CacheStore.AddScores:input_type -> protocol.AddScoresRequest
	11, // 11: protocol.CacheStore.SearchScores:input_type -> protocol.SearchScoresRequest
	12, // 12: protocol.CacheStore.DeleteScores:input_type -> protocol.DeleteScoresRequest
	13, // 13: protocol.CacheStore.UpdateScores:input_type -> protocol.UpdateScoresRequest
	14, // 14: protocol.CacheStore.AddTimeSeriesPoints:input_type -> protocol.AddTimeSeriesPointsRequest
	15, // 15: protocol.CacheStore.GetTimeSeriesPoints:input_type -> protocol.GetTimeSeriesPointsRequest
	16, // 16: protocol.CacheStore.Get:output_type -> protocol.GetResponse
	17, // 17: protocol.CacheStore.Set:output_type -> protocol.SetResponse
	18, // 18: protocol.CacheStore.Delete:output_type -> protocol.DeleteResponse
	19, // 19: protocol.CacheStore.GetSet:output_type -> protocol.GetSetResponse
	20, // 20: protocol.CacheStore.SetSet:output_type -> protocol.SetSetResponse
	21, // 21: protocol.CacheStore.AddSet:output_type -> protocol.AddSetResponse
	22, // 22: protocol.CacheStore.RemSet:output_type -> protocol.RemSetResponse
	23, // 23: protocol.CacheStore.Push:output_type -> protocol.PushResponse
	24, // 24: protocol.CacheStore.Pop:output_type -> protocol.PopResponse
	25, // 25: protocol.CacheStore.Remain:output_type -> protocol.RemainResponse
	26, // 26: protocol.CacheStore.AddScores:output_type -> protocol.AddScoresResponse
	27, // 27: protocol.CacheStore.SearchScores:output_type -> protocol.SearchScoresResponse
	28, // 28: protocol.CacheStore.DeleteScores:output_type -> protocol.DeleteScoresResponse
	29, // 29: protocol.CacheStore.UpdateScores:output_type -> protocol.UpdateScoresResponse
	30, // 30: protocol.CacheStore.AddTimeSeriesPoints:output_type -> protocol.AddTimeSeriesPointsResponse
	31, // 31: protocol.CacheStore.GetTimeSeriesPoints:output_type -> protocol.GetTimeSeriesPointsResponse
	16, // [16:32] is the sub-list for method output_type
	0,  // [0:16] is the sub-list for method input_type
	0,  // [0:0] is the sub-list for extension type_name
	0,  // [0:0] is the sub-list for extension extendee
	0,  // [0:0] is the sub-list for field type_name
}

func init() { file_cache_store_proto_init() }
func file_cache_store_proto_init() {
	if File_cache_store_proto != nil {
		return
	}
	file_protocol_proto_init()
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_cache_store_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   0,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_cache_store_proto_goTypes,
		DependencyIndexes: file_cache_store_proto_depIdxs,
	}.Build()
	File_cache_store_proto = out.File
	file_cache_store_proto_rawDesc = nil
	file_cache_store_proto_goTypes = nil
	file_cache_store_proto_depIdxs = nil
}
