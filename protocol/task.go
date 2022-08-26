// Copyright 2022 gorse Project Authors
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
	"github.com/zhenghaoz/gorse/base/task"
	"time"
)

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative protocol.proto

func DecodeTask(in *PushTaskInfoRequest) *task.Task {
	return &task.Task{
		Name:       in.GetName(),
		Status:     task.Status(in.GetStatus()),
		Done:       int(in.GetDone()),
		Total:      int(in.GetTotal()),
		StartTime:  time.UnixMilli(in.GetStartTime()),
		FinishTime: time.UnixMilli(in.GetFinishTime()),
	}
}

func EncodeTask(t *task.Task) *PushTaskInfoRequest {
	return &PushTaskInfoRequest{
		Name:       t.Name,
		Status:     string(t.Status),
		Done:       int64(t.Done),
		Total:      int64(t.Total),
		StartTime:  t.StartTime.UnixMilli(),
		FinishTime: t.FinishTime.UnixMilli(),
	}
}
