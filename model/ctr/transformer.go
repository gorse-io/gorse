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

package ctr

import (
	"io"

	"github.com/chewxy/math32"
	"github.com/gorse-io/gorse/common/encoding"
	"github.com/juju/errors"
)

// MinMaxScaler transforms a single feature by scaling to [0, 1] range.
// The transformation is given by:
//
//	X_scaled = (X - X.min) / (X.max - X.min)
type MinMaxScaler struct {
	Min float32
	Max float32
}

// NewMinMaxScaler creates a MinMaxScaler.
func NewMinMaxScaler() *MinMaxScaler {
	return &MinMaxScaler{
		Min: math32.Inf(1),
		Max: math32.Inf(-1),
	}
}

// Fit computes the minimum and maximum values from the given values.
func (s *MinMaxScaler) Fit(values []float32) {
	for _, v := range values {
		if v < s.Min {
			s.Min = v
		}
		if v > s.Max {
			s.Max = v
		}
	}
}

// Transform scales a value to [0, 1] range.
func (s *MinMaxScaler) Transform(value float32) float32 {
	range_ := s.Max - s.Min
	if range_ == 0 {
		return 0.5
	}
	return (value - s.Min) / range_
}

// Marshal writes the scaler to a writer.
func (s *MinMaxScaler) Marshal(w io.Writer) error {
	if err := encoding.WriteGob(w, s.Min); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.WriteGob(w, s.Max); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// Unmarshal reads the scaler from a reader.
func (s *MinMaxScaler) Unmarshal(r io.Reader) error {
	if err := encoding.ReadGob(r, &s.Min); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.ReadGob(r, &s.Max); err != nil {
		return errors.Trace(err)
	}
	return nil
}
