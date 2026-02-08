// Copyright 2024 gorse Project Authors
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
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

var (
	milvusUri string
)

func init() {
	// get environment variables
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	milvusUri = env("MILVUS_URI", "milvus://127.0.0.1:19530")
}

type MilvusTestSuite struct {
	vectorsTestSuite
}

func (suite *MilvusTestSuite) SetupSuite() {
	var err error
	suite.Database, err = Open(milvusUri, "gorse_")
	suite.NoError(err)
}

func TestMilvus(t *testing.T) {
	suite.Run(t, new(MilvusTestSuite))
}
