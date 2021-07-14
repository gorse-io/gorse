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

package server

import (
	"encoding/gob"
	"os"
	"path/filepath"
)

// LocalCache is local cache for the server node.
type LocalCache struct {
	path       string
	ServerName string
}

// LoadLocalCache loads local cache from a file.
func LoadLocalCache(path string) (*LocalCache, error) {
	state := &LocalCache{path: path}
	// check if file exists
	if _, err := os.Stat(path); err != nil {
		return state, err
	}
	// open file
	f, err := os.Open(path)
	if err != nil {
		return state, err
	}
	defer f.Close()
	decoder := gob.NewDecoder(f)
	if err = decoder.Decode(&state.ServerName); err != nil {
		return nil, err
	}
	return state, nil
}

// WriteLocalCache writes local cache to a file.
func (s *LocalCache) WriteLocalCache() error {
	// create parent folder if not exists
	parent := filepath.Dir(s.path)
	if _, err := os.Stat(parent); os.IsNotExist(err) {
		err = os.MkdirAll(parent, os.ModePerm)
		if err != nil {
			return err
		}
	}
	// create file
	f, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer f.Close()
	// write file
	encoder := gob.NewEncoder(f)
	return encoder.Encode(s.ServerName)
}
