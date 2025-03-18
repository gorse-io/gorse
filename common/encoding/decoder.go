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
	"io"

	"github.com/zhenghaoz/gorse/base/log"
	"github.com/zhenghaoz/gorse/model/cf"
	"github.com/zhenghaoz/gorse/model/ctr"
	"github.com/zhenghaoz/gorse/protocol"
	"go.uber.org/zap"
)

// UnmarshalClickModel unmarshal click model from gRPC.
func UnmarshalClickModel(receiver protocol.Master_GetClickModelClient) (ctr.FactorizationMachine, error) {
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
	model, err := ctr.UnmarshalModel(reader)
	if err != nil {
		return nil, err
	}
	if finalError != nil {
		return nil, finalError
	}
	return model, nil
}

// UnmarshalRankingModel unmarshal ranking model from gRPC.
func UnmarshalRankingModel(receiver protocol.Master_GetRankingModelClient) (cf.MatrixFactorization, error) {
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
	model, err := cf.UnmarshalModel(reader)
	if err != nil {
		return nil, err
	}
	if receiverError != nil {
		return nil, receiverError
	}
	return model, nil
}
