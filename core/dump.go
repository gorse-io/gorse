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
	defer file.Close()
	if err == nil {
		decoder := gob.NewDecoder(file)
		decoder.Decode(object)
	}
	return err
}

// Save a object to file.
func Save(fileName string, object interface{}) error {
	// Create all directories
	if err := os.MkdirAll(filepath.Dir(fileName), os.ModePerm); err != nil {
		return err
	}
	file, err := os.Create(fileName)
	defer file.Close()
	if err == nil {
		encoder := gob.NewEncoder(file)
		encoder.Encode(object)
	}
	return err
}

// Copy a object from src to dst.
func Copy(dst, src interface{}) {
	buffer := new(bytes.Buffer)
	encoder := gob.NewEncoder(buffer)
	encoder.Encode(src)
	decoder := gob.NewDecoder(buffer)
	decoder.Decode(dst)
}
