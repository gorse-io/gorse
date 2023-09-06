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
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/zhenghaoz/gorse/base/progress"
	"github.com/zhenghaoz/gorse/config"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
)

type MasterTestSuite struct {
	suite.Suite
	Master
}

func (s *MasterTestSuite) SetupTest() {
	// open database
	var err error
	s.tracer = progress.NewTracer("test")
	s.Settings = config.NewSettings()
	tmpDir := s.T().TempDir()
	//tmpDir = "F:\\www\\4thpd\\gorse-master-lbd\\temp\\sqlite"
	fmt.Println("++37++ TempDir", s.T().TempDir(), tmpDir)
	s.DataClient, err = data.Open(fmt.Sprintf("sqlite://%s/data.db", tmpDir), "")
	//s.DataClient, err = data.Open("mysql://newuser:Password_123@tcp(172.27.128.220:3306)/gorse", "test_")
	s.NoError(err)
	s.CacheClient, err = cache.Open(fmt.Sprintf("sqlite://%s/cache.db", tmpDir), "")
	//s.CacheClient, err = cache.Open("mysql://newuser:Password_123@tcp(172.27.128.220:3306)/gorse", "cache_")
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
