package cache

import (
	"context"
	"encoding/base64"
	"encoding/hex"
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

func EnqueueItemEmbeddings(ctx context.Context, db Database, ids []string) error {
	for chunk := range slices.Chunk(ids, itemEmbeddingBatchSize) {
		now := time.Now()
		deliveries := make([]Score, 0, len(chunk))
		for _, id := range lo.Uniq(chunk) {
			key, err := uuid.NewRandom()
			if err != nil {
				return err
			}
			payload := base64.RawStdEncoding.EncodeToString([]byte(id))
			deliveryID := "d" + hex.EncodeToString(key[:])
			deliveries = append(deliveries, Score{Id: deliveryID, Categories: []string{payload}, Timestamp: now, Score: -float64(now.UnixMicro())})
		}
		if err := db.AddScores(ctx, itemEmbeddingQueue, "", deliveries); err != nil {
			return err
		}
	}
	return nil
}

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
		id, err := base64.RawStdEncoding.DecodeString(d.Categories[0])
		if err != nil {
			return fmt.Errorf("invalid item embedding delivery %q: %w", d.Id, err)
		}
		ids = append(ids, string(id))
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
