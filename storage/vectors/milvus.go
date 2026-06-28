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
	"sync"
	"time"

	"github.com/gorse-io/gorse/storage"
	"github.com/juju/errors"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
	milvusIdField         = "id"
	milvusVectorField     = "vector"
	milvusCategoriesField = "categories"
	milvusTimestampField  = "timestamp"

	milvusIVFRQIndexType = entity.IndexType("IVF_RABITQ")

	defaultMilvusIVFNList    = 128
	defaultMilvusRQNProbe    = 8
	defaultMilvusRQQueryBits = 0
	defaultMilvusRQRefineK   = 1
)

func init() {
	Register([]string{storage.MilvusPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		database := &Milvus{
			rqQueryBits: make(map[string]int),
		}
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
	client      client.Client
	mu          sync.RWMutex
	rqQueryBits map[string]int
}

func (db *Milvus) Init() error {
	return nil
}

func (db *Milvus) Optimize() error {
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

func (db *Milvus) DescribeCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	collection, err := db.client.DescribeCollection(ctx, name)
	if err != nil {
		return nil, errors.Trace(err)
	}
	dimension, err := milvusVectorDimension(collection)
	if err != nil {
		return nil, errors.Trace(err)
	}
	indexes, err := db.client.DescribeIndex(ctx, name, milvusVectorField)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(indexes) == 0 {
		return nil, errors.NotFoundf("index for collection %s", name)
	}
	var distance Distance
	switch metricType := entity.MetricType(indexes[0].Params()["metric_type"]); metricType {
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
	switch indexes[0].IndexType() {
	case milvusIVFRQIndexType:
		config.Type = QuantizationRQ
		config.Bits = db.getRQQueryBits(name)
	}
	return &CollectionInfo{
		Name:         name,
		Dimension:    dimension,
		Distance:     distance,
		VectorConfig: config,
	}, nil
}

func (db *Milvus) AddCollection(ctx context.Context, name string, dimensions int, distance Distance, config VectorConfig) error {
	// Milvus SQ support requires Int8Vector which may not be available in older SDK versions
	if config.Type == QuantizationSQ {
		return errors.NotSupportedf("SQ quantization for Milvus")
	}

	schema := entity.NewSchema().WithName(name).WithDescription("gorse collection").
		WithField(entity.NewField().WithName(milvusIdField).WithDataType(entity.FieldTypeVarChar).WithMaxLength(65535).WithIsPrimaryKey(true)).
		WithField(entity.NewField().WithName(milvusCategoriesField).WithDataType(entity.FieldTypeArray).WithElementType(entity.FieldTypeVarChar).WithMaxCapacity(100).WithMaxLength(65535)).
		WithField(entity.NewField().WithName(milvusTimestampField).WithDataType(entity.FieldTypeInt64)).
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

	idx, err := milvusIndex(metricType, config)
	if err != nil {
		return errors.Trace(err)
	}
	err = db.client.CreateIndex(ctx, name, milvusVectorField, idx, false)
	if err != nil {
		return errors.Trace(err)
	}
	if config.Type == QuantizationRQ {
		db.setRQQueryBits(name, config.Bits)
	}

	scalarIdx := entity.NewScalarIndex()
	err = db.client.CreateIndex(ctx, name, milvusTimestampField, scalarIdx, false)
	if err != nil {
		return errors.Trace(err)
	}

	// Load collection
	err = db.client.LoadCollection(ctx, name, false)
	return errors.Trace(err)
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
	exists, err := db.client.HasCollection(ctx, name)
	if err != nil {
		return errors.Trace(err)
	}
	if !exists {
		return errors.NotFoundf("collection %s", name)
	}
	err = db.client.DropCollection(ctx, name)
	db.deleteRQQueryBits(name)
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

	idCol := entity.NewColumnVarChar(milvusIdField, ids)
	categoriesCol := entity.NewColumnVarCharArray(milvusCategoriesField, milvusStringsToBytes(categories))
	timestampCol := entity.NewColumnInt64(milvusTimestampField, timestamps)
	vectorCol := entity.NewColumnFloatVector(milvusVectorField, len(data[0]), data)

	_, err := db.client.Upsert(ctx, collection, "", idCol, categoriesCol, timestampCol, vectorCol)
	return errors.Trace(err)
}

func (db *Milvus) DeleteVectors(ctx context.Context, collection string, timestamp time.Time) error {
	err := db.client.Delete(ctx, collection, "", fmt.Sprintf("%s < %d", milvusTimestampField, timestamp.UnixMilli()))
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

func milvusIndex(metricType entity.MetricType, config VectorConfig) (entity.Index, error) {
	switch config.Type {
	case QuantizationNone:
		return entity.NewIndexHNSW(metricType, 16, 200)
	case QuantizationRQ:
		if _, err := milvusRQQueryBits(config.Bits); err != nil {
			return nil, errors.Trace(err)
		}
		params := map[string]string{
			"metric_type": string(metricType),
			"nlist":       strconv.Itoa(defaultMilvusIVFNList),
		}
		return entity.NewGenericIndex(milvusVectorField, milvusIVFRQIndexType, params), nil
	case QuantizationPQ:
		return nil, errors.NotSupportedf("PQ quantization for Milvus")
	case QuantizationSQ:
		return nil, errors.NotSupportedf("SQ quantization for Milvus")
	default:
		return nil, errors.NotSupportedf("quantization type %s for Milvus", config.Type)
	}
}

func (db *Milvus) searchParam(ctx context.Context, collection string) (entity.SearchParam, error) {
	indexes, err := db.client.DescribeIndex(ctx, collection, milvusVectorField)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(indexes) == 0 {
		return nil, errors.NotFoundf("index for collection %s", collection)
	}
	switch indexes[0].IndexType() {
	case milvusIVFRQIndexType:
		return milvusSearchParam{
			"nprobe":         defaultMilvusRQNProbe,
			"rbq_query_bits": db.getRQQueryBits(collection),
			"refine_k":       defaultMilvusRQRefineK,
		}, nil
	default:
		return entity.NewIndexHNSWSearchParam(100)
	}
}

func milvusRQQueryBits(bits int) (int, error) {
	if bits < 0 || bits > 8 {
		return 0, errors.NotSupportedf("RQ quantization bits %d for Milvus", bits)
	}
	return bits, nil
}

func (db *Milvus) setRQQueryBits(collection string, bits int) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.rqQueryBits[collection] = bits
}

func (db *Milvus) getRQQueryBits(collection string) int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if bits, ok := db.rqQueryBits[collection]; ok {
		return bits
	}
	return defaultMilvusRQQueryBits
}

func (db *Milvus) deleteRQQueryBits(collection string) {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.rqQueryBits, collection)
}

type milvusSearchParam map[string]any

func (p milvusSearchParam) Params() map[string]any {
	params := make(map[string]any, len(p))
	for k, v := range p {
		params[k] = v
	}
	return params
}

func (p milvusSearchParam) AddRadius(radius float64) {
	p["radius"] = radius
}

func (p milvusSearchParam) AddRangeFilter(rangeFilter float64) {
	p["range_filter"] = rangeFilter
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
