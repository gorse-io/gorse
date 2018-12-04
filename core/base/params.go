package base

// ParamName is a string.
type ParamName string

// Predefined parameter names
const (
	Lr            ParamName = "lr"
	Reg           ParamName = "reg"
	NEpochs       ParamName = "n_epochs"
	NFactors      ParamName = "n_factors"
	RandomState   ParamName = "random_state"
	UseBias       ParamName = "use_bias"
	InitMean      ParamName = "init_mean"
	InitStdDev    ParamName = "init_std_dev"
	InitLow       ParamName = "init_low"
	InitHigh      ParamName = "init_high"
	NUserClusters ParamName = "n_user_clusters"
	NItemClusters ParamName = "n_item_clusters"
	KNNType       ParamName = "knn_type"
)

// KNN types
type KNNTypeValue string

const (
	basic    KNNTypeValue = "basic"
	centered KNNTypeValue = "centered"
	zScore   KNNTypeValue = "z_score"
	baseline KNNTypeValue = "baseline"
)

// Params for an algorithm. Given by:
//  map[string]interface{}{
//     "<parameter name 1>": <parameter value 1>,
//     "<parameter name 2>": <parameter value 2>,
//     ...
//     "<parameter name n>": <parameter value n>,
//  }
type Params map[ParamName]interface{}

// Copy parameters.
func (parameters Params) Copy() Params {
	newParams := make(Params)
	for k, v := range parameters {
		newParams[k] = v
	}
	return newParams
}

// Get a integer parameter.
func (parameters Params) GetInt(name ParamName, _default int) int {
	if val, exist := parameters[name]; exist {
		return val.(int)
	}
	return _default
}

// Get a bool parameter.
func (parameters Params) GetBool(name ParamName, _default bool) bool {
	if val, exist := parameters[name]; exist {
		return val.(bool)
	}
	return _default
}

// Get a float parameter.
func (parameters Params) GetFloat64(name ParamName, _default float64) float64 {
	if val, exist := parameters[name]; exist {
		return val.(float64)
	}
	return _default
}

// Get a string parameter
func (parameters Params) GetString(name ParamName, _default string) string {
	if val, exist := parameters[name]; exist {
		return val.(string)
	}
	return _default
}

// Get a similarity function from parameters.
func (parameters Params) GetSim(name ParamName, _default Similarity) Similarity {
	if val, exist := parameters[name]; exist {
		return val.(Similarity)
	}
	return _default
}
