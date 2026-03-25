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
// If all values are non-negative, it applies log1p transformation first.
// The transformation is given by:
//
//	Without log: X_scaled = (X - X.min) / (X.max - X.min)
//	With log:    X_scaled = (log1p(X) - log1p(X.min)) / (log1p(X.max) - log1p(X.min))
type MinMaxScaler struct {
	Min    float32
	Max    float32
	UseLog bool // true if log1p preprocessing is applied
}

// NewMinMaxScaler creates a MinMaxScaler.
func NewMinMaxScaler() *MinMaxScaler {
	return &MinMaxScaler{
		Min:    math32.Inf(1),
		Max:    math32.Inf(-1),
		UseLog: false,
	}
}

// Fit computes the minimum and maximum values from the given values.
// If all values are non-negative, it enables log1p preprocessing.
func (s *MinMaxScaler) Fit(values []float32) {
	hasNegative := false
	for _, v := range values {
		if v < s.Min {
			s.Min = v
		}
		if v > s.Max {
			s.Max = v
		}
		if v < 0 {
			hasNegative = true
		}
	}
	// Use log1p preprocessing if all values are non-negative
	s.UseLog = !hasNegative
}

// Transform scales a value to [0, 1] range.
func (s *MinMaxScaler) Transform(value float32) float32 {
	var minVal, maxVal, val float32
	if s.UseLog {
		minVal = math32.Log1p(s.Min)
		maxVal = math32.Log1p(s.Max)
		val = math32.Log1p(value)
	} else {
		minVal = s.Min
		maxVal = s.Max
		val = value
	}

	range_ := maxVal - minVal
	if range_ == 0 {
		return 0.5
	}
	return (val - minVal) / range_
}

// Marshal writes the scaler to a writer.
func (s *MinMaxScaler) Marshal(w io.Writer) error {
	if err := encoding.WriteGob(w, s.Min); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.WriteGob(w, s.Max); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.WriteGob(w, s.UseLog); err != nil {
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
	if err := encoding.ReadGob(r, &s.UseLog); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// RobustScaler transforms features using statistics that are robust to outliers.
// It uses the median and interquartile range (IQR) for scaling:
//
//	X_scaled = (X - median) / IQR
//
// where IQR = Q3 - Q1 (75th percentile - 25th percentile)
type RobustScaler struct {
	Median float32
	Q1     float32 // 25th percentile
	Q3     float32 // 75th percentile
	IQR    float32 // Interquartile range (Q3 - Q1)
}

// NewRobustScaler creates a RobustScaler.
func NewRobustScaler() *RobustScaler {
	return &RobustScaler{}
}

// Fit computes the median and IQR from the given values.
func (s *RobustScaler) Fit(values []float32) {
	if len(values) == 0 {
		return
	}

	// Sort values to compute percentiles
	sorted := make([]float32, len(values))
	copy(sorted, values)
	sortFloat32(sorted)

	n := len(sorted)
	
	// Compute median
	if n%2 == 0 {
		s.Median = (sorted[n/2-1] + sorted[n/2]) / 2
	} else {
		s.Median = sorted[n/2]
	}

	// Compute Q1 (25th percentile) and Q3 (75th percentile)
	q1Index := n / 4
	q3Index := 3 * n / 4

	if q1Index >= n {
		q1Index = n - 1
	}
	if q3Index >= n {
		q3Index = n - 1
	}

	s.Q1 = sorted[q1Index]
	s.Q3 = sorted[q3Index]
	s.IQR = s.Q3 - s.Q1

	// Avoid division by zero
	if s.IQR == 0 {
		s.IQR = 1.0
	}
}

// Transform scales a value using median and IQR.
func (s *RobustScaler) Transform(value float32) float32 {
	return (value - s.Median) / s.IQR
}

// Marshal writes the scaler to a writer.
func (s *RobustScaler) Marshal(w io.Writer) error {
	if err := encoding.WriteGob(w, s.Median); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.WriteGob(w, s.Q1); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.WriteGob(w, s.Q3); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.WriteGob(w, s.IQR); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// Unmarshal reads the scaler from a reader.
func (s *RobustScaler) Unmarshal(r io.Reader) error {
	if err := encoding.ReadGob(r, &s.Median); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.ReadGob(r, &s.Q1); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.ReadGob(r, &s.Q3); err != nil {
		return errors.Trace(err)
	}
	if err := encoding.ReadGob(r, &s.IQR); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// sortFloat32 sorts a slice of float32 in ascending order.
func sortFloat32(s []float32) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
