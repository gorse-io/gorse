package model

import (
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
)

type FM struct {
	Base
	// Model parameters
	GlobalBias   float64
	Bias         []float64
	Factors      [][]float64
	UserFeatures []*base.SparseVector
	ItemFeatures []*base.SparseVector
	// Hyper parameters
	useBias    bool
	nFactors   int
	nEpochs    int
	lr         float64
	reg        float64
	initMean   float64
	initStdDev float64
}

func NewFM(params base.Params) *FM {
	fm := new(FM)
	fm.SetParams(params)
	return fm
}

func (fm *FM) SetParams(params base.Params) {
	fm.Base.SetParams(params)
	// Setup hyper-parameters
	fm.useBias = fm.Params.GetBool(base.UseBias, true)
	fm.nFactors = fm.Params.GetInt(base.NFactors, 100)
	fm.nEpochs = fm.Params.GetInt(base.NEpochs, 20)
	fm.lr = fm.Params.GetFloat64(base.Lr, 0.005)
	fm.reg = fm.Params.GetFloat64(base.Reg, 0.02)
	fm.initMean = fm.Params.GetFloat64(base.InitMean, 0)
	fm.initStdDev = fm.Params.GetFloat64(base.InitStdDev, 0.1)
}

func (fm *FM) Predict(userId int, itemId int) float64 {
	// Convert sparse IDs to dense IDs
	userIndex := fm.UserIndexer.ToIndex(userId)
	itemIndex := fm.ItemIndexer.ToIndex(itemId)
	return fm.predict(userIndex, itemIndex)
}

func (fm *FM) encode(userIndex int, itemIndex int) *base.SparseVector {
	//userFeature := fm.UserFeatures[userIndex]
	//itemFeature := fm.ItemFeatures[itemIndex]
	return nil
}

func (fm *FM) predict(userIndex int, itemIndex int) float64 {
	return 0
}

func (fm *FM) Fit(trainSet core.DataSetInterface, options *base.RuntimeOptions) {
	fm.Init(trainSet)
	fm.UserFeatures = trainSet.UserFeatures()
	fm.ItemFeatures = trainSet.ItemFeatures()
	// Initialization
	paramCount := trainSet.UserCount() + trainSet.ItemCount() + trainSet.FeatureCount()
	fm.GlobalBias = trainSet.GlobalMean()
	fm.Bias = make([]float64, paramCount)
	fm.Factors = fm.rng.NewNormalMatrix(paramCount, fm.nFactors, fm.initMean, fm.initStdDev)
}
