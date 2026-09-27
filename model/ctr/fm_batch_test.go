//go:build !cgo || !xla

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

package ctr

import (
	"math/rand"
	"testing"

	"github.com/gorse-io/gorse/common/nn"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/model"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

// Compare against the former full-dataset tensor layout, including a partial
// final batch, variable feature counts, scaling, and missing embeddings.
func TestAFMBatchedTrainingMatchesFullTensors(t *testing.T) {
	t.Setenv("GODEBUG", "randseednop=0")
	for _, optimizer := range []string{model.Adam, model.SGD} {
		for _, autoScale := range []bool{false, true} {
			for _, embeddings := range []bool{false, true} {
				t.Run(optimizer+lo.Ternary(autoScale, "/scaled", "/raw")+lo.Ternary(embeddings, "/embeddings", "/no_embeddings"), func(t *testing.T) {
					d := newSynthesisDataset()
					d.Users = append(d.Users, d.Users[:3]...)
					d.Items = append(d.Items, d.Items[:3]...)
					d.Target = append(d.Target, d.Target[:3]...)
					d.UserLabels[0] = d.UserLabels[0][:1]
					if embeddings {
						d.ItemEmbeddings[1][0] = nil
						d.ItemEmbeddings[1][1] = d.ItemEmbeddings[1][1][:1]
					} else {
						d.ItemEmbeddingIndex = dataset.NewMapIndex()
						d.ItemEmbeddingDimension = nil
						d.ItemEmbeddings = nil
					}
					params := model.Params{model.NEpochs: 2, model.BatchSize: 3, model.AutoScale: autoScale, model.Optimizer: optimizer}
					rand.Seed(42)
					got := NewAFM(params)
					got.Fit(t.Context(), d, d, NewFitConfig().SetJobs(1).SetPatience(0))
					rand.Seed(42)
					want := NewAFM(params)
					want.Init(d)
					fitFullTensors(want, d)
					for i, parameter := range want.Parameters() {
						require.InDeltaSlice(t, parameter.Data(), got.Parameters()[i].Data(), 1e-6, "parameter %d", i)
					}
				})
			}
		}
	}
}

// Reference update loop using the old full-training-set input tensors.
func fitFullTensors(fm *AFM, d *Dataset) {
	x := make([]lo.Tuple2[[]int32, []float32], d.Count())
	e := make([][][]uint16, d.Count())
	y := make([]float32, d.Count())
	for i := range x {
		x[i].A, x[i].B, e[i], y[i] = d.Get(i)
	}
	indices, values, embeddings, target := fm.convertToTensors(fm.applyScalers(x), e, y)
	var optimizer nn.Optimizer
	if fm.optimizer == model.Adam {
		optimizer = nn.NewAdam(fm.Parameters(), fm.lr)
	} else {
		optimizer = nn.NewSGD(fm.Parameters(), fm.lr)
	}
	optimizer.SetWeightDecay(fm.reg)
	optimizer.SetJobs(1)
	for epoch := 0; epoch < fm.nEpochs; epoch++ {
		for i := 0; i < d.Count(); i += fm.batchSize {
			j := min(i+fm.batchSize, d.Count())
			batchE := make([]*nn.Tensor, len(embeddings))
			for k := range embeddings {
				batchE[k] = embeddings[k].Slice(i, j)
			}
			output := fm.Forward(indices.Slice(i, j), values.Slice(i, j), batchE, 1)
			loss := nn.BCEWithLogits(target.Slice(i, j), output, nil)
			optimizer.ZeroGrad()
			loss.Backward()
			optimizer.Step()
		}
	}
}

func TestAFMBatchedPredictionMatchesFullTensors(t *testing.T) {
	for _, autoScale := range []bool{false, true} {
		for _, withEmbeddings := range []bool{false, true} {
			d := newSynthesisDataset()
			if !withEmbeddings {
				d.ItemEmbeddingIndex = dataset.NewMapIndex()
				d.ItemEmbeddingDimension = nil
				d.ItemEmbeddings = nil
			}
			fm := NewAFM(model.Params{model.BatchSize: 3, model.AutoScale: autoScale})
			fm.Init(d)
			x := make([]lo.Tuple2[[]int32, []float32], 7)
			e := make([][][]uint16, len(x))
			for i := range x {
				x[i].A, x[i].B, e[i], _ = d.Get(i % d.Count())
			}
			// More features than training, in the final partial batch only.
			x[6].A = append(x[6].A, x[6].A...)
			x[6].B = append(x[6].B, x[6].B...)
			e[0] = nil
			if !withEmbeddings {
				e = nil
			}
			scaled := x
			if autoScale {
				scaled = fm.applyScalers(x)
			}
			indices, values, embeddings, _ := fm.convertToTensors(scaled, e, nil)
			var want []float32
			for i := 0; i < len(x); i += fm.batchSize {
				j := min(i+fm.batchSize, len(x))
				batchE := make([]*nn.Tensor, len(embeddings))
				for k := range embeddings {
					batchE[k] = embeddings[k].Slice(i, j)
				}
				want = append(want, fm.Forward(indices.Slice(i, j), values.Slice(i, j), batchE, 1).Data()...)
			}
			require.InDeltaSlice(t, want, fm.BatchInternalPredict(x, e, 1), 1e-5)
			require.Empty(t, fm.BatchInternalPredict(nil, nil, 1))
			// Scaling must not mutate the caller's features.
			_, original, _, _ := d.Get(0)
			require.Equal(t, original, x[0].B)
		}
	}
}
