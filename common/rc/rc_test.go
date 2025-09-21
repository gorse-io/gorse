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

package rc

import (
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type Foo struct {
	mock.Mock
}

func NewFoo() *Foo {
	f := &Foo{}
	f.On("Close").Return(nil)
	return f
}

func (f *Foo) Close() error {
	f.Called()
	return nil
}

func TestRc_Get(t *testing.T) {
	f := NewFoo()
	var rc Rc[*Foo]
	rc.Reset(f)
	_, drop := rc.Get()
	drop()
	rc.Reset(nil)
	f.AssertNumberOfCalls(t, "Close", 1)
}

func TestRc_Apply(t *testing.T) {
	f := NewFoo()
	var rc Rc[*Foo]
	rc.Reset(f)
	rc.Apply(func(f *Foo) {
		assert.NotNil(t, f)
	})
	rc.Reset(nil)
	f.AssertNumberOfCalls(t, "Close", 1)
}

func TestRc_Concurrent(t *testing.T) {
	f := make([]*Foo, 1000)
	for i := range f {
		f[i] = NewFoo()
	}

	var rc Rc[*Foo]
	drops := make([]DropFunc, 0)
	for i := range f {
		rc.Reset(f[i])
		cnt := rand.IntN(10)
		for j := 0; j < cnt; j++ {
			_, drop := rc.Get()
			drops = append(drops, drop)
		}
	}

	rc.Reset(nil)
	var wg sync.WaitGroup
	for _, drop := range drops {
		wg.Go(func() {
			drop()
		})
	}
	wg.Wait()
	for i := range f {
		f[i].AssertNumberOfCalls(t, "Close", 1)
	}
}
