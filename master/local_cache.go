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
	"github.com/pkg/errors"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model/click"
	"github.com/zhenghaoz/gorse/model/ranking"
	"go.uber.org/zap"
	"os"
	"path/filepath"
)

// LocalCache is local cache for the master node.
type LocalCache struct {
	path                string
	RankingModelName    string
	RankingModelVersion int64
	RankingModel        ranking.Model
	RankingModelScore   ranking.Score
	UserIndexVersion    int64
	UserIndex           base.Index
	ClickModelVersion   int64
	ClickModelScore     click.Score
	ClickModel          click.FactorizationMachine
}

// LoadLocalCache loads local cache from a file.
// If the ranking model is invalid, RankingModel == nil.
// If the click model is invalid, ClickModel == nil.
func LoadLocalCache(path string) (*LocalCache, error) {
	base.Logger().Info("load cache", zap.String("path", path))
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
	// 1. ranking model name
	err = decoder.Decode(&state.RankingModelName)
	if err != nil {
		return state, errors.Wrap(err, "failed to load ranking model name")
	}
	// 2. ranking model version
	err = decoder.Decode(&state.RankingModelVersion)
	if err != nil {
		return state, errors.Wrap(err, "failed to load ranking model version")
	}
	// 3. ranking model
	state.RankingModel, err = ranking.NewModel(state.RankingModelName, nil)
	if err != nil {
		return state, errors.Wrap(err, "failed to create ranking model")
	}
	err = decoder.Decode(state.RankingModel)
	if err != nil {
		return state, errors.Wrap(err, "failed to decode ranking model")
	}
	state.RankingModel.SetParams(state.RankingModel.GetParams())
	// 4. ranking model score
	err = decoder.Decode(&state.RankingModelScore)
	if err != nil {
		return state, errors.Wrap(err, "failed to decode ranking model score")
	}
	// 5. user index version
	err = decoder.Decode(&state.UserIndexVersion)
	if err != nil {
		return state, errors.Wrap(err, "failed to decode user index version")
	}
	// 6. user index
	state.UserIndex = base.NewMapIndex()
	err = decoder.Decode(state.UserIndex)
	if err != nil {
		return state, errors.Wrap(err, "failed to load user index")
	}
	// 7. click model version
	err = decoder.Decode(&state.ClickModelVersion)
	if err != nil {
		return state, errors.Wrap(err, "failed to load click model version")
	}
	// 8. click model score
	err = decoder.Decode(&state.ClickModelScore)
	if err != nil {
		return state, errors.Wrap(err, "failed to load click model score")
	}
	// 9. click model
	state.ClickModel = click.NewFM(click.FMClassification, nil)
	err = decoder.Decode(state.ClickModel)
	if err != nil {
		return state, errors.Wrap(err, "failed to load click model")
	}
	state.ClickModel.SetParams(state.ClickModel.GetParams())
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
		return errors.Wrap(err, "failed to create file")
	}
	defer f.Close()
	encoder := gob.NewEncoder(f)
	// 1. ranking model name
	err = encoder.Encode(c.RankingModelName)
	if err != nil {
		return errors.Wrap(err, "failed to write ranking model name")
	}
	// 2. ranking model version
	err = encoder.Encode(c.RankingModelVersion)
	if err != nil {
		return errors.Wrap(err, "failed to write ranking model version")
	}
	// 3. ranking model
	err = encoder.Encode(c.RankingModel)
	if err != nil {
		return errors.Wrap(err, "failed to write ranking model")
	}
	// 4. ranking model score
	err = encoder.Encode(c.RankingModelScore)
	if err != nil {
		return errors.Wrap(err, "failed to write ranking model score")
	}
	// 5. user index version
	err = encoder.Encode(c.UserIndexVersion)
	if err != nil {
		return errors.Wrap(err, "failed to write user index version")
	}
	// 6. user index
	err = encoder.Encode(c.UserIndex)
	if err != nil {
		return errors.Wrap(err, "failed to write user index")
	}
	// 7. click model version
	err = encoder.Encode(c.ClickModelVersion)
	if err != nil {
		return errors.Wrap(err, "failed to write click model version")
	}
	// 8. click model score
	err = encoder.Encode(c.ClickModelScore)
	if err != nil {
		return errors.Wrap(err, "failed to write click model score")
	}
	// 9. click model
	err = encoder.Encode(c.ClickModel)
	if err != nil {
		return errors.Wrap(err, "failed to write click model")
	}
	return nil
}
