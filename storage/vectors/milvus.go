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
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorse-io/gorse/storage"
	"github.com/juju/errors"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/milvus-io/milvus/pkg/v2/util/merr"
)

const (
	milvusIdField         = "id"
	milvusVectorField     = "vector"
	milvusCategoriesField = "categories"
	milvusTimestampField  = "timestamp"

	milvusIVFRQIndexType = index.IvfRabitQ

	defaultMilvusIVFNList  = 128
	defaultMilvusPQBits    = 8
	defaultMilvusRQNProbe  = 8
	defaultMilvusRQRefineK = 1
)

func init() {
	Register([]string{storage.MilvusPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		database := &Milvus{}
		u, err := url.Parse(path)
		if err != nil {
			return nil, errors.Trace(err)
		}
		database.client, err = milvusclient.New(context.Background(), &milvusclient.ClientConfig{
			Address: u.Host,
		})
		if err != nil {
			return nil, errors.Trace(err)
		}
		return database, nil
	})
}

type Milvus struct {
	client *milvusclient.Client
}

func (db *Milvus) Init() error {
	return nil
}

func (db *Milvus) Optimize() error {
	return nil
}

func (db *Milvus) Close() error {
	return db.client.Close(context.Background())
}

func (db *Milvus) ListCollections(ctx context.Context) ([]string, error) {
	collections, err := db.client.ListCollections(ctx, milvusclient.NewListCollectionOption())
	if err != nil {
		return nil, errors.Trace(err)
	}
	return collections, nil
}

func (db *Milvus) DescribeCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	collection, err := db.client.DescribeCollection(ctx, milvusclient.NewDescribeCollectionOption(name))
	if err != nil {
		if errors.Is(err, merr.ErrCollectionNotFound) {
			return nil, errors.NotFoundf("collection %s", name)
		}
		return nil, errors.Trace(err)
	}
	dimension, err := milvusVectorDimension(collection)
	if err != nil {
		return nil, errors.Trace(err)
	}
	idx, err := db.client.DescribeIndex(ctx, milvusclient.NewDescribeIndexOption(name, milvusVectorField))
	if err != nil {
		return nil, errors.Trace(err)
	}
	var distance Distance
	switch metricType := entity.MetricType(idx.Params()[index.MetricTypeKey]); metricType {
	case "", entity.COSINE:
		distance = Cosine
	case entity.L2:
		distance = Euclidean
	case entity.IP:
		distance = Dot
	default:
		return nil, errors.NotSupportedf("distance method %s", metricType)
	}
	config := VectorConfig{}
	switch index.IndexType(idx.Params()[index.IndexTypeKey]) {
	case milvusIVFRQIndexType:
		config.Type = QuantizationRQ
	case index.IvfSQ8:
		config.Type = QuantizationSQ
		config.Bits = 8
	case index.IvfPQ:
		config.Type = QuantizationPQ
		m, err := strconv.Atoi(idx.Params()["m"])
		if err != nil {
			return nil, errors.Trace(err)
		}
		nbits, err := strconv.Atoi(idx.Params()["nbits"])
		if err != nil {
			return nil, errors.Trace(err)
		}
		if dimension > 0 {
			config.Bits = m * nbits / dimension
		}
	}
	return &CollectionInfo{
		Name:         name,
		Dimension:    dimension,
		Distance:     distance,
		VectorConfig: config,
	}, nil
}

func (db *Milvus) AddCollection(ctx context.Context, name string, dimensions int, distance Distance, config VectorConfig) error {
	schema := entity.NewSchema().WithName(name).WithDescription("gorse collection").
		WithField(entity.NewField().WithName(milvusIdField).WithDataType(entity.FieldTypeVarChar).WithMaxLength(65535).WithIsPrimaryKey(true)).
		WithField(entity.NewField().WithName(milvusCategoriesField).WithDataType(entity.FieldTypeArray).WithElementType(entity.FieldTypeVarChar).WithMaxCapacity(100).WithMaxLength(65535)).
		WithField(entity.NewField().WithName(milvusTimestampField).WithDataType(entity.FieldTypeInt64)).
		WithField(entity.NewField().WithName(milvusVectorField).WithDataType(entity.FieldTypeFloatVector).WithDim(int64(dimensions)))

	err := db.client.CreateCollection(ctx, milvusclient.NewCreateCollectionOption(name, schema).WithShardNum(entity.DefaultShardNumber))
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

	idx, err := milvusIndex(metricType, dimensions, config)
	if err != nil {
		return errors.Trace(err)
	}
	indexTask, err := db.client.CreateIndex(ctx, milvusclient.NewCreateIndexOption(name, milvusVectorField, idx).WithIndexName(milvusVectorField))
	if err != nil {
		return errors.Trace(err)
	}
	if err = indexTask.Await(ctx); err != nil {
		return errors.Trace(err)
	}

	indexTask, err = db.client.CreateIndex(ctx, milvusclient.NewCreateIndexOption(name, milvusTimestampField, index.NewSortedIndex()).WithIndexName(milvusTimestampField))
	if err != nil {
		return errors.Trace(err)
	}
	if err = indexTask.Await(ctx); err != nil {
		return errors.Trace(err)
	}

	// Load collection
	loadTask, err := db.client.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(name))
	if err != nil {
		return errors.Trace(err)
	}
	return errors.Trace(loadTask.Await(ctx))
}

func milvusVectorDimension(collection *entity.Collection) (int, error) {
	if collection == nil || collection.Schema == nil {
		return 0, errors.NotFoundf("collection schema")
	}
	for _, field := range collection.Schema.Fields {
		if field.Name == milvusVectorField {
			dimension, err := strconv.Atoi(field.TypeParams[entity.TypeParamDim])
			if err != nil {
				return 0, errors.Trace(err)
			}
			return dimension, nil
		}
	}
	return 0, errors.NotFoundf("vector field")
}

func (db *Milvus) DeleteCollection(ctx context.Context, name string) error {
	exists, err := db.client.HasCollection(ctx, milvusclient.NewHasCollectionOption(name))
	if err != nil {
		return errors.Trace(err)
	}
	if !exists {
		return errors.NotFoundf("collection %s", name)
	}
	err = db.client.DropCollection(ctx, milvusclient.NewDropCollectionOption(name))
	return errors.Trace(err)
}

func (db *Milvus) AddVectors(ctx context.Context, collection string, vectors []Vector) error {
	if len(vectors) == 0 {
		return nil
	}
	ids := make([]string, 0, len(vectors))
	categories := make([][]string, 0, len(vectors))
	timestamps := make([]int64, 0, len(vectors))
	data := make([][]float32, 0, len(vectors))
	for _, v := range vectors {
		ids = append(ids, v.Id)
		categories = append(categories, v.Categories)
		timestamps = append(timestamps, v.Timestamp.UnixMilli())
		data = append(data, v.Vector)
	}

	idCol := column.NewColumnVarChar(milvusIdField, ids)
	categoriesCol := column.NewColumnVarCharArray(milvusCategoriesField, categories)
	timestampCol := column.NewColumnInt64(milvusTimestampField, timestamps)
	vectorCol := column.NewColumnFloatVector(milvusVectorField, len(data[0]), data)

	_, err := db.client.Upsert(ctx, milvusclient.NewColumnBasedInsertOption(collection, idCol, categoriesCol, timestampCol, vectorCol))
	return errors.Trace(err)
}

func (db *Milvus) DeleteVectors(ctx context.Context, collection string, timestamp time.Time) error {
	_, err := db.client.Delete(ctx, milvusclient.NewDeleteOption(collection).WithExpr(fmt.Sprintf("%s < %d", milvusTimestampField, timestamp.UnixMilli())))
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

	searchParam, err := db.searchParam(ctx, collection)
	if err != nil {
		return nil, errors.Trace(err)
	}
	results, err := db.client.Search(ctx, milvusclient.NewSearchOption(collection, topK, []entity.Vector{entity.FloatVector(q)}).
		WithANNSField(milvusVectorField).
		WithFilter(expr).
		WithOutputFields(milvusIdField, milvusCategoriesField).
		WithAnnParam(searchParam).
		WithConsistencyLevel(entity.ClStrong))
	if err != nil {
		return nil, errors.Trace(err)
	}

	var vectors []Vector
	for _, result := range results {
		if result.Err != nil {
			return nil, errors.Trace(result.Err)
		}

		var idCol *column.ColumnVarChar
		if col := result.GetColumn(milvusIdField); col != nil {
			idCol = col.(*column.ColumnVarChar)
		} else if result.IDs != nil {
			idCol = result.IDs.(*column.ColumnVarChar)
		}

		var categoriesCol *column.ColumnVarCharArray
		if col := result.GetColumn(milvusCategoriesField); col != nil {
			categoriesCol = col.(*column.ColumnVarCharArray)
		}

		for i := 0; i < result.ResultCount; i++ {
			var id string
			if idCol != nil {
				id, err = idCol.Value(i)
				if err != nil {
					return nil, errors.Trace(err)
				}
			}

			var cats []string
			if categoriesCol != nil {
				cats, err = categoriesCol.Value(i)
				if err != nil {
					return nil, errors.Trace(err)
				}
			}

			vectors = append(vectors, Vector{
				Id:         id,
				Categories: cats,
			})
		}
	}
	return vectors, nil
}

func milvusIndex(metricType entity.MetricType, dimensions int, config VectorConfig) (index.Index, error) {
	switch config.Type {
	case QuantizationNone:
		return index.NewHNSWIndex(metricType, 16, 200), nil
	case QuantizationRQ:
		if config.Bits != 0 {
			return nil, errors.NotSupportedf("RQ quantization bits %d for Milvus", config.Bits)
		}
		return index.NewIvfRabitQIndex(metricType, defaultMilvusIVFNList), nil
	case QuantizationPQ:
		bits := config.Bits
		if bits == 0 {
			bits = defaultMilvusPQBits
		}
		if bits <= 0 || dimensions <= 0 || dimensions*bits%defaultMilvusPQBits != 0 {
			return nil, errors.NotSupportedf("PQ quantization bits %d for Milvus", config.Bits)
		}
		m := dimensions * bits / defaultMilvusPQBits
		if m <= 0 || m > dimensions || dimensions%m != 0 {
			return nil, errors.NotSupportedf("PQ quantization bits %d for Milvus", config.Bits)
		}
		return index.NewIvfPQIndex(metricType, defaultMilvusIVFNList, m, defaultMilvusPQBits), nil
	case QuantizationSQ:
		if config.Bits != 0 && config.Bits != 8 {
			return nil, errors.NotSupportedf("SQ quantization bits %d for Milvus", config.Bits)
		}
		return index.NewIvfSQ8Index(metricType, defaultMilvusIVFNList), nil
	default:
		return nil, errors.NotSupportedf("quantization type %s for Milvus", config.Type)
	}
}

func (db *Milvus) searchParam(ctx context.Context, collection string) (index.AnnParam, error) {
	idx, err := db.client.DescribeIndex(ctx, milvusclient.NewDescribeIndexOption(collection, milvusVectorField))
	if err != nil {
		return nil, errors.Trace(err)
	}
	switch index.IndexType(idx.Params()[index.IndexTypeKey]) {
	case milvusIVFRQIndexType:
		return index.NewIvfRabitQAnnParam(defaultMilvusRQNProbe).WithRefineK(defaultMilvusRQRefineK), nil
	case index.IvfPQ, index.IvfSQ8:
		searchParam := index.NewCustomAnnParam()
		searchParam.WithExtraParam("nprobe", defaultMilvusRQNProbe)
		return searchParam, nil
	default:
		return index.NewHNSWAnnParam(100), nil
	}
}
