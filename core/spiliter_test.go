package core

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSplit(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	train, test := Split(data, 0.2)
	assert.Equal(t, 80000, train.Count())
	assert.Equal(t, 20000, test.Count())
}

func TestKFoldSplitter(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	kfold := NewKFoldSplitter(5)
	trains, tests := kfold(data, 0)
	for i := range trains {
		assert.Equal(t, 80000, trains[i].Count())
		assert.Equal(t, 20000, tests[i].Count())
	}
	// Check nil
	nilTrains, nilTests := kfold(nil, 0)
	for i := range nilTrains {
		assert.Equal(t, nil, nilTrains[i])
		assert.Equal(t, nil, nilTests[i])
	}
}

func TestRatioSplitter(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	ratio := NewRatioSplitter(1, 0.2)
	trains, tests := ratio(data, 0)
	assert.Equal(t, 80000, trains[0].Count())
	assert.Equal(t, 20000, tests[0].Count())
	// Check nil
	nilTrains, nilTests := ratio(nil, 0)
	for i := range nilTrains {
		assert.Equal(t, nil, nilTrains[i])
		assert.Equal(t, nil, nilTests[i])
	}
}

func TestUserLOOSplitter(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	loo := NewUserLOOSplitter(1)
	_, tests := loo(data, 0)
	for i := 0; i < tests[0].UserCount(); i++ {
		ratings := tests[0].UserByIndex(i)
		assert.Equal(t, 1, ratings.Len())
	}
	// Check nil
	nilTrains, nilTests := loo(nil, 0)
	for i := range nilTrains {
		assert.Equal(t, nil, nilTrains[i])
		assert.Equal(t, nil, nilTests[i])
	}
}
