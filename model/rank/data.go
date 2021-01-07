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
package rank

import (
	"bufio"
	log "github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/model"
	"os"
	"strconv"
	"strings"
)

type Dataset struct {
	UnifiedIndex   UnifiedIndex
	FeedbackUsers  []int
	FeedbackItems  []int
	FeedbackLabels [][]int
	FeedbackTarget []float32
	UserItemLabels [][]int
}

func (dataset *Dataset) Len() int {
	return len(dataset.FeedbackTarget)
}

func (dataset *Dataset) Get(i int) ([]int, float32) {
	x := make([]int, 0)
	if dataset.FeedbackUsers != nil {
		// append user id
		userIndex := dataset.FeedbackUsers[i]
		x = append(x, userIndex)
		// append user labels
		if dataset.UserItemLabels != nil {
			x = append(x, dataset.UserItemLabels[userIndex]...)
		}
	}
	if dataset.FeedbackItems != nil {
		// append item id
		itemIndex := dataset.FeedbackItems[i]
		x = append(x, itemIndex)
		// append item labels
		if dataset.UserItemLabels != nil {
			x = append(x, dataset.UserItemLabels[itemIndex]...)
		}
	}
	return append(x, dataset.FeedbackLabels[i]...), dataset.FeedbackTarget[i]
}

func LoadLibFMFile(path string) (labels [][]int, targets []float32, maxLabel int, err error) {
	labels = make([][]int, 0)
	targets = make([]float32, 0)
	// open file
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, 0, err
	}
	defer file.Close()
	// read lines
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, " ")
		// fetch target
		target, err := strconv.ParseFloat(fields[0], 32)
		if err != nil {
			return nil, nil, 0, err
		}
		targets = append(targets, float32(target))
		// fetch labels
		lineLabels := make([]int, len(fields[1:]))
		for i, field := range fields[1:] {
			kv := strings.Split(field, ":")
			k, v := kv[0], kv[1]
			if v != "1" {
				log.Error("load: support binary features only")
			}
			label, err := strconv.Atoi(k)
			if err != nil {
				return nil, nil, 0, err
			}
			lineLabels[i] = label
			maxLabel = base.Max(maxLabel, base.Max(lineLabels...))
		}
		labels = append(labels, lineLabels)
	}
	// check error
	if err = scanner.Err(); err != nil {
		return nil, nil, 0, err
	}
	return
}

func LoadDataFromBuiltIn(name string) (train *Dataset, test *Dataset, err error) {
	trainFilePath, testFilePath, err := model.LocateBuiltInDataset(name, model.FormatLibFM)
	if err != nil {
		return nil, nil, err
	}
	train, test = &Dataset{}, &Dataset{}
	trainMaxLabel, testMaxLabel := 0, 0
	if train.FeedbackLabels, train.FeedbackTarget, trainMaxLabel, err = LoadLibFMFile(trainFilePath); err != nil {
		return nil, nil, err
	}
	if test.FeedbackLabels, test.FeedbackTarget, testMaxLabel, err = LoadLibFMFile(testFilePath); err != nil {
		return nil, nil, err
	}
	unifiedIndex := NewUnifiedDirectIndex(base.Max(trainMaxLabel, testMaxLabel) + 1)
	train.UnifiedIndex = unifiedIndex
	test.UnifiedIndex = unifiedIndex
	return
}
