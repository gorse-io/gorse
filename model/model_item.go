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
package model

import (
	"github.com/chewxy/math32"
	log "github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/floats"
)

type FISM struct {
	BaseItemBased
	// Model parameters
	P    [][]float32
	Q    [][]float32
	Bias []float32
	// Hyper parameters
	nFactors   int
	nEpochs    int
	lr         float32
	reg        float32
	initMean   float32
	initStdDev float32
	alpha      float32
}

func NewFISM(params Params) *FISM {
	f := new(FISM)
	f.SetParams(params)
	return f
}

func (f *FISM) SetParams(params Params) {
	f.BaseItemBased.SetParams(params)
	// Setup hyper-parameters
	f.nFactors = f.Params.GetInt(NFactors, 16)
	f.nEpochs = f.Params.GetInt(NEpochs, 100)
	f.lr = f.Params.GetFloat32(Lr, 0.0001)
	f.reg = f.Params.GetFloat32(Reg, 0.1)
	f.initMean = f.Params.GetFloat32(InitMean, 0)
	f.initStdDev = f.Params.GetFloat32(InitStdDev, 0.01)
	f.alpha = 0
}

func (f *FISM) GetParamsGrid() ParamsGrid {
	return ParamsGrid{}
}

func (f *FISM) Fit(trainSet *DataSet, validateSet *DataSet, config *config.FitConfig) Score {
	config = config.LoadDefaultIfNil()
	log.Infof("fit FISM with hyper-parameters: "+
		"n_factors = %v, n_epochs = %v, lr = %v, reg = %v, init_low = %v, init_high = %v",
		f.nFactors, f.nEpochs, f.lr, f.reg, f.initMean, f.initStdDev)
	f.Init(trainSet)
	// Create buffers
	x := make([]float32, f.nFactors)
	t := make([]float32, f.nFactors)
	a := make([]float32, f.nFactors)
	// Convert array to hashmap
	userFeedback := make([]map[int]interface{}, trainSet.UserCount())
	for u := range userFeedback {
		userFeedback[u] = make(map[int]interface{})
		for _, i := range trainSet.UserFeedback[u] {
			userFeedback[u][i] = nil
		}
	}
	// Training
	for epoch := 1; epoch <= f.nEpochs; epoch++ {
		// Training epoch
		cost := float32(0.0)
		for userIndex := 0; userIndex < trainSet.UserCount(); userIndex++ {
			c := math32.Pow(float32(len(trainSet.UserFeedback[userIndex]))-1, -f.alpha)
			for _, posIndex := range trainSet.UserFeedback[userIndex] {
				// Select a negative sample
				negIndex := -1
				for {
					temp := f.rng.Intn(trainSet.ItemCount())
					if _, exist := userFeedback[userIndex][temp]; !exist {
						negIndex = temp
						break
					}
				}
				// x <- 0
				floats.Zeros(x)
				// t <- (n^+_u - 1)^{-\alpha} \sum_{j \in R^+_u\{i}} p_j
				floats.Zeros(t)
				for _, j := range trainSet.UserFeedback[userIndex] {
					if j != posIndex {
						floats.MulConstAddTo(f.P[j], c, t)
					}
				}
				// for all j \in Z do
				// \tilde{r}_{ui} <- b_i + t * q^T_i
				rui := f.Bias[posIndex] + floats.Dot(t, f.Q[posIndex])
				// \tilde{r}_{uj} <- b_j + t * q^T_j
				ruj := f.Bias[negIndex] + floats.Dot(t, f.Q[negIndex])
				// e <- (r_{ui} - r_{uj}) - (\tilde r_{ui} - \tilde r_{uj})
				diff := (rui - ruj)
				cost += math32.Log(1 + math32.Exp(-diff))
				e := math32.Exp(-diff) / (1.0 + math32.Exp(-diff))
				// b_i <- b_i + lr * (e - reg * b_i)
				f.Bias[posIndex] += f.lr * (e - f.reg*f.Bias[posIndex])
				// b_j <- b_i - lr * (e - reg * b_j)
				f.Bias[negIndex] -= f.lr * (e - f.reg*f.Bias[negIndex])
				// q_i <- q_i + lr * (e * t - reg * q_i)
				floats.MulConstTo(t, e, a)
				floats.MulConstAddTo(f.Q[posIndex], -f.reg, a)
				floats.MulConstAddTo(a, f.lr, f.Q[posIndex])
				// q_j <- q_j + lr * (e * t - reg * q_j)
				floats.MulConstTo(t, e, a)
				floats.MulConstAddTo(f.Q[negIndex], -f.reg, a)
				floats.MulConstAddTo(a, -f.lr, f.Q[negIndex])
				// x <- x + e * (q_i - q_j)
				floats.SubTo(f.Q[posIndex], f.Q[negIndex], a)
				floats.MulConstAddTo(a, e, x)
				// for all j \in R^+_u\{i} do
				for _, j := range trainSet.UserFeedback[userIndex] {
					if j != posIndex {
						// p_j <- p_j + lr * ((n^+-U - 1)^-a \dot x - \beta \dot p_j)
						floats.MulConstTo(x, c, a)
						floats.MulConstAddTo(f.P[j], -f.reg, a)
						floats.MulConstAddTo(a, f.lr, f.P[j])
					}
				}
			}
		}
		// Cross validation
		if epoch%config.Verbose == 0 {
			scores := Evaluate(f, validateSet, trainSet, config.TopK, config.Candidates, NDCG, Precision, Recall)
			log.Infof("epoch %v/%v: loss=%v, NDCG@%v=%v, Precision@%v=%v, Recall@%v=%v",
				epoch, f.nEpochs, cost, config.TopK, scores[0], config.TopK, scores[1], config.TopK, scores[2])
		}
	}
	scores := Evaluate(f, validateSet, trainSet, config.TopK, config.Candidates, NDCG, Precision, Recall)
	return Score{NDCG: scores[0], Precision: scores[1], Recall: scores[2]}
}

func (f *FISM) Init(trainSet *DataSet) {
	// Initialize parameters
	newP := f.rng.NormalMatrix(trainSet.ItemCount(), f.nFactors, f.initMean, f.initStdDev)
	newQ := f.rng.NormalMatrix(trainSet.ItemCount(), f.nFactors, f.initMean, f.initStdDev)
	newBias := make([]float32, trainSet.ItemCount())
	// Relocate parameters
	if f.ItemIndex != nil {
		for _, itemId := range trainSet.ItemIndex.GetNames() {
			oldIndex := f.ItemIndex.ToNumber(itemId)
			newIndex := trainSet.ItemIndex.ToNumber(itemId)
			if oldIndex != base.NotId {
				newP[newIndex] = f.P[oldIndex]
				newQ[newIndex] = f.Q[oldIndex]
				newBias[newIndex] = f.Bias[oldIndex]
			}
		}
	}
	// Initialize base
	f.P = newP
	f.Q = newQ
	f.Bias = newBias
	f.BaseItemBased.Init(trainSet)
}

func (f *FISM) Predict(supportItems []string, itemId string) float32 {
	itemIndex := f.ItemIndex.ToNumber(itemId)
	if itemIndex == base.NotId {
		log.Warn("unknown item:", itemId)
		return 0
	}
	supportItemIndices := make([]int, len(supportItems))
	for i, itemId := range supportItems {
		supportItemIndices[i] = f.ItemIndex.ToNumber(itemId)
		if supportItemIndices[i] == base.NotId {
			log.Warn("unknown item:", itemId)
			return 0
		}
	}
	return f.InternalPredict(supportItemIndices, itemIndex)
}

func (f *FISM) InternalPredict(supportItems []int, itemId int) float32 {
	c := math32.Pow(float32(len(supportItems)), -f.alpha)
	sum := float32(0)
	for _, j := range supportItems {
		sum += floats.Dot(f.P[j], f.Q[itemId]) * c
	}
	return sum*c + f.Bias[itemId]
}

func (f *FISM) Clear() {
	f.P = nil
	f.Q = nil
	f.Bias = nil
}
