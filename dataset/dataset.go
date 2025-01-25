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
	"time"

	"github.com/chewxy/math32"
	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/storage/data"
	"modernc.org/strutil"
)

type ID int

type Dataset struct {
	timestamp    time.Time
	users        []data.User
	items        []data.Item
	userLabels   *Labels
	itemLabels   *Labels
	userFeedback [][]ID
	itemFeedback [][]ID
	userDict     *FreqDict
	itemDict     *FreqDict
}

func NewDataset(timestamp time.Time, userCount, itemCount int) *Dataset {
	return &Dataset{
		timestamp:    timestamp,
		users:        make([]data.User, 0, userCount),
		items:        make([]data.Item, 0, itemCount),
		userLabels:   NewLabels(),
		itemLabels:   NewLabels(),
		userFeedback: make([][]ID, userCount),
		itemFeedback: make([][]ID, itemCount),
		userDict:     NewFreqDict(),
		itemDict:     NewFreqDict(),
	}
}

func (d *Dataset) GetTimestamp() time.Time {
	return d.timestamp
}

func (d *Dataset) GetUsers() []data.User {
	return d.users
}

func (d *Dataset) GetItems() []data.Item {
	return d.items
}

func (d *Dataset) GetUserFeedback() [][]ID {
	return d.userFeedback
}

func (d *Dataset) GetItemFeedback() [][]ID {
	return d.itemFeedback
}

// GetUserIDF returns the IDF of users.
//
//	IDF(u) = log(I/freq(u))
//
// I is the number of items.
// freq(u) is the frequency of user u in all feedback.
func (d *Dataset) GetUserIDF() []float32 {
	idf := make([]float32, d.userDict.Count())
	for i := 0; i < d.userDict.Count(); i++ {
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
	for i := 0; i < d.itemDict.Count(); i++ {
		// Since zero IDF will cause NaN in the future, we set the minimum value to 1e-3.
		idf[i] = max(math32.Log(float32(len(d.users))/float32(d.itemDict.Freq(i))), 1e-3)
	}
	return idf
}

func (d *Dataset) GetUserColumnValuesIDF() []float32 {
	idf := make([]float32, d.userLabels.values.Count())
	for i := 0; i < d.userLabels.values.Count(); i++ {
		// Since zero IDF will cause NaN in the future, we set the minimum value to 1e-3.
		idf[i] = max(math32.Log(float32(len(d.users))/float32(d.userLabels.values.Freq(i))), 1e-3)
	}
	return idf
}

func (d *Dataset) GetItemColumnValuesIDF() []float32 {
	idf := make([]float32, d.itemLabels.values.Count())
	for i := 0; i < d.itemLabels.values.Count(); i++ {
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
	d.userDict.NotCount(user.UserId)
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
	d.itemDict.NotCount(item.ItemId)
	if len(d.itemFeedback) < len(d.items) {
		d.itemFeedback = append(d.itemFeedback, nil)
	}
}

func (d *Dataset) AddFeedback(userId, itemId string) {
	userIndex := d.userDict.Id(userId)
	itemIndex := d.itemDict.Id(itemId)
	d.userFeedback[userIndex] = append(d.userFeedback[userIndex], ID(itemIndex))
	d.itemFeedback[itemIndex] = append(d.itemFeedback[itemIndex], ID(userIndex))
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
				return ID(l.values.Id(parent + ":" + e.(string)))
			})
		}
		return typed
	case string:
		return ID(l.values.Id(parent + ":" + typed))
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
