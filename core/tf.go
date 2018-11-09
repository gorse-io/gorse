// +build tf

package core

// AutoRec: Autoencoders meet collaborative filtering [8]
type AutoRec struct {
	Base
	// Hyper parameters
	bias       bool
	batchSize  int
	nFactors   int
	nEpochs    int
	lr         float64
	reg        float64
	initMean   float64
	initStdDev float64
	userBased  bool
	verbose    bool
}

func NewAutoRec(params Parameters) *AutoRec {
	auto := new(AutoRec)
	auto.SetParams(params)
	return auto
}

func (auto *AutoRec) SetParams(params Parameters) {
	auto.Base.SetParams(params)
	auto.bias = auto.Params.GetBool("bias", true)
	auto.batchSize = auto.Params.GetInt("batchSize", 500)
	auto.nFactors = auto.Params.GetInt("nFactors", 50)
	auto.nEpochs = auto.Params.GetInt("nEpochs", 50)
	auto.lr = auto.Params.GetFloat64("lr", 1e-3)
	auto.reg = auto.Params.GetFloat64("reg", 0.05)
	auto.initMean = auto.Params.GetFloat64("initMean", 0)
	auto.initStdDev = auto.Params.GetFloat64("initStdDev", 0.03)
	auto.userBased = auto.Params.GetBool("userBased", false)
	auto.verbose = auto.Params.GetBool("verbose", true)
}
