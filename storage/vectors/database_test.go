// Copyright 2026 gorse Project Authors
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

package vectors

import (
	"time"

	"github.com/gorse-io/gorse/common/log"
	"github.com/juju/errors"
	"github.com/stretchr/testify/suite"
)

const defaultVectorSize = 4

type vectorsTestSuite struct {
	suite.Suite
	Database
}

func (suite *vectorsTestSuite) SetupTest() {
	log.SetTestLogger(suite.T())
	// purge
	ctx := suite.T().Context()
	collections, err := suite.Database.ListCollections(ctx)
	suite.NoError(err)
	for _, collection := range collections {
		err = suite.Database.DeleteCollection(ctx, collection)
		suite.NoError(err)
	}
}

func (suite *vectorsTestSuite) TestCollections() {
	ctx := suite.T().Context()
	// list collections
	collections, err := suite.Database.ListCollections(ctx)
	suite.NoError(err)
	suite.Empty(collections)
	// create collection
	err = suite.Database.AddCollection(ctx, "test", defaultVectorSize, Cosine, VectorConfig{})
	suite.NoError(err)
	// describe collection
	info, err := suite.Database.DescribeCollection(ctx, "test")
	suite.NoError(err)
	suite.Equal("test", info.Name)
	if info.Dimension > 0 {
		suite.Equal(defaultVectorSize, info.Dimension)
	}
	suite.Equal(Cosine, info.Distance)
	suite.Equal(QuantizationNone, info.Type)
	suite.Zero(info.Bits)
	// list collections
	collections, err = suite.Database.ListCollections(ctx)
	suite.NoError(err)
	suite.Equal([]string{"test"}, collections)
	// delete collection
	err = suite.Database.DeleteCollection(ctx, "test")
	suite.NoError(err)
	// describe deleted collection
	_, err = suite.Database.DescribeCollection(ctx, "test")
	suite.True(errors.Is(err, errors.NotFound), err)
	// list collections
	collections, err = suite.Database.ListCollections(ctx)
	suite.NoError(err)
	suite.Empty(collections)
	// delete non-existent collection
	err = suite.Database.DeleteCollection(ctx, "non-existent")
	suite.Error(err)
}

func (suite *vectorsTestSuite) TestVectors() {
	ctx := suite.T().Context()
	err := suite.Database.AddCollection(ctx, "test", defaultVectorSize, Cosine, VectorConfig{})
	suite.NoError(err)
	count, err := suite.Database.CountVectors(ctx, "test")
	suite.NoError(err)
	suite.Zero(count)

	vectorA := make([]float32, defaultVectorSize)
	vectorA[0] = 1
	vectorB := make([]float32, defaultVectorSize)
	vectorB[0] = 0.9
	vectorB[1] = 0.1

	err = suite.Database.AddVectors(ctx, "test", []Vector{
		{
			Id:         "a",
			Values:     vectorA,
			Categories: []string{"cat-a", "common"},
		},
		{
			Id:         "b",
			Values:     vectorB,
			Categories: []string{"cat-b", "common"},
		},
	})
	suite.NoError(err)
	count, err = suite.Database.CountVectors(ctx, "test")
	suite.NoError(err)
	suite.Equal(int64(2), count)

	results, err := suite.Database.QueryVectors(ctx, "test", Vector{Values: vectorA}, []string{"cat-a"}, 10)
	suite.NoError(err)
	suite.Len(results, 1)
	suite.Equal("a", results[0].Id)
	suite.NotEmpty(results[0].Categories)

	results, err = suite.Database.QueryVectors(ctx, "test", Vector{Values: vectorA}, []string{"common"}, 10)
	suite.NoError(err)
	suite.Len(results, 2)
	suite.Equal("a", results[0].Id)
	suite.Equal("b", results[1].Id)
	suite.Greater(results[0].Score, results[1].Score)
	for _, result := range results {
		suite.NotEmpty(result.Categories)
	}

	results, err = suite.Database.QueryVectors(ctx, "test", Vector{Values: vectorA}, []string{"cat-a", "cat-b"}, 10)
	suite.NoError(err)
	suite.Len(results, 2)

	results, err = suite.Database.QueryVectors(ctx, "test", Vector{Values: vectorA}, nil, 1)
	suite.NoError(err)
	suite.NotEmpty(results)
	for _, result := range results {
		suite.NotEmpty(result.Categories)
	}
}

func (suite *vectorsTestSuite) TestGetVectors() {
	ctx := suite.T().Context()
	err := suite.Database.AddCollection(ctx, "test", defaultVectorSize, Cosine, VectorConfig{})
	suite.Require().NoError(err)

	timestampA := time.Now().UTC().Truncate(time.Millisecond)
	timestampB := timestampA.Add(time.Second)
	vectorA := Vector{
		Id:         "a",
		Values:     []float32{1, 0, 0, 0},
		Categories: []string{"cat-a", "common"},
		Timestamp:  timestampA,
	}
	vectorB := Vector{
		Id:         "b",
		Values:     []float32{0, 1, 0, 0},
		IsHidden:   true,
		Categories: []string{"cat-b", "common"},
		Timestamp:  timestampB,
	}
	err = suite.Database.AddVectors(ctx, "test", []Vector{vectorA, vectorB})
	suite.Require().NoError(err)

	results, err := suite.Database.GetVectors(ctx, "test", []string{"b", "missing", "a"})
	suite.Require().NoError(err)
	suite.Equal([]Vector{vectorB, vectorA}, results)

	results, err = suite.Database.GetVectors(ctx, "test", nil)
	suite.Require().NoError(err)
	suite.Empty(results)
}

func (suite *vectorsTestSuite) TestSparse() {
	ctx := suite.T().Context()
	err := suite.Database.AddCollection(ctx, "test_sparse", 0, Dot, VectorConfig{})
	suite.Require().NoError(err)

	info, err := suite.Database.DescribeCollection(ctx, "test_sparse")
	suite.Require().NoError(err)
	suite.Zero(info.Dimension)
	suite.Equal(Dot, info.Distance)

	cutoff := time.Now().UTC().Truncate(time.Millisecond)
	err = suite.Database.AddVectors(ctx, "test_sparse", []Vector{
		{Id: "old", Indices: []uint32{1, 100}, Values: []float32{1, 1}, Timestamp: cutoff.Add(-time.Hour)},
		{Id: "match", Indices: []uint32{1, 100}, Values: []float32{1, 2}, Timestamp: cutoff},
		{Id: "other", Indices: []uint32{2, 200}, Values: []float32{1, 2}, Timestamp: cutoff},
	})
	suite.Require().NoError(err)

	count, err := suite.Database.CountVectors(ctx, "test_sparse")
	suite.Require().NoError(err)
	suite.Equal(int64(3), count)

	vectors, err := suite.Database.GetVectors(ctx, "test_sparse", []string{"match", "missing"})
	suite.Require().NoError(err)
	suite.Require().Len(vectors, 1)
	suite.Equal("match", vectors[0].Id)
	suite.Equal([]uint32{1, 100}, vectors[0].Indices)
	suite.Equal([]float32{1, 2}, vectors[0].Values)
	suite.Equal(cutoff, vectors[0].Timestamp)

	results, err := suite.Database.QueryVectors(ctx, "test_sparse", Vector{
		Indices: []uint32{1, 100},
		Values:  []float32{1, 2},
	}, nil, 10)
	suite.Require().NoError(err)
	suite.Require().Len(results, 2)
	suite.Equal("match", results[0].Id)

	err = suite.Database.DeleteVectors(ctx, "test_sparse", cutoff)
	suite.Require().NoError(err)
	count, err = suite.Database.CountVectors(ctx, "test_sparse")
	suite.Require().NoError(err)
	suite.Equal(int64(2), count)

	err = suite.Database.DeleteCollection(ctx, "test_sparse")
	suite.Require().NoError(err)
	_, err = suite.Database.DescribeCollection(ctx, "test_sparse")
	suite.True(errors.Is(err, errors.NotFound), err)
}

func (suite *vectorsTestSuite) TestHidden() {
	ctx := suite.T().Context()
	err := suite.Database.AddCollection(ctx, "test_hidden", defaultVectorSize, Cosine, VectorConfig{})
	suite.Require().NoError(err)

	query := []float32{1, 0, 0, 0}
	err = suite.Database.AddVectors(ctx, "test_hidden", []Vector{
		{Id: "visible", Values: []float32{0.9, 0.1, 0, 0}, Categories: []string{"common", "quo'te"}},
		{Id: "hidden", Values: query, IsHidden: true, Categories: []string{"common", "quo'te"}},
	})
	suite.Require().NoError(err)

	count, err := suite.Database.CountVectors(ctx, "test_hidden")
	suite.Require().NoError(err)
	suite.Equal(int64(2), count)

	results, err := suite.Database.QueryVectors(ctx, "test_hidden", Vector{Values: query}, nil, 10)
	suite.Require().NoError(err)
	suite.Require().Len(results, 1)
	suite.Equal("visible", results[0].Id)

	results, err = suite.Database.QueryVectors(ctx, "test_hidden", Vector{Values: query}, []string{"common"}, 10)
	suite.Require().NoError(err)
	suite.Require().Len(results, 1)
	suite.Equal("visible", results[0].Id)

	results, err = suite.Database.QueryVectors(ctx, "test_hidden", Vector{Values: query}, []string{"quo'te"}, 10)
	suite.Require().NoError(err)
	suite.Require().Len(results, 1)
	suite.Equal("visible", results[0].Id)
}

func (suite *vectorsTestSuite) TestDot() {
	ctx := suite.T().Context()
	err := suite.Database.AddCollection(ctx, "test_dot", defaultVectorSize, Dot, VectorConfig{})
	suite.Require().NoError(err)

	query := []float32{1, 0, 0, 0}
	err = suite.Database.AddVectors(ctx, "test_dot", []Vector{
		{Id: "a", Values: []float32{2, 0, 0, 0}},
		{Id: "b", Values: []float32{1, 1, 0, 0}},
	})
	suite.Require().NoError(err)

	results, err := suite.Database.QueryVectors(ctx, "test_dot", Vector{Values: query}, nil, 2)
	suite.Require().NoError(err)
	suite.Require().Len(results, 2)
	suite.Equal("a", results[0].Id)
	suite.Equal("b", results[1].Id)
	suite.Greater(results[0].Score, results[1].Score)
}

func (suite *vectorsTestSuite) TestDeleteVectors() {
	ctx := suite.T().Context()
	err := suite.Database.AddCollection(ctx, "test", defaultVectorSize, Cosine, VectorConfig{})
	suite.NoError(err)

	vectorA := make([]float32, defaultVectorSize)
	vectorA[0] = 1
	vectorB := make([]float32, defaultVectorSize)
	vectorB[0] = 0.9
	vectorB[1] = 0.1

	cutoff := time.Now().UTC().Truncate(time.Millisecond)
	err = suite.Database.AddVectors(ctx, "test", []Vector{
		{
			Id:         "old",
			Values:     vectorA,
			Categories: []string{"common"},
			Timestamp:  cutoff.Add(-time.Hour),
		},
		{
			Id:         "new",
			Values:     vectorB,
			Categories: []string{"common"},
			Timestamp:  cutoff,
		},
	})
	suite.NoError(err)

	err = suite.Database.DeleteVectors(ctx, "test", cutoff)
	suite.NoError(err)
	count, err := suite.Database.CountVectors(ctx, "test")
	suite.NoError(err)
	suite.Equal(int64(1), count)

	results, err := suite.Database.QueryVectors(ctx, "test", Vector{Values: vectorA}, []string{"common"}, 10)
	suite.NoError(err)
	suite.Len(results, 1)
	suite.Equal("new", results[0].Id)
}

func (suite *vectorsTestSuite) testQuantization(quantization QuantizationType, bits int) {
	suite.T().Helper()
	ctx := suite.T().Context()

	err := suite.Database.AddCollection(ctx, "test_quantization", defaultVectorSize, Cosine, VectorConfig{
		Type: quantization,
		Bits: bits,
	})
	suite.Require().NoError(err)

	cfg, err := suite.Database.DescribeCollection(ctx, "test_quantization")
	suite.NoError(err)
	suite.Equal(quantization, cfg.Type)
	if bits > 0 {
		suite.Equal(bits, cfg.Bits)
	}

	vectorA := make([]float32, defaultVectorSize)
	vectorA[0] = 1
	vectorB := make([]float32, defaultVectorSize)
	vectorB[0] = 0.9
	vectorB[1] = 0.1

	err = suite.Database.AddVectors(ctx, "test_quantization", []Vector{
		{
			Id:         "a",
			Values:     vectorA,
			Categories: []string{"cat-a", "common"},
		},
		{
			Id:         "b",
			Values:     vectorB,
			Categories: []string{"cat-b", "common"},
		},
	})
	suite.NoError(err)

	results, err := suite.Database.QueryVectors(ctx, "test_quantization", Vector{Values: vectorA}, []string{"common"}, 10)
	suite.NoError(err)
	suite.Len(results, 2)

	err = suite.Database.DeleteCollection(ctx, "test_quantization")
	suite.NoError(err)
}
