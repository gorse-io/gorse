package cache

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

const (
	itemEmbeddingQueue     = "item_embedding_queue"
	itemEmbeddingBatchSize = 128
)

// EnqueueItemEmbeddings records refresh requests independently of earlier writes
// for the same item. Each score ID identifies a delivery, not the item.
func EnqueueItemEmbeddings(ctx context.Context, db Database, ids []string) error {
	for chunk := range slices.Chunk(ids, itemEmbeddingBatchSize) {
		now := time.Now()
		deliveries := make([]Score, 0, len(chunk))
		for _, id := range lo.Uniq(chunk) {
			key, err := uuid.NewRandom()
			if err != nil {
				return err
			}
			deliveries = append(deliveries, Score{Id: key.String(), Categories: []string{id}, Timestamp: now, Score: -float64(now.UnixMicro())})
		}
		if err := db.AddScores(ctx, itemEmbeddingQueue, "", deliveries); err != nil {
			return err
		}
	}
	return nil
}

// ProcessItemEmbeddings processes at most limit deliveries (capped at 128).
// Use a single consumer and an idempotent apply: failed or unacknowledged
// deliveries may be retried. An empty queue does not invoke apply.
func ProcessItemEmbeddings(ctx context.Context, db Database, limit int, apply func([]string) error) error {
	if limit <= 0 || apply == nil {
		return fmt.Errorf("item embedding queue requires a positive limit and an apply callback")
	}
	deliveries, err := db.SearchScores(ctx, itemEmbeddingQueue, "", nil, 0, min(limit, itemEmbeddingBatchSize))
	if err != nil || len(deliveries) == 0 {
		return err
	}
	ids := make([]string, 0, len(deliveries))
	for _, d := range deliveries {
		if len(d.Categories) != 1 {
			return fmt.Errorf("invalid item embedding delivery %q", d.Id)
		}
		ids = append(ids, d.Categories[0])
	}
	if err := apply(lo.Uniq(ids)); err != nil {
		return err
	}
	for _, d := range deliveries {
		if err := db.DeleteScores(ctx, []string{itemEmbeddingQueue}, ScoreCondition{Id: &d.Id}); err != nil {
			return err
		}
	}
	return nil
}
