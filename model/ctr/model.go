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

package ctr

import (
	"context"
	"fmt"
	"io"
	"reflect"

	"github.com/gorse-io/gorse/common/encoding"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/model"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type Score struct {
	RMSE      float32
	Precision float32
	Recall    float32
	Accuracy  float32
	AUC       float32
}

func (score Score) ZapFields() []zap.Field {
	return []zap.Field{
		zap.Float32("Accuracy", score.Accuracy),
		zap.Float32("Precision", score.Precision),
		zap.Float32("Recall", score.Recall),
		zap.Float32("AUC", score.AUC),
	}
}

func (score Score) GetValue() float32 {
	return score.Precision
}

func (score Score) BetterThan(s Score) bool {
	return score.AUC > s.AUC
}

type FitConfig struct {
	Jobs     int
	Verbose  int
	Patience int
}

func NewFitConfig() *FitConfig {
	return &FitConfig{
		Jobs:     1,
		Verbose:  10,
		Patience: 10,
	}
}

func (config *FitConfig) SetVerbose(verbose int) *FitConfig {
	config.Verbose = verbose
	return config
}

func (config *FitConfig) SetJobs(jobs int) *FitConfig {
	config.Jobs = jobs
	return config
}

func (config *FitConfig) SetPatience(patience int) *FitConfig {
	config.Patience = patience
	return config
}

func (config *FitConfig) LoadDefaultIfNil() *FitConfig {
	if config == nil {
		return NewFitConfig()
	}
	return config
}

type FactorizationMachines interface {
	model.Model
	Predict(userId, itemId string, userFeatures, itemFeatures []Label) float32
	InternalPredict(x []int32, values []float32) float32
	Fit(ctx context.Context, trainSet, testSet dataset.CTRSplit, config *FitConfig) Score
	Marshal(w io.Writer) error
}

type BatchInference interface {
	BatchPredict(inputs []lo.Tuple4[string, string, []Label, []Label], e [][]Embedding, jobs int) []float32
	BatchInternalPredict(x []lo.Tuple2[[]int32, []float32], e [][][]float32, jobs int) []float32
}

type BaseFactorizationMachines struct {
	model.BaseModel
	Index dataset.UnifiedIndex
}

func (b *BaseFactorizationMachines) Init(trainSet dataset.CTRSplit) {
	b.Index = trainSet.GetIndex()
}

func MarshalModel(w io.Writer, m FactorizationMachines) error {
	// write header
	var err error
	switch m.(type) {
	case *AFM:
		err = encoding.WriteString(w, headerAFM)
	default:
		return fmt.Errorf("unknown model: %v", reflect.TypeOf(m))
	}
	if err != nil {
		return err
	}
	return m.Marshal(w)
}

func UnmarshalModel(r io.Reader) (FactorizationMachines, error) {
	// read header
	header, err := encoding.ReadString(r)
	if err != nil {
		return nil, err
	}
	switch header {
	case headerAFM:
		var fm AFM
		if err := fm.Unmarshal(r); err != nil {
			return nil, errors.Trace(err)
		}
		return &fm, nil
	}
	return nil, fmt.Errorf("unknown model: %v", header)
}

