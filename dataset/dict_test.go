// Copyright 2025 gorse Project Authors
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

package dataset

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFreqDict(t *testing.T) {
	dict := NewFreqDict()
	assert.Equal(t, int32(0), dict.Add("a"))
	assert.Equal(t, int32(1), dict.Add("b"))
	assert.Equal(t, int32(1), dict.Add("b"))
	assert.Equal(t, int32(2), dict.Add("c"))
	assert.Equal(t, int32(2), dict.Add("c"))
	assert.Equal(t, int32(2), dict.Add("c"))
	assert.Equal(t, int32(3), dict.Count())
	assert.Equal(t, int32(1), dict.Freq(0))
	assert.Equal(t, int32(2), dict.Freq(1))
	assert.Equal(t, int32(3), dict.Freq(2))
	assert.Equal(t, int32(0), dict.Id("a"))
	assert.Equal(t, int32(-1), dict.Id("e"))
}

func TestFreqDictLock(t *testing.T) {
	dict := NewFreqDict()
	index := dict.AddNoCount("a")
	dict.Lock(index)

	locked := make(chan struct{})
	released := make(chan struct{})
	var acquired atomic.Bool
	go func() {
		close(locked)
		dict.Lock(index)
		acquired.Store(true)
		dict.Unlock(index)
		close(released)
	}()

	<-locked
	select {
	case <-released:
		t.Fatal("lock should block concurrent access")
	case <-time.After(10 * time.Millisecond):
	}
	assert.False(t, acquired.Load())

	dict.Unlock(index)

	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("lock should be released")
	}
	assert.True(t, acquired.Load())
}
