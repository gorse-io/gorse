// Copyright 2020 Zhenghao Zhang
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package model

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

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
	assert.Equal(t, data.UserCount(), tests[0].UserCount())
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

func TestNewRatioUserLOOSplitter(t *testing.T) {
	data := LoadDataFromBuiltIn("ml-100k")
	loo := NewRatioUserLOOSplitter(1, 0.5)
	_, tests := loo(data, 0)
	assert.True(t, float64(tests[0].UserCount())/float64(data.UserCount()) < 0.6)
	assert.True(t, float64(tests[0].UserCount())/float64(data.UserCount()) > 0.4)
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
