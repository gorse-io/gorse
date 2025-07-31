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

package ann

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/gorse-io/gorse/common/datautil"
	"github.com/gorse-io/gorse/common/floats"
	"github.com/gorse-io/gorse/common/util"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"
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
	path, err := datautil.DownloadAndUnzip("mnist")
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
		_ = bf.Add(image)
	}

	// Create HNSW index
	hnsw := NewHNSW(floats.Euclidean)
	for _, image := range dat.TrainImages[:trainSize] {
		_ = hnsw.Add(image)
	}

	// Test search
	r := 0.0
	for _, image := range dat.TestImages[:testSize] {
		gt := bf.SearchVector(image, 100, false)
		assert.Len(t, gt, 100)
		scores := hnsw.SearchVector(image, 100, false)
		assert.Len(t, scores, 100)
		r += recall(gt, scores)
	}
	r /= float64(testSize)
	assert.Greater(t, r, 0.99)
}

func TestMultithread(t *testing.T) {
	dat, err := mnist()
	assert.NoError(t, err)

	// Create HNSW index
	indices := make([]int, trainSize)
	hnsw := NewHNSW(floats.Euclidean)
	var wg1 sync.WaitGroup
	wg1.Add(trainSize)
	for i := range dat.TrainImages[:trainSize] {
		go func(i int) {
			defer wg1.Done()
			indices[i] = hnsw.Add(dat.TrainImages[i])
		}(i)
	}
	wg1.Wait()

	// Create brute-force index
	reverse := make([]int, trainSize)
	for i, index := range indices {
		reverse[index] = i
	}
	bf := NewBruteforce(floats.Euclidean)
	for i := range reverse {
		_ = bf.Add(dat.TrainImages[reverse[i]])
	}

	// Test search
	var r atomic.Float64
	var wg2 sync.WaitGroup
	wg2.Add(testSize)
	for _, image := range dat.TestImages[:testSize] {
		go func(image []float32) {
			defer wg2.Done()
			gt := bf.SearchVector(image, 100, false)
			assert.Len(t, gt, 100)
			scores := hnsw.SearchVector(image, 100, false)
			assert.Len(t, scores, 100)
			r.Add(recall(gt, scores))
		}(image)
	}
	wg2.Wait()
	assert.Greater(t, r.Load()/float64(testSize), 0.99)
}

func movieLens() ([][]int, error) {
	// Download and unzip dataset
	path, err := datautil.DownloadAndUnzip("ml-1m")
	if err != nil {
		return nil, err
	}
	// Open file
	f, err := os.Open(filepath.Join(path, "train.txt"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Read data line by line
	movies := make([][]int, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		splits := strings.Split(line, "\t")
		userId, err := strconv.Atoi(splits[0])
		if err != nil {
			return nil, err
		}
		movieId, err := strconv.Atoi(splits[1])
		if err != nil {
			return nil, err
		}
		for movieId >= len(movies) {
			movies = append(movies, make([]int, 0))
		}
		movies[movieId] = append(movies[movieId], userId)
	}
	return movies, nil
}

func jaccard(a, b []int) float32 {
	var i, j, intersection int
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			intersection++
			i++
			j++
		} else if a[i] < b[j] {
			i++
		} else {
			j++
		}
	}
	if len(a)+len(b)-intersection == 0 {
		return 1
	}
	return 1 - float32(intersection)/float32(len(a)+len(b)-intersection)
}

func TestMovieLens(t *testing.T) {
	movies, err := movieLens()
	assert.NoError(t, err)

	// Create brute-force index
	bf := NewBruteforce(jaccard)
	for _, movie := range movies {
		_ = bf.Add(movie)
	}

	// Create HNSW index
	hnsw := NewHNSW(jaccard)
	for _, movie := range movies {
		_ = hnsw.Add(movie)
	}

	// Test search
	r := 0.0
	for i := range movies[:testSize] {
		gt, err := bf.SearchIndex(i, 100, false)
		assert.NoError(t, err)
		scores, err := hnsw.SearchIndex(i, 100, false)
		assert.NoError(t, err)
		r += recall(gt, scores)
	}
	r /= float64(testSize)
	assert.Greater(t, r, 0.98)
}
