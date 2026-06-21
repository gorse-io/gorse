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

import (
	"context"
	"strings"
	"time"

	"github.com/gorse-io/gorse/storage"
	"github.com/juju/errors"
)

type Distance int

const (
	Cosine Distance = iota
	Euclidean
	Dot
)

// QuantizationType represents a vector quantization type.
type QuantizationType string

const (
	QuantizationNone QuantizationType = "none" // No quantization (float32)
	QuantizationSQ   QuantizationType = "sq"   // Scalar quantization
	QuantizationPQ   QuantizationType = "pq"   // Product quantization
	QuantizationRQ   QuantizationType = "rq"   // Rotational quantization/RaBitQ/TurboQuant
)

const (
	defaultHNSWM           = 16
	defaultHNSWEfConstruct = 200
	defaultHNSWEfSearch    = 64
)

// VectorConfig configures vector storage.
type VectorConfig struct {
	// Quantization configures the vector quantization type.
	Quantization QuantizationType // none | sq | pq | rq
	// QuantizationBits is the number of quantization bits. 0 uses the backend default.
	QuantizationBits int
}

// DefaultVectorConfig returns the default vector config.
func DefaultVectorConfig() VectorConfig {
	return VectorConfig{
		Quantization: QuantizationNone,
	}
}

type Vector struct {
	Id         string
	Vector     []float32
	IsHidden   bool      `json:"-"`
	Categories []string  `json:"-" gorm:"type:text;serializer:json"`
	Timestamp  time.Time `json:"-"`
}

type Database interface {
	Init() error
	Optimize() error
	Close() error
	ListCollections(ctx context.Context) ([]string, error)
	AddCollection(ctx context.Context, name string, dimensions int, distance Distance, config VectorConfig) error
	DeleteCollection(ctx context.Context, name string) error
	AddVectors(ctx context.Context, collection string, vectors []Vector) error
	DeleteVectors(ctx context.Context, collection string, timestamp time.Time) error
	QueryVectors(ctx context.Context, collection string, q []float32, categories []string, topK int) ([]Vector, error)
}

// Creator creates a database instance.
type Creator func(path, tablePrefix string, opts ...storage.Option) (Database, error)

var creators = make(map[string]Creator)

var ErrNoDatabase = errors.NotAssignedf("database")

// Register a database creator.
func Register(prefixes []string, creator Creator) {
	for _, p := range prefixes {
		creators[p] = creator
	}
}

// Open a connection to a database.
func Open(path, tablePrefix string, opts ...storage.Option) (Database, error) {
	for prefix, creator := range creators {
		if strings.HasPrefix(path, prefix) {
			return creator(path, tablePrefix, opts...)
		}
	}
	return nil, errors.Errorf("Unknown database: %s", path)
}
