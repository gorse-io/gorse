// Copyright 2023 gorse Project Authors
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

package progress

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type ProgressTestSuite struct {
	suite.Suite
	tracer *Tracer
}

func (suite *ProgressTestSuite) SetupTest() {
	suite.tracer = NewTracer("test")
}

func (suite *ProgressTestSuite) TestLeafProgress() {
	_, span := suite.tracer.Start(context.Background(), "root", 100)
	progresList := suite.tracer.List()
	suite.Equal(1, len(progresList))
	suite.Equal("test", progresList[0].Tracer)
	suite.Equal("root", progresList[0].Name)
	suite.Equal(StatusRunning, progresList[0].Status)
	suite.Empty(progresList[0].Error)
	suite.Equal(100, progresList[0].Total)
	suite.Empty(progresList[0].Count)
	suite.LessOrEqual(progresList[0].StartTime, time.Now())

	span.Add(10)
	progresList = suite.tracer.List()
	suite.Equal(1, len(progresList))
	suite.Equal("test", progresList[0].Tracer)
	suite.Equal("root", progresList[0].Name)
	suite.Equal(StatusRunning, progresList[0].Status)
	suite.Empty(progresList[0].Error)
	suite.Equal(100, progresList[0].Total)
	suite.Equal(10, progresList[0].Count)

	span.End()
	progresList = suite.tracer.List()
	suite.Equal(1, len(progresList))
	suite.Equal("test", progresList[0].Tracer)
	suite.Equal("root", progresList[0].Name)
	suite.Equal(StatusComplete, progresList[0].Status)
	suite.Empty(progresList[0].Error)
	suite.Equal(100, progresList[0].Total)
	suite.Equal(100, progresList[0].Count)
	suite.Less(progresList[0].StartTime, progresList[0].FinishTime)

	span.Fail(errors.New("some error"))
	progresList = suite.tracer.List()
	suite.Equal(1, len(progresList))
	suite.Equal("test", progresList[0].Tracer)
	suite.Equal("root", progresList[0].Name)
	suite.Equal(StatusFailed, progresList[0].Status)
	suite.Equal("some error", progresList[0].Error)
	suite.Equal(100, progresList[0].Total)
	suite.Equal(100, progresList[0].Count)
}

func (suite *ProgressTestSuite) TestMultiLevelProgress() {
	newCtx, rootSpan := suite.tracer.Start(context.Background(), "root", 100)
	rootSpan.Add(10)
	progresList := suite.tracer.List()
	suite.Equal(1, len(progresList))
	suite.Equal("test", progresList[0].Tracer)
	suite.Equal("root", progresList[0].Name)
	suite.Equal(StatusRunning, progresList[0].Status)
	suite.Empty(progresList[0].Error)
	suite.Equal(100, progresList[0].Total)
	suite.Equal(10, progresList[0].Count)
	suite.LessOrEqual(progresList[0].StartTime, time.Now())

	childCtx, childSpan := Start(newCtx, "child", 8)
	childSpan.Add(2)
	progresList = suite.tracer.List()
	suite.Equal(1, len(progresList))
	suite.Equal("test", progresList[0].Tracer)
	suite.Equal("root", progresList[0].Name)
	suite.Equal(StatusRunning, progresList[0].Status)
	suite.Empty(progresList[0].Error)
	suite.Equal(800, progresList[0].Total)
	suite.Equal(82, progresList[0].Count)

	childSpan.End()
	progresList = suite.tracer.List()
	suite.Equal(1, len(progresList))
	suite.Equal("test", progresList[0].Tracer)
	suite.Equal("root", progresList[0].Name)
	suite.Equal(StatusRunning, progresList[0].Status)
	suite.Empty(progresList[0].Error)
	suite.Equal(100, progresList[0].Total)
	suite.Equal(10, progresList[0].Count)

	Fail(childCtx, errors.New("some error"))
	progresList = suite.tracer.List()
	suite.Equal(1, len(progresList))
	suite.Equal("test", progresList[0].Tracer)
	suite.Equal("root", progresList[0].Name)
	suite.Equal(StatusFailed, progresList[0].Status)
	suite.Equal("some error", progresList[0].Error)
	suite.Equal(100, progresList[0].Total)
	suite.Equal(10, progresList[0].Count)
}

func TestProgressTestSuite(t *testing.T) {
	suite.Run(t, new(ProgressTestSuite))
}
