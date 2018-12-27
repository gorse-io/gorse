package core

import (
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
)

func TestKFoldSplitter(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	kfold := NewKFoldSplitter(5)
	trains, tests := kfold(data, 0)
	for i := range trains {
		assert.Equal(t, 80000, trains[i].Len())
		assert.Equal(t, 20000, tests[i].Len())
	}
}

func TestRatioSplitter(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	ratio := NewRatioSplitter(1, 0.2)
	trains, tests := ratio(data, 0)
	assert.Equal(t, 80000, trains[0].Len())
	assert.Equal(t, 20000, tests[0].Len())
}

func TestUserLOOSplitter(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	loo := NewUserLOOSplitter(1)
	_, tests := loo(data, 0)
	for _, ratings := range tests[0].DenseUserRatings {
		assert.Equal(t, 1, ratings.Len())
	}
}

func TestUserKeepNSplitter(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	keep := NewUserKeepNSplitter(1, 3, 0.2)
	trains, _ := keep(data, 0)
	nCount := 0
	for _, ratings := range trains[0].DenseUserRatings {
		if ratings.Len() == 3 {
			nCount++
		}
	}
	assert.True(t,
		math.Abs(float64(data.UserCount())*0.2-float64(nCount)) < 1)
}
