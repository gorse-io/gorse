package core

import (
	"bytes"
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"
)

// Load a object from file.
func Load(fileName string, src interface{}) error {
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	if err == nil {
		decoder := gob.NewDecoder(file)
		if err = decoder.Decode(src); err != nil {
			return err
		}
	}
	// Restore parameters
	switch src.(type) {
	case Model:
		model := src.(Model)
		model.SetParams(model.GetParams())
	default:
		return errors.New("the file is not a model dump")
	}
	return nil
}

// Save a object to file.
func Save(fileName string, dst interface{}) error {
	// Create all directories
	if err := os.MkdirAll(filepath.Dir(fileName), os.ModePerm); err != nil {
		return err
	}
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	if err == nil {
		encoder := gob.NewEncoder(file)
		if err = encoder.Encode(dst); err != nil {
			return err
		}
	}
	return nil
}

// Copy a object from src to dst.
func Copy(dst, src interface{}) error {
	buffer := new(bytes.Buffer)
	encoder := gob.NewEncoder(buffer)
	if err := encoder.Encode(src); err != nil {
		return err
	}
	decoder := gob.NewDecoder(buffer)
	err := decoder.Decode(dst)
	return err
}
