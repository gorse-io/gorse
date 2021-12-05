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
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/zhenghaoz/gorse/base"
	"go.uber.org/zap"
)

type DatasetFormat int

const (
	FormatNCF DatasetFormat = iota
	FormatLibFM
)

// Built-in Data set
type _BuiltInDataSet struct {
	downloadURL string
	trainFile   string
	testFile    string
	format      DatasetFormat
}

var builtInDataSets = map[string]_BuiltInDataSet{
	"pinterest-20": {
		downloadURL: "https://cdn.gorse.io/datasets/pinterest-20.zip",
		trainFile:   "pinterest-20/train.txt",
		testFile:    "pinterest-20/test.txt",
		format:      FormatNCF,
	},
	"ml-1m": {
		downloadURL: "https://cdn.gorse.io/datasets/ml-1m.zip",
		trainFile:   "ml-1m/train.txt",
		testFile:    "ml-1m/test.txt",
		format:      FormatNCF,
	},
	"ml-tag": {
		downloadURL: "https://cdn.gorse.io/datasets/ml-tag.zip",
		trainFile:   "ml-tag/train.libfm",
		testFile:    "ml-tag/test.libfm",
		format:      FormatLibFM,
	},
	"frappe": {
		downloadURL: "https://cdn.gorse.io/datasets/frappe.zip",
		trainFile:   "frappe/train.libfm",
		testFile:    "frappe/test.libfm",
		format:      FormatLibFM,
	},
	"criteo": {
		downloadURL: "https://cdn.gorse.io/datasets/criteo.zip",
		trainFile:   "criteo/train.libfm",
		testFile:    "criteo/test.libfm",
		format:      FormatLibFM,
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
		base.Logger().Fatal("failed to get user directory", zap.Error(err))
	}

	GorseDir = usr.HomeDir + "/.gorse"
	DataSetDir = GorseDir + "/dataset"
	TempDir = GorseDir + "/temp"

	// create all folders
	if err = os.MkdirAll(DataSetDir, os.ModePerm); err != nil {
		base.Logger().Fatal("failed to create directory", zap.Error(err), zap.String("path", DataSetDir))
	}
	if err = os.MkdirAll(TempDir, os.ModePerm); err != nil {
		base.Logger().Fatal("failed to create directory", zap.Error(err), zap.String("path", TempDir))
	}
}

func LocateBuiltInDataset(name string, format DatasetFormat) (string, string, error) {
	// Extract Data set information
	dataSet, exist := builtInDataSets[name]
	if !exist {
		return "", "", fmt.Errorf("no such dataset %v", name)
	}
	if dataSet.format != format {
		return "", "", fmt.Errorf("format not matchs %v != %v", format, dataSet.format)
	}
	// Download if not exists
	trainFilePah := filepath.Join(DataSetDir, dataSet.trainFile)
	testFilePath := filepath.Join(DataSetDir, dataSet.testFile)
	if _, err := os.Stat(trainFilePah); os.IsNotExist(err) {
		zipFileName, _ := downloadFromUrl(dataSet.downloadURL, TempDir)
		if _, err := unzip(zipFileName, DataSetDir); err != nil {
			return "", "", err
		}
	}
	return trainFilePah, testFilePath, nil
}

// downloadFromUrl downloads file from URL.
func downloadFromUrl(src, dst string) (string, error) {
	base.Logger().Info("Download dataset", zap.String("source", src))
	// Extract file name
	tokens := strings.Split(src, "/")
	fileName := filepath.Join(dst, tokens[len(tokens)-1])
	// Create file
	if err := os.MkdirAll(filepath.Dir(fileName), os.ModePerm); err != nil {
		return fileName, err
	}
	output, err := os.Create(fileName)
	if err != nil {
		base.Logger().Error("failed to create file", zap.Error(err), zap.String("filename", fileName))
		return fileName, err
	}
	defer output.Close()
	// Download file
	response, err := http.Get(src)
	if err != nil {
		base.Logger().Error("failed to download", zap.Error(err), zap.String("source", src))
		return fileName, err
	}
	defer response.Body.Close()
	// Save file
	_, err = io.Copy(output, response.Body)
	if err != nil {
		base.Logger().Error("failed to download", zap.Error(err), zap.String("source", src))
		return fileName, err
	}
	return fileName, nil
}

// unzip zip file.
func unzip(src, dst string) ([]string, error) {
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
			if err != nil {
				return nil, err
			}
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
