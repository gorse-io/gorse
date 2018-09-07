package core

type Algorithm interface {
	Predict(userId int, itemId int) float64
	Fit(trainSet TrainSet, options Options)
}

type Options struct {
	options map[string]interface{}
	//reg           float64
	//regUserFactor float64
	//regItemFactor float64
	//lr            float64
	//lrUserFactor  float64
	//lrItemFactor  float64
	//nEpochs       int
	//nFactors      int
	//biased        bool
	//initMean      float64
	//initStdDev    float64
	//initLow       float64
	//initHigh      float64
	//// Neighborhood-based recommendation method
	//k         int
	//minK      int
	//userBased bool
	//sim       Sim
	//nUserClusters int
	//nItemClusters int
}

func NewOptions(options map[string]interface{}) Options {
	return Options{options: options}
}

func DefaultOptions() Options {
	return Options{}
}

func (options *Options) GetInt(name string, _default int) int {
	if val, exist := options.options[name]; exist {
		return val.(int)
	}
	return _default
}

func (options *Options) GetBool(name string, _default bool) bool {
	if val, exist := options.options[name]; exist {
		return val.(bool)
	}
	return _default
}

func (options *Options) GetFloat64(name string, _default float64) float64 {
	if val, exist := options.options[name]; exist {
		return val.(float64)
	}
	return _default
}

func (options *Options) GetSim(name string, _default Sim) Sim {
	if val, exist := options.options[name]; exist {
		return val.(Sim)
	}
	return _default
}
