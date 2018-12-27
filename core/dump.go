package core

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
)

// Load a object from file.
func Load(fileName string, object interface{}) error {
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	if err == nil {
		decoder := gob.NewDecoder(file)
		if err = decoder.Decode(object); err != nil {
			return err
		}
	}
	// Restore parameters
	switch object.(type) {
	case Model:
		model := object.(Model)
		model.SetParams(model.GetParams())
	}
	return nil
}

// Save a object to file.
func Save(fileName string, object interface{}) error {
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
		if err = encoder.Encode(object); err != nil {
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
