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

package logics

import (
	"os"
	"strings"

	"github.com/pkg/errors"
	"modernc.org/quickjs"
)

type External struct {
	vm *quickjs.VM
}

func NewExternal() (*External, error) {
	vm, err := quickjs.NewVM()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Add environment variables
	env, err := vm.NewObjectValue()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, err := vm.NewAtom(parts[0])
		if err != nil {
			return nil, errors.WithStack(err)
		}
		value := parts[1]
		if err := env.SetProperty(key, value); err != nil {
			return nil, errors.WithStack(err)
		}
	}
	envKey, err := vm.NewAtom("env")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := vm.GlobalObject().SetProperty(envKey, env); err != nil {
		return nil, errors.WithStack(err)
	}

	return &External{vm: vm}, nil
}

func (e *External) Close() error {
	return e.vm.Close()
}
