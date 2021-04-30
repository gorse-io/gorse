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
	"os"
)

type LocalCache struct {
	path       string
	WorkerName string
}

func LoadLocalCache(path string) (*LocalCache, error) {
	state := &LocalCache{path: path}
	// check if file exists
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return state, nil
		} else {
			return nil, err
		}
	}
	// open file
	f, err := os.Open(path)
	if err != nil {
		return state, err
	}
	decoder := gob.NewDecoder(f)
	if err = decoder.Decode(&state.WorkerName); err != nil {
		return nil, err
	}
	return state, nil
}

func (c *LocalCache) WriteLocalCache() error {
	// create file
	f, err := os.Create(c.path)
	if err != nil {
		return err
	}
	// write file
	encoder := gob.NewEncoder(f)
	return encoder.Encode(c.WorkerName)
}
