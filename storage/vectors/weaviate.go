// Copyright 2026 gorse Project Authors
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

package vectors

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/gorse-io/gorse/storage"
	"github.com/juju/errors"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/fault"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

const (
	weaviatePayloadCategoriesKey = "categories"
	weaviatePayloadHiddenKey     = "hidden"
	weaviatePayloadTimestampKey  = "timestamp"
)

func init() {
	Register([]string{storage.WeaviatePrefix, storage.WeaviatesPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		database := new(Weaviate)
		u, err := url.Parse(path)
		if err != nil {
			return nil, errors.Trace(err)
		}
		scheme := "http"
		if strings.HasPrefix(path, storage.WeaviatesPrefix) {
			scheme = "https"
		}
		cfg := weaviate.Config{
			Host:   u.Host,
			Scheme: scheme,
		}
		database.client, err = weaviate.NewClient(cfg)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return database, nil
	})
}

type Weaviate struct {
	client *weaviate.Client
}

func (db *Weaviate) Init() error {
	return nil
}

func (db *Weaviate) Optimize(_ context.Context, _ string) error {
	return nil
}

func (db *Weaviate) Close() error {
	return nil
}

func (db *Weaviate) ListCollections(ctx context.Context) ([]string, error) {
	s, err := db.client.Schema().Getter().Do(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var names []string
	for _, class := range s.Classes {
		names = append(names, uncapitalize(class.Class))
	}
	return names, nil
}

func (db *Weaviate) DescribeCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	class, err := db.client.Schema().ClassGetter().WithClassName(capitalize(name)).Do(ctx)
	if err != nil {
		var clientErr *fault.WeaviateClientError
		if errors.As(err, &clientErr) && clientErr.StatusCode == http.StatusNotFound {
			return nil, errors.NotFoundf("collection %s", name)
		}
		return nil, errors.Trace(err)
	}
	vectorIndexConfig, ok := class.VectorIndexConfig.(map[string]any)
	if !ok {
		return nil, errors.Errorf("failed to parse vector index config for collection %s", name)
	}
	var distance Distance
	switch distanceValue := vectorIndexConfig["distance"].(string); distanceValue {
	case "", "cosine":
		distance = Cosine
	case "l2-squared":
		distance = Euclidean
	case "dot":
		distance = Dot
	default:
		return nil, errors.NotSupportedf("distance method %s", distanceValue)
	}
	config, err := weaviateVectorConfig(vectorIndexConfig)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &CollectionInfo{
		Name:         name,
		Distance:     distance,
		VectorConfig: config,
	}, nil
}

func (db *Weaviate) AddCollection(ctx context.Context, name string, dimensions int, distance Distance, config VectorConfig) error {
	var weaviateDistance string
	switch distance {
	case Cosine:
		weaviateDistance = "cosine"
	case Euclidean:
		weaviateDistance = "l2-squared"
	case Dot:
		weaviateDistance = "dot"
	default:
		return errors.NotSupportedf("distance method")
	}

	// Build VectorIndexConfig.
	vectorIndexConfig := map[string]any{
		"distance": weaviateDistance,
	}
	if err := weaviateApplyQuantization(vectorIndexConfig, config); err != nil {
		return errors.Trace(err)
	}

	class := &models.Class{
		Class:      capitalize(name),
		Vectorizer: "none",
		Properties: []*models.Property{
			{
				Name:     "originalId",
				DataType: []string{"string"},
			},
			{
				Name:     weaviatePayloadCategoriesKey,
				DataType: []string{"string[]"},
			},
			{
				Name:            weaviatePayloadHiddenKey,
				DataType:        []string{"boolean"},
				IndexFilterable: new(true),
			},
			{
				Name:              weaviatePayloadTimestampKey,
				DataType:          []string{"date"},
				IndexFilterable:   new(true),
				IndexRangeFilters: new(true),
			},
		},
		VectorIndexConfig: vectorIndexConfig,
	}
	err := db.client.Schema().ClassCreator().WithClass(class).Do(ctx)
	return errors.Trace(err)
}

func weaviateApplyQuantization(vectorIndexConfig map[string]any, config VectorConfig) error {
	switch config.Type {
	case QuantizationNone:
		return nil
	case QuantizationSQ:
		vectorIndexConfig["sq"] = map[string]any{
			"enabled": true,
		}
		if config.Bits != 0 {
			return errors.NotSupportedf("quantization bits for SQ")
		}
		return nil
	case QuantizationPQ:
		vectorIndexConfig["pq"] = map[string]any{
			"enabled": true,
		}
		if config.Bits != 0 {
			return errors.NotSupportedf("quantization bits for PQ")
		}
		return nil
	case QuantizationRQ:
		rq := map[string]any{
			"enabled": true,
		}
		if config.Bits != 0 {
			rq["bits"] = config.Bits
		}
		vectorIndexConfig["rq"] = rq
		return nil
	default:
		return errors.NotSupportedf("quantization type %s for Weaviate", config.Type)
	}
}

func weaviateVectorConfig(vectorIndexConfig map[string]any) (VectorConfig, error) {
	if quantizationConfig, ok := vectorIndexConfig["rq"].(map[string]any); ok && quantizationConfig["enabled"].(bool) {
		return VectorConfig{
			Type: QuantizationRQ,
			Bits: int(quantizationConfig["bits"].(float64)),
		}, nil
	}
	if quantizationConfig, ok := vectorIndexConfig["pq"].(map[string]any); ok && quantizationConfig["enabled"].(bool) {
		return VectorConfig{Type: QuantizationPQ}, nil
	}
	if quantizationConfig, ok := vectorIndexConfig["sq"].(map[string]any); ok && quantizationConfig["enabled"].(bool) {
		return VectorConfig{Type: QuantizationSQ}, nil
	}
	return VectorConfig{}, nil
}

func (db *Weaviate) DeleteCollection(ctx context.Context, name string) error {
	exists, err := db.client.Schema().ClassExistenceChecker().WithClassName(capitalize(name)).Do(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	if !exists {
		return errors.NotFoundf("collection %s", name)
	}
	err = db.client.Schema().ClassDeleter().WithClassName(capitalize(name)).Do(ctx)
	return errors.Trace(err)
}

func (db *Weaviate) CountVectors(ctx context.Context, collection string) (int64, error) {
	result, err := db.client.GraphQL().Aggregate().
		WithClassName(capitalize(collection)).
		WithFields(graphql.Field{
			Name:   "meta",
			Fields: []graphql.Field{{Name: "count"}},
		}).
		Do(ctx)
	if err != nil {
		return 0, errors.Trace(err)
	}
	if len(result.Errors) > 0 {
		return 0, errors.New(result.Errors[0].Message)
	}
	aggregate, ok := result.Data["Aggregate"].(map[string]any)
	if !ok {
		return 0, errors.Errorf("failed to parse aggregate response for collection %s", collection)
	}
	groups, ok := aggregate[capitalize(collection)].([]any)
	if !ok || len(groups) == 0 {
		return 0, errors.Errorf("failed to parse aggregate response for collection %s", collection)
	}
	group, ok := groups[0].(map[string]any)
	if !ok {
		return 0, errors.Errorf("failed to parse aggregate response for collection %s", collection)
	}
	meta, ok := group["meta"].(map[string]any)
	if !ok {
		return 0, errors.Errorf("failed to parse aggregate response for collection %s", collection)
	}
	count, ok := meta["count"].(float64)
	if !ok {
		return 0, errors.Errorf("failed to parse aggregate response for collection %s", collection)
	}
	return int64(count), nil
}

func (db *Weaviate) AddVectors(ctx context.Context, collection string, vectors []Vector) error {
	if len(vectors) == 0 {
		return nil
	}
	objects := make([]*models.Object, 0, len(vectors))
	for _, vector := range vectors {
		objects = append(objects, &models.Object{
			Class: capitalize(collection),
			ID:    strfmt.UUID(uuid.NewMD5(uuid.NameSpaceURL, []byte(vector.Id)).String()),
			Properties: map[string]any{
				"originalId":                 vector.Id,
				weaviatePayloadCategoriesKey: vector.Categories,
				weaviatePayloadHiddenKey:     vector.IsHidden,
				weaviatePayloadTimestampKey:  vector.Timestamp,
			},
			Vector: models.C11yVector(vector.Values),
		})
	}
	_, err := db.client.Batch().ObjectsBatcher().WithObjects(objects...).Do(ctx)
	return errors.Trace(err)
}

func (db *Weaviate) GetVectors(ctx context.Context, collection string, ids []string) ([]Vector, error) {
	if len(ids) == 0 {
		return []Vector{}, nil
	}
	objectIDs := make([]string, len(ids))
	for i, id := range ids {
		objectIDs[i] = uuid.NewMD5(uuid.NameSpaceURL, []byte(id)).String()
	}
	fields := []graphql.Field{
		{Name: "originalId"},
		{Name: weaviatePayloadCategoriesKey},
		{Name: weaviatePayloadHiddenKey},
		{Name: weaviatePayloadTimestampKey},
		{Name: "_additional", Fields: []graphql.Field{{Name: "vector"}}},
	}
	result, err := db.client.GraphQL().Get().
		WithClassName(capitalize(collection)).
		WithFields(fields...).
		WithWhere(filters.Where().
			WithPath([]string{"id"}).
			WithOperator(filters.ContainsAny).
			WithValueString(objectIDs...)).
		WithLimit(len(ids)).
		Do(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(result.Errors) > 0 {
		return nil, errors.New(result.Errors[0].Message)
	}
	data, ok := result.Data["Get"].(map[string]any)
	if !ok {
		return nil, errors.Errorf("failed to parse vectors for collection %s", collection)
	}
	items, ok := data[capitalize(collection)].([]any)
	if !ok {
		return nil, errors.Errorf("failed to parse vectors for collection %s", collection)
	}
	vectors := make([]Vector, 0, len(items))
	for _, item := range items {
		properties, ok := item.(map[string]any)
		if !ok {
			return nil, errors.Errorf("failed to parse vector for collection %s", collection)
		}
		vector := Vector{
			Id:       properties["originalId"].(string),
			IsHidden: properties[weaviatePayloadHiddenKey].(bool),
		}
		if categories, ok := properties[weaviatePayloadCategoriesKey].([]any); ok {
			vector.Categories = make([]string, len(categories))
			for i, category := range categories {
				vector.Categories[i] = category.(string)
			}
		}
		if timestamp, ok := properties[weaviatePayloadTimestampKey].(string); ok {
			vector.Timestamp, err = time.Parse(time.RFC3339Nano, timestamp)
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
		additional, ok := properties["_additional"].(map[string]any)
		if !ok {
			return nil, errors.Errorf("failed to parse vector values for collection %s", collection)
		}
		values, ok := additional["vector"].([]any)
		if !ok {
			return nil, errors.Errorf("failed to parse vector values for collection %s", collection)
		}
		vector.Values = make([]float32, len(values))
		for i, value := range values {
			vector.Values[i] = float32(value.(float64))
		}
		vectors = append(vectors, vector)
	}
	return orderVectors(ids, vectors), nil
}

func (db *Weaviate) DeleteVectors(ctx context.Context, collection string, timestamp time.Time) error {
	_, err := db.client.Batch().ObjectsBatchDeleter().
		WithClassName(capitalize(collection)).
		WithWhere(filters.Where().
			WithPath([]string{weaviatePayloadTimestampKey}).
			WithOperator(filters.LessThan).
			WithValueDate(timestamp)).
		Do(ctx)
	return errors.Trace(err)
}

func (db *Weaviate) QueryVectors(ctx context.Context, collection string, q Vector, categories []string, topK int) ([]ScoredVector, error) {
	if topK <= 0 {
		return []ScoredVector{}, nil
	}

	fields := []graphql.Field{
		{Name: "originalId"},
		{Name: weaviatePayloadCategoriesKey},
		{Name: weaviatePayloadHiddenKey},
		{Name: "_additional", Fields: []graphql.Field{{Name: "distance"}}},
	}

	explore := db.client.GraphQL().NearVectorArgBuilder().WithVector(q.Values)
	builder := db.client.GraphQL().Get().
		WithClassName(capitalize(collection)).
		WithFields(fields...).
		WithNearVector(explore).
		WithLimit(topK)

	where := filters.Where().
		WithPath([]string{weaviatePayloadHiddenKey}).
		WithOperator(filters.Equal).
		WithValueBoolean(false)
	if len(categories) > 0 {
		categoriesWhere := filters.Where().
			WithPath([]string{weaviatePayloadCategoriesKey}).
			WithOperator(filters.ContainsAny).
			WithValueString(categories...)
		where = filters.Where().
			WithOperator(filters.And).
			WithOperands([]*filters.WhereBuilder{where, categoriesWhere})
	}
	builder = builder.WithWhere(where)

	result, err := builder.Do(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if len(result.Errors) > 0 {
		return nil, errors.New(result.Errors[0].Message)
	}

	data := result.Data["Get"].(map[string]any)
	items := data[capitalize(collection)].([]any)
	results := make([]ScoredVector, 0, len(items))
	for _, item := range items {
		m := item.(map[string]any)
		id := m["originalId"].(string)
		var cats []string
		if m[weaviatePayloadCategoriesKey] != nil {
			for _, c := range m[weaviatePayloadCategoriesKey].([]any) {
				cats = append(cats, c.(string))
			}
		}
		additional := m["_additional"].(map[string]any)
		distance := additional["distance"].(float64)
		results = append(results, ScoredVector{
			Vector: Vector{
				Id:         id,
				IsHidden:   m[weaviatePayloadHiddenKey].(bool),
				Categories: cats,
			},
			Score: -float32(distance),
		})
	}
	return results, nil
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func uncapitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
