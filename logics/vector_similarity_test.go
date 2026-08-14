// Copyright 2026 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logics

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/dataset"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/vectors"
	"github.com/stretchr/testify/require"
)

type trackingVectorDatabase struct {
	vectors.Database
	mu      sync.Mutex
	queries []string
	written map[string][]vectors.Vector
}

func (d *trackingVectorDatabase) AddVectors(ctx context.Context, collection string, values []vectors.Vector) error {
	d.mu.Lock()
	d.written[collection] = append(d.written[collection], values...)
	d.mu.Unlock()
	return d.Database.AddVectors(ctx, collection, values)
}

func (d *trackingVectorDatabase) QueryVectors(ctx context.Context, collection string, query vectors.Vector, categories []string, topK int) ([]vectors.ScoredVector, error) {
	d.mu.Lock()
	d.queries = append(d.queries, collection)
	d.mu.Unlock()
	return d.Database.QueryVectors(ctx, collection, query, categories, topK)
}

func (d *trackingVectorDatabase) queryCount(collection string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	count := 0
	for _, name := range d.queries {
		if name == collection {
			count++
		}
	}
	return count
}

func openTrackingVectorDatabase(t *testing.T) *trackingVectorDatabase {
	t.Helper()
	database, err := vectors.Open(fmt.Sprintf("xvec://%s/vectors", t.TempDir()), "")
	require.NoError(t, err)
	require.NoError(t, database.Init())
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	return &trackingVectorDatabase{Database: database, written: make(map[string][]vectors.Vector)}
}

func TestItemToItemUsesVectorDatabase(t *testing.T) {
	for _, test := range []struct {
		name      string
		config    config.ItemToItemConfig
		tagsIDF   []float32
		usersIDF  []float32
		dimension int
		distance  vectors.Distance
		items     []data.Item
		feedback  [][]int32
		want      string
	}{
		{
			name: "embedding", config: config.ItemToItemConfig{Name: "embedding", Type: "embedding", Column: "item.Labels.embedding"},
			dimension: 2, distance: vectors.Euclidean,
			items: []data.Item{
				{ItemId: "query", Labels: map[string]any{"embedding": []float32{0, 0}}},
				{ItemId: "near", Labels: map[string]any{"embedding": []float32{0.1, 0}}},
				{ItemId: "far", Labels: map[string]any{"embedding": []float32{10, 0}}},
			}, want: "near",
		},
		{
			name: "tags", config: config.ItemToItemConfig{Name: "tags", Type: "tags", Column: "item.Labels"},
			tagsIDF: []float32{4, 1}, distance: vectors.Dot,
			items: []data.Item{
				{ItemId: "query", Labels: []dataset.ID{0, 1}},
				{ItemId: "high-idf", Labels: []dataset.ID{0}},
				{ItemId: "low-idf", Labels: []dataset.ID{1}},
			}, want: "high-idf",
		},
		{
			name: "users", config: config.ItemToItemConfig{Name: "users", Type: "users"},
			usersIDF: []float32{4, 1}, distance: vectors.Dot,
			items:    []data.Item{{ItemId: "query"}, {ItemId: "high-idf"}, {ItemId: "low-idf"}},
			feedback: [][]int32{{0, 1}, {0}, {1}}, want: "high-idf",
		},
		{
			name: "auto", config: config.ItemToItemConfig{Name: "auto", Type: "auto"},
			tagsIDF: []float32{4}, usersIDF: []float32{4}, distance: vectors.Dot,
			items: []data.Item{
				{ItemId: "query", Labels: []dataset.ID{0}},
				{ItemId: "same-tag", Labels: []dataset.ID{0}},
				{ItemId: "same-numeric-id-only"},
			}, feedback: [][]int32{nil, nil, {0}}, want: "same-tag",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			database := openTrackingVectorDatabase(t)
			recommender, err := NewItemToItem(test.config, 2, time.Now(), &ItemToItemOptions{
				Context: ctx, VectorClient: database, TagsIDF: test.tagsIDF, UsersIDF: test.usersIDF,
			})
			require.NoError(t, err)
			for i := range test.items {
				var feedback []int32
				if i < len(test.feedback) {
					feedback = test.feedback[i]
				}
				recommender.Push(&test.items[i], feedback)
			}
			require.NoError(t, recommender.Finish())

			collection := vectors.ItemToItemCollection(test.config.Name)
			info, err := database.DescribeCollection(ctx, collection)
			require.NoError(t, err)
			require.Equal(t, test.dimension, info.Dimension)
			require.Equal(t, test.distance, info.Distance)
			require.Len(t, database.written[collection], 3)

			scores := recommender.PopAll(0)
			require.NotEmpty(t, scores)
			require.Equal(t, test.want, scores[0].Id)
			require.Equal(t, 1, database.queryCount(collection))
			for _, score := range scores {
				require.NotEqual(t, "query", score.Id)
			}

			if test.name == "tags" || test.name == "users" {
				values := database.written[collection][0].Values
				require.InDelta(t, 2, values[0], 1e-6)
				require.InDelta(t, 1, values[1], 1e-6)
			}
			if test.name == "auto" {
				written := database.written[collection]
				require.Equal(t, []uint32{0}, written[0].Indices)
				require.Equal(t, []uint32{1}, written[2].Indices)
			}
		})
	}
}

func TestItemToItemVectorDatabasePreservesHiddenItemsAndCategories(t *testing.T) {
	ctx := t.Context()
	database := openTrackingVectorDatabase(t)
	recommender, err := NewItemToItem(config.ItemToItemConfig{
		Name: "hidden", Type: "tags", Column: "item.Labels",
	}, 5, time.Now(), &ItemToItemOptions{Context: ctx, VectorClient: database, TagsIDF: []float32{1}})
	require.NoError(t, err)
	recommender.Push(&data.Item{ItemId: "visible", Labels: []dataset.ID{0}, Categories: []string{"movie"}}, nil)
	recommender.Push(&data.Item{ItemId: "hidden", Labels: []dataset.ID{0}, IsHidden: true}, nil)
	require.NoError(t, recommender.Finish())

	visible := recommender.PopAll(0)
	require.Empty(t, visible)
	hidden := recommender.PopAll(1)
	require.Len(t, hidden, 1)
	require.Equal(t, "visible", hidden[0].Id)
	require.Equal(t, []string{"movie"}, hidden[0].Categories)
}

func TestUserToUserUsesVectorDatabase(t *testing.T) {
	for _, test := range []struct {
		name      string
		config    config.UserToUserConfig
		tagsIDF   []float32
		itemsIDF  []float32
		dimension int
		distance  vectors.Distance
		users     []data.User
		feedback  [][]int32
		want      string
	}{
		{
			name: "embedding", config: config.UserToUserConfig{Name: "embedding", Type: "embedding", Column: "user.Labels.embedding"},
			dimension: 2, distance: vectors.Euclidean,
			users: []data.User{
				{UserId: "query", Labels: map[string]any{"embedding": []float32{0, 0}}},
				{UserId: "near", Labels: map[string]any{"embedding": []float32{0.1, 0}}},
				{UserId: "far", Labels: map[string]any{"embedding": []float32{10, 0}}},
			}, want: "near",
		},
		{
			name: "tags", config: config.UserToUserConfig{Name: "tags", Type: "tags", Column: "user.Labels"},
			tagsIDF: []float32{4, 1}, distance: vectors.Dot,
			users: []data.User{
				{UserId: "query", Labels: []dataset.ID{0, 1}},
				{UserId: "high-idf", Labels: []dataset.ID{0}},
				{UserId: "low-idf", Labels: []dataset.ID{1}},
			}, want: "high-idf",
		},
		{
			name: "items", config: config.UserToUserConfig{Name: "items", Type: "items"},
			itemsIDF: []float32{4, 1}, distance: vectors.Dot,
			users:    []data.User{{UserId: "query"}, {UserId: "high-idf"}, {UserId: "low-idf"}},
			feedback: [][]int32{{0, 1}, {0}, {1}}, want: "high-idf",
		},
		{
			name: "auto", config: config.UserToUserConfig{Name: "auto", Type: "auto"},
			tagsIDF: []float32{4}, itemsIDF: []float32{4}, distance: vectors.Dot,
			users: []data.User{
				{UserId: "query", Labels: []dataset.ID{0}},
				{UserId: "same-tag", Labels: []dataset.ID{0}},
				{UserId: "same-numeric-id-only"},
			}, feedback: [][]int32{nil, nil, {0}}, want: "same-tag",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			database := openTrackingVectorDatabase(t)
			recommender, err := NewUserToUser(test.config, 2, time.Now(), &UserToUserOptions{
				Context: ctx, VectorClient: database, TagsIDF: test.tagsIDF, ItemsIDF: test.itemsIDF,
			})
			require.NoError(t, err)
			for i := range test.users {
				var feedback []int32
				if i < len(test.feedback) {
					feedback = test.feedback[i]
				}
				recommender.Push(&test.users[i], feedback)
			}
			require.NoError(t, recommender.Finish())

			collection := vectors.UserToUserCollection(test.config.Name)
			info, err := database.DescribeCollection(ctx, collection)
			require.NoError(t, err)
			require.Equal(t, test.dimension, info.Dimension)
			require.Equal(t, test.distance, info.Distance)
			require.Len(t, database.written[collection], 3)

			scores := recommender.PopAll(0)
			require.NotEmpty(t, scores)
			require.Equal(t, test.want, scores[0].Id)
			require.Equal(t, 1, database.queryCount(collection))
			for _, score := range scores {
				require.NotEqual(t, "query", score.Id)
			}

			if test.name == "tags" || test.name == "items" {
				values := database.written[collection][0].Values
				require.InDelta(t, 2, values[0], 1e-6)
				require.InDelta(t, 1, values[1], 1e-6)
			}
			if test.name == "auto" {
				written := database.written[collection]
				require.Equal(t, []uint32{0}, written[0].Indices)
				require.Equal(t, []uint32{1}, written[2].Indices)
			}
		})
	}
}
