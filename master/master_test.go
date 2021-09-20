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
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/storage/cache"
	"github.com/zhenghaoz/gorse/storage/data"
	"testing"
)

type mockMaster struct {
	Master
	dataStoreServer  *miniredis.Miniredis
	cacheStoreServer *miniredis.Miniredis
}

func (m *mockMaster) Close() {
	m.dataStoreServer.Close()
	m.cacheStoreServer.Close()
}

func newMockMaster(t *testing.T) *mockMaster {
	s := new(mockMaster)
	s.taskMonitor = NewTaskMonitor()
	// create mock database
	var err error
	s.dataStoreServer, err = miniredis.Run()
	assert.NoError(t, err)
	s.cacheStoreServer, err = miniredis.Run()
	assert.NoError(t, err)
	// open database
	s.DataClient, err = data.Open("redis://" + s.dataStoreServer.Addr())
	assert.NoError(t, err)
	s.CacheClient, err = cache.Open("redis://" + s.cacheStoreServer.Addr())
	assert.NoError(t, err)
	return s
}
