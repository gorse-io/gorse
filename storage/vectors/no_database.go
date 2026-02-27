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

package vectors

import "context"

// NoDatabase means no database used for vectors.
type NoDatabase struct{}

func (NoDatabase) Init() error {
	return ErrNoDatabase
}

func (NoDatabase) Close() error {
	return ErrNoDatabase
}

func (NoDatabase) ListCollections(_ context.Context) ([]string, error) {
	return nil, ErrNoDatabase
}

func (NoDatabase) AddCollection(_ context.Context, _ string, _ int, _ Distance) error {
	return ErrNoDatabase
}

func (NoDatabase) DeleteCollection(_ context.Context, _ string) error {
	return ErrNoDatabase
}

func (NoDatabase) AddVectors(_ context.Context, _ string, _ []Vector) error {
	return ErrNoDatabase
}

func (NoDatabase) QueryVectors(_ context.Context, _ string, _ []float32, _ []string, _ int) ([]Vector, error) {
	return nil, ErrNoDatabase
}
