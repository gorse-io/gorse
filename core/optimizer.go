package core

import (
	"math"
	"math/rand"
)

// OptModel supports multiple optimizers.
type OptModel interface {
	Model
	// PointUpdate updates model parameters by points.
	PointUpdate(upGrad float64, innerUserId, innerItemId int)
	// PairUpdate updates model parameters by pairs.
	PairUpdate(upGrad float64, innerUserId, positiveItemId, negativeItemId int)
}

// Optimizer optimizes OptModel.
type Optimizer func(OptModel, TrainSet, int)

// SGDOptimizer optimizes a factor by SGD on square error.
func SGDOptimizer(model OptModel, trainSet TrainSet, nEpochs int) {
	for epoch := 0; epoch < nEpochs; epoch++ {
		for i := 0; i < trainSet.Length(); i++ {
			userId, itemId, rating := trainSet.Index(i)
			innerUserId := trainSet.ConvertUserId(userId)
			innerItemId := trainSet.ConvertItemId(itemId)
			// Compute error
			diff := rating - model.Predict(userId, itemId)
			// Point-wise update
			model.PointUpdate(diff, innerUserId, innerItemId)
		}
	}
}

// BPROptimizer optimizes a factor model by LearnBPR algorithm.
func BPROptimizer(model OptModel, trainSet TrainSet, nEpochs int) {
	positiveSet := make([]map[int]bool, trainSet.UserCount)
	pos := 0
	for u, b := range trainSet.UserRatings() {
		positiveSet[u] = make(map[int]bool)
		for _, d := range b {
			positiveSet[u][d.Id] = true
			pos++
		}
	}
	for epoch := 0; epoch < nEpochs; epoch++ {
		for i := 0; i < trainSet.Length(); i++ {
			// Select a positive
			index := rand.Intn(trainSet.Length())
			userId, posId, _ := trainSet.Index(index)
			innerUserId := trainSet.ConvertUserId(userId)
			innerPosId := trainSet.ConvertItemId(posId)
			// Select a negative
			negId := -1
			for {
				temp := rand.Intn(trainSet.ItemCount)
				if _, exist := positiveSet[innerUserId][temp]; !exist {
					negId = temp
					break
				}
			}
			outerNegId := trainSet.outerItemIds[negId]
			diff := model.Predict(userId, posId) - model.Predict(userId, outerNegId)
			grad := math.Exp(-diff) / (1.0 + math.Exp(-diff))
			// Pairwise update
			model.PairUpdate(grad, innerUserId, innerPosId, negId)
		}
	}
}
