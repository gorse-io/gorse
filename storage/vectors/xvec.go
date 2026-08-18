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
	stderrors "errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorse-io/gorse/storage"
	"github.com/gorse-io/xvec"
	"github.com/pkg/errors"
)

const (
	xvecVectorField     = "vector"
	xvecCategoriesField = "categories"
	xvecTimestampField  = "timestamp"
	xvecHiddenField     = "hidden"
)

func init() {
	Register([]string{storage.XvecPrefix}, func(path, tablePrefix string, _ ...storage.Option) (Database, error) {
		root := strings.TrimPrefix(path, storage.XvecPrefix)
		if root == "" {
			return nil, errors.New("xvec path is empty")
		}
		return &Xvec{root: root, tablePrefix: tablePrefix}, nil
	})
}

// Xvec stores each Gorse vector collection in a xvec collection directory.
type Xvec struct {
	root        string
	tablePrefix string
	collections sync.Map // map[string]*xvec.Collection
	closed      atomic.Bool
}

func (db *Xvec) Init() error {
	if err := os.MkdirAll(db.root, 0o755); err != nil {
		return errors.WithStack(err)
	}
	entries, err := os.ReadDir(db.root)
	if err != nil {
		return errors.WithStack(err)
	}

	if db.closed.Load() {
		return errors.New("xvec database is closed")
	}
	hasCollections := false
	db.collections.Range(func(_, _ any) bool {
		hasCollections = true
		return false
	})
	if hasCollections {
		return nil
	}
	type openedCollection struct {
		name       string
		collection *xvec.Collection
	}
	opened := make([]openedCollection, 0, len(entries))
	cleanup := func() {
		for _, item := range opened {
			if db.collections.CompareAndDelete(item.name, item.collection) {
				_ = item.collection.Close()
			}
		}
	}
	for _, entry := range entries {
		if db.closed.Load() {
			cleanup()
			return errors.New("xvec database is closed")
		}
		if !entry.IsDir() {
			continue
		}
		if db.tablePrefix != "" && !strings.HasPrefix(entry.Name(), db.tablePrefix) {
			continue
		}
		name := strings.TrimPrefix(entry.Name(), db.tablePrefix)
		if name == "" {
			continue
		}
		collection, err := xvec.Open(context.Background(), filepath.Join(db.root, entry.Name()), xvec.CollectionOptions{})
		if err != nil {
			cleanup()
			return errors.WithStack(err)
		}
		if _, loaded := db.collections.LoadOrStore(name, collection); loaded {
			_ = collection.Close()
			continue
		}
		opened = append(opened, openedCollection{name: name, collection: collection})
	}
	if db.closed.Load() {
		cleanup()
		return errors.New("xvec database is closed")
	}
	return nil
}

func (db *Xvec) Optimize(ctx context.Context, name string) error {
	collection, err := db.collection(name)
	if err != nil {
		return err
	}
	return errors.WithStack(collection.Optimize(ctx, xvec.OptimizeOptions{}))
}

func (db *Xvec) Close() error {
	if !db.closed.CompareAndSwap(false, true) {
		return nil
	}

	var errs []error
	db.collections.Range(func(key, value any) bool {
		if db.collections.CompareAndDelete(key, value) {
			if err := value.(*xvec.Collection).Close(); err != nil {
				errs = append(errs, err)
			}
		}
		return true
	})
	return stderrors.Join(errs...)
}

func (db *Xvec) ListCollections(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.WithStack(err)
	}
	if db.closed.Load() {
		return nil, errors.New("xvec database is closed")
	}
	collections := make([]string, 0)
	db.collections.Range(func(key, _ any) bool {
		collections = append(collections, key.(string))
		return true
	})
	sort.Strings(collections)
	return collections, nil
}

func (db *Xvec) DescribeCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.WithStack(err)
	}
	collection, err := db.collection(name)
	if err != nil {
		return nil, err
	}
	field, found := collection.Schema().Field(xvecVectorField)
	if !found {
		return nil, errors.Errorf("xvec collection %s has no vector field", name)
	}
	var metric xvec.MetricType
	switch params := field.EffectiveIndex().(type) {
	case xvec.DiskANNIndexParams:
		metric = params.Metric
	case xvec.FlatIndexParams:
		metric = params.Metric
	default:
		return nil, errors.Errorf("xvec collection %s uses an unsupported vector index", name)
	}
	distance, err := xvecDistance(metric)
	if err != nil {
		return nil, err
	}
	return &CollectionInfo{
		Name: name, Dimension: int(field.Dimension), Distance: distance,
		VectorConfig: VectorConfig{Type: QuantizationNone},
	}, nil
}

func (db *Xvec) AddCollection(ctx context.Context, name string, dimensions int, distance Distance, config VectorConfig) error {
	schema, err := db.collectionSchema(ctx, name, dimensions, distance, config)
	if err != nil {
		return err
	}
	physicalName := db.tablePrefix + name

	if db.closed.Load() {
		return errors.New("xvec database is closed")
	}
	if _, found := db.collections.Load(name); found {
		return errors.Errorf("collection %s already exists", name)
	}
	collection, err := xvec.CreateAndOpen(ctx, filepath.Join(db.root, physicalName), schema, xvec.CollectionOptions{})
	if err != nil {
		return errors.WithStack(err)
	}
	if _, loaded := db.collections.LoadOrStore(name, collection); loaded {
		_ = collection.Close()
		return errors.Errorf("collection %s already exists", name)
	}
	if db.closed.Load() {
		if db.collections.CompareAndDelete(name, collection) {
			_ = collection.Close()
		}
		return errors.New("xvec database is closed")
	}
	return nil
}

func (db *Xvec) collectionSchema(ctx context.Context, name string, dimensions int, distance Distance, config VectorConfig) (xvec.CollectionSchema, error) {
	if err := ctx.Err(); err != nil {
		return xvec.CollectionSchema{}, errors.WithStack(err)
	}
	if dimensions < 0 || uint64(dimensions) > uint64(math.MaxUint32) {
		return xvec.CollectionSchema{}, errors.Errorf("invalid vector dimension %d", dimensions)
	}
	if config.Type != QuantizationNone {
		return xvec.CollectionSchema{}, errors.Errorf("quantization type %s for xvec not supported", config.Type)
	}
	metric, err := distanceToXvec(distance)
	if err != nil {
		return xvec.CollectionSchema{}, err
	}
	var vectorField xvec.FieldSchema
	if dimensions == 0 {
		if distance != Dot {
			return xvec.CollectionSchema{}, fmt.Errorf("distance method for sparse vector %w", ErrNotSupported)
		}
		vectorField = xvec.FieldSchema{Name: xvecVectorField, DataType: xvec.DataTypeSparseVectorFP32, Index: xvec.NewFlatIndexParams(metric)}
	} else {
		vectorField = xvec.FieldSchema{Name: xvecVectorField, DataType: xvec.DataTypeVectorFP32, Dimension: uint32(dimensions), Index: xvec.NewDiskANNIndexParams(metric)}
	}
	physicalName := db.tablePrefix + name
	schema := xvec.NewCollectionSchema(physicalName,
		xvec.FieldSchema{Name: xvecCategoriesField, DataType: xvec.DataTypeArrayString, Index: xvec.NewInvertIndexParams()},
		xvec.FieldSchema{Name: xvecTimestampField, DataType: xvec.DataTypeInt64, Index: xvec.NewInvertIndexParams()},
		xvec.NewField(xvecHiddenField, xvec.DataTypeBool),
		vectorField,
	)
	if err := schema.Validate(); err != nil {
		return xvec.CollectionSchema{}, errors.WithStack(err)
	}
	return schema, nil
}

func (db *Xvec) DeleteCollection(ctx context.Context, name string) error {
	if db.closed.Load() {
		return errors.New("xvec database is closed")
	}
	value, found := db.collections.LoadAndDelete(name)
	if !found {
		return errors.Wrapf(ErrNotFound, "collection %s", name)
	}
	collection := value.(*xvec.Collection)
	if err := collection.Destroy(ctx); err != nil {
		if !db.closed.Load() {
			db.collections.LoadOrStore(name, collection)
		}
		return errors.WithStack(err)
	}
	return nil
}

func (db *Xvec) CountVectors(ctx context.Context, name string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, errors.WithStack(err)
	}
	collection, err := db.collection(name)
	if err != nil {
		return 0, err
	}
	count := collection.Stats().DocumentCount
	if count > math.MaxInt64 {
		return 0, errors.Errorf("xvec collection %s contains too many vectors", name)
	}
	return int64(count), nil
}

func (db *Xvec) AddVectors(ctx context.Context, name string, vectors []Vector) error {
	if len(vectors) == 0 {
		return nil
	}
	collection, err := db.collection(name)
	if err != nil {
		return err
	}
	schema := collection.Schema()
	documents := make([]xvec.Document, len(vectors))
	for i, vector := range vectors {
		var value any = xvec.VectorFP32(vector.Values)
		if len(vector.Indices) > 0 {
			value = xvec.SparseVectorFP32{Indices: vector.Indices, Values: vector.Values}
		}
		document := xvec.Document{PrimaryKey: vector.Id, Fields: map[string]any{
			xvecVectorField:     value,
			xvecCategoriesField: xvec.StringArray(vector.Categories),
			xvecTimestampField:  vector.Timestamp.UnixMilli(),
			xvecHiddenField:     vector.IsHidden,
		}}
		if err := document.Validate(schema); err != nil {
			return errors.WithStack(err)
		}
		documents[i] = document
	}
	_, err = collection.Upsert(ctx, documents)
	return errors.WithStack(err)
}

func (db *Xvec) GetVectors(ctx context.Context, name string, ids []string) ([]Vector, error) {
	if len(ids) == 0 {
		return []Vector{}, nil
	}
	collection, err := db.collection(name)
	if err != nil {
		return nil, err
	}
	documents, err := collection.Fetch(ctx, ids, xvec.Projection{
		OutputFields:   []string{xvecCategoriesField, xvecTimestampField, xvecHiddenField},
		IncludeVectors: true,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	vectors := make([]Vector, 0, len(documents))
	for _, document := range documents {
		if document == nil {
			continue
		}
		vector := Vector{Id: document.PrimaryKey}
		if value, found := document.Field(xvecCategoriesField); found {
			vector.Categories = []string(value.(xvec.StringArray))
		}
		if value, found := document.Field(xvecTimestampField); found {
			vector.Timestamp = time.UnixMilli(value.(int64)).UTC()
		}
		if value, found := document.Field(xvecHiddenField); found {
			vector.IsHidden = value.(bool)
		}
		if value, found := document.Field(xvecVectorField); found {
			switch value := value.(type) {
			case xvec.VectorFP32:
				vector.Values = []float32(value)
			case xvec.SparseVectorFP32:
				vector.Indices = value.Indices
				vector.Values = value.Values
			}
		}
		vectors = append(vectors, vector)
	}
	return orderVectors(ids, vectors), nil
}

func (db *Xvec) DeleteVectors(ctx context.Context, name string, timestamp time.Time) error {
	collection, err := db.collection(name)
	if err != nil {
		return err
	}
	return errors.WithStack(collection.DeleteByFilter(ctx, fmt.Sprintf("%s < %d", xvecTimestampField, timestamp.UnixMilli())))
}

func (db *Xvec) QueryVectors(ctx context.Context, name string, q Vector, categories []string, topK int) ([]ScoredVector, error) {
	if topK <= 0 {
		return []ScoredVector{}, nil
	}
	collection, err := db.collection(name)
	if err != nil {
		return nil, err
	}
	filter := fmt.Sprintf("%s = false", xvecHiddenField)
	if len(categories) > 0 {
		quoted := make([]string, len(categories))
		for i, category := range categories {
			escaped := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(category)
			quoted[i] = "'" + escaped + "'"
		}
		filter += fmt.Sprintf(" AND %s CONTAIN_ANY (%s)", xvecCategoriesField, strings.Join(quoted, ", "))
	}
	query := xvec.VectorQuery{
		Field:  xvecVectorField,
		TopK:   topK,
		Filter: filter,
		Projection: xvec.Projection{
			OutputFields:   []string{xvecCategoriesField, xvecTimestampField, xvecHiddenField},
			IncludeVectors: true,
		},
	}
	if len(q.Indices) > 0 {
		query.SparseVector = xvec.SparseVectorFP32{Indices: q.Indices, Values: q.Values}
		query.Params = xvec.NewFlatQueryParams()
	} else {
		query.DenseVector = xvec.VectorFP32(q.Values)
		query.Params = xvec.NewDiskANNQueryParams()
	}
	documents, err := collection.Query(ctx, query)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	info, err := db.DescribeCollection(ctx, name)
	if err != nil {
		return nil, err
	}
	results := make([]ScoredVector, 0, len(documents))
	for _, document := range documents {
		if len(q.Indices) > 0 && document.Score == 0 {
			continue
		}
		result := ScoredVector{Vector: Vector{Id: document.PrimaryKey}, Score: document.Score}
		if info.Distance != Dot {
			result.Score = -result.Score
		}
		if value, found := document.Field(xvecCategoriesField); found {
			result.Categories = []string(value.(xvec.StringArray))
		}
		if value, found := document.Field(xvecTimestampField); found {
			result.Timestamp = time.UnixMilli(value.(int64))
		}
		if value, found := document.Field(xvecHiddenField); found {
			result.IsHidden = value.(bool)
		}
		if value, found := document.Field(xvecVectorField); found {
			switch vector := value.(type) {
			case xvec.VectorFP32:
				result.Vector.Values = []float32(vector)
			case xvec.SparseVectorFP32:
				result.Vector.Indices = vector.Indices
				result.Vector.Values = vector.Values
			}
		}
		results = append(results, result)
	}
	return results, nil
}

func (db *Xvec) collection(name string) (*xvec.Collection, error) {
	if db.closed.Load() {
		return nil, errors.New("xvec database is closed")
	}
	value, found := db.collections.Load(name)
	if !found {
		return nil, errors.Wrapf(ErrNotFound, "collection %s", name)
	}
	return value.(*xvec.Collection), nil
}

func distanceToXvec(distance Distance) (xvec.MetricType, error) {
	switch distance {
	case Cosine:
		return xvec.MetricTypeCosine, nil
	case Euclidean:
		return xvec.MetricTypeL2, nil
	case Dot:
		return xvec.MetricTypeIP, nil
	default:
		return xvec.MetricTypeUndefined, errors.Errorf("distance method %v not supported", distance)
	}
}

func xvecDistance(metric xvec.MetricType) (Distance, error) {
	switch metric {
	case xvec.MetricTypeCosine:
		return Cosine, nil
	case xvec.MetricTypeL2:
		return Euclidean, nil
	case xvec.MetricTypeIP:
		return Dot, nil
	default:
		return Cosine, errors.Errorf("xvec metric %s not supported", metric)
	}
}
