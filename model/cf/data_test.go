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
package cf

import (
	"crypto/md5"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"log"
	"os"
	"strconv"
	"testing"
)

func TestNewMapIndexDataset(t *testing.T) {
	dataSet := NewMapIndexDataset()
	for i := 0; i < 4; i++ {
		for j := i; j < 5; j++ {
			dataSet.AddFeedback(strconv.Itoa(i), strconv.Itoa(j), true)
		}
	}
	assert.Equal(t, 14, dataSet.Count())
	assert.Equal(t, 4, dataSet.UserCount())
	assert.Equal(t, 5, dataSet.ItemCount())
	dataSet.AddUser("10")
	dataSet.AddItem("10")
	assert.Equal(t, 5, dataSet.UserCount())
	assert.Equal(t, 6, dataSet.ItemCount())
}

func md5Sum(fileName string) string {
	// Open file
	f, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	// Generate check sum
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func TestLoadDataFromCSV(t *testing.T) {
	data := LoadDataFromCSV("../../misc/csv/feedback.csv", ",", true)
	assert.Equal(t, 5, data.Count())
	for i := 0; i < data.Count(); i++ {
		userIndex, itemIndex := data.GetIndex(i)
		assert.Equal(t, i, userIndex)
		assert.Equal(t, i, itemIndex)
	}
}
