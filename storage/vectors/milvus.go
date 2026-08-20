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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gorse-io/gorse/storage"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/milvus-io/milvus/pkg/v2/util/merr"
	"github.com/pkg/errors"
)

const (
	milvusIdField         = "id"
	milvusVectorField     = "vector"
	milvusCategoriesField = "categories"
	milvusHiddenField     = "hidden"
	milvusTimestampField  = "timestamp"
	milvusDefaultDatabase = "default"

	milvusIVFRQIndexType = index.IvfRabitQ

	defaultMilvusIVFNList  = 128
	defaultMilvusPQBits    = 8
	defaultMilvusRQNProbe  = 8
	defaultMilvusRQRefineK = 1
)

func init() {
	Register([]string{storage.MilvusPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		u, err := url.Parse(path)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		databaseName := strings.Trim(u.Path, "/")
		if databaseName == "" {
			databaseName = milvusDefaultDatabase
		}
		database := &Milvus{database: databaseName}
		database.client, err = milvusclient.New(context.Background(), &milvusclient.ClientConfig{
			Address: u.Host,
			DBName:  databaseName,
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return database, nil
	})
}

type Milvus struct {
	client   *milvusclient.Client
	database string
}

func (db *Milvus) Init() error {
	return nil
}

func (db *Milvus) Optimize(_ context.Context, _ string) error {
	return nil
}

func (db *Milvus) Close() error {
	return db.client.Close(context.Background())
}

func (db *Milvus) Purge() error {
	if db.database == milvusDefaultDatabase {
		return fmt.Errorf("purging the default Milvus database %w", storage.ErrNotSupported)
	}
	ctx := context.Background()
	if err := db.client.UseDatabase(ctx, milvusclient.NewUseDatabaseOption(milvusDefaultDatabase)); err != nil {
		return errors.WithStack(err)
	}
	if err := db.client.DropDatabase(ctx, milvusclient.NewDropDatabaseOption(db.database)); err != nil {
		return errors.WithStack(err)
	}
	if err := db.client.CreateDatabase(ctx, milvusclient.NewCreateDatabaseOption(db.database)); err != nil {
		return errors.WithStack(err)
	}
	return errors.WithStack(db.client.UseDatabase(ctx, milvusclient.NewUseDatabaseOption(db.database)))
}

func (db *Milvus) ListCollections(ctx context.Context) ([]string, error) {
	collections, err := db.client.ListCollections(ctx, milvusclient.NewListCollectionOption())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return collections, nil
}

func (db *Milvus) DescribeCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	collection, err := db.client.DescribeCollection(ctx, milvusclient.NewDescribeCollectionOption(name))
	if err != nil {
		if errors.Is(err, merr.ErrCollectionNotFound) {
			return nil, fmt.Errorf("collection %s: %w", name, storage.ErrNotFound)
		}
		return nil, errors.WithStack(err)
	}
	dimension, err := milvusVectorDimension(collection)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	idx, err := db.client.DescribeIndex(ctx, milvusclient.NewDescribeIndexOption(name, milvusVectorField))
	if err != nil {
		return nil, errors.WithStack(err)
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
		return nil, fmt.Errorf("distance method %s %w", metricType, storage.ErrNotSupported)
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
			return nil, errors.WithStack(err)
		}
		nbits, err := strconv.Atoi(idx.Params()["nbits"])
		if err != nil {
			return nil, errors.WithStack(err)
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
	if dimensions == 0 {
		if distance != Dot {
			return fmt.Errorf("distance method for sparse vector %w", storage.ErrNotSupported)
		}
		if config != (VectorConfig{}) {
			return fmt.Errorf("quantization for sparse vector %w", storage.ErrNotSupported)
		}
	}

	schema := entity.NewSchema().WithName(name).WithDescription("gorse collection").
		WithField(entity.NewField().WithName(milvusIdField).WithDataType(entity.FieldTypeVarChar).WithMaxLength(65535).WithIsPrimaryKey(true)).
		WithField(entity.NewField().WithName(milvusCategoriesField).WithDataType(entity.FieldTypeArray).WithElementType(entity.FieldTypeVarChar).WithMaxCapacity(100).WithMaxLength(65535)).
		WithField(entity.NewField().WithName(milvusHiddenField).WithDataType(entity.FieldTypeBool)).
		WithField(entity.NewField().WithName(milvusTimestampField).WithDataType(entity.FieldTypeInt64))
	if dimensions == 0 {
		schema.WithField(entity.NewField().WithName(milvusVectorField).WithDataType(entity.FieldTypeSparseVector))
	} else {
		schema.WithField(entity.NewField().WithName(milvusVectorField).WithDataType(entity.FieldTypeFloatVector).WithDim(int64(dimensions)))
	}

	err := db.client.CreateCollection(ctx, milvusclient.NewCreateCollectionOption(name, schema).WithShardNum(entity.DefaultShardNumber))
	if err != nil {
		return errors.WithStack(err)
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
		return fmt.Errorf("distance method %w", storage.ErrNotSupported)
	}

	var idx index.Index
	if dimensions == 0 {
		idx = index.NewSparseInvertedIndex(entity.IP, 0)
	} else {
		idx, err = milvusIndex(metricType, dimensions, config)
	}
	if err != nil {
		return errors.WithStack(err)
	}
	indexTask, err := db.client.CreateIndex(ctx, milvusclient.NewCreateIndexOption(name, milvusVectorField, idx).WithIndexName(milvusVectorField))
	if err != nil {
		return errors.WithStack(err)
	}
	if err = indexTask.Await(ctx); err != nil {
		return errors.WithStack(err)
	}

	indexTask, err = db.client.CreateIndex(ctx, milvusclient.NewCreateIndexOption(name, milvusTimestampField, index.NewSortedIndex()).WithIndexName(milvusTimestampField))
	if err != nil {
		return errors.WithStack(err)
	}
	if err = indexTask.Await(ctx); err != nil {
		return errors.WithStack(err)
	}

	// Load collection
	loadTask, err := db.client.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(name))
	if err != nil {
		return errors.WithStack(err)
	}
	return errors.WithStack(loadTask.Await(ctx))
}

func milvusVectorDimension(collection *entity.Collection) (int, error) {
	if collection == nil || collection.Schema == nil {
		return 0, fmt.Errorf("collection schema: %w", storage.ErrNotFound)
	}
	for _, field := range collection.Schema.Fields {
		if field.Name == milvusVectorField {
			if field.DataType == entity.FieldTypeSparseVector {
				return 0, nil
			}
			dimension, err := strconv.Atoi(field.TypeParams[entity.TypeParamDim])
			if err != nil {
				return 0, errors.WithStack(err)
			}
			return dimension, nil
		}
	}
	return 0, fmt.Errorf("vector field: %w", storage.ErrNotFound)
}

func (db *Milvus) DeleteCollection(ctx context.Context, name string) error {
	exists, err := db.client.HasCollection(ctx, milvusclient.NewHasCollectionOption(name))
	if err != nil {
		return errors.WithStack(err)
	}
	if !exists {
		return fmt.Errorf("collection %s: %w", name, storage.ErrNotFound)
	}
	err = db.client.DropCollection(ctx, milvusclient.NewDropCollectionOption(name))
	return errors.WithStack(err)
}

func (db *Milvus) CountVectors(ctx context.Context, collection string) (int64, error) {
	result, err := db.client.Query(ctx, milvusclient.NewQueryOption(collection).
		WithOutputFields("count(*)").
		WithConsistencyLevel(entity.ClStrong))
	if err != nil {
		return 0, errors.WithStack(err)
	}
	countCol, ok := result.GetColumn("count(*)").(*column.ColumnInt64)
	if !ok {
		return 0, errors.Errorf("failed to parse vector count for collection %s", collection)
	}
	count, err := countCol.Value(0)
	return count, errors.WithStack(err)
}

func (db *Milvus) AddVectors(ctx context.Context, collection string, vectors []Vector) error {
	if len(vectors) == 0 {
		return nil
	}
	ids := make([]string, 0, len(vectors))
	categories := make([][]string, 0, len(vectors))
	hidden := make([]bool, 0, len(vectors))
	timestamps := make([]int64, 0, len(vectors))
	data := make([][]float32, 0, len(vectors))
	sparseData := make([]entity.SparseEmbedding, 0, len(vectors))
	for _, v := range vectors {
		ids = append(ids, v.Id)
		categories = append(categories, v.Categories)
		hidden = append(hidden, v.IsHidden)
		timestamps = append(timestamps, v.Timestamp.UnixMilli())
		data = append(data, v.Values)
		if len(v.Indices) > 0 {
			sparse, err := entity.NewSliceSparseEmbedding(v.Indices, v.Values)
			if err != nil {
				return errors.WithStack(err)
			}
			sparseData = append(sparseData, sparse)
		}
	}

	idCol := column.NewColumnVarChar(milvusIdField, ids)
	categoriesCol := column.NewColumnVarCharArray(milvusCategoriesField, categories)
	hiddenCol := column.NewColumnBool(milvusHiddenField, hidden)
	timestampCol := column.NewColumnInt64(milvusTimestampField, timestamps)
	var vectorCol column.Column
	if len(sparseData) > 0 {
		if len(sparseData) != len(vectors) {
			return errors.Errorf("cannot mix dense and sparse vectors")
		}
		vectorCol = column.NewColumnSparseVectors(milvusVectorField, sparseData)
	} else {
		vectorCol = column.NewColumnFloatVector(milvusVectorField, len(data[0]), data)
	}

	_, err := db.client.Upsert(ctx, milvusclient.NewColumnBasedInsertOption(collection, idCol, categoriesCol, hiddenCol, timestampCol, vectorCol))
	return errors.WithStack(err)
}

func (db *Milvus) GetVectors(ctx context.Context, collection string, ids []string) ([]Vector, error) {
	if len(ids) == 0 {
		return []Vector{}, nil
	}
	result, err := db.client.Query(ctx, milvusclient.NewQueryOption(collection).
		WithIDs(column.NewColumnVarChar(milvusIdField, slices.Clone(ids))).
		WithOutputFields(milvusIdField, milvusCategoriesField, milvusHiddenField, milvusTimestampField, milvusVectorField).
		WithConsistencyLevel(entity.ClStrong))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return milvusVectors(collection, ids, result)
}

func milvusVectors(collection string, ids []string, result milvusclient.ResultSet) ([]Vector, error) {
	idCol, ok := result.GetColumn(milvusIdField).(*column.ColumnVarChar)
	if !ok {
		return nil, errors.Errorf("failed to parse vector ids for collection %s", collection)
	}
	categoriesCol, ok := result.GetColumn(milvusCategoriesField).(*column.ColumnVarCharArray)
	if !ok {
		return nil, errors.Errorf("failed to parse vector categories for collection %s", collection)
	}
	hiddenCol, ok := result.GetColumn(milvusHiddenField).(*column.ColumnBool)
	if !ok {
		return nil, errors.Errorf("failed to parse vector visibility for collection %s", collection)
	}
	timestampCol, ok := result.GetColumn(milvusTimestampField).(*column.ColumnInt64)
	if !ok {
		return nil, errors.Errorf("failed to parse vector timestamps for collection %s", collection)
	}
	vectorCol := result.GetColumn(milvusVectorField)
	vectors := make([]Vector, 0, result.Len())
	for i := 0; i < result.Len(); i++ {
		id, err := idCol.Value(i)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		categories, err := categoriesCol.Value(i)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		hidden, err := hiddenCol.Value(i)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		timestamp, err := timestampCol.Value(i)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		vector := Vector{
			Id:         id,
			IsHidden:   hidden,
			Categories: categories,
			Timestamp:  time.UnixMilli(timestamp).UTC(),
		}
		switch values := vectorCol.(type) {
		case *column.ColumnFloatVector:
			value, err := values.Value(i)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			vector.Values = []float32(value)
		case *column.ColumnSparseFloatVector:
			value, err := values.Value(i)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			vector.Indices = make([]uint32, value.Len())
			vector.Values = make([]float32, value.Len())
			for j := 0; j < value.Len(); j++ {
				vector.Indices[j], vector.Values[j], _ = value.Get(j)
			}
		default:
			return nil, errors.Errorf("failed to parse vector values for collection %s", collection)
		}
		vectors = append(vectors, vector)
	}
	return orderVectors(ids, vectors), nil
}

func (db *Milvus) DeleteVectors(ctx context.Context, collection string, timestamp time.Time) error {
	_, err := db.client.Delete(ctx, milvusclient.NewDeleteOption(collection).WithExpr(fmt.Sprintf("%s < %d", milvusTimestampField, timestamp.UnixMilli())))
	return errors.WithStack(err)
}

func (db *Milvus) QueryVectors(ctx context.Context, collection string, q Vector, categories []string, topK int) ([]ScoredVector, error) {
	if topK <= 0 {
		return []ScoredVector{}, nil
	}

	expr := fmt.Sprintf("%s == false", milvusHiddenField)
	if len(categories) > 0 {
		var conditions []string
		for i := range categories {
			conditions = append(conditions, fmt.Sprintf("array_contains(%s, {category_%d})", milvusCategoriesField, i))
		}
		expr += " and (" + strings.Join(conditions, " or ") + ")"
	}

	searchParam, distance, err := db.searchParam(ctx, collection)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var query entity.Vector
	if len(q.Indices) > 0 {
		query, err = entity.NewSliceSparseEmbedding(q.Indices, q.Values)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	} else {
		query = entity.FloatVector(q.Values)
	}
	searchOption := milvusclient.NewSearchOption(collection, topK, []entity.Vector{query}).
		WithANNSField(milvusVectorField).
		WithFilter(expr).
		WithOutputFields(milvusIdField, milvusCategoriesField, milvusHiddenField).
		WithAnnParam(searchParam).
		WithConsistencyLevel(entity.ClStrong)
	for i, category := range categories {
		searchOption.WithTemplateParam(fmt.Sprintf("category_%d", i), category)
	}
	results, err := db.client.Search(ctx, searchOption)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var vectors []ScoredVector
	for _, result := range results {
		if result.Err != nil {
			return nil, errors.WithStack(result.Err)
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

		var hiddenCol *column.ColumnBool
		if col := result.GetColumn(milvusHiddenField); col != nil {
			hiddenCol = col.(*column.ColumnBool)
		}

		for i := 0; i < result.ResultCount; i++ {
			var id string
			if idCol != nil {
				id, err = idCol.Value(i)
				if err != nil {
					return nil, errors.WithStack(err)
				}
			}

			var cats []string
			if categoriesCol != nil {
				cats, err = categoriesCol.Value(i)
				if err != nil {
					return nil, errors.WithStack(err)
				}
			}

			var hidden bool
			if hiddenCol != nil {
				hidden, err = hiddenCol.Value(i)
				if err != nil {
					return nil, errors.WithStack(err)
				}
			}

			score := result.Scores[i]
			if distance == Euclidean {
				score = -score
			}
			vectors = append(vectors, ScoredVector{
				Vector: Vector{
					Id:         id,
					IsHidden:   hidden,
					Categories: cats,
				},
				Score: score,
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
			return nil, fmt.Errorf("RQ quantization bits %d for Milvus %w", config.Bits, storage.ErrNotSupported)
		}
		return index.NewIvfRabitQIndex(metricType, defaultMilvusIVFNList), nil
	case QuantizationPQ:
		bits := config.Bits
		if bits == 0 {
			bits = defaultMilvusPQBits
		}
		if bits <= 0 || dimensions <= 0 || dimensions*bits%defaultMilvusPQBits != 0 {
			return nil, fmt.Errorf("PQ quantization bits %d for Milvus %w", config.Bits, storage.ErrNotSupported)
		}
		m := dimensions * bits / defaultMilvusPQBits
		if m <= 0 || m > dimensions || dimensions%m != 0 {
			return nil, fmt.Errorf("PQ quantization bits %d for Milvus %w", config.Bits, storage.ErrNotSupported)
		}
		return index.NewIvfPQIndex(metricType, defaultMilvusIVFNList, m, defaultMilvusPQBits), nil
	case QuantizationSQ:
		if config.Bits != 0 && config.Bits != 8 {
			return nil, fmt.Errorf("SQ quantization bits %d for Milvus %w", config.Bits, storage.ErrNotSupported)
		}
		return index.NewIvfSQ8Index(metricType, defaultMilvusIVFNList), nil
	default:
		return nil, fmt.Errorf("quantization type %s for Milvus %w", config.Type, storage.ErrNotSupported)
	}
}

func (db *Milvus) searchParam(ctx context.Context, collection string) (index.AnnParam, Distance, error) {
	idx, err := db.client.DescribeIndex(ctx, milvusclient.NewDescribeIndexOption(collection, milvusVectorField))
	if err != nil {
		return nil, Cosine, errors.WithStack(err)
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
		return nil, Cosine, fmt.Errorf("distance method %s %w", metricType, storage.ErrNotSupported)
	}
	switch index.IndexType(idx.Params()[index.IndexTypeKey]) {
	case index.SparseInverted, index.SparseWAND:
		return index.NewSparseAnnParam(), distance, nil
	case milvusIVFRQIndexType:
		return index.NewIvfRabitQAnnParam(defaultMilvusRQNProbe).WithRefineK(defaultMilvusRQRefineK), distance, nil
	case index.IvfPQ, index.IvfSQ8:
		searchParam := index.NewCustomAnnParam()
		searchParam.WithExtraParam("nprobe", defaultMilvusRQNProbe)
		return searchParam, distance, nil
	default:
		return index.NewHNSWAnnParam(100), distance, nil
	}
}
