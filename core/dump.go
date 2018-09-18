package core

import (
	"encoding/gob"
	"os"
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
	file, err := os.Create(fileName)
	defer file.Close()
	if err == nil {
		encoder := gob.NewEncoder(file)
		encoder.Encode(object)
	}
	return err
}
