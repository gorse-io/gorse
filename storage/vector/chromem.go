package vector

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/philippgille/chromem-go"
)

type Chromem struct {
	db *chromem.DB
}

func (c *Chromem) Close() error {
	return nil
}

func (c *Chromem) Create(name string, dimension int) error {
	_, err := c.db.CreateCollection(name, nil, nil)
	return err
}

func (c *Chromem) Drop(name string) error {
	return c.db.DeleteCollection(name)
}

func (c *Chromem) List() ([]string, error) {
	collections := c.db.ListCollections()
	names := make([]string, 0, len(collections))
	for name := range collections {
		names = append(names, name)
	}
	return names, nil
}

func (c *Chromem) Add(name string, vectors []Vector) error {
	ctx := context.Background()
	collection, err := c.db.GetOrCreateCollection(name, nil, nil)
	if err != nil {
		return err
	}

	if len(vectors) == 0 {
		return nil
	}

	docs := make([]chromem.Document, 0, len(vectors))
	for _, v := range vectors {
		metadata := make(map[string]string, 3)
		if len(v.Categories) > 0 {
			metadata["categories"] = strings.Join(v.Categories, ",")
		}
		metadata["is_hidden"] = strconv.FormatBool(v.IsHidden)
		if !v.Timestamp.IsZero() {
			metadata["timestamp"] = v.Timestamp.Format(time.RFC3339Nano)
		}
		if len(metadata) == 0 {
			metadata = nil
		}

		docs = append(docs, chromem.Document{
			ID:        v.Id,
			Metadata:  metadata,
			Embedding: v.Value,
		})
	}

	return collection.AddDocuments(ctx, docs, runtime.NumCPU())
}

func (c *Chromem) Search(name string, query []float32, topK int, filter map[string]any) ([]Vector, []float32, error) {
	collection := c.db.GetCollection(name, nil)
	if collection == nil {
		return nil, nil, fmt.Errorf("collection %s not found", name)
	}

	ctx := context.Background()
	where := make(map[string]string, len(filter))
	for k, v := range filter {
		where[k] = fmt.Sprint(v)
	}
	if len(where) == 0 {
		where = nil
	}

	results, err := collection.QueryEmbedding(ctx, query, topK, where, nil)
	if err != nil {
		return nil, nil, err
	}

	vectors := make([]Vector, 0, len(results))
	cons := make([]float32, 0, len(results))
	for _, r := range results {
		vec := Vector{Id: r.ID, Value: r.Embedding}
		if r.Metadata != nil {
			if cats, ok := r.Metadata["categories"]; ok && cats != "" {
				vec.Categories = strings.Split(cats, ",")
			}
			if hidden, ok := r.Metadata["is_hidden"]; ok {
				if b, err := strconv.ParseBool(hidden); err == nil {
					vec.IsHidden = b
				}
			}
			if ts, ok := r.Metadata["timestamp"]; ok {
				if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
					vec.Timestamp = t
				}
			}
		}

		vectors = append(vectors, vec)
		cons = append(cons, r.Similarity)
	}

	return vectors, cons, nil
}
