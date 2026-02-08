package vectors

import (
	"context"

	"github.com/stretchr/testify/suite"
)

const defaultVectorSize = 4

type vectorsTestSuite struct {
	suite.Suite
	Database
}

func (suite *vectorsTestSuite) SetupTest() {
	// purge
	ctx := context.Background()
	collections, err := suite.Database.ListCollections(ctx)
	suite.NoError(err)
	for _, collection := range collections {
		err = suite.Database.DeleteCollection(ctx, collection)
		suite.NoError(err)
	}
}

func (suite *vectorsTestSuite) TestCollections() {
	ctx := context.Background()
	// list collections
	collections, err := suite.Database.ListCollections(ctx)
	suite.NoError(err)
	suite.Empty(collections)
	// create collection
	err = suite.Database.AddCollection(ctx, "test", defaultVectorSize, Cosine)
	suite.NoError(err)
	// list collections
	collections, err = suite.Database.ListCollections(ctx)
	suite.NoError(err)
	suite.Equal([]string{"test"}, collections)
	// delete collection
	err = suite.Database.DeleteCollection(ctx, "test")
	suite.NoError(err)
	// list collections
	collections, err = suite.Database.ListCollections(ctx)
	suite.NoError(err)
	suite.Empty(collections)
	// delete non-existent collection
	err = suite.Database.DeleteCollection(ctx, "non-existent")
	suite.Error(err)
}

func (suite *vectorsTestSuite) TestVectors() {
	ctx := context.Background()
	err := suite.Database.AddCollection(ctx, "test", defaultVectorSize, Cosine)
	suite.NoError(err)

	vectorA := make([]float32, defaultVectorSize)
	vectorA[0] = 1
	vectorB := make([]float32, defaultVectorSize)
	vectorB[0] = 0.9
	vectorB[1] = 0.1

	err = suite.Database.AddVectors(ctx, "test", []Vector{
		{
			Id:         "a",
			Vector:     vectorA,
			Categories: []string{"cat-a", "common"},
		},
		{
			Id:         "b",
			Vector:     vectorB,
			Categories: []string{"cat-b", "common"},
		},
	})
	suite.NoError(err)

	results, err := suite.Database.QueryVectors(ctx, "test", vectorA, []string{"cat-a"}, 10)
	suite.NoError(err)
	suite.Len(results, 1)
	suite.Equal("a", results[0].Id)
	suite.NotEmpty(results[0].Categories)

	results, err = suite.Database.QueryVectors(ctx, "test", vectorA, []string{"common"}, 10)
	suite.NoError(err)
	suite.Len(results, 2)
	ids := map[string]bool{}
	for _, result := range results {
		ids[result.Id] = true
		suite.NotEmpty(result.Categories)
	}
	suite.True(ids["a"])
	suite.True(ids["b"])

	results, err = suite.Database.QueryVectors(ctx, "test", vectorA, nil, 1)
	suite.NoError(err)
	suite.NotEmpty(results)
	for _, result := range results {
		suite.NotEmpty(result.Categories)
	}
}
