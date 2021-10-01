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

package base

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"github.com/juju/errors"
	"go.uber.org/zap"
	"io"
	"time"
)

var logger *zap.Logger

func init() {
	SetProductionLogger()
}

// Logger get current logger
func Logger() *zap.Logger {
	return logger
}

// SetProductionLogger set current logger in production mode.
func SetProductionLogger() {
	logger, _ = zap.NewProduction()
}

// SetDevelopmentLogger set current logger in development mode.
func SetDevelopmentLogger() {
	logger, _ = zap.NewDevelopment()
}

// Max finds the maximum in a vector of integers. Panic if the slice is empty.
func Max(a int32, b ...int32) int32 {
	for _, value := range b {
		if value > a {
			a = value
		}
	}
	return a
}

// Min finds the minimum in a vector of integers. Panic if the slice is empty.
func Min(a int, b ...int) int {
	for _, value := range b {
		if value < a {
			a = value
		}
	}
	return a
}

// RangeInt generate a slice [0, ..., n-1].
func RangeInt(n int) []int {
	a := make([]int, n)
	for i := range a {
		a[i] = i
	}
	return a
}

// RepeatFloat32s repeats value n times.
func RepeatFloat32s(n int, value float32) []float32 {
	a := make([]float32, n)
	for i := range a {
		a[i] = value
	}
	return a
}

// NewMatrix32 creates a 2D matrix of 32-bit floats.
func NewMatrix32(row, col int) [][]float32 {
	ret := make([][]float32, row)
	for i := range ret {
		ret[i] = make([]float32, col)
	}
	return ret
}

// NewMatrixInt creates a 2D matrix of integers.
func NewMatrixInt(row, col int) [][]int {
	ret := make([][]int, row)
	for i := range ret {
		ret[i] = make([]int, col)
	}
	return ret
}

// Now returns the current time in the format of `2006-01-02T15:04:05Z07:00`.
func Now() string {
	return time.Now().Format("2006-01-02T15:04:05Z07:00")
}

func DateNow() time.Time {
	timestamp := time.Now()
	return time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 0, 0, 0, 0, time.UTC)
}

// CheckPanic catches panic.
func CheckPanic() {
	if r := recover(); r != nil {
		Logger().Error("panic recovered", zap.Any("panic", r))
	}
}

// Hex returns the hex form of a 64-bit integer.
func Hex(v int64) string {
	return fmt.Sprintf("%x", v)
}

// WriteMatrix writes matrix to byte stream.
func WriteMatrix(w io.Writer, m [][]float32) error {
	for i := range m {
		err := binary.Write(w, binary.LittleEndian, m[i])
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// ReadMatrix reads matrix from byte stream.
func ReadMatrix(r io.Reader, m [][]float32) error {
	for i := range m {
		err := binary.Read(r, binary.LittleEndian, m[i])
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// WriteString writes string to byte stream.
func WriteString(w io.Writer, s string) error {
	return WriteBytes(w, []byte(s))
}

// ReadString reads string from byte stream.
func ReadString(r io.Reader) (string, error) {
	data, err := ReadBytes(r)
	return string(data), err
}

// WriteBytes writes bytes to byte stream.
func WriteBytes(w io.Writer, s []byte) error {
	err := binary.Write(w, binary.LittleEndian, int32(len(s)))
	if err != nil {
		return err
	}
	n, err := w.Write(s)
	if err != nil {
		return err
	} else if n != len(s) {
		return errors.New("fail to write string")
	}
	return nil
}

// ReadBytes reads bytes from byte stream.
func ReadBytes(r io.Reader) ([]byte, error) {
	var length int32
	err := binary.Read(r, binary.LittleEndian, &length)
	if err != nil {
		return nil, err
	}
	data := make([]byte, length)
	n, err := r.Read(data)
	if err != nil {
		return nil, err
	} else if n != len(data) {
		return nil, errors.New("fail to read string")
	}
	return data, nil
}

// WriteGob writes object to byte stream.
func WriteGob(w io.Writer, v interface{}) error {
	buffer := bytes.NewBuffer(nil)
	encoder := gob.NewEncoder(buffer)
	err := encoder.Encode(v)
	if err != nil {
		return err
	}
	return WriteBytes(w, buffer.Bytes())
}

// ReadGob read object from byte stream.
func ReadGob(r io.Reader, v interface{}) error {
	data, err := ReadBytes(r)
	if err != nil {
		return err
	}
	buffer := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buffer)
	return decoder.Decode(v)
}
