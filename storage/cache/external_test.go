// Copyright 2026 gorse Project Authors
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

package cache_test

import (
	"errors"
	"testing"

	"github.com/gorse-io/gorse/storage/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExternalDatabaseAPI(t *testing.T) {
	value := cache.String("name", "value")
	assert.Equal(t, "name", value.Name())
	assert.Equal(t, "value", value.Value())

	ret := cache.NewReturnValue("value", true)
	actual, err := ret.String()
	require.NoError(t, err)
	assert.Equal(t, "value", actual)
	assert.True(t, ret.Exists())

	expectedErr := errors.New("database error")
	ret = cache.NewReturnValueWithError(expectedErr)
	_, err = ret.String()
	assert.ErrorIs(t, err, expectedErr)
	assert.False(t, ret.Exists())
}
