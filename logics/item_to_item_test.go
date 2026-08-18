// Copyright 2021 zhenghaoz <zhangzhenghao@hotmail.com>. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package logics

import (
	"testing"
	"time"

	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/vectors"
	"github.com/stretchr/testify/suite"
)

type ItemToItemTestSuite struct {
	suite.Suite
}

func (suite *ItemToItemTestSuite) SetupTest() {
	log.SetTestLogger(suite.T())
}

func (suite *ItemToItemTestSuite) newWriter(cfg config.ItemToItemConfig, tagsIDF, usersIDF []float32) (ItemToItem, vectors.Database) {
	vectorClient := openTrackingVectorDatabase(suite.T())
	writer, err := NewItemToItem(cfg, time.Now(), &ItemToItemOptions{
		Context: suite.T().Context(), VectorClient: vectorClient, TagsIDF: tagsIDF, UsersIDF: usersIDF,
	})
	suite.NoError(err)
	return writer, vectorClient
}

func (suite *ItemToItemTestSuite) TestEmbedding() {
	cfg := config.ItemToItemConfig{Name: "embedding", Type: "embedding", Column: "item.Labels.description"}
	writer, vectorClient := suite.newWriter(cfg, nil, nil)
	writer.Push(&data.Item{ItemId: "source", Labels: map[string]any{"description": []float32{0, 0}}}, nil)
	writer.Push(&data.Item{ItemId: "near", Labels: map[string]any{"description": []float32{0.1, 0}}}, nil)
	writer.Push(&data.Item{ItemId: "far", Labels: map[string]any{"description": []float32{1, 0}}}, nil)
	writer.Push(&data.Item{ItemId: "invalid", Labels: map[string]any{"description": "invalid"}}, nil)
	writer.Push(&data.Item{ItemId: "wrong-dimension", Labels: map[string]any{"description": []float32{0, 0, 0}}}, nil)
	suite.NoError(writer.Finish())

	scores, err := QueryItemToItem(suite.T().Context(), vectorClient, cfg, "source", nil, 2)
	suite.NoError(err)
	suite.Require().Len(scores, 2)
	suite.Equal("near", scores[0].Id)
	suite.Equal("far", scores[1].Id)
}

func (suite *ItemToItemTestSuite) TestHidden() {
	cfg := config.ItemToItemConfig{Name: "hidden", Type: "embedding", Column: "item.Labels.description"}
	writer, vectorClient := suite.newWriter(cfg, nil, nil)
	writer.Push(&data.Item{ItemId: "visible_1", Labels: map[string]any{"description": []float32{0, 0}}}, nil)
	writer.Push(&data.Item{ItemId: "visible_2", Labels: map[string]any{"description": []float32{0.1, 0}}}, nil)
	writer.Push(&data.Item{ItemId: "hidden", IsHidden: true, Labels: map[string]any{"description": []float32{0.05, 0}}}, nil)
	suite.NoError(writer.Finish())

	visibleScores, err := QueryItemToItem(suite.T().Context(), vectorClient, cfg, "visible_1", nil, 10)
	suite.NoError(err)
	suite.Equal([]string{"visible_2"}, scoreIDs(visibleScores))
	hiddenScores, err := QueryItemToItem(suite.T().Context(), vectorClient, cfg, "hidden", nil, 10)
	suite.NoError(err)
	suite.Equal([]string{"visible_1", "visible_2"}, scoreIDs(hiddenScores))
}

func (suite *ItemToItemTestSuite) TestTags() {
	cfg := config.ItemToItemConfig{Name: "tags", Type: "tags", Column: "item.Labels"}
	writer, vectorClient := suite.newWriter(cfg, []float32{0, 1, 2}, nil)
	writer.Push(&data.Item{ItemId: "query", Labels: []dataset.ID{1, 2}}, nil)
	writer.Push(&data.Item{ItemId: "idf-2", Labels: []dataset.ID{2}}, nil)
	writer.Push(&data.Item{ItemId: "idf-1", Labels: []dataset.ID{1}}, nil)
	suite.NoError(writer.Finish())

	scores, err := QueryItemToItem(suite.T().Context(), vectorClient, cfg, "query", nil, 2)
	suite.NoError(err)
	suite.Require().Len(scores, 2)
	suite.Equal("idf-2", scores[0].Id)
	suite.InDelta(2, scores[0].Score, 1e-6)
	suite.Equal("idf-1", scores[1].Id)
	suite.InDelta(1, scores[1].Score, 1e-6)
}

func (suite *ItemToItemTestSuite) TestUsers() {
	cfg := config.ItemToItemConfig{Name: "users", Type: "users"}
	writer, vectorClient := suite.newWriter(cfg, nil, []float32{0, 1, 2})
	writer.Push(&data.Item{ItemId: "query"}, []int32{1, 2})
	writer.Push(&data.Item{ItemId: "idf-2"}, []int32{2})
	writer.Push(&data.Item{ItemId: "idf-1"}, []int32{1})
	suite.NoError(writer.Finish())

	scores, err := QueryItemToItem(suite.T().Context(), vectorClient, cfg, "query", nil, 2)
	suite.NoError(err)
	suite.Require().Len(scores, 2)
	suite.Equal("idf-2", scores[0].Id)
	suite.InDelta(2, scores[0].Score, 1e-6)
	suite.Equal("idf-1", scores[1].Id)
	suite.InDelta(1, scores[1].Score, 1e-6)
}

func (suite *ItemToItemTestSuite) TestAuto() {
	cfg := config.ItemToItemConfig{Name: "auto", Type: "auto"}
	idf := []float32{0, 1, 2}
	writer, vectorClient := suite.newWriter(cfg, idf, idf)
	writer.Push(&data.Item{ItemId: "query", Labels: []dataset.ID{1}}, []int32{1})
	writer.Push(&data.Item{ItemId: "same", Labels: []dataset.ID{1}}, []int32{1})
	suite.NoError(writer.Finish())

	scores, err := QueryItemToItem(suite.T().Context(), vectorClient, cfg, "query", nil, 1)
	suite.NoError(err)
	suite.Require().Len(scores, 1)
	suite.Equal("same", scores[0].Id)
	suite.InDelta(1, scores[0].Score, 1e-6)
}

func (suite *ItemToItemTestSuite) TestIDFInnerProduct() {
	idf := IDF[dataset.ID]{0, 1, 2, 3, 4}
	suite.InDelta(5, idf.similarity([]dataset.ID{1, 2, 3}, []dataset.ID{2, 3, 4}), 1e-6)
	suite.InDelta(0, idf.similarity([]dataset.ID{1}, []dataset.ID{4}), 1e-6)
}

func scoreIDs(scores []cache.Score) []string {
	ids := make([]string, len(scores))
	for i := range scores {
		ids[i] = scores[i].Id
	}
	return ids
}

func TestItemToItem(t *testing.T) {
	suite.Run(t, new(ItemToItemTestSuite))
}
