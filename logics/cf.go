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
	"io"

	"github.com/gorse-io/gorse/common/encoding"
	"github.com/pkg/errors"
)

type MatrixFactorizationUsers struct {
	embeddings map[string][]float32
}

func NewMatrixFactorizationUsers() *MatrixFactorizationUsers {
	return &MatrixFactorizationUsers{
		embeddings: make(map[string][]float32),
	}
}

func (users *MatrixFactorizationUsers) Add(userId string, v []float32) {
	users.embeddings[userId] = v
}

func (users *MatrixFactorizationUsers) Get(userId string) ([]float32, bool) {
	v, ok := users.embeddings[userId]
	return v, ok
}

func (users *MatrixFactorizationUsers) Marshal(w io.Writer) error {
	numUsers := int64(len(users.embeddings))
	if err := encoding.WriteGob(w, numUsers); err != nil {
		return errors.WithStack(err)
	}
	for userId, embedding := range users.embeddings {
		if err := encoding.WriteString(w, userId); err != nil {
			return errors.WithStack(err)
		}
		if err := encoding.WriteSlice(w, embedding); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (users *MatrixFactorizationUsers) Unmarshal(r io.Reader) error {
	var numUsers int64
	if err := encoding.ReadGob(r, &numUsers); err != nil {
		return errors.WithStack(err)
	}
	users.embeddings = make(map[string][]float32, numUsers)
	for i := int64(0); i < numUsers; i++ {
		userId, err := encoding.ReadString(r)
		if err != nil {
			return errors.WithStack(err)
		}
		var embedding []float32
		if err = encoding.ReadSlice(r, &embedding); err != nil {
			return errors.WithStack(err)
		}
		users.embeddings[userId] = embedding
	}
	return nil
}
