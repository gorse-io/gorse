package core

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
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
func Copy(dst, src interface{}) {
	buffer := new(bytes.Buffer)
	encoder := gob.NewEncoder(buffer)
	encoder.Encode(src)
	decoder := gob.NewDecoder(buffer)
	decoder.Decode(dst)
}

// Save data set to csv.
func (dataSet *RawDataSet) ToCSV(fileName string, sep string) error {
	file, err := os.Create(fileName)
	defer file.Close()
	if err == nil {
		writer := bufio.NewWriter(file)
		for i := range dataSet.Ratings {
			writer.WriteString(fmt.Sprintf("%v%s%v%s%v\n",
				dataSet.Users[i], sep,
				dataSet.Items[i], sep,
				dataSet.Ratings[i]))
		}
	}
	return err
}

// Predict ratings for a set of <userId, itemId>s.
func Predict(dataSet DataSet, estimator Model) []float64 {
	predictions := make([]float64, dataSet.Len())
	for j := 0; j < dataSet.Len(); j++ {
		userId, itemId, _ := dataSet.Get(j)
		predictions[j] = estimator.Predict(userId, itemId)
	}
	return predictions
}
