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

package protocol

import (
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"go.uber.org/zap"
	"io"
)

// UnmarshalIndex unmarshal index from gRPC.
func UnmarshalIndex(receiver Master_GetUserIndexClient) (base.Index, error) {
	// receive model
	reader, writer := io.Pipe()
	var receiverError error
	go func() {
		defer func(writer *io.PipeWriter) {
			err := writer.Close()
			if err != nil {
				base.Logger().Error("fail to close pipe", zap.Error(err))
			}
		}(writer)
		for {
			// receive from stream
			fragment, err := receiver.Recv()
			if err == io.EOF {
				base.Logger().Info("complete receiving user index")
				break
			} else if err != nil {
				receiverError = err
				base.Logger().Error("fail to receive stream", zap.Error(err))
				return
			}
			// send to pipe
			_, err = writer.Write(fragment.Data)
			if err != nil {
				receiverError = err
				base.Logger().Error("fail to write pipe", zap.Error(err))
				return
			}
		}
	}()
	// unmarshal model
	index, err := base.UnmarshalIndex(reader)
	if err != nil {
		return nil, err
	}
	if receiverError != nil {
		return nil, receiverError
	}
	return index, nil
}

// UnmarshalClickModel unmarshal click model from gRPC.
func UnmarshalClickModel(receiver Master_GetClickModelClient) (click.FactorizationMachine, error) {
	// receive model
	reader, writer := io.Pipe()
	var finalError error
	go func() {
		defer func(writer *io.PipeWriter) {
			err := writer.Close()
			if err != nil {
				base.Logger().Error("fail to close pipe", zap.Error(err))
			}
		}(writer)
		for {
			// receive from stream
			fragment, err := receiver.Recv()
			if err == io.EOF {
				base.Logger().Info("complete receiving click model")
				break
			} else if err != nil {
				finalError = err
				base.Logger().Error("fail to receive stream", zap.Error(err))
				return
			}
			// send to pipe
			_, err = writer.Write(fragment.Data)
			if err != nil {
				finalError = err
				base.Logger().Error("fail to write pipe", zap.Error(err))
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
func UnmarshalRankingModel(receiver Master_GetRankingModelClient) (ranking.MatrixFactorization, error) {
	// receive model
	reader, writer := io.Pipe()
	var receiverError error
	go func() {
		defer func(writer *io.PipeWriter) {
			err := writer.Close()
			if err != nil {
				base.Logger().Error("fail to close pipe", zap.Error(err))
			}
		}(writer)
		for {
			// receive from stream
			fragment, err := receiver.Recv()
			if err == io.EOF {
				base.Logger().Info("complete receiving ranking model")
				break
			} else if err != nil {
				receiverError = err
				base.Logger().Error("fail to receive stream", zap.Error(err))
				return
			}
			// send to pipe
			_, err = writer.Write(fragment.Data)
			if err != nil {
				receiverError = err
				base.Logger().Error("fail to write pipe", zap.Error(err))
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
