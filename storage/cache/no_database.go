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

package cache

// NoDatabase means no database used for cache.
type NoDatabase struct{}

// Close method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) Close() error {
	return ErrNoDatabase
}

// Init method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) Init() error {
	return ErrNoDatabase
}

func (NoDatabase) Scan(_ func(string) error) error {
	return ErrNoDatabase
}

func (NoDatabase) Set(_ ...Value) error {
	return ErrNoDatabase
}

// Get method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) Get(_ string) *ReturnValue {
	return &ReturnValue{err: ErrNoDatabase}
}

// Delete method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) Delete(_ string) error {
	return ErrNoDatabase
}

// GetSet method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetSet(_ string) ([]string, error) {
	return nil, ErrNoDatabase
}

// SetSet method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) SetSet(_ string, _ ...string) error {
	return ErrNoDatabase
}

// AddSet method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) AddSet(_ string, _ ...string) error {
	return ErrNoDatabase
}

// RemSet method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) RemSet(_ string, _ ...string) error {
	return ErrNoDatabase
}

// GetSorted method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetSorted(_ string, _, _ int) ([]Scored, error) {
	return nil, ErrNoDatabase
}

// GetSortedByScore method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetSortedByScore(_ string, _, _ float64) ([]Scored, error) {
	return nil, ErrNoDatabase
}

// RemSortedByScore method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) RemSortedByScore(_ string, _, _ float64) error {
	return ErrNoDatabase
}

// AddSorted method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) AddSorted(_ ...SortedSet) error {
	return ErrNoDatabase
}

// SetSorted method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) SetSorted(_ string, _ []Scored) error {
	return ErrNoDatabase
}

// RemSorted method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) RemSorted(_ ...SetMember) error {
	return ErrNoDatabase
}
