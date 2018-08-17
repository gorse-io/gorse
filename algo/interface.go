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
	nFactors       int
}

func Copy(dst []float64, src []float64) []float64 {
	copy(dst, src)
	return dst
}

func MulConst(c float64, dst []float64) []float64 {
	for i := 0; i < len(dst); i++ {
		dst[i] *= c
	}
	return dst
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

func SetNFactors(nFactors int) OptionEditor {
	return func(option *Option) {
		option.nFactors = nFactors
	}
}
