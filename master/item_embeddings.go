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

	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/logics"
	"github.com/gorse-io/gorse/model/ctr"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/gorse-io/gorse/storage/vectors"
	"go.uber.org/zap"
)

func (m *Master) runItemEmbeddingLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.processItemEmbeddings(ctx); err != nil && ctx.Err() == nil {
				log.Logger().Error("failed to refresh item embeddings", zap.Error(err))
			}
		}
	}
}

func (m *Master) processItemEmbeddings(ctx context.Context) error {
	if !m.itemEmbeddingMu.TryLock() {
		return nil
	}
	defer m.itemEmbeddingMu.Unlock()
	m.ConfigMutex.RLock()
	configs := m.Config.Recommend.ItemToItem
	ttl := m.Config.Recommend.DataSource.ItemTTL
	quant := m.Config.Database.Vector
	m.ConfigMutex.RUnlock()
	return cache.ProcessItemEmbeddings(ctx, m.CacheClient, 128, func(ids []string) error {
		var items map[string]data.Item
		now := time.Now()
		opts := data.GetOptions{}
		if ttl > 0 {
			opts.After = new(now.AddDate(0, 0, -int(ttl)))
		}
		for _, cfg := range configs {
			if cfg.Type != "embedding" {
				continue
			}
			if items == nil {
				rows, err := m.DataClient.BatchGetItems(ctx, ids, opts)
				if err != nil {
					return err
				}
				items = make(map[string]data.Item, len(rows))
				for _, item := range rows {
					item.Labels = ctr.ConvertEmbeddingLabels(item.Labels)
					items[item.ItemId] = item
				}
			}
			if err := logics.RefreshItemEmbeddings(ctx, m.VectorClient, cfg, vectors.VectorConfig{Type: vectors.QuantizationType(quant.QuantizationType), Bits: quant.QuantizationBits}, ids, items, now); err != nil {
				return err
			}
		}
		return nil
	})
}
