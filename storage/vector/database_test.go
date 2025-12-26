package vector

import "github.com/stretchr/testify/suite"

type baseTestSuite struct {
	suite.Suite
	Database
}

func (suite *baseTestSuite) TestVectors() {
	// create a collection
	err := suite.Create("vectors", 4)
	suite.NoError(err)
	collections, err := suite.List()
	suite.NoError(err)
	suite.Contains(collections, "vectors")

	// add vectors
	vectors := []Vector{
		{Id: "vec1", Value: []float32{0.1, 0.2, 0.3, 0.4}},
		{Id: "vec2", Value: []float32{0.2, 0.3, 0.4, 0.5}},
		{Id: "vec3", Value: []float32{0.3, 0.4, 0.5, 0.6}},
		{Id: "vec4", Value: []float32{0.4, 0.5, 0.6, 0.7}},
		{Id: "vec5", Value: []float32{0.5, 0.6, 0.7, 0.8}},
	}
	err = suite.Add("vectors", vectors)
	suite.NoError(err)

	// search vectors
	results, distances, err := suite.Search("vectors", []float32{0.1, 0.2, 0.3, 0.4}, 3, nil)
	suite.NoError(err)
	suite.Len(results, 3)
	suite.Len(distances, 3)
	suite.Equal("vec1", results[0].Id)
	suite.Equal("vec2", results[1].Id)
	suite.Equal("vec3", results[2].Id)

	// drop the collection
	err = suite.Drop("vectors")
	suite.NoError(err)
	collections, err = suite.List()
	suite.NoError(err)
	suite.NotContains(collections, "vectors")
}
