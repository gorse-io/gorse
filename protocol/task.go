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
	"time"

	"github.com/gorse-io/gorse/base/progress"
)

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative cache_store.proto
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative data_store.proto
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative encoding.proto
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative protocol.proto

func DecodeProgress(in *PushProgressRequest) []progress.Progress {
	var progressList []progress.Progress
	for _, p := range in.Progress {
		progressList = append(progressList, progress.Progress{
			Tracer:     p.GetTracer(),
			Name:       p.GetName(),
			Status:     progress.Status(p.GetStatus()),
			Count:      int(p.GetCount()),
			Total:      int(p.GetTotal()),
			StartTime:  time.UnixMilli(p.GetStartTime()),
			FinishTime: time.UnixMilli(p.GetFinishTime()),
		})
	}
	return progressList
}

func EncodeProgress(progressList []progress.Progress) *PushProgressRequest {
	var pbList []*Progress
	for _, p := range progressList {
		pbList = append(pbList, &Progress{
			Tracer:     p.Tracer,
			Name:       p.Name,
			Status:     string(p.Status),
			Count:      int64(p.Count),
			Total:      int64(p.Total),
			StartTime:  p.StartTime.UnixMilli(),
			FinishTime: p.FinishTime.UnixMilli(),
		})
	}
	return &PushProgressRequest{
		Progress: pbList,
	}
}
