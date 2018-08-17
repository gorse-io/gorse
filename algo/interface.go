package algo

import "../data"

type Algorithm interface {
	Predict(userId int, itemId int) float64
	Fit(trainSet data.Set, options ...OptionEditor)
}

type Option struct {
	regularization float64
	learningRate   float64
	nEpoch         int
}

type OptionEditor func(*Option)

func SetLearningRate(learningRate float64) OptionEditor {
	return func(options *Option) {
		options.learningRate = learningRate
	}
}

func SetRegularization(regularization float64) OptionEditor {
	return func(options *Option) {
		options.regularization = regularization
	}
}

func SetNEpoch(nEpoch int) OptionEditor {
	return func(option *Option) {
		option.nEpoch = nEpoch
	}
}
