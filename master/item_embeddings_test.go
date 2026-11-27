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

package master

import (
	"context"
	"time"

	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/vectors"
)

type capturedItemStream struct {
	data.Database
	afterScan func()
}

func (d capturedItemStream) GetItemStream(ctx context.Context, n int, after *time.Time) (chan []data.Item, chan error) {
	items, errs := d.Database.GetItemStream(ctx, n, after)
	var batches [][]data.Item
	for batch := range items {
		batches = append(batches, batch)
	}
	out := make(chan []data.Item, len(batches))
	for _, batch := range batches {
		out <- batch
	}
	close(out)
	d.afterScan()
	return out, errs
}

func (s *MasterTestSuite) TestEmbeddingSnapshot() {
	ctx := s.T().Context()
	cfg := config.ItemToItemConfig{Name: "race", Type: "embedding", Column: "item.Labels.v"}
	s.Config.Recommend.ItemToItem = []config.ItemToItemConfig{cfg}
	s.Config.Recommend.NonPersonalized = nil
	put := func(x float32) {
		s.Require().NoError(s.DataClient.BatchInsertItems(ctx, []data.Item{{ItemId: "a", Labels: map[string]any{"v": []float32{x, 2}}}}))
	}
	put(1)
	s.DataClient = capturedItemStream{s.DataClient, func() {
		put(3)
		s.Require().NoError(cache.EnqueueItemEmbeddings(ctx, s.CacheClient, []string{"a"}))
		s.Require().NoError(s.processItemEmbeddings(ctx))
	}}
	_, err := s.loadAndIndexItems(ctx)
	s.Require().NoError(err)
	s.backgroundWaitGroup.Wait()
	s.Require().NoError(s.processItemEmbeddings(ctx))
	got, err := s.VectorClient.GetVectors(ctx, vectors.ItemToItemCollection(cfg.Name), []string{"a"})
	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.Equal([]float32{3, 2}, got[0].Float32Values())
}
