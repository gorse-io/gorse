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
package worker

import (
	"github.com/sirupsen/logrus"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/storage"
)

const batchSize = 1024

type Collector struct {
	db          storage.Database
	collectSize int
	items       map[string]storage.Item
}

func NewCollector(db storage.Database, collectSize int) (*Collector, error) {
	collector := &Collector{
		db:          db,
		collectSize: collectSize,
	}
	// pull items
	if err := collector.pullItems(); err != nil {
		return nil, err
	}
	return collector, nil
}

func (c *Collector) pullItems() error {
	c.items = make(map[string]storage.Item)
	cursor := ""
	for {
		var err error
		var items []storage.Item
		cursor, items, err = c.db.GetItems("", cursor, batchSize)
		if err != nil {
			return err
		}
		for _, item := range items {
			c.items[item.ItemId] = item
		}
	}
}

func (c *Collector) pullUserFeedback(userId string) ([]string, error) {
	// Get feedback
	feedback, err := c.db.GetUserFeedback(userId)
	if err != nil {
		return nil, err
	}
	items := make([]string, len(feedback))
	for i, f := range feedback {
		items[i] = f.ItemId
	}
	return items, err
}

type CollectorFunc func(collected base.StringSet, exclude base.StringSet, _ []string) (base.StringSet, error)

func (c *Collector) newCollectFunc(name string) CollectorFunc {
	switch name {
	case "all":
		return c.CollectAll
	case "pop":
		return c.CollectPop
	case "latest":
		return c.CollectLatest
	case "label_pop":
		return c.CollectLabelPop
	case "label_latest":
		return c.CollectLabelLatest
	default:
		logrus.Error("no known collector %v", name)
		return c.CollectNothing
	}
}

func (c *Collector) CollectNothing(collected base.StringSet, exclude base.StringSet, _ []string) (base.StringSet, error) {
	return collected, nil
}

func (c *Collector) CollectPop(collected base.StringSet, exclude base.StringSet, _ []string) (base.StringSet, error) {
	items, err := c.db.GetPop("", c.collectSize, 0)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if !exclude.Contain(item.ItemId) {
			collected[item.ItemId] = item
		}
	}
	return collected, nil
}

func (c *Collector) CollectLatest(collected base.StringSet, exclude base.StringSet, _ []string) (base.StringSet, error) {
	items, err := c.db.GetLatest("", c.collectSize, 0)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if !exclude.Contain(item.ItemId) {
			collected[item.ItemId] = item
		}
	}
	return collected, nil
}

func (c *Collector) CollectLabelPop(collected base.StringSet, exclude base.StringSet, userFeedback []string) (base.StringSet, error) {
	labelSet := base.NewStringSet()
	for _, itemId := range userFeedback {
		item := c.items[itemId]
		for _, label := range item.Labels {
			labelSet.Add(label)
		}
	}
	// Get items
	for label := range labelSet {
		items, err := c.db.GetPop(label, c.collectSize, 0)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if !exclude.Contain(item.ItemId) {
				collected.Add(item.ItemId)
			}
		}
	}
	return collected, nil
}

func (c *Collector) CollectLabelLatest(collected base.StringSet, exclude base.StringSet, userFeedback []string) (base.StringSet, error) {
	// Get feedback
	labelSet := base.NewStringSet()
	for _, itemId := range userFeedback {
		item := c.items[itemId]
		for _, label := range item.Labels {
			labelSet[label] = nil
		}
	}
	// Get items
	for label := range labelSet {
		items, err := c.db.GetLatest(label, c.collectSize, 0)
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

func (c *Collector) CollectNeighbors(collected base.StringSet, exclude base.StringSet, userFeedback []string) (base.StringSet, error) {
	for _, itemId := range userFeedback {
		items, err := c.db.GetNeighbors(itemId, c.collectSize, 0)
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

func (c *Collector) CollectAll(collected base.StringSet, exclude base.StringSet, _ []string) (base.StringSet, error) {
	for _, item := range c.items {
		if _, excluded := exclude[item.ItemId]; !excluded {
			collected.Add(item.ItemId)
		}
	}
	return collected, nil
}

func GetExclude(db storage.Database, userId string) (base.StringSet, error) {
	exclude := base.NewStringSet()
	// Get ignore
	ignored, err := db.GetUserIgnore(userId)
	for _, itemId := range ignored {
		exclude.Add(itemId)
	}
	// Get feedback
	feedback, err := db.GetUserFeedback(userId)
	if err != nil {
		return nil, err
	}
	for _, f := range feedback {
		exclude.Add(f.ItemId)
	}
	return exclude, nil
}

func (c *Collector) Collect(userId string, collectors []string) (base.StringSet, error) {
	exclude, err := GetExclude(c.db, userId)
	if err != nil {
		return nil, err
	}
	userFeedback, err := c.pullUserFeedback(userId)
	if err != nil {
		return nil, err
	}
	collected := base.NewStringSet()
	for _, name := range collectors {
		collector := c.newCollectFunc(name)
		collected, err = collector(collected, exclude, userFeedback)
		if err != nil {
			return nil, err
		}
	}
	if collected.Len() == 0 {
		collected, err = c.CollectAll(collected, exclude, userFeedback)
		if err != nil {
			return nil, err
		}
	}
	return collected, nil
}
