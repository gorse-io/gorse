// Copyright 2024 gorse Project Authors
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

package datautil

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/gorse-io/gorse/base/log"
	"github.com/gorse-io/gorse/common/util"
	"go.uber.org/zap"
)

var (
	tempDir    string
	datasetDir string
)

func init() {
	usr, err := user.Current()
	if err != nil {
		log.Logger().Fatal("failed to get user directory", zap.Error(err))
	}
	datasetDir = filepath.Join(usr.HomeDir, ".gorse", "dataset")
	tempDir = filepath.Join(usr.HomeDir, ".gorse", "temp")
}

func LoadIris() ([][]float32, []int, error) {
	// Download dataset
	path, err := DownloadAndUnzip("iris")
	if err != nil {
		return nil, nil, err
	}
	dataFile := filepath.Join(path, "iris.data")
	// Load data
	f, err := os.Open(dataFile)
	if err != nil {
		return nil, nil, err
	}
	reader := csv.NewReader(f)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	// Parse data
	data := make([][]float32, len(rows))
	target := make([]int, len(rows))
	types := make(map[string]int)
	for i, row := range rows {
		data[i] = make([]float32, 4)
		for j, cell := range row[:4] {
			data[i][j], err = util.ParseFloat[float32](cell)
			if err != nil {
				return nil, nil, err
			}
		}
		if _, exist := types[row[4]]; !exist {
			types[row[4]] = len(types)
		}
		target[i] = types[row[4]]
	}
	return data, target, nil
}

func DownloadAndUnzip(name string) (string, error) {
	url := fmt.Sprintf("https://cdn.gorse.io/datasets/%s.zip", name)
	path := filepath.Join(datasetDir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		zipFileName, _ := downloadFromUrl(url, tempDir)
		if _, err := unzip(zipFileName, datasetDir); err != nil {
			return "", err
		}
	}
	return path, nil
}

// downloadFromUrl downloads file from URL.
func downloadFromUrl(src, dst string) (string, error) {
	log.Logger().Info("Download dataset", zap.String("source", src), zap.String("destination", dst))
	// Extract file name
	tokens := strings.Split(src, "/")
	fileName := filepath.Join(dst, tokens[len(tokens)-1])
	// Create file
	if err := os.MkdirAll(filepath.Dir(fileName), os.ModePerm); err != nil {
		return fileName, err
	}
	output, err := os.Create(fileName)
	if err != nil {
		log.Logger().Error("failed to create file", zap.Error(err), zap.String("filename", fileName))
		return fileName, err
	}
	defer output.Close()
	// Download file
	response, err := http.Get(src)
	if err != nil {
		log.Logger().Error("failed to download", zap.Error(err), zap.String("source", src))
		return fileName, err
	}
	defer response.Body.Close()
	// Save file
	_, err = io.Copy(output, response.Body)
	if err != nil {
		log.Logger().Error("failed to download", zap.Error(err), zap.String("source", src))
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
