package master

import (
	"encoding/gob"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model/pr"
	"os"
	"path/filepath"
)

type LocalCache struct {
	path         string
	ModelName    string
	ModelVersion int64
	Model        pr.Model
	ModelScore   pr.Score
	UserIndex    base.Index
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
	// 1. model name
	err = decoder.Decode(&state.ModelName)
	if err != nil {
		return state, err
	}
	// 2. model version
	err = decoder.Decode(&state.ModelVersion)
	if err != nil {
		return state, err
	}
	// 3. model
	state.Model, err = pr.NewModel(state.ModelName, nil)
	if err != nil {
		return state, err
	}
	err = decoder.Decode(state.Model)
	if err != nil {
		return state, err
	}
	state.Model.SetParams(state.Model.GetParams())
	// 4. model score
	err = decoder.Decode(&state.ModelScore)
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
	err = encoder.Encode(c.ModelName)
	if err != nil {
		return err
	}
	// 2. model version
	err = encoder.Encode(c.ModelVersion)
	if err != nil {
		return err
	}
	// 3. model
	err = encoder.Encode(c.Model)
	if err != nil {
		return err
	}
	// 4. model score
	err = encoder.Encode(c.ModelScore)
	if err != nil {
		return err
	}
	// 5. user index
	return encoder.Encode(c.UserIndex)
}
