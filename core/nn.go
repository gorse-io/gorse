package core

import (
	"gorgonia.org/gorgonia"
	"gorgonia.org/tensor"
)

// AutoRec: Autoencoders meet collaborative filtering [8]
type AutoRec struct {
	Base
	// Model parameters
	g       *gorgonia.ExprGraph
	in, out *gorgonia.Node
	// Hyper parameters
	bias       bool
	nFactors   int
	nEpochs    int
	lr         float64
	reg        float64
	initMean   float64
	initStdDev float64
	// Cache
	inputCache  []*tensor.Dense
	outputCache [][]float64
}

func NewAutoRec(params Parameters) *AutoRec {
	auto := new(AutoRec)
	auto.SetParams(params)
	return auto
}

func (auto *AutoRec) SetParams(params Parameters) {
	auto.Base.SetParams(params)
	auto.bias = auto.Params.GetBool("bias", true)
	auto.nFactors = auto.Params.GetInt("nFactors", 50)
	auto.nEpochs = auto.Params.GetInt("nEpochs", 3)
	auto.lr = auto.Params.GetFloat64("lr", 0.001)
	auto.reg = auto.Params.GetFloat64("reg", 0.01)
	auto.initMean = auto.Params.GetFloat64("initMean", 0)
	auto.initStdDev = auto.Params.GetFloat64("initStdDev", 0.1)
}

func (auto *AutoRec) Predict(userId int, itemId int) float64 {
	innerUserId := auto.Data.ConvertUserId(userId)
	innerItemId := auto.Data.ConvertItemId(itemId)
	if innerUserId == NewId || innerItemId == NewId {
		return auto.Data.GlobalMean
	}
	if auto.outputCache[innerItemId] != nil {
		return auto.outputCache[innerItemId][innerUserId]
	}
	vm := gorgonia.NewTapeMachine(auto.g)
	if err := gorgonia.Let(auto.in, auto.inputCache[innerItemId]); err != nil {
		panic(err)
	}
	if err := vm.RunAll(); err != nil {
		panic(err)
	}
	auto.outputCache[innerItemId] = auto.out.Value().Data().([]float64)
	return auto.outputCache[innerItemId][innerUserId]
}

func (auto *AutoRec) Fit(set TrainSet) {
	auto.Base.Fit(set)
	// Reset cache
	auto.inputCache = make([]*tensor.Dense, set.ItemCount)
	auto.outputCache = make([][]float64, set.ItemCount)
	for i := range auto.inputCache {
		inputVector := encode(set.UserCount, set.ItemRatings()[i])
		auto.inputCache[i] = tensor.New(tensor.WithShape(auto.Data.UserCount), tensor.WithBacking(inputVector))
	}
	// Build auto decoder & encoder
	auto.g = gorgonia.NewGraph()
	auto.in = gorgonia.NewVector(auto.g, gorgonia.Float64,
		gorgonia.WithShape(set.UserCount))
	b1 := gorgonia.NewVector(auto.g, gorgonia.Float64,
		gorgonia.WithShape(auto.nFactors),
		gorgonia.WithInit(gorgonia.Zeroes()))
	//b2 := gorgonia.NewVector(auto.g, gorgonia.Float64,
	//	gorgonia.WithShape(set.UserCount),
	//	gorgonia.WithInit(gorgonia.Zeroes()))
	w1 := gorgonia.NewMatrix(auto.g, gorgonia.Float64,
		gorgonia.WithShape(set.UserCount, auto.nFactors),
		gorgonia.WithInit(gorgonia.Gaussian(auto.initMean, auto.initStdDev)))
	w2 := gorgonia.NewMatrix(auto.g, gorgonia.Float64,
		gorgonia.WithShape(auto.nFactors, set.UserCount),
		gorgonia.WithInit(gorgonia.Gaussian(auto.initMean, auto.initStdDev)))
	hidden := gorgonia.Must(gorgonia.Sigmoid(
		gorgonia.Must(gorgonia.Add(
			gorgonia.Must(gorgonia.Mul(auto.in, w1)), b1))))
	auto.out = gorgonia.Must(gorgonia.Mul(hidden, w2))
	parameters := gorgonia.Nodes{w1, w2, b1}
	// Cost function: \sum^n_{i=1} || r^i - h(r^i;\theta) ||^2_O
	cost := gorgonia.Must(gorgonia.Gt(auto.in, gorgonia.NewConstant(0.0), true))
	cost = gorgonia.Must(gorgonia.Sum(gorgonia.Must(gorgonia.Mul(cost, gorgonia.Must(gorgonia.Square(gorgonia.Must(gorgonia.Sub(auto.in, auto.out))))))))
	// + ||W||_F^2
	cost = gorgonia.Must(gorgonia.Add(cost,
		gorgonia.Must(gorgonia.Mul(
			gorgonia.NewConstant(auto.reg),
			gorgonia.Must(gorgonia.Sum(
				gorgonia.Must(gorgonia.Square(w1))))))))
	// + ||V||_F^2
	cost = gorgonia.Must(gorgonia.Add(cost,
		gorgonia.Must(gorgonia.Mul(
			gorgonia.NewConstant(auto.reg),
			gorgonia.Must(gorgonia.Sum(
				gorgonia.Must(gorgonia.Square(w2))))))))
	// Training neural network
	if _, err := gorgonia.Grad(cost, parameters...); err != nil {
		panic(err)
	}
	solver := gorgonia.NewRMSPropSolver(gorgonia.WithLearnRate(auto.lr))
	vm := gorgonia.NewTapeMachine(auto.g,
		gorgonia.BindDualValues(parameters...),
		gorgonia.UseCudaFor("mul", "add", "sigmoid"))
	for ep := 0; ep < auto.nEpochs; ep++ {
		for i := range auto.inputCache {
			if err := gorgonia.Let(auto.in, auto.inputCache[i]); err != nil {
				panic(err)
			}
			if err := vm.RunAll(); err != nil {
				panic(err)
			}
			if err := solver.Step(gorgonia.NodesToValueGrads(parameters)); err != nil {
				panic(err)
			}
			vm.Reset()
		}
	}
}

func encode(count int, idRatings []IdRating) []float64 {
	code := make([]float64, count)
	for _, ir := range idRatings {
		code[ir.Id] = ir.Rating
	}
	return code
}

type RBM struct {
	Base
	hidden  []float64
	nFactor int
}

func NewRBM(params Parameters) *RBM {
	rbm := new(RBM)
	rbm.SetParams(params)
	return rbm
}

func (rbm *RBM) SetParams(params Parameters) {
	rbm.nFactor = rbm.Params.GetInt("nFactors", 10)
}

func (rbm *RBM) Fit(set TrainSet) {
	rbm.hidden = make([]float64, rbm.nFactor)
}
