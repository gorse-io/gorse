package vectors

import (
	"context"

	"github.com/stretchr/testify/suite"
)

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
	err = suite.Database.AddCollection(ctx, "test")
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
