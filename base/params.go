package base

import (
	"log"
	"reflect"
)

/* ParamName */

// ParamName is the type of hyper-parameter names.
type ParamName string

// Predefined hyper-parameter names
const (
	Lr            ParamName = "Lr"            // learning rate
	Reg           ParamName = "Reg"           // regularization strength
	NEpochs       ParamName = "NEpochs"       // number of epochs
	NFactors      ParamName = "NFactors"      // number of factors
	RandomState   ParamName = "RandomState"   // random state (seed)
	UseBias       ParamName = "UseBias"       // use bias
	InitMean      ParamName = "InitMean"      // mean of gaussian initial parameter
	InitStdDev    ParamName = "InitStdDev"    // standard deviation of gaussian initial parameter
	InitLow       ParamName = "InitLow"       // lower bound of uniform initial parameter
	InitHigh      ParamName = "InitHigh"      // upper bound of uniform initial parameter
	NUserClusters ParamName = "NUserClusters" // number of user cluster
	NItemClusters ParamName = "NItemClusters" // number of item cluster
	Type          ParamName = "Type"          // type for KNN
	UserBased     ParamName = "UserBased"     // user based if true. otherwise item based.
	Similarity    ParamName = "Similarity"    // similarity metrics
	K             ParamName = "K"             // number of neighbors
	MinK          ParamName = "MinK"          // least number of neighbors
	Optimizer     ParamName = "Optimizer"     // optimizer for optimization (SGD/ALS/BPR)
	Shrinkage     ParamName = "Shrinkage"     // shrinkage strength of similarity
	Alpha         ParamName = "Alpha"         // alpha value, depend on context
)

/* ParamString */

// Predefined values for hyper-parameter Type.
const (
	Basic    string = "basic"    // Basic KNN
	Centered string = "Centered" // KNN with centered ratings
	ZScore   string = "ZScore"   // KNN with standardized ratings
	Baseline string = "Baseline" // KNN with baseline ratings
)

// Predefined values for hyper-parameter Optimizer.
const (
	SGD string = "sgd" // Fit model (MF) with stochastic gradient descent.
	BPR string = "bpr" // Fit model (MF) with bayesian personal ranking.
)

// Predefined values for hyper-parameter Similarity.
const (
	Pearson string = "Pearson" // Pearson similarity
	Cosine  string = "Cosine"  // Cosine similarity
	MSD     string = "MSD"     // MSD similarity
)

// Params stores hyper-parameters for an model. It is a map between strings
// (names) and interface{}s (values). For example, hyper-parameters for SVD
// is given by:
//  base.Params{
//		base.Lr:       0.007,
//		base.NEpochs:  100,
//		base.NFactors: 80,
//		base.Reg:      0.1,
//	}
type Params map[ParamName]interface{}

// Copy hyper-parameters.
func (parameters Params) Copy() Params {
	newParams := make(Params)
	for k, v := range parameters {
		newParams[k] = v
	}
	return newParams
}

// GetInt gets a integer parameter by name. Returns _default if not exists or type doesn't match.
func (parameters Params) GetInt(name ParamName, _default int) int {
	if val, exist := parameters[name]; exist {
		switch val.(type) {
		case int:
			return val.(int)
		default:
			log.Printf("Expect %v to be int, but get %v", name, reflect.TypeOf(name))
		}
	}
	return _default
}

// GetInt64 gets a int64 parameter by name. Returns _default if not exists or type doesn't match. The
// type will be converted if given int.
func (parameters Params) GetInt64(name ParamName, _default int64) int64 {
	if val, exist := parameters[name]; exist {
		switch val.(type) {
		case int64:
			return val.(int64)
		case int:
			return int64(val.(int))
		default:
			log.Printf("Expect %v to be int, but get %v", name, reflect.TypeOf(name))
		}
	}
	return _default
}

// GetBool gets a bool parameter by name. Returns _default if not exists or type doesn't match.
func (parameters Params) GetBool(name ParamName, _default bool) bool {
	if val, exist := parameters[name]; exist {
		switch val.(type) {
		case bool:
			return val.(bool)
		default:
			log.Printf("Expect %v to be int, but get %v", name, reflect.TypeOf(name))
		}
	}
	return _default
}

// GetFloat64 gets a float parameter by name. Returns _default if not exists or type doesn't match. The
// type will be converted if given int.
func (parameters Params) GetFloat64(name ParamName, _default float64) float64 {
	if val, exist := parameters[name]; exist {
		switch val.(type) {
		case float64:
			return val.(float64)
		case int:
			return float64(val.(int))
		default:
			log.Printf("Expect %v to be int, but get %v", name, reflect.TypeOf(name))
		}
	}
	return _default
}

// GetString gets a string parameter. Returns _default if not exists or type doesn't match.
func (parameters Params) GetString(name ParamName, _default string) string {
	if val, exist := parameters[name]; exist {
		switch val.(type) {
		case string:
			return val.(string)
		default:
			log.Printf("Expect %v to be string, but get %v", name, reflect.TypeOf(name))
		}
	}
	return _default
}

// Merge another group of hyper-parameters to current group of hyper-parameters.
func (parameters Params) Merge(params Params) {
	for k, v := range params {
		parameters[k] = v
	}
}
