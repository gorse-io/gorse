// Copyright 2025 gorse Project Authors
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

package worker

import (
	"fmt"
	"testing"

	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type PipelineTestSuite struct {
	suite.Suite
	dataClient data.Database
}

func (suite *PipelineTestSuite) SetupSuite() {
	var err error
	suite.dataClient, err = data.Open(fmt.Sprintf("sqlite://%s/data.db", suite.T().TempDir()), "")
	suite.NoError(err)
	err = suite.dataClient.Init()
	suite.NoError(err)

	// insert items
	err = suite.dataClient.BatchInsertItems(suite.T().Context(), []data.Item{
		{ItemId: "1"},
		{ItemId: "2"},
		{ItemId: "3"},
		{ItemId: "4"},
		{ItemId: "5"},
	})
	suite.NoError(err)
}

func (suite *PipelineTestSuite) TearDownSuite() {
	err := suite.dataClient.Close()
	suite.NoError(err)
}

func (suite *PipelineTestSuite) TestGetSlice() {
	c := NewItemCache(suite.dataClient)
	items, err := c.GetSlice(suite.T().Context(), []string{"1", "2", "3", "4", "5", "6"})
	suite.NoError(err)
	suite.Equal(5, len(items))
}

func (suite *PipelineTestSuite) TestGetMap() {
	c := NewItemCache(suite.dataClient)
	items, err := c.GetMap(suite.T().Context(), []string{"1", "2", "3", "4", "5", "6"})
	suite.NoError(err)
	suite.Equal(5, len(items))
}

func TestPipeline(t *testing.T) {
	suite.Run(t, new(PipelineTestSuite))
}

func TestCompressLabelsEmbeddings(t *testing.T) {
	// Test nil input
	assert.Nil(t, compressLabelsEmbeddings(nil))

	// Test embedding vector as []any
	input := []any{1.0, 2.0, 3.0}
	output := compressLabelsEmbeddings(input)
	assert.IsType(t, []uint16{}, output)
	assert.Len(t, output.([]uint16), 3)

	// Test embedding vector as []float32
	input32 := []float32{1.0, 2.0, 3.0}
	output32 := compressLabelsEmbeddings(input32)
	assert.IsType(t, []uint16{}, output32)
	assert.Len(t, output32.([]uint16), 3)

	// Test embedding vector as []float64
	input64 := []float64{1.0, 2.0, 3.0}
	output64 := compressLabelsEmbeddings(input64)
	assert.IsType(t, []uint16{}, output64)
	assert.Len(t, output64.([]uint16), 3)

	// Test already compressed []uint16
	inputU16 := []uint16{0x3f80, 0x4000, 0x4040}
	outputU16 := compressLabelsEmbeddings(inputU16)
	assert.Equal(t, inputU16, outputU16)

	// Test map with embedding
	mapInput := map[string]any{
		"title":     "test item",
		"embedding": []any{1.0, 2.0, 3.0},
	}
	mapOutput := compressLabelsEmbeddings(mapInput)
	assert.IsType(t, []uint16{}, mapOutput.(map[string]any)["embedding"])
	assert.Equal(t, "test item", mapOutput.(map[string]any)["title"])

	// Test nested map
	nestedInput := map[string]any{
		"features": map[string]any{
			"embedding": []float32{1.0, 2.0},
		},
	}
	nestedOutput := compressLabelsEmbeddings(nestedInput)
	assert.IsType(t, []uint16{}, nestedOutput.(map[string]any)["features"].(map[string]any)["embedding"])

	// Test non-embedding []any (strings)
	strSlice := []any{"tag1", "tag2"}
	strOutput := compressLabelsEmbeddings(strSlice)
	assert.IsType(t, []any{}, strOutput)
	assert.Equal(t, strSlice, strOutput)
}
