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
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorse-io/gorse/storage"
	"github.com/juju/errors"
	"github.com/qdrant/go-client/qdrant"
)

const (
	qdrantPayloadCategoriesKey = "categories"
	qdrantPayloadIdKey         = "id"
	qdrantPayloadTimestampKey  = "timestamp"
)

func init() {
	Register([]string{storage.QdrantPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		database := new(Qdrant)
		u, err := url.Parse(path)
		if err != nil {
			return nil, errors.Trace(err)
		}
		host := u.Hostname()
		port := u.Port()
		portInt, err := strconv.Atoi(port)
		if err != nil {
			return nil, errors.Trace(err)
		}
		database.client, err = qdrant.NewClient(&qdrant.Config{
			Host: host,
			Port: portInt,
		})
		if err != nil {
			return nil, errors.Trace(err)
		}
		return database, nil
	})
}

type Qdrant struct {
	client *qdrant.Client
}

func (db *Qdrant) Init() error {
	return nil
}

func (db *Qdrant) Optimize() error {
	return nil
}

func (db *Qdrant) Close() error {
	return db.client.Close()
}

func (db *Qdrant) ListCollections(ctx context.Context) ([]string, error) {
	return db.client.ListCollections(ctx)
}

func (db *Qdrant) AddCollection(ctx context.Context, name string, dimensions int, distance Distance) error {
	var qdrantDistance qdrant.Distance
	switch distance {
	case Cosine:
		qdrantDistance = qdrant.Distance_Cosine
	case Euclidean:
		qdrantDistance = qdrant.Distance_Euclid
	case Dot:
		qdrantDistance = qdrant.Distance_Dot
	default:
		return errors.NotSupportedf("distance method")
	}
	err := db.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     uint64(dimensions),
			Distance: qdrantDistance,
		}),
	})
	if err != nil {
		return errors.Trace(err)
	}
	_, err = db.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
		CollectionName: name,
		Wait:           new(true),
		FieldName:      qdrantPayloadTimestampKey,
		FieldType:      qdrant.FieldType_FieldTypeInteger.Enum(),
	})
	return errors.Trace(err)
}

func (db *Qdrant) DeleteCollection(ctx context.Context, name string) error {
	return db.client.DeleteCollection(ctx, name)
}

func (db *Qdrant) AddVectors(ctx context.Context, collection string, vectors []Vector) error {
	if len(vectors) == 0 {
		return nil
	}
	points := make([]*qdrant.PointStruct, 0, len(vectors))
	for _, vector := range vectors {
		points = append(points, &qdrant.PointStruct{
			Id: qdrant.NewID(uuid.NewMD5(uuid.NameSpaceURL, []byte(vector.Id)).String()),
			Payload: map[string]*qdrant.Value{
				qdrantPayloadCategoriesKey: qdrantListValue(vector.Categories),
				qdrantPayloadIdKey:         qdrant.NewValueString(vector.Id),
				qdrantPayloadTimestampKey:  qdrant.NewValueInt(vector.Timestamp.UnixMilli()),
			},
			Vectors: qdrant.NewVectorsDense(vector.Vector),
		})
	}
	_, err := db.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Points:         points,
	})
	return errors.Trace(err)
}

func (db *Qdrant) DeleteVectors(ctx context.Context, collection string, timestamp time.Time) error {
	lt := float64(timestamp.UnixMilli())
	_, err := db.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collection,
		Points: qdrant.NewPointsSelectorFilter(&qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewRange(qdrantPayloadTimestampKey, &qdrant.Range{Lt: &lt}),
			},
		}),
	})
	return errors.Trace(err)
}

func (db *Qdrant) QueryVectors(ctx context.Context, collection string, q []float32, categories []string, topK int) ([]Vector, error) {
	if topK <= 0 {
		return []Vector{}, nil
	}
	request := &qdrant.QueryPoints{
		CollectionName: collection,
		Query:          qdrant.NewQueryDense(q),
		Limit:          new(uint64(topK)),
		WithPayload:    qdrant.NewWithPayloadEnable(true),
		WithVectors:    qdrant.NewWithVectorsEnable(true),
	}
	if len(categories) > 0 {
		request.Filter = &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatchKeywords(qdrantPayloadCategoriesKey, categories...),
			},
		}
	}
	response, err := db.client.Query(ctx, request)
	if err != nil {
		return nil, errors.Trace(err)
	}
	results := make([]Vector, 0, len(response))
	for _, scored := range response {
		results = append(results, Vector{
			Id:         qdrantId(scored.GetPayload()),
			Vector:     qdrantVectorOutput(scored.GetVectors()),
			Categories: qdrantCategories(scored.GetPayload()),
		})
	}
	return results, nil
}

func qdrantId(payload map[string]*qdrant.Value) string {
	if payload == nil {
		return ""
	}
	if value, ok := payload[qdrantPayloadIdKey]; ok {
		return value.GetStringValue()
	}
	return ""
}

func qdrantListValue(items []string) *qdrant.Value {
	values := make([]*qdrant.Value, 0, len(items))
	for _, item := range items {
		values = append(values, qdrant.NewValueString(item))
	}
	return qdrant.NewValueFromList(values...)
}

func qdrantCategories(payload map[string]*qdrant.Value) []string {
	if payload == nil {
		return []string{}
	}
	value, ok := payload[qdrantPayloadCategoriesKey]
	if !ok || value == nil {
		return []string{}
	}
	list := value.GetListValue()
	if list == nil {
		return []string{}
	}
	categories := make([]string, 0, len(list.GetValues()))
	for _, item := range list.GetValues() {
		if item == nil {
			continue
		}
		categories = append(categories, item.GetStringValue())
	}
	return categories
}

func qdrantVectorOutput(output *qdrant.VectorsOutput) []float32 {
	if output == nil {
		return nil
	}

	vector := output.GetVector()
	if vector == nil {
		return nil
	}
	return vector.GetDenseVector().GetData()
}
