package core

// Algorithm

type Algorithm interface {
	Predict(userId, itemId int) float64
	Fit(trainSet TrainSet, params Parameters)
}

// Parameters

type Parameters map[string]interface{}

type parameterReader struct {
	parameters map[string]interface{}
}

func newParameterReader(options map[string]interface{}) parameterReader {
	return parameterReader{parameters: options}
}

func (options *parameterReader) getInt(name string, _default int) int {
	if val, exist := options.parameters[name]; exist {
		return val.(int)
	}
	return _default
}

func (options *parameterReader) getBool(name string, _default bool) bool {
	if val, exist := options.parameters[name]; exist {
		return val.(bool)
	}
	return _default
}

func (options *parameterReader) getFloat64(name string, _default float64) float64 {
	if val, exist := options.parameters[name]; exist {
		return val.(float64)
	}
	return _default
}

func (options *parameterReader) getSim(name string, _default Sim) Sim {
	if val, exist := options.parameters[name]; exist {
		return val.(Sim)
	}
	return _default
}
