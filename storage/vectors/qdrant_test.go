package vectors

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

var (
	qdrantUri string
)

func init() {
	// get environment variables
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	qdrantUri = env("QDRANT_URI", "qdrant://127.0.0.1:6334")
}

type QdrantTestSuite struct {
	vectorsTestSuite
}

func (suite *QdrantTestSuite) SetupSuite() {
	var err error
	suite.Database, err = Open(qdrantUri, "gorse_")
	suite.NoError(err)
}

func TestQdrant(t *testing.T) {
	suite.Run(t, new(QdrantTestSuite))
}
