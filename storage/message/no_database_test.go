// Copyright 2022 gorse Project Authors
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
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNoDatabase(t *testing.T) {
	var database NoDatabase
	err := database.Init()
	assert.ErrorIs(t, err, ErrNoDatabase)
	err = database.Push("test", Message{})
	assert.ErrorIs(t, err, ErrNoDatabase)
	_, err = database.Pop("test")
	assert.ErrorIs(t, err, ErrNoDatabase)
}
