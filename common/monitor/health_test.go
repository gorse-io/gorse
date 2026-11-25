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

package monitor

import (
	"encoding/json/v2"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type driverError struct{ Code int }

func (*driverError) Error() string { return "driver failure" }

func TestHealthStatusJSON(t *testing.T) {
	b, err := json.Marshal(HealthStatus{DataStoreError: errors.New("opaque"), CacheStoreError: &driverError{Code: 42}})
	require.NoError(t, err)
	require.JSONEq(t, `{"Ready":false,"DataStoreError":{},"CacheStoreError":{"Code":42},"DataStoreConnected":false,"CacheStoreConnected":false}`, string(b))
	b, err = json.Marshal(HealthStatus{})
	require.NoError(t, err)
	require.JSONEq(t, `{"Ready":false,"DataStoreError":null,"CacheStoreError":null,"DataStoreConnected":false,"CacheStoreConnected":false}`, string(b))
}
