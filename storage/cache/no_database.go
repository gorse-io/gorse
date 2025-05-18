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

import (
	"context"
	"time"
)

// NoDatabase means no database used for cache.
type NoDatabase struct{}

// Close method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) Close() error {
	return ErrNoDatabase
}

func (NoDatabase) Ping() error {
	return ErrNoDatabase
}

// Init method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) Init() error {
	return ErrNoDatabase
}

func (NoDatabase) Scan(_ func(string) error) error {
	return ErrNoDatabase
}

func (NoDatabase) Purge() error {
	return ErrNoDatabase
}

func (NoDatabase) Set(_ context.Context, _ ...Value) error {
	return ErrNoDatabase
}

// Get method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) Get(_ context.Context, _ string) *ReturnValue {
	return &ReturnValue{err: ErrNoDatabase}
}

// Delete method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) Delete(_ context.Context, _ string) error {
	return ErrNoDatabase
}

// GetSet method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) GetSet(_ context.Context, _ string) ([]string, error) {
	return nil, ErrNoDatabase
}

// SetSet method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) SetSet(_ context.Context, _ string, _ ...string) error {
	return ErrNoDatabase
}

// AddSet method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) AddSet(_ context.Context, _ string, _ ...string) error {
	return ErrNoDatabase
}

// RemSet method of NoDatabase returns ErrNoDatabase.
func (NoDatabase) RemSet(_ context.Context, _ string, _ ...string) error {
	return ErrNoDatabase
}

func (NoDatabase) Push(_ context.Context, _, _ string) error {
	return ErrNoDatabase
}

func (NoDatabase) Pop(_ context.Context, _ string) (string, error) {
	return "", ErrNoDatabase
}

func (NoDatabase) Remain(_ context.Context, _ string) (int64, error) {
	return 0, ErrNoDatabase
}

func (NoDatabase) AddScores(_ context.Context, _, _ string, _ []Score) error {
	return ErrNoDatabase
}

func (NoDatabase) SearchScores(_ context.Context, _, _ string, _ []string, _, _ int) ([]Score, error) {
	return nil, ErrNoDatabase
}

func (NoDatabase) UpdateScores(context.Context, []string, *string, string, ScorePatch) error {
	return ErrNoDatabase
}

func (NoDatabase) DeleteScores(_ context.Context, _ []string, _ ScoreCondition) error {
	return ErrNoDatabase
}

func (NoDatabase) ScanScores(context.Context, func(collection, id, subset string, timestamp time.Time) error) error {
	return ErrNoDatabase
}

func (NoDatabase) AddTimeSeriesPoints(_ context.Context, _ []TimeSeriesPoint) error {
	return ErrNoDatabase
}

func (NoDatabase) GetTimeSeriesPoints(_ context.Context, _ string, _, _ time.Time, _ time.Duration) ([]TimeSeriesPoint, error) {
	return nil, ErrNoDatabase
}
