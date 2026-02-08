// Copyright 2024 gorse Project Authors
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
	"fmt"
	"net/url"
	"strings"

	"github.com/gorse-io/gorse/storage"
	"github.com/juju/errors"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
	milvusIdField         = "id"
	milvusVectorField     = "vector"
	milvusCategoriesField = "categories"
)

func init() {
	Register([]string{storage.MilvusPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		database := new(Milvus)
		u, err := url.Parse(path)
		if err != nil {
			return nil, errors.Trace(err)
		}
		database.client, err = client.NewClient(context.Background(), client.Config{
			Address: u.Host,
		})
		if err != nil {
			return nil, errors.Trace(err)
		}
		return database, nil
	})
}

type Milvus struct {
	client client.Client
}

func (db *Milvus) Init() error {
	return nil
}

func (db *Milvus) Close() error {
	return db.client.Close()
}

func (db *Milvus) ListCollections(ctx context.Context) ([]string, error) {
	collections, err := db.client.ListCollections(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var names []string
	for _, collection := range collections {
		names = append(names, collection.Name)
	}
	return names, nil
}

func (db *Milvus) AddCollection(ctx context.Context, name string, dimensions int, distance Distance) error {
	schema := entity.NewSchema().WithName(name).WithDescription("gorse collection").
		WithField(entity.NewField().WithName(milvusIdField).WithDataType(entity.FieldTypeVarChar).WithMaxLength(65535).WithIsPrimaryKey(true)).
		WithField(entity.NewField().WithName(milvusCategoriesField).WithDataType(entity.FieldTypeArray).WithElementType(entity.FieldTypeVarChar).WithMaxCapacity(100).WithMaxLength(65535)).
		WithField(entity.NewField().WithName(milvusVectorField).WithDataType(entity.FieldTypeFloatVector).WithDim(int64(dimensions)))

	err := db.client.CreateCollection(ctx, schema, entity.DefaultShardNumber)
	if err != nil {
		return errors.Trace(err)
	}

	// Create index
	var metricType entity.MetricType
	switch distance {
	case Cosine:
		metricType = entity.COSINE
	case Euclidean:
		metricType = entity.L2
	case Dot:
		metricType = entity.IP
	default:
		return errors.NotSupportedf("distance method")
	}
	idx, err := entity.NewIndexHNSW(metricType, 8, 200)
	if err != nil {
		return errors.Trace(err)
	}
	err = db.client.CreateIndex(ctx, name, milvusVectorField, idx, false)
	if err != nil {
		return errors.Trace(err)
	}

	// Load collection
	err = db.client.LoadCollection(ctx, name, false)
	return errors.Trace(err)
}

func (db *Milvus) DeleteCollection(ctx context.Context, name string) error {
	exists, err := db.client.HasCollection(ctx, name)
	if err != nil {
		return errors.Trace(err)
	}
	if !exists {
		return errors.NotFoundf("collection %s", name)
	}
	err = db.client.DropCollection(ctx, name)
	return errors.Trace(err)
}

func (db *Milvus) AddVectors(ctx context.Context, collection string, vectors []Vector) error {
	if len(vectors) == 0 {
		return nil
	}
	ids := make([]string, 0, len(vectors))
	categories := make([][]string, 0, len(vectors))
	data := make([][]float32, 0, len(vectors))
	for _, v := range vectors {
		ids = append(ids, v.Id)
		categories = append(categories, v.Categories)
		data = append(data, v.Vector)
	}

	idCol := entity.NewColumnVarChar(milvusIdField, ids)
	categoriesCol := entity.NewColumnVarCharArray(milvusCategoriesField, milvusStringsToBytes(categories))
	vectorCol := entity.NewColumnFloatVector(milvusVectorField, len(data[0]), data)

	_, err := db.client.Upsert(ctx, collection, "", idCol, categoriesCol, vectorCol)
	return errors.Trace(err)
}

func (db *Milvus) QueryVectors(ctx context.Context, collection string, q []float32, categories []string, topK int) ([]Vector, error) {
	if topK <= 0 {
		return []Vector{}, nil
	}

	var expr string
	if len(categories) > 0 {
		var conditions []string
		for _, category := range categories {
			conditions = append(conditions, fmt.Sprintf("array_contains(%s, '%s')", milvusCategoriesField, category))
		}
		expr = strings.Join(conditions, " or ")
	}

	searchParam, _ := entity.NewIndexHNSWSearchParam(64)
	results, err := db.client.Search(ctx, collection, []string{}, expr, []string{milvusIdField, milvusCategoriesField}, []entity.Vector{entity.FloatVector(q)}, milvusVectorField, entity.COSINE, topK, searchParam, client.WithSearchQueryConsistencyLevel(entity.ClStrong))
	if err != nil {
		return nil, errors.Trace(err)
	}

	var vectors []Vector
	for _, result := range results {
		var idCol *entity.ColumnVarChar
		if col := result.Fields.GetColumn(milvusIdField); col != nil {
			idCol = col.(*entity.ColumnVarChar)
		} else if result.IDs != nil {
			idCol = result.IDs.(*entity.ColumnVarChar)
		}

		var categoriesCol *entity.ColumnVarCharArray
		if col := result.Fields.GetColumn(milvusCategoriesField); col != nil {
			categoriesCol = col.(*entity.ColumnVarCharArray)
		}

		for i := 0; i < result.ResultCount; i++ {
			var id string
			if idCol != nil {
				id, err = idCol.ValueByIdx(i)
				if err != nil {
					return nil, errors.Trace(err)
				}
			}

			var cats []string
			if categoriesCol != nil {
				catsValue, err := categoriesCol.ValueByIdx(i)
				if err != nil {
					return nil, errors.Trace(err)
				}
				cats = milvusBytesToStrings(catsValue)
			}

			vectors = append(vectors, Vector{
				Id:         id,
				Categories: cats,
			})
		}
	}
	return vectors, nil
}

func milvusStringsToBytes(ss [][]string) [][][]byte {
	res := make([][][]byte, len(ss))
	for i, s1 := range ss {
		res[i] = make([][]byte, len(s1))
		for j, s2 := range s1 {
			res[i][j] = []byte(s2)
		}
	}
	return res
}

func milvusBytesToStrings(bs [][]byte) []string {
	res := make([]string, len(bs))
	for i, b := range bs {
		res[i] = string(b)
	}
	return res
}
