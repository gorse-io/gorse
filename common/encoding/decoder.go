// Copyright 2021 gorse Project Authors
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

package encoding

import (
	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"github.com/zhenghaoz/gorse/protocol"
	"go.uber.org/zap"
	"io"
)

// UnmarshalClickModel unmarshal click model from gRPC.
func UnmarshalClickModel(receiver protocol.Master_GetClickModelClient) (click.FactorizationMachine, error) {
	// receive model
	reader, writer := io.Pipe()
	var finalError error
	go func() {
		defer func(writer *io.PipeWriter) {
			err := writer.Close()
			if err != nil {
				log.Logger().Error("fail to close pipe", zap.Error(err))
			}
		}(writer)
		for {
			// receive from stream
			fragment, err := receiver.Recv()
			if err == io.EOF {
				log.Logger().Info("complete receiving click model")
				break
			} else if err != nil {
				finalError = err
				log.Logger().Error("fail to receive stream", zap.Error(err))
				return
			}
			// send to pipe
			_, err = writer.Write(fragment.Data)
			if err != nil {
				finalError = err
				log.Logger().Error("fail to write pipe", zap.Error(err))
				return
			}
		}
	}()
	// unmarshal model
	model, err := click.UnmarshalModel(reader)
	if err != nil {
		return nil, err
	}
	if finalError != nil {
		return nil, finalError
	}
	return model, nil
}

// UnmarshalRankingModel unmarshal ranking model from gRPC.
func UnmarshalRankingModel(receiver protocol.Master_GetRankingModelClient) (ranking.MatrixFactorization, error) {
	// receive model
	reader, writer := io.Pipe()
	var receiverError error
	go func() {
		defer func(writer *io.PipeWriter) {
			err := writer.Close()
			if err != nil {
				log.Logger().Error("fail to close pipe", zap.Error(err))
			}
		}(writer)
		for {
			// receive from stream
			fragment, err := receiver.Recv()
			if err == io.EOF {
				log.Logger().Info("complete receiving ranking model")
				break
			} else if err != nil {
				receiverError = err
				log.Logger().Error("fail to receive stream", zap.Error(err))
				return
			}
			// send to pipe
			_, err = writer.Write(fragment.Data)
			if err != nil {
				receiverError = err
				log.Logger().Error("fail to write pipe", zap.Error(err))
				return
			}
		}
	}()
	// unmarshal model
	model, err := ranking.UnmarshalModel(reader)
	if err != nil {
		return nil, err
	}
	if receiverError != nil {
		return nil, receiverError
	}
	return model, nil
}
