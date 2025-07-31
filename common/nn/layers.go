// Copyright 2024 gorse Project Authors
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

package nn

import (
	"io"
	"reflect"
	"strconv"

	"github.com/chewxy/math32"
	"github.com/gorse-io/gorse/protocol"
	"github.com/juju/errors"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
)

type Layer interface {
	Parameters() []*Tensor
	Forward(x *Tensor) *Tensor
	SetJobs(jobs int)
}

type Model Layer

type LinearLayer struct {
	W    *Tensor
	B    *Tensor
	jobs int
}

func NewLinear(in, out int) Layer {
	bound := 1.0 / math32.Sqrt(float32(in))
	return &LinearLayer{
		W: Uniform(-bound, bound, in, out),
		B: Zeros(out),
	}
}

func (l *LinearLayer) Forward(x *Tensor) *Tensor {
	return Add(MatMul(x, l.W, false, false, l.jobs), l.B)
}

func (l *LinearLayer) Parameters() []*Tensor {
	return []*Tensor{l.W, l.B}
}

func (l *LinearLayer) SetJobs(jobs int) {
	l.jobs = max(1, jobs)
}

type flattenLayer struct{}

func NewFlatten() Layer {
	return &flattenLayer{}
}

func (f *flattenLayer) Parameters() []*Tensor {
	return nil
}

func (f *flattenLayer) Forward(x *Tensor) *Tensor {
	return Flatten(x)
}

func (f *flattenLayer) SetJobs(int) {}

type EmbeddingLayer struct {
	W *Tensor
}

func NewEmbedding(n int, shape ...int) Layer {
	wShape := append([]int{n}, shape...)
	return &EmbeddingLayer{
		W: Normal(0, 0.01, wShape...),
	}
}

func (e *EmbeddingLayer) Parameters() []*Tensor {
	return []*Tensor{e.W}
}

func (e *EmbeddingLayer) Forward(x *Tensor) *Tensor {
	return Embedding(e.W, x)
}

func (e *EmbeddingLayer) SetJobs(int) {}

type sigmoidLayer struct{}

func NewSigmoid() Layer {
	return &sigmoidLayer{}
}

func (s *sigmoidLayer) Parameters() []*Tensor {
	return nil
}

func (s *sigmoidLayer) Forward(x *Tensor) *Tensor {
	return Sigmoid(x)
}

func (e *sigmoidLayer) SetJobs(int) {}

type reluLayer struct{}

func NewReLU() Layer {
	return &reluLayer{}
}

func (r *reluLayer) Parameters() []*Tensor {
	return nil
}

func (r *reluLayer) Forward(x *Tensor) *Tensor {
	return ReLu(x)
}

func (r *reluLayer) SetJobs(int) {}

type Sequential struct {
	Layers []Layer
}

func NewSequential(layers ...Layer) Model {
	return &Sequential{Layers: layers}
}

func (s *Sequential) Parameters() []*Tensor {
	var params []*Tensor
	for _, l := range s.Layers {
		params = append(params, l.Parameters()...)
	}
	return params
}

func (s *Sequential) Forward(x *Tensor) *Tensor {
	for _, l := range s.Layers {
		x = l.Forward(x)
	}
	return x
}

func (s *Sequential) SetJobs(jobs int) {
	for _, l := range s.Layers {
		l.SetJobs(jobs)
	}
}

func Save(o any, w io.Writer) error {
	var save func(o any, key []string) error
	save = func(o any, key []string) error {
		switch typed := o.(type) {
		case *Tensor:
			pb := typed.toPB()
			pb.Key = key
			_, err := pbutil.WriteDelimited(w, pb)
			if err != nil {
				return err
			}
		default:
			tp := reflect.TypeOf(o)
			if tp.Kind() == reflect.Ptr {
				return save(reflect.ValueOf(o).Elem().Interface(), key)
			} else if tp.Kind() == reflect.Struct {
				for i := 0; i < tp.NumField(); i++ {
					field := tp.Field(i)
					if field.IsExported() {
						newKey := make([]string, len(key))
						copy(newKey, key)
						newKey = append(newKey, field.Name)
						if err := save(reflect.ValueOf(o).Field(i).Interface(), newKey); err != nil {
							return err
						}
					}
				}
			} else if tp.Kind() == reflect.Slice {
				for i := 0; i < reflect.ValueOf(o).Len(); i++ {
					newKey := make([]string, len(key))
					copy(newKey, key)
					newKey = append(newKey, strconv.Itoa(i))
					if err := save(reflect.ValueOf(o).Index(i).Interface(), newKey); err != nil {
						return err
					}
				}
			} else {
				return errors.New("unexpected type")
			}
		}
		return nil
	}
	return save(o, nil)
}

func Load(o any, r io.Reader) error {
	var place func(o any, key []string, pb *protocol.Tensor) error
	place = func(o any, key []string, pb *protocol.Tensor) error {
		switch typed := o.(type) {
		case *Tensor:
			typed.fromPB(pb)
		default:
			tp := reflect.TypeOf(o)
			if tp.Kind() == reflect.Ptr {
				return place(reflect.ValueOf(o).Elem().Interface(), key, pb)
			} else if tp.Kind() == reflect.Struct {
				field := reflect.ValueOf(o).FieldByName(key[0])
				if field.IsValid() {
					if err := place(field.Interface(), key[1:], pb); err != nil {
						return err
					}
				}
			} else if tp.Kind() == reflect.Slice {
				index, err := strconv.Atoi(key[0])
				if err != nil {
					return err
				}
				elem := reflect.ValueOf(o).Index(index)
				if elem.IsValid() {
					if err := place(elem.Interface(), key[1:], pb); err != nil {
						return err
					}
				}
			} else {
				return errors.New("unexpected type")
			}
		}
		return nil
	}

	// Read data
	for {
		pb := new(protocol.Tensor)
		if _, err := pbutil.ReadDelimited(r, pb); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if err := place(o, pb.Key, pb); err != nil {
			return err
		}
	}
	return nil
}
