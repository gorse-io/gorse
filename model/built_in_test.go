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
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestUnzip(t *testing.T) {
	// Download
	zipName, err := downloadFromUrl("https://cdn.gorse.io/datasets/yelp.zip", os.TempDir())
	assert.Nil(t, err, "download file failed ")
	// Extract files
	fileNames, err := unzip(zipName, DataSetDir)
	// Check
	assert.Nil(t, err, "unzip file failed ")
	assert.Equal(t, 2, len(fileNames), "Number of file doesn't match")
}

func TestLocateBuiltInDataset(t *testing.T) {
	trainFilePath, testFilePath, err := LocateBuiltInDataset("ml-1m", FormatNCF)
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(DataSetDir, "ml-1m", "train.txt"), trainFilePath)
	assert.Equal(t, filepath.Join(DataSetDir, "ml-1m", "test.txt"), testFilePath)
}
