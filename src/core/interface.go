package core

type Algorithm interface {
	Predict(userId int, itemId int) float64
	Fit(trainSet TrainSet, options ...OptionSetter)
}

type Option struct {
	reg           float64
	regUserFactor float64
	regItemFactor float64
	lr            float64
	lrUserFactor  float64
	lrItemFactor  float64
	nEpochs       int
	nFactors      int
	biased        bool
	initMean      float64
	initStdDev    float64
	initLow       float64
	initHigh      float64
	// Neighborhood-based recommendation method
	k         int
	minK      int
	userBased bool
	sim       Sim
}

type OptionSetter func(*Option)

func SetLR(learningRate float64) OptionSetter {
	return func(options *Option) {
		options.lr = learningRate
	}
}

func SetReg(regularization float64) OptionSetter {
	return func(options *Option) {
		options.reg = regularization
	}
}

func SetNEpoch(nEpoch int) OptionSetter {
	return func(option *Option) {
		option.nEpochs = nEpoch
	}
}

func SetNFactors(nFactors int) OptionSetter {
	return func(option *Option) {
		option.nFactors = nFactors
	}
}
