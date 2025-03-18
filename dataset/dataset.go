// Copyright 2025 gorse Project Authors
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

package dataset

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chewxy/math32"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/juju/errors"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/common/util"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/storage/data"
	"modernc.org/strutil"
)

type ID int32

// CFSplit is the dataset split for collaborative filtering.
type CFSplit interface {
	// CountUsers returns the number of users.
	CountUsers() int
	// CountItems returns the number of items.
	CountItems() int
	// CountFeedback returns the number of (positive) feedback.
	CountFeedback() int
	// GetUserDict returns the frequency dictionary of users.
	GetUserDict() *FreqDict
	// GetItemDict returns the frequency dictionary of items.
	GetItemDict() *FreqDict
	// GetUserFeedback returns the (positive) feedback of users.
	GetUserFeedback() [][]int32
	// GetItemFeedback returns the (positive) feedback of items.
	GetItemFeedback() [][]int32
	// SampleUserNegatives samples negative (feedback) for users.
	SampleUserNegatives(excludeSet CFSplit, numCandidates int) [][]int32
}

// CTRSplit is the dataset split for click-through rate prediction.
type CTRSplit interface {
	Count() int
	CountUsers() int
	CountItems() int
	CountUserLabels() int
	CountItemLabels() int
	CountContextLabels() int
	CountPositive() int
	CountNegative() int
	GetIndex() base.UnifiedIndex
	GetTarget(i int) float32
	Get(i int) ([]int32, []float32, float32)
}

type Dataset struct {
	timestamp    time.Time
	users        []data.User
	items        []data.Item
	userLabels   *Labels
	itemLabels   *Labels
	userFeedback [][]int32
	itemFeedback [][]int32
	negatives    [][]int32
	userDict     *FreqDict
	itemDict     *FreqDict
	numFeedback  int
	categories   mapset.Set[string]
}

func NewDataset(timestamp time.Time, userCount, itemCount int) *Dataset {
	return &Dataset{
		timestamp:    timestamp,
		users:        make([]data.User, 0, userCount),
		items:        make([]data.Item, 0, itemCount),
		userLabels:   NewLabels(),
		itemLabels:   NewLabels(),
		userFeedback: make([][]int32, userCount),
		itemFeedback: make([][]int32, itemCount),
		userDict:     NewFreqDict(),
		itemDict:     NewFreqDict(),
		categories:   mapset.NewSet[string](),
	}
}

func (d *Dataset) GetTimestamp() time.Time {
	return d.timestamp
}

func (d *Dataset) CountFeedback() int {
	return d.numFeedback
}

func (d *Dataset) GetUsers() []data.User {
	return d.users
}

func (d *Dataset) GetUserDict() *FreqDict {
	return d.userDict
}

func (d *Dataset) CountUsers() int {
	return len(d.users)
}

func (d *Dataset) GetItems() []data.Item {
	return d.items
}

func (d *Dataset) GetItemDict() *FreqDict {
	return d.itemDict
}

func (d *Dataset) CountItems() int {
	return len(d.items)
}

func (d *Dataset) GetUserFeedback() [][]int32 {
	return d.userFeedback
}

func (d *Dataset) GetItemFeedback() [][]int32 {
	return d.itemFeedback
}

func (d *Dataset) GetCategories() []string {
	return d.categories.ToSlice()
}

// GetUserIDF returns the IDF of users.
//
//	IDF(u) = log(I/freq(u))
//
// I is the number of items.
// freq(u) is the frequency of user u in all feedback.
func (d *Dataset) GetUserIDF() []float32 {
	idf := make([]float32, d.userDict.Count())
	for i := int32(0); i < d.userDict.Count(); i++ {
		// Since zero IDF will cause NaN in the future, we set the minimum value to 1e-3.
		idf[i] = max(math32.Log(float32(len(d.items))/float32(d.userDict.Freq(i))), 1e-3)
	}
	return idf
}

// GetItemIDF returns the IDF of items.
//
//	IDF(i) = log(U/freq(i))
//
// U is the number of users.
// freq(i) is the frequency of item i in all feedback.
func (d *Dataset) GetItemIDF() []float32 {
	idf := make([]float32, d.itemDict.Count())
	for i := int32(0); i < d.itemDict.Count(); i++ {
		// Since zero IDF will cause NaN in the future, we set the minimum value to 1e-3.
		idf[i] = max(math32.Log(float32(len(d.users))/float32(d.itemDict.Freq(i))), 1e-3)
	}
	return idf
}

func (d *Dataset) GetUserColumnValuesIDF() []float32 {
	idf := make([]float32, d.userLabels.values.Count())
	for i := int32(0); i < d.userLabels.values.Count(); i++ {
		// Since zero IDF will cause NaN in the future, we set the minimum value to 1e-3.
		idf[i] = max(math32.Log(float32(len(d.users))/float32(d.userLabels.values.Freq(i))), 1e-3)
	}
	return idf
}

func (d *Dataset) GetItemColumnValuesIDF() []float32 {
	idf := make([]float32, d.itemLabels.values.Count())
	for i := int32(0); i < d.itemLabels.values.Count(); i++ {
		// Since zero IDF will cause NaN in the future, we set the minimum value to 1e-3.
		idf[i] = max(math32.Log(float32(len(d.items))/float32(d.itemLabels.values.Freq(i))), 1e-3)
	}
	return idf
}

func (d *Dataset) AddUser(user data.User) {
	d.users = append(d.users, data.User{
		UserId:    user.UserId,
		Labels:    d.userLabels.processLabels(user.Labels, ""),
		Subscribe: user.Subscribe,
		Comment:   user.Comment,
	})
	d.userDict.AddNoCount(user.UserId)
	if len(d.userFeedback) < len(d.users) {
		d.userFeedback = append(d.userFeedback, nil)
	}
}

func (d *Dataset) AddItem(item data.Item) {
	d.items = append(d.items, data.Item{
		ItemId:     item.ItemId,
		IsHidden:   item.IsHidden,
		Categories: item.Categories,
		Timestamp:  item.Timestamp,
		Labels:     d.itemLabels.processLabels(item.Labels, ""),
		Comment:    item.Comment,
	})
	d.itemDict.AddNoCount(item.ItemId)
	if len(d.itemFeedback) < len(d.items) {
		d.itemFeedback = append(d.itemFeedback, nil)
	}
	d.categories.Append(item.Categories...)
}

func (d *Dataset) AddFeedback(userId, itemId string) {
	userIndex := d.userDict.Add(userId)
	itemIndex := d.itemDict.Add(itemId)
	d.userFeedback[userIndex] = append(d.userFeedback[userIndex], itemIndex)
	d.itemFeedback[itemIndex] = append(d.itemFeedback[itemIndex], userIndex)
	d.numFeedback++
}

func (d *Dataset) SampleUserNegatives(excludeSet CFSplit, numCandidates int) [][]int32 {
	if len(d.negatives) == 0 {
		rng := base.NewRandomGenerator(0)
		d.negatives = make([][]int32, d.CountUsers())
		for userIndex := 0; userIndex < d.CountUsers(); userIndex++ {
			s1 := mapset.NewSet(d.GetUserFeedback()[userIndex]...)
			s2 := mapset.NewSet(excludeSet.GetUserFeedback()[userIndex]...)
			d.negatives[userIndex] = rng.SampleInt32(0, int32(d.CountItems()), numCandidates, s1, s2)
		}
	}
	return d.negatives
}

// SplitCF splits dataset by user-leave-one-out method. The argument `numTestUsers` determines the number of users in the test
// set. If numTestUsers is equal or greater than the number of total users or numTestUsers <= 0, all users are presented
// in the test set.
func (d *Dataset) SplitCF(numTestUsers int, seed int64) (CFSplit, CFSplit) {
	trainSet, testSet := new(Dataset), new(Dataset)
	trainSet.users, testSet.users = d.users, d.users
	trainSet.items, testSet.items = d.items, d.items
	trainSet.userFeedback, testSet.userFeedback = make([][]int32, d.CountUsers()), make([][]int32, d.CountUsers())
	trainSet.itemFeedback, testSet.itemFeedback = make([][]int32, d.CountItems()), make([][]int32, d.CountItems())
	trainSet.userDict, testSet.userDict = d.userDict, d.userDict
	trainSet.itemDict, testSet.itemDict = d.itemDict, d.itemDict
	rng := base.NewRandomGenerator(seed)
	if numTestUsers >= d.CountUsers() || numTestUsers <= 0 {
		for userIndex := int32(0); userIndex < int32(d.CountUsers()); userIndex++ {
			if len(d.userFeedback[userIndex]) > 0 {
				k := rng.Intn(len(d.userFeedback[userIndex]))
				testSet.userFeedback[userIndex] = append(testSet.userFeedback[userIndex], d.userFeedback[userIndex][k])
				testSet.itemFeedback[d.userFeedback[userIndex][k]] = append(testSet.itemFeedback[d.userFeedback[userIndex][k]], userIndex)
				testSet.numFeedback++
				for i, itemIndex := range d.userFeedback[userIndex] {
					if i != k {
						trainSet.userFeedback[userIndex] = append(trainSet.userFeedback[userIndex], itemIndex)
						trainSet.itemFeedback[itemIndex] = append(trainSet.itemFeedback[itemIndex], userIndex)
						trainSet.numFeedback++
					}
				}
			}
		}
	} else {
		testUsers := rng.SampleInt32(0, int32(d.CountUsers()), numTestUsers)
		for _, userIndex := range testUsers {
			if len(d.userFeedback[userIndex]) > 0 {
				k := rng.Intn(len(d.userFeedback[userIndex]))
				testSet.userFeedback[userIndex] = append(testSet.userFeedback[userIndex], d.userFeedback[userIndex][k])
				testSet.itemFeedback[d.userFeedback[userIndex][k]] = append(testSet.itemFeedback[d.userFeedback[userIndex][k]], userIndex)
				testSet.numFeedback++
				for i, itemIndex := range d.userFeedback[userIndex] {
					if i != k {
						trainSet.userFeedback[userIndex] = append(trainSet.userFeedback[userIndex], itemIndex)
						trainSet.itemFeedback[itemIndex] = append(trainSet.itemFeedback[itemIndex], userIndex)
						trainSet.numFeedback++
					}
				}
			}
		}
		testUserSet := mapset.NewSet(testUsers...)
		for userIndex := int32(0); userIndex < int32(d.CountUsers()); userIndex++ {
			if !testUserSet.Contains(userIndex) {
				for _, itemIndex := range d.userFeedback[userIndex] {
					trainSet.userFeedback[userIndex] = append(trainSet.userFeedback[userIndex], itemIndex)
					trainSet.itemFeedback[itemIndex] = append(trainSet.itemFeedback[itemIndex], userIndex)
					trainSet.numFeedback++
				}
			}
		}
	}
	return trainSet, testSet
}

type Labels struct {
	fields *strutil.Pool
	values *FreqDict
}

func NewLabels() *Labels {
	return &Labels{
		fields: strutil.NewPool(),
		values: NewFreqDict(),
	}
}

func (l *Labels) processLabels(labels any, parent string) any {
	switch typed := labels.(type) {
	case map[string]any:
		o := make(map[string]any)
		for k, v := range typed {
			o[l.fields.Align(k)] = l.processLabels(v, parent+"."+k)
		}
		return o
	case []any:
		if isSliceOf[float64](typed) {
			return lo.Map(typed, func(e any, _ int) float32 {
				return float32(e.(float64))
			})
		} else if isSliceOf[string](typed) {
			return lo.Map(typed, func(e any, _ int) ID {
				return ID(l.values.Add(parent + ":" + e.(string)))
			})
		}
		return typed
	case string:
		return ID(l.values.Add(parent + ":" + typed))
	default:
		return labels
	}
}

func isSliceOf[T any](v []any) bool {
	for _, e := range v {
		if _, ok := e.(T); !ok {
			return false
		}
	}
	return true
}

func LoadDataFromBuiltIn(dataSetName string) (*Dataset, *Dataset, error) {
	// Extract Data set information
	trainFilePath, testFilePath, err := model.LocateBuiltInDataset(dataSetName, model.FormatNCF)
	if err != nil {
		return nil, nil, err
	}
	// Load dataset
	train, err := loadTrain(trainFilePath)
	if err != nil {
		return nil, nil, err
	}
	test := NewDataset(train.GetTimestamp(), 0, 0)
	test.users, test.items = train.users, train.items
	test.userDict, test.itemDict = train.userDict, train.itemDict
	test.userFeedback = make([][]int32, len(train.userFeedback))
	test.itemFeedback = make([][]int32, len(train.itemFeedback))
	test.negatives = make([][]int32, len(train.userFeedback))
	err = loadTest(test, testFilePath)
	if err != nil {
		return nil, nil, err
	}
	return train, test, nil
}

func loadTrain(path string) (*Dataset, error) {
	dataset := NewDataset(time.Now(), 0, 0)
	// Open
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	// Read lines
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		// add users
		userId, err := util.ParseInt[int32](fields[0])
		if err != nil {
			return nil, err
		}
		for i := dataset.userDict.Count(); i <= userId; i++ {
			dataset.AddUser(data.User{UserId: util.FormatInt(i)})
		}
		// add items
		itemId, err := util.ParseInt[int32](fields[1])
		if err != nil {
			return nil, err
		}
		for i := dataset.itemDict.Count(); i <= itemId; i++ {
			dataset.AddItem(data.Item{ItemId: util.FormatInt(i)})
		}
		// add feedback
		dataset.AddFeedback(fields[0], fields[1])
	}
	return dataset, scanner.Err()
}

func loadTest(dataset *Dataset, path string) error {
	// Open
	file, err := os.Open(path)
	if err != nil {
		return errors.Trace(err)
	}
	defer file.Close()
	// Read lines
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// parse line
		fields := strings.Split(line, "\t")
		positive, negatives := fields[0], fields[1:]
		if positive[0] != '(' || positive[len(positive)-1] != ')' {
			return fmt.Errorf("wrong foramt: %v", line)
		}
		positive = positive[1 : len(positive)-1]
		fields = strings.Split(positive, ",")
		// add feedback
		dataset.AddFeedback(fields[0], fields[1])
		// add negatives
		userId, err := strconv.Atoi(fields[0])
		if err != nil {
			return err
		}
		dataset.negatives[userId] = make([]int32, len(negatives))
		for i, negative := range negatives {
			dataset.negatives[userId][i] = dataset.itemDict.Add(negative)
		}
	}
	return scanner.Err()
}
