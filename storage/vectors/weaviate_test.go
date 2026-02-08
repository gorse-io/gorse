package vectors

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

var (
	weaviateUri string
)

func init() {
	// get environment variables
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	weaviateUri = env("WEAVIATE_URI", "weaviate://127.0.0.1:8080")
}

type WeaviateTestSuite struct {
	vectorsTestSuite
}

func (suite *WeaviateTestSuite) SetupSuite() {
	var err error
	suite.Database, err = Open(weaviateUri, "gorse_")
	suite.NoError(err)
}

func TestWeaviate(t *testing.T) {
	suite.Run(t, new(WeaviateTestSuite))
}
