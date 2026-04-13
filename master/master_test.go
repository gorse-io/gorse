// Copyright 2020 gorse Project Authors
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
package master

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gorse-io/gorse/common/monitor"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/suite"
)

type MasterTestSuite struct {
	suite.Suite
	Master
}

func (s *MasterTestSuite) SetupTest() {
	// open database
	var err error
	s.tracer = monitor.NewTracer("test")
	s.Config = config.GetDefaultConfig()
	s.DataClient, err = data.Open(fmt.Sprintf("sqlite://%s/data.db", s.T().TempDir()), "")
	s.NoError(err)
	s.CacheClient, err = cache.Open(fmt.Sprintf("sqlite://%s/cache.db", s.T().TempDir()), "")
	s.NoError(err)
	// init database
	err = s.DataClient.Init()
	s.NoError(err)
	err = s.CacheClient.Init()
	s.NoError(err)
}

func (s *MasterTestSuite) TearDownTest() {
	s.NoError(s.DataClient.Close())
	s.NoError(s.CacheClient.Close())
}

func TestMaster(t *testing.T) {
	suite.Run(t, new(MasterTestSuite))
}

func (s *MasterTestSuite) TestRequestTrainingForceQueuesSingleReplacementRun() {
	s.scheduled = make(chan struct{}, 1)

	s.startTrainingRun()

	resp, status := s.requestTraining(true)
	s.Equal(http.StatusAccepted, status)
	s.Equal("scheduled", resp.Status)
	s.True(resp.Force)
	s.True(resp.Canceled)

	resp, status = s.requestTraining(true)
	s.Equal(http.StatusAccepted, status)
	s.Equal("scheduled", resp.Status)
	s.True(resp.Force)
	s.True(resp.Canceled)

	if s.finishTrainingRun() {
		select {
		case s.scheduled <- struct{}{}:
		default:
		}
	}

	s.False(s.trainingInProgress)
	s.False(s.trainingReplacementQueued)
	s.Equal(1, len(s.scheduled))
}

func (s *MasterTestSuite) TestRequestTrainingForceCancelsActiveRunContext() {
	s.scheduled = make(chan struct{}, 1)

	ctx := s.startTrainingRun()

	resp, status := s.requestTraining(true)
	s.Equal(http.StatusAccepted, status)
	s.Equal("scheduled", resp.Status)
	s.True(resp.Canceled)

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		s.Fail("expected active training context to be canceled")
	}
}

func (s *MasterTestSuite) TestNeedUpdateItemToItemReturnsFalseOnCanceledContext() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.False(s.needUpdateItemToItem(ctx, "item-1", config.ItemToItemConfig{Name: "neighbors"}))
}

func (s *MasterTestSuite) TestNeedUpdateUserToUserReturnsFalseOnCanceledContext() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.False(s.needUpdateUserToUser(ctx, "user-1", config.UserToUserConfig{Name: "neighbors"}))
}
