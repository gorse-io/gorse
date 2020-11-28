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
package model

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// Built-in Data set
type _BuiltInDataSet struct {
	downloadURL string
	trainFile   string
	testFile    string
}

var builtInDataSets = map[string]_BuiltInDataSet{
	"pinterest-20": {
		downloadURL: "https://cdn.sine-x.com/datasets/pinterest-20.zip",
		trainFile:   "pinterest-20/train.txt",
		testFile:    "pinterest-20/test.txt",
	},
	"ml-1m": {
		downloadURL: "https://cdn.sine-x.com/datasets/ml-1m.zip",
		trainFile:   "ml-1m/train.txt",
		testFile:    "ml-1m/test.txt",
	},
}

// The Data directories
var (
	GorseDir   string
	DataSetDir string
	TempDir    string
)

func init() {
	usr, err := user.Current()
	if err != nil {
		log.Fatal("Error while init() file built_in.go in package core "+
			"see https://github.com/zhenghaoz/gorse/issues/3", err)
	}

	GorseDir = usr.HomeDir + "/.gorse"
	DataSetDir = GorseDir + "/dataset"
	TempDir = GorseDir + "/temp"

	// create all folders
	if err = os.MkdirAll(DataSetDir, os.ModePerm); err != nil {
		panic(err)
	}
	if err = os.MkdirAll(TempDir, os.ModePerm); err != nil {
		panic(err)
	}
}

func loadTest(dataset *DataSet, path string) error {
	// Open
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	// Read lines
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		positive, negative := fields[0], fields[1:]
		if positive[0] != '(' || positive[len(positive)-1] != ')' {
			return fmt.Errorf("wrong foramt: %v", line)
		}
		positive = positive[1 : len(positive)-1]
		fields = strings.Split(positive, ",")
		userId, itemId := fields[0], fields[1]
		dataset.Add(userId, itemId, true)
		dataset.SetNegatives(userId, negative)
	}
	return scanner.Err()
}

func loadTrain(path string) (*DataSet, error) {
	// Open
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	// Read lines
	scanner := bufio.NewScanner(file)
	dataset := NewDirectIndexDataset()
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		userId, itemId := fields[0], fields[1]
		dataset.Add(userId, itemId, true)
	}
	return dataset, scanner.Err()
}

// loadDataFromBuiltIn loads a built-in Data set. Now support:
func loadDataFromBuiltIn(dataSetName string) (*DataSet, *DataSet) {
	// Extract Data set information
	dataSet, exist := builtInDataSets[dataSetName]
	if !exist {
		log.Fatal("no such Data set ", dataSetName)
	}
	dataFileName := filepath.Join(DataSetDir, dataSet.trainFile)
	// Download if not exists
	if _, err := os.Stat(dataFileName); os.IsNotExist(err) {
		zipFileName, _ := downloadFromUrl(dataSet.downloadURL, TempDir)
		if _, err := unzip(zipFileName, DataSetDir); err != nil {
			panic(err)
		}
	}
	// Load dataset
	trainSet, err := loadTrain(filepath.Join(DataSetDir, dataSet.trainFile))
	if err != nil {
		log.Fatal(err)
	}
	testSet := NewDirectIndexDataset()
	testSet.UserIndex = trainSet.UserIndex
	testSet.ItemIndex = trainSet.ItemIndex
	err = loadTest(testSet, filepath.Join(DataSetDir, dataSet.testFile))
	if err != nil {
		log.Fatal(err)
	}
	return trainSet, testSet
}

// downloadFromUrl downloads file from URL.
func downloadFromUrl(src string, dst string) (string, error) {
	fmt.Printf("Download dataset from %s\n", src)
	// Extract file name
	tokens := strings.Split(src, "/")
	fileName := filepath.Join(dst, tokens[len(tokens)-1])
	// Create file
	if err := os.MkdirAll(filepath.Dir(fileName), os.ModePerm); err != nil {
		return fileName, err
	}
	output, err := os.Create(fileName)
	if err != nil {
		log.Println("Error while creating", fileName, "-", err)
		return fileName, err
	}
	defer output.Close()
	// Download file
	response, err := http.Get(src)
	if err != nil {
		log.Println("Error while downloading", src, "-", err)
		return fileName, err
	}
	defer response.Body.Close()
	// Save file
	_, err = io.Copy(output, response.Body)
	if err != nil {
		log.Println("Error while downloading", src, "-", err)
		return fileName, err
	}
	return fileName, nil
}

// unzip zip file.
func unzip(src string, dst string) ([]string, error) {
	log.Printf("Unzip dataset %s\n", src)
	var fileNames []string
	// Open zip file
	r, err := zip.OpenReader(src)
	if err != nil {
		return fileNames, err
	}
	defer r.Close()
	// Extract files
	for _, f := range r.File {
		// Open file
		rc, err := f.Open()
		if err != nil {
			return fileNames, err
		}
		// Store filename/path for returning and using later on
		filePath := filepath.Join(dst, f.Name)
		// Check for ZipSlip. More Info: http://bit.ly/2MsjAWE
		if !strings.HasPrefix(filePath, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fileNames, fmt.Errorf("%s: illegal file path", filePath)
		}
		// Add filename
		fileNames = append(fileNames, filePath)
		if f.FileInfo().IsDir() {
			// Create folder
			if err = os.MkdirAll(filePath, os.ModePerm); err != nil {
				return fileNames, err
			}
		} else {
			// Create all folders
			if err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
				return fileNames, err
			}
			// Create file
			outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return fileNames, err
			}
			// Save file
			_, err = io.Copy(outFile, rc)
			// Close the file without defer to close before next iteration of loop
			err = outFile.Close()
			if err != nil {
				return nil, err
			}
		}
		// Close file
		err = rc.Close()
		if err != nil {
			return nil, err
		}
	}
	return fileNames, nil
}
