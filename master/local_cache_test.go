// Copyright 2021 gorse Project Authors
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

package master

import (
	"context"
	"github.com/zhenghaoz/gorse/base"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/dataset"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/cf"
	"github.com/zhenghaoz/gorse/model/ctr"
)

func newRankingDataset() (*dataset.Dataset, *dataset.Dataset) {
	return dataset.NewDataset(time.Now(), 0, 0), dataset.NewDataset(time.Now(), 0, 0)
}

func newClickDataset() (*ctr.Dataset, *ctr.Dataset) {
	dataset := &ctr.Dataset{
		Index: base.NewUnifiedMapIndexBuilder().Build(),
	}
	return dataset, dataset
}

func TestLocalCache(t *testing.T) {
	// delete test file if exists
	path := t.TempDir()

	// load non-existed file
	cache, err := LoadLocalCache(path)
	assert.Error(t, err)
	assert.Equal(t, path, cache.path)
	assert.Empty(t, cache.CollaborativeFilteringModelName)
	assert.Zero(t, cache.CollaborativeFilteringModelVersion)
	assert.Zero(t, cache.CollaborativeFilteringModelScore)
	assert.Nil(t, cache.CollaborativeFilteringModel)
	assert.Zero(t, cache.ClickModelVersion)
	assert.Zero(t, cache.ClickModelScore)
	assert.Nil(t, cache.ClickModel)

	// write and load
	trainSet, testSet := newRankingDataset()
	bpr := cf.NewBPR(model.Params{model.NEpochs: 0})
	bpr.Fit(context.Background(), trainSet, testSet, cf.NewFitConfig())
	cache.CollaborativeFilteringModel = bpr
	cache.CollaborativeFilteringModelName = "bpr"
	cache.CollaborativeFilteringModelVersion = 123
	cache.CollaborativeFilteringModelScore = cf.Score{Precision: 1, NDCG: 2, Recall: 3}

	train, test := newClickDataset()
	fm := ctr.NewFM(model.Params{model.NEpochs: 0})
	fm.Fit(context.Background(), train, test, nil)
	cache.ClickModel = fm
	cache.ClickModelVersion = 456
	cache.ClickModelScore = ctr.Score{Precision: 1, RMSE: 100}
	assert.NoError(t, cache.WriteLocalCache())

	read, err := LoadLocalCache(path)
	assert.NoError(t, err)
	assert.NotNil(t, read.CollaborativeFilteringModel)
	assert.Equal(t, "bpr", read.CollaborativeFilteringModelName)
	assert.Equal(t, int64(123), read.CollaborativeFilteringModelVersion)
	assert.Equal(t, cf.Score{Precision: 1, NDCG: 2, Recall: 3}, read.CollaborativeFilteringModelScore)
	assert.NotNil(t, read.ClickModel)
	assert.Equal(t, int64(456), read.ClickModelVersion)
	assert.Equal(t, ctr.Score{Precision: 1, RMSE: 100}, read.ClickModelScore)
}
