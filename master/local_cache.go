// Copyright 2021 gorse Project Authors
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

package master

import (
	"encoding/gob"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model/ranking"
	"os"
	"path/filepath"
)

// LocalCache is local cache for the master node.
type LocalCache struct {
	path                string
	RankingModelName    string
	RankingModelVersion int64
	RankingModel        ranking.Model
	RankingScore        ranking.Score
	UserIndex           base.Index
}

// LoadLocalCache loads local cache from a file.
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
	// 1. model name
	err = decoder.Decode(&state.RankingModelName)
	if err != nil {
		return state, err
	}
	// 2. model version
	err = decoder.Decode(&state.RankingModelVersion)
	if err != nil {
		return state, err
	}
	// 3. model
	state.RankingModel, err = ranking.NewModel(state.RankingModelName, nil)
	if err != nil {
		return state, err
	}
	err = decoder.Decode(state.RankingModel)
	if err != nil {
		return state, err
	}
	state.RankingModel.SetParams(state.RankingModel.GetParams())
	// 4. model score
	err = decoder.Decode(&state.RankingScore)
	if err != nil {
		return state, err
	}
	// 5. user index
	state.UserIndex = base.NewMapIndex()
	err = decoder.Decode(state.UserIndex)
	if err != nil {
		return state, err
	}
	return state, nil
}

// WriteLocalCache writes local cache to a file.
func (c *LocalCache) WriteLocalCache() error {
	// create parent folder if not exists
	parent := filepath.Dir(c.path)
	if _, err := os.Stat(parent); os.IsNotExist(err) {
		err = os.MkdirAll(parent, os.ModePerm)
		if err != nil {
			return err
		}
	}
	// create file
	f, err := os.Create(c.path)
	if err != nil {
		return err
	}
	encoder := gob.NewEncoder(f)
	// 1. model name
	err = encoder.Encode(c.RankingModelName)
	if err != nil {
		return err
	}
	// 2. model version
	err = encoder.Encode(c.RankingModelVersion)
	if err != nil {
		return err
	}
	// 3. model
	err = encoder.Encode(c.RankingModel)
	if err != nil {
		return err
	}
	// 4. model score
	err = encoder.Encode(c.RankingScore)
	if err != nil {
		return err
	}
	// 5. user index
	return encoder.Encode(c.UserIndex)
}
