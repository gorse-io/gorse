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
	"sort"

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
	if s.Min > s.Max {
		return value
	}
	range_ := s.Max - s.Min
	if range_ == 0 {
		return 1
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
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

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
	if s.IQR == 0 {
		return value
	}
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

// AutoScaler automatically selects the appropriate scaling method based on data distribution.
// - If data contains negative values: uses RobustScaler
// - If all values are non-negative: applies log1p then MinMaxScaler
type AutoScaler struct {
	UseLog bool         // true if log1p preprocessing is applied, false if RobustScaler is used
	MinMax MinMaxScaler // for non-negative values (after log1p if UseLog)
	Robust RobustScaler // for data with negative values
}

// NewAutoScaler creates an AutoScaler.
func NewAutoScaler() *AutoScaler {
	return &AutoScaler{
		MinMax: *NewMinMaxScaler(),
	}
}

// Fit analyzes the data and selects the appropriate scaling method.
func (s *AutoScaler) Fit(values []float32) {
	if len(values) == 0 {
		return
	}

	// Check for negative values
	hasNegative := false
	for _, v := range values {
		if v < 0 {
			hasNegative = true
			break
		}
	}

	if hasNegative {
		// Use RobustScaler then MinMaxScaler for data with negative values
		s.UseLog = false
		s.Robust.Fit(values)

		robustValues := make([]float32, len(values))
		for i, v := range values {
			robustValues[i] = s.Robust.Transform(v)
		}
		s.MinMax.Fit(robustValues)
	} else {
		// Apply log1p then MinMaxScaler for non-negative data
		s.UseLog = true

		// Apply log1p transformation
		logValues := make([]float32, len(values))
		for i, v := range values {
			logValues[i] = math32.Log1p(v)
		}
		s.MinMax.Fit(logValues)
	}
}

// Transform scales a value using the selected method.
func (s *AutoScaler) Transform(value float32) float32 {
	if !s.UseLog {
		robustValue := s.Robust.Transform(value)
		return s.MinMax.Transform(robustValue)
	}

	// Apply log1p then MinMax
	logValue := math32.Log1p(max(0, value))
	return s.MinMax.Transform(logValue)
}

// Marshal writes the scaler to a writer.
func (s *AutoScaler) Marshal(w io.Writer) error {
	if err := encoding.WriteGob(w, s.UseLog); err != nil {
		return errors.Trace(err)
	}
	if err := s.MinMax.Marshal(w); err != nil {
		return errors.Trace(err)
	}
	if !s.UseLog {
		if err := s.Robust.Marshal(w); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// Unmarshal reads the scaler from a reader.
func (s *AutoScaler) Unmarshal(r io.Reader) error {
	if err := encoding.ReadGob(r, &s.UseLog); err != nil {
		return errors.Trace(err)
	}
	if err := s.MinMax.Unmarshal(r); err != nil {
		return errors.Trace(err)
	}
	if !s.UseLog {
		if err := s.Robust.Unmarshal(r); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}
