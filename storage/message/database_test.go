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

package message

import (
	"github.com/stretchr/testify/suite"
	"io"
	"time"
)

type baseTestSuite struct {
	suite.Suite
	Database
}

func (suite *baseTestSuite) TestPushPop() {
	err := suite.Push("a", Message{Data: "1", Timestamp: time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC)})
	suite.NoError(err)
	err = suite.Push("a", Message{Data: "2", Timestamp: time.Date(2001, 1, 1, 1, 1, 1, 1, time.UTC)})
	suite.NoError(err)
	err = suite.Push("b", Message{Data: "3", Timestamp: time.Date(2002, 1, 1, 1, 1, 1, 1, time.UTC)})
	suite.NoError(err)
	err = suite.Push("b", Message{Data: "4", Timestamp: time.Date(2003, 1, 1, 1, 1, 1, 1, time.UTC)})
	suite.NoError(err)

	message, err := suite.Pop("a")
	suite.NoError(err)
	suite.Equal("1", message.Data)
	message, err = suite.Pop("a")
	suite.NoError(err)
	suite.Equal("2", message.Data)
	message, err = suite.Pop("b")
	suite.NoError(err)
	suite.Equal("3", message.Data)
	message, err = suite.Pop("b")
	suite.NoError(err)
	suite.Equal("4", message.Data)

	message, err = suite.Pop("a")
	suite.ErrorIs(err, io.EOF)
	message, err = suite.Pop("b")
	suite.ErrorIs(err, io.EOF)
}

func (suite *baseTestSuite) TestDuplicateTimestamp() {
	for i := 0; i < 100; i++ {
		err := suite.Push("c", Message{Data: "1", Timestamp: time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC)})
		suite.NoError(err)
	}
	for i := 0; i < 100; i++ {
		message, err := suite.Pop("c")
		suite.NoError(err)
		suite.Equal("1", message.Data)
	}
	_, err := suite.Pop("c")
	suite.ErrorIs(err, io.EOF)
}
