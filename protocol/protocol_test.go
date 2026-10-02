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

package protocol

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestUpdateAtRoundTrip(t *testing.T) {
	updated := timestamppb.New(time.Date(2026, time.October, 2, 8, 39, 1, 123456789, time.UTC))
	for _, test := range []struct {
		name    string
		message proto.Message
	}{
		{"user", &User{UserId: "user", UpdateAt: updated}},
		{"item", &Item{ItemId: "item", Timestamp: timestamppb.New(time.Unix(1, 0)), UpdateAt: updated}},
		{"legacy_user", &User{UserId: "user"}},
		{"legacy_item", &Item{ItemId: "item"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, encoding := range []struct {
				name      string
				marshal   func(proto.Message) ([]byte, error)
				unmarshal func([]byte, proto.Message) error
			}{
				{"protobuf", proto.Marshal, proto.Unmarshal},
				{"json", protojson.Marshal, protojson.Unmarshal},
			} {
				t.Run(encoding.name, func(t *testing.T) {
					encoded, err := encoding.marshal(test.message)
					require.NoError(t, err)
					decoded := test.message.ProtoReflect().New().Interface()
					require.NoError(t, encoding.unmarshal(encoded, decoded))
					require.True(t, proto.Equal(test.message, decoded), "round trip changed message: %v", decoded)
				})
			}
		})
	}
}
