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
	"os"
	"path/filepath"
	"testing"

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

func TestNewMasterBlobServer(t *testing.T) {
	t.Run("local URI", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "blob")
		server := newMasterBlobServer(dir)
		if server == nil {
			t.Fatal("expected local blob server")
		}
		if _, err := os.Stat(dir); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("cloud URI", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		server := newMasterBlobServer("s3://bucket/path")
		if server != nil {
			t.Fatal("expected no local blob server for cloud URI")
		}
		if _, err := os.Stat(filepath.Join(dir, "s3:")); !os.IsNotExist(err) {
			t.Fatalf("expected no s3: directory, got %v", err)
		}
	})
}
