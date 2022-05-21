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
	"encoding/binary"
	std_errors "errors"
	"github.com/juju/errors"
	"github.com/zhenghaoz/gorse/base/encoding"
	"github.com/zhenghaoz/gorse/base/log"
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
	RankingModel        ranking.MatrixFactorization
	RankingModelScore   ranking.Score
	ClickModelVersion   int64
	ClickModelScore     click.Score
	ClickModel          click.FactorizationMachine
}

// LoadLocalCache loads local cache from a file.
// If the ranking model is invalid, RankingModel == nil.
// If the click model is invalid, ClickModel == nil.
func LoadLocalCache(path string) (*LocalCache, error) {
	log.Logger().Info("load cache", zap.String("path", path))
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
	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			log.Logger().Error("fail to close file", zap.Error(err))
		}
	}(f)
	// 1. ranking model name
	state.RankingModelName, err = encoding.ReadString(f)
	if err != nil {
		return state, errors.Trace(err)
	}
	// 2. ranking model version
	err = binary.Read(f, binary.LittleEndian, &state.RankingModelVersion)
	if err != nil {
		return state, errors.Trace(err)
	}
	// 3. ranking model
	state.RankingModel, err = ranking.UnmarshalModel(f)
	if err != nil {
		return state, errors.Trace(err)
	}
	// 4. ranking model score
	err = encoding.ReadGob(f, &state.RankingModelScore)
	if err != nil {
		return state, errors.Trace(err)
	}
	// 7. click model version
	err = binary.Read(f, binary.LittleEndian, &state.ClickModelVersion)
	if err != nil {
		return state, errors.Trace(err)
	}
	// 8. click model score
	err = encoding.ReadGob(f, &state.ClickModelScore)
	if err != nil {
		return state, errors.Trace(err)
	}
	// 9. click model
	state.ClickModel, err = click.UnmarshalModel(f)
	if err != nil {
		return state, errors.Trace(err)
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
			return errors.Trace(err)
		}
	}
	// create file
	f, err := os.Create(c.path)
	if err != nil {
		return errors.Trace(err)
	}
	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			log.Logger().Error("fail to close file", zap.Error(err))
		}
	}(f)
	// 1. ranking model name
	err = encoding.WriteString(f, c.RankingModelName)
	if err != nil {
		return errors.Trace(err)
	}
	// 2. ranking model version
	err = binary.Write(f, binary.LittleEndian, c.RankingModelVersion)
	if err != nil {
		return errors.Trace(err)
	}
	// 3. ranking model
	err = ranking.MarshalModel(f, c.RankingModel)
	if err != nil {
		return errors.Trace(err)
	}
	// 4. ranking model score
	err = encoding.WriteGob(f, c.RankingModelScore)
	if err != nil {
		return errors.Trace(err)
	}
	// 7. click model version
	err = binary.Write(f, binary.LittleEndian, c.ClickModelVersion)
	if err != nil {
		return errors.Trace(err)
	}
	// 8. click model score
	err = encoding.WriteGob(f, c.ClickModelScore)
	if err != nil {
		return errors.Trace(err)
	}
	// 9. click model
	err = click.MarshalModel(f, c.ClickModel)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
