// Copyright 2020 Zhenghao Zhang
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
package engine

import (
	"github.com/zhenghaoz/gorse/database"
)

type Collector func(db *database.Database, collected map[string]database.RecommendedItem, exclude map[string]interface{}, userId string, collectSize int) (map[string]database.RecommendedItem, error)

func CollectPop(db *database.Database, collected map[string]database.RecommendedItem, exclude map[string]interface{}, userId string, collectSize int) (map[string]database.RecommendedItem, error) {
	items, err := db.GetPop("", collectSize, 0)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if _, excluded := exclude[item.ItemId]; !excluded {
			collected[item.ItemId] = item
		}
	}
	return collected, nil
}

func CollectLastest(db *database.Database, collected map[string]database.RecommendedItem, exclude map[string]interface{}, userId string, collectSize int) (map[string]database.RecommendedItem, error) {
	items, err := db.GetLatest("", collectSize, 0)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if _, excluded := exclude[item.ItemId]; !excluded {
			collected[item.ItemId] = item
		}
	}
	return collected, nil
}

func CollectLabelPop(db *database.Database, collected map[string]database.RecommendedItem, exclude map[string]interface{}, userId string, collectSize int) (map[string]database.RecommendedItem, error) {
	// Get feedback
	feedback, err := db.GetFeedbackByUser(userId)
	if err != nil {
		return nil, err
	}
	labelSet := make(map[string]interface{})
	for _, f := range feedback {
		item, err := db.GetItem(f.ItemId)
		if err != nil {
			return nil, err
		}
		for _, label := range item.Labels {
			labelSet[label] = nil
		}
	}
	// Get items
	for label := range labelSet {
		items, err := db.GetPop(label, collectSize, 0)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if _, excluded := exclude[item.ItemId]; !excluded {
				collected[item.ItemId] = item
			}
		}
	}
	return collected, nil
}

func CollectLabelLastest(db *database.Database, collected map[string]database.RecommendedItem, exclude map[string]interface{}, userId string, collectSize int) (map[string]database.RecommendedItem, error) {
	// Get feedback
	feedback, err := db.GetFeedbackByUser(userId)
	if err != nil {
		return nil, err
	}
	labelSet := make(map[string]interface{})
	for _, f := range feedback {
		item, err := db.GetItem(f.ItemId)
		if err != nil {
			return nil, err
		}
		for _, label := range item.Labels {
			labelSet[label] = nil
		}
	}
	// Get items
	for label := range labelSet {
		items, err := db.GetLatest(label, collectSize, 0)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if _, excluded := exclude[item.ItemId]; !excluded {
				collected[item.ItemId] = item
			}
		}
	}
	return collected, nil
}

func CollectNeighbors(db *database.Database, collected map[string]database.RecommendedItem, exclude map[string]interface{}, userId string, collectSize int) (map[string]database.RecommendedItem, error) {
	feedback, err := db.GetFeedbackByUser(userId)
	if err != nil {
		return nil, err
	}
	for _, f := range feedback {
		items, err := db.GetNeighbors(f.ItemId, collectSize, 0)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if _, excluded := exclude[item.ItemId]; !excluded {
				collected[item.ItemId] = item
			}
		}
	}
	return collected, nil
}

func CollectAll(db *database.Database, collected map[string]database.RecommendedItem, exclude map[string]interface{}, userId string, collectSize int) (map[string]database.RecommendedItem, error) {
	items, err := db.GetItems(0, 0)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if _, excluded := exclude[item.ItemId]; !excluded {
			collected[item.ItemId] = database.RecommendedItem{Item: item}
		}
	}
	return collected, nil
}

func CollectExclude(db *database.Database, userId string) (map[string]interface{}, error) {
	collected := make(map[string]interface{})
	// Get ignore
	ignored, err := db.GetIgnore(userId)
	for _, itemId := range ignored {
		collected[itemId] = nil
	}
	// Get feedback
	feedback, err := db.GetFeedbackByUser(userId)
	if err != nil {
		return nil, err
	}
	for _, f := range feedback {
		collected[f.ItemId] = nil
	}
	return collected, nil
}

func Collect(db *database.Database, userId string, collectSize int, collectors ...Collector) (map[string]database.RecommendedItem, error) {
	exclude, err := CollectExclude(db, userId)
	if err != nil {
		return nil, err
	}
	collected := make(map[string]database.RecommendedItem)
	for _, collector := range collectors {
		collected, err = collector(db, collected, exclude, userId, 0)
		if err != nil {
			return nil, err
		}
	}
	return collected, nil
}
