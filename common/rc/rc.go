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
	"io"
	"sync"
	"sync/atomic"

	"github.com/modern-go/reflect2"
)

type DropFunc func()

type rcPointer[T io.Closer] struct {
	pointer   T
	reference atomic.Int32
}

type Rc[T io.Closer] struct {
	pointer *rcPointer[T]
	drop    DropFunc
	mu      sync.RWMutex
}

func (r *Rc[T]) Reset(pointer T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pointer != nil {
		r.drop()
	}
	if reflect2.IsNil(pointer) {
		r.pointer = nil
	} else {
		r.pointer = &rcPointer[T]{pointer: pointer}
		_, r.drop = r.get()
	}
}

func (r *Rc[T]) Get() (T, DropFunc) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.get()
}

func (r *Rc[T]) get() (T, DropFunc) {
	if r.pointer == nil {
		var niL T
		return niL, func() {}
	}
	p := r.pointer
	p.reference.Add(1)
	return p.pointer, func() {
		p.reference.Add(-1)
		if p.reference.Load() == 0 {
			_ = p.pointer.Close()
		}
	}
}

func (r *Rc[T]) Apply(f func(T)) {
	p, drop := r.Get()
	defer drop()
	f(p)
}
