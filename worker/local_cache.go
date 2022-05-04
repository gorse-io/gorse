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

package worker

import (
	"encoding/gob"
	std_errors "errors"
	"github.com/juju/errors"
	"os"
	"path/filepath"
)

// LocalCache for the worker node.
type LocalCache struct {
	path       string
	WorkerName string
}

// LoadLocalCache loads cache from a local file.
func LoadLocalCache(path string) (*LocalCache, error) {
	state := &LocalCache{path: path}
	// check if file exists
	if _, err := os.Stat(path); err != nil {
		if std_errors.Is(err, os.ErrNotExist) {
			return state, errors.NotFoundf("cache file %s", path)
		}
		return state, errors.Trace(err)
	}
	// open file
	f, err := os.Open(path)
	if err != nil {
		return state, errors.Trace(err)
	}
	defer f.Close()
	decoder := gob.NewDecoder(f)
	if err = decoder.Decode(&state.WorkerName); err != nil {
		return state, errors.Trace(err)
	}
	return state, nil
}

// WriteLocalCache writes cache to a local file.
func (c *LocalCache) WriteLocalCache() error {
	// create parent folder if not exists
	parent := filepath.Dir(c.path)
	if _, err := os.Stat(parent); os.IsNotExist(err) {
		err = os.MkdirAll(parent, os.ModePerm)
		if err != nil {
			return errors.Trace(err)
		}
	}
	// create file
	f, err := os.Create(c.path)
	if err != nil {
		return errors.Trace(err)
	}
	defer f.Close()
	// write file
	encoder := gob.NewEncoder(f)
	return errors.Trace(encoder.Encode(c.WorkerName))
}
