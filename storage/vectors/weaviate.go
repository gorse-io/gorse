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
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/gorse-io/gorse/storage"
	"github.com/juju/errors"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

const (
	weaviatePayloadCategoriesKey = "categories"
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

func (db *Weaviate) Optimize() error {
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
		return nil, errors.Trace(err)
	}
	vectorIndexConfig, ok := class.VectorIndexConfig.(map[string]any)
	if !ok {
		return nil, errors.Errorf("failed to parse vector index config for collection %s", name)
	}
	distance, err := weaviateDistanceToDistance(stringValue(vectorIndexConfig["distance"]))
	if err != nil {
		return nil, errors.Trace(err)
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
		"distance":       weaviateDistance,
		"ef":             defaultHNSWEfSearch,
		"maxConnections": defaultHNSWM,
		"efConstruction": defaultHNSWEfConstruct,
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
	switch config.Quantization {
	case QuantizationNone, "":
		return nil
	case QuantizationSQ:
		return errors.NotSupportedf("SQ quantization for Weaviate")
	case QuantizationPQ:
		vectorIndexConfig["pq"] = map[string]any{
			"enabled": true,
		}
		return nil
	case QuantizationRQ:
		rq := map[string]any{
			"enabled": true,
		}
		if config.QuantizationBits != 0 {
			switch config.QuantizationBits {
			case 1, 8:
				rq["bits"] = config.QuantizationBits
			default:
				return errors.NotSupportedf("RQ quantization bits %d for Weaviate", config.QuantizationBits)
			}
		}
		vectorIndexConfig["rq"] = rq
		return nil
	default:
		return errors.NotSupportedf("quantization type %s for Weaviate", config.Quantization)
	}
}

func weaviateVectorConfig(vectorIndexConfig map[string]any) (VectorConfig, error) {
	if quantizationConfig, ok := mapValue(vectorIndexConfig["rq"]); ok && boolValue(quantizationConfig["enabled"]) {
		bits, err := intValue(quantizationConfig["bits"])
		if err != nil {
			return VectorConfig{}, errors.Trace(err)
		}
		return VectorConfig{
			Quantization:     QuantizationRQ,
			QuantizationBits: bits,
		}, nil
	}
	if quantizationConfig, ok := mapValue(vectorIndexConfig["pq"]); ok && boolValue(quantizationConfig["enabled"]) {
		return VectorConfig{Quantization: QuantizationPQ}, nil
	}
	if quantizationConfig, ok := mapValue(vectorIndexConfig["sq"]); ok && boolValue(quantizationConfig["enabled"]) {
		return VectorConfig{Quantization: QuantizationSQ}, nil
	}
	return DefaultVectorConfig(), nil
}

func weaviateDistanceToDistance(distance string) (Distance, error) {
	switch distance {
	case "", "cosine":
		return Cosine, nil
	case "l2-squared":
		return Euclidean, nil
	case "dot":
		return Dot, nil
	default:
		return Cosine, errors.NotSupportedf("distance method %s", distance)
	}
}

func mapValue(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	default:
		return nil, false
	}
}

func stringValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func boolValue(value any) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	return false
}

func intValue(value any) (int, error) {
	switch typed := value.(type) {
	case nil:
		return 0, nil
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case float64:
		return int(typed), nil
	case json.Number:
		i, err := typed.Int64()
		return int(i), err
	case string:
		return strconv.Atoi(typed)
	default:
		return 0, errors.NotSupportedf("integer value %T", value)
	}
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
				weaviatePayloadTimestampKey:  vector.Timestamp,
			},
			Vector: models.C11yVector(vector.Vector),
		})
	}
	_, err := db.client.Batch().ObjectsBatcher().WithObjects(objects...).Do(ctx)
	return errors.Trace(err)
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

func (db *Weaviate) QueryVectors(ctx context.Context, collection string, q []float32, categories []string, topK int) ([]Vector, error) {
	if topK <= 0 {
		return []Vector{}, nil
	}

	fields := []graphql.Field{
		{Name: "originalId"},
		{Name: weaviatePayloadCategoriesKey},
	}

	explore := db.client.GraphQL().NearVectorArgBuilder().WithVector(q)
	builder := db.client.GraphQL().Get().
		WithClassName(capitalize(collection)).
		WithFields(fields...).
		WithNearVector(explore).
		WithLimit(topK)

	if len(categories) > 0 {
		operands := make([]*filters.WhereBuilder, 0, len(categories))
		for _, category := range categories {
			operands = append(operands, filters.Where().
				WithPath([]string{weaviatePayloadCategoriesKey}).
				WithOperator(filters.ContainsAny).
				WithValueString(category))
		}
		var where *filters.WhereBuilder
		if len(operands) == 1 {
			where = operands[0]
		} else {
			where = filters.Where().
				WithOperator(filters.Or).
				WithOperands(operands)
		}
		builder = builder.WithWhere(where)
	}

	result, err := builder.Do(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if len(result.Errors) > 0 {
		return nil, errors.New(result.Errors[0].Message)
	}

	data := result.Data["Get"].(map[string]any)
	items := data[capitalize(collection)].([]any)
	results := make([]Vector, 0, len(items))
	for _, item := range items {
		m := item.(map[string]any)
		id := m["originalId"].(string)
		var cats []string
		if m[weaviatePayloadCategoriesKey] != nil {
			for _, c := range m[weaviatePayloadCategoriesKey].([]any) {
				cats = append(cats, c.(string))
			}
		}
		results = append(results, Vector{
			Id:         id,
			Categories: cats,
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
