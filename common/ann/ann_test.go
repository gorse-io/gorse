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

package search

import (
	"bufio"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base/floats"
	"github.com/zhenghaoz/gorse/common/dataset"
	"github.com/zhenghaoz/gorse/common/util"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	trainSize = 6000
	testSize  = 1000
)

func recall(gt, pred []lo.Tuple2[int, float32]) float64 {
	s := mapset.NewSet[int]()
	for _, pair := range gt {
		s.Add(pair.A)
	}
	hit := 0
	for _, pair := range pred {
		if s.Contains(pair.A) {
			hit++
		}
	}
	return float64(hit) / float64(len(gt))
}

type MNIST struct {
	TrainImages [][]float32
	TrainLabels []uint8
	TestImages  [][]float32
	TestLabels  []uint8
}

func mnist() (*MNIST, error) {
	// Download and unzip dataset
	path, err := dataset.DownloadAndUnzip("mnist")
	if err != nil {
		return nil, err
	}
	// Open dataset
	m := new(MNIST)
	m.TrainImages, m.TrainLabels, err = m.openFile(filepath.Join(path, "train.libfm"))
	if err != nil {
		return nil, err
	}
	m.TestImages, m.TestLabels, err = m.openFile(filepath.Join(path, "test.libfm"))
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *MNIST) openFile(path string) ([][]float32, []uint8, error) {
	// Open file
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	// Read data line by line
	var (
		images [][]float32
		labels []uint8
	)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		splits := strings.Split(line, " ")
		// Parse label
		label, err := util.ParseUInt[uint8](splits[0])
		if err != nil {
			return nil, nil, err
		}
		labels = append(labels, label)
		// Parse image
		image := make([]float32, 784)
		for _, split := range splits[1:] {
			kv := strings.Split(split, ":")
			index, err := strconv.Atoi(kv[0])
			if err != nil {
				return nil, nil, err
			}
			value, err := util.ParseFloat[float32](kv[1])
			if err != nil {
				return nil, nil, err
			}
			image[index] = value
		}
		images = append(images, image)
	}
	return images, labels, nil
}

func TestMNIST(t *testing.T) {
	dat, err := mnist()
	assert.NoError(t, err)

	// Create brute-force index
	bf := NewBruteforce(floats.Euclidean)
	for _, image := range dat.TrainImages[:trainSize] {
		_, err := bf.Add(image)
		assert.NoError(t, err)
	}

	// Create HNSW index
	hnsw := NewHNSW(floats.Euclidean)
	for _, image := range dat.TrainImages[:trainSize] {
		_, err := hnsw.Add(image)
		assert.NoError(t, err)
	}

	// Test search
	r := 0.0
	for _, image := range dat.TestImages[:testSize] {
		gt, err := bf.SearchVector(image, 100, false)
		assert.NoError(t, err)
		assert.Len(t, gt, 100)
		scores, err := hnsw.SearchVector(image, 100, false)
		assert.NoError(t, err)
		assert.Len(t, scores, 100)
		r += recall(gt, scores)
	}
	assert.Greater(t, r, 0.99)
}
