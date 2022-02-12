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
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"os"
	"path/filepath"
	"testing"
)

func newRankingDataset() (*ranking.DataSet, *ranking.DataSet) {
	dataset := &ranking.DataSet{
		UserIndex: base.NewMapIndex(),
		ItemIndex: base.NewMapIndex(),
	}
	return dataset, dataset
}

func newClickDataset() (*click.Dataset, *click.Dataset) {
	dataset := &click.Dataset{
		Index: click.NewUnifiedMapIndexBuilder().Build(),
	}
	return dataset, dataset
}

func TestLocalCache(t *testing.T) {
	// delete test file if exists
	path := filepath.Join(os.TempDir(), "TestLocalCache_Master")
	_ = os.Remove(path)

	// load non-existed file
	cache, err := LoadLocalCache(path)
	assert.Error(t, err)
	assert.Equal(t, path, cache.path)
	assert.Empty(t, cache.RankingModelName)
	assert.Zero(t, cache.RankingModelVersion)
	assert.Zero(t, cache.RankingModelScore)
	assert.Nil(t, cache.RankingModel)
	assert.Zero(t, cache.ClickModelVersion)
	assert.Zero(t, cache.ClickModelScore)
	assert.Nil(t, cache.ClickModel)

	// write and load
	trainSet, testSet := newRankingDataset()
	bpr := ranking.NewBPR(model.Params{model.NEpochs: 0})
	bpr.Fit(trainSet, testSet, nil)
	cache.RankingModel = bpr
	cache.RankingModelName = "bpr"
	cache.RankingModelVersion = 123
	cache.RankingModelScore = ranking.Score{Precision: 1, NDCG: 2, Recall: 3}

	train, test := newClickDataset()
	fm := click.NewFM(click.FMClassification, model.Params{model.NEpochs: 0})
	fm.Fit(train, test, nil)
	cache.ClickModel = fm
	cache.ClickModelVersion = 456
	cache.ClickModelScore = click.Score{Precision: 1, RMSE: 100, Task: click.FMClassification}
	assert.NoError(t, cache.WriteLocalCache())

	read, err := LoadLocalCache(path)
	assert.NoError(t, err)
	assert.NotNil(t, read.RankingModel)
	assert.Equal(t, "bpr", read.RankingModelName)
	assert.Equal(t, int64(123), read.RankingModelVersion)
	assert.Equal(t, ranking.Score{Precision: 1, NDCG: 2, Recall: 3}, read.RankingModelScore)
	assert.NotNil(t, read.ClickModel)
	assert.Equal(t, int64(456), read.ClickModelVersion)
	assert.Equal(t, click.Score{Precision: 1, RMSE: 100, Task: click.FMClassification}, read.ClickModelScore)

	// delete test file
	assert.NoError(t, os.Remove(path))
}
