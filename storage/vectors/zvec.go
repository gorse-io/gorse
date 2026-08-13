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
	zvecdb "github.com/gorse-io/zvec"
	"github.com/juju/errors"
)

const (
	zvecVectorField     = "vector"
	zvecCategoriesField = "categories"
	zvecTimestampField  = "timestamp"
	zvecHiddenField     = "hidden"
)

func init() {
	Register([]string{storage.ZvecPrefix}, func(path, tablePrefix string, _ ...storage.Option) (Database, error) {
		root := strings.TrimPrefix(path, storage.ZvecPrefix)
		if root == "" {
			return nil, errors.New("zvec path is empty")
		}
		return &Zvec{root: root, tablePrefix: tablePrefix}, nil
	})
}

// Zvec stores each Gorse vector collection in a zvec collection directory.
type Zvec struct {
	root        string
	tablePrefix string
	collections sync.Map // map[string]*zvecdb.Collection
	closed      atomic.Bool
}

func (db *Zvec) Init() error {
	if err := os.MkdirAll(db.root, 0o755); err != nil {
		return errors.Trace(err)
	}
	entries, err := os.ReadDir(db.root)
	if err != nil {
		return errors.Trace(err)
	}

	if db.closed.Load() {
		return errors.New("zvec database is closed")
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
		collection *zvecdb.Collection
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
			return errors.New("zvec database is closed")
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
		collection, err := zvecdb.Open(context.Background(), filepath.Join(db.root, entry.Name()), zvecdb.CollectionOptions{})
		if err != nil {
			cleanup()
			return errors.Trace(err)
		}
		if _, loaded := db.collections.LoadOrStore(name, collection); loaded {
			_ = collection.Close()
			continue
		}
		opened = append(opened, openedCollection{name: name, collection: collection})
	}
	if db.closed.Load() {
		cleanup()
		return errors.New("zvec database is closed")
	}
	return nil
}

func (db *Zvec) Optimize(ctx context.Context, name string) error {
	collection, err := db.collection(name)
	if err != nil {
		return err
	}
	return errors.Trace(collection.Optimize(ctx, zvecdb.OptimizeOptions{}))
}

func (db *Zvec) Close() error {
	if !db.closed.CompareAndSwap(false, true) {
		return nil
	}

	var errs []error
	db.collections.Range(func(key, value any) bool {
		if db.collections.CompareAndDelete(key, value) {
			if err := value.(*zvecdb.Collection).Close(); err != nil {
				errs = append(errs, err)
			}
		}
		return true
	})
	return stderrors.Join(errs...)
}

func (db *Zvec) ListCollections(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Trace(err)
	}
	if db.closed.Load() {
		return nil, errors.New("zvec database is closed")
	}
	collections := make([]string, 0)
	db.collections.Range(func(key, _ any) bool {
		collections = append(collections, key.(string))
		return true
	})
	sort.Strings(collections)
	return collections, nil
}

func (db *Zvec) DescribeCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Trace(err)
	}
	collection, err := db.collection(name)
	if err != nil {
		return nil, err
	}
	field, found := collection.Schema().Field(zvecVectorField)
	if !found {
		return nil, errors.Errorf("zvec collection %s has no vector field", name)
	}
	params, ok := field.EffectiveIndex().(zvecdb.DiskANNIndexParams)
	if !ok {
		return nil, errors.Errorf("zvec collection %s does not use DiskANN", name)
	}
	distance, err := zvecDistance(params.Metric)
	if err != nil {
		return nil, err
	}
	return &CollectionInfo{
		Name: name, Dimension: int(field.Dimension), Distance: distance,
		VectorConfig: VectorConfig{Type: QuantizationNone},
	}, nil
}

func (db *Zvec) AddCollection(ctx context.Context, name string, dimensions int, distance Distance, config VectorConfig) error {
	schema, err := db.collectionSchema(ctx, name, dimensions, distance, config)
	if err != nil {
		return err
	}
	physicalName := db.tablePrefix + name

	if db.closed.Load() {
		return errors.New("zvec database is closed")
	}
	if _, found := db.collections.Load(name); found {
		return errors.AlreadyExistsf("collection %s", name)
	}
	collection, err := zvecdb.CreateAndOpen(ctx, filepath.Join(db.root, physicalName), schema, zvecdb.CollectionOptions{})
	if err != nil {
		return errors.Trace(err)
	}
	if _, loaded := db.collections.LoadOrStore(name, collection); loaded {
		_ = collection.Close()
		return errors.AlreadyExistsf("collection %s", name)
	}
	if db.closed.Load() {
		if db.collections.CompareAndDelete(name, collection) {
			_ = collection.Close()
		}
		return errors.New("zvec database is closed")
	}
	return nil
}

func (db *Zvec) collectionSchema(ctx context.Context, name string, dimensions int, distance Distance, config VectorConfig) (zvecdb.CollectionSchema, error) {
	if err := ctx.Err(); err != nil {
		return zvecdb.CollectionSchema{}, errors.Trace(err)
	}
	if dimensions <= 0 || uint64(dimensions) > uint64(math.MaxUint32) {
		return zvecdb.CollectionSchema{}, errors.Errorf("invalid vector dimension %d", dimensions)
	}
	if config.Type != QuantizationNone {
		return zvecdb.CollectionSchema{}, errors.NotSupportedf("quantization type %s for zvec", config.Type)
	}
	metric, err := distanceToZvec(distance)
	if err != nil {
		return zvecdb.CollectionSchema{}, err
	}
	index := zvecdb.NewDiskANNIndexParams(metric)
	physicalName := db.tablePrefix + name
	schema := zvecdb.NewCollectionSchema(physicalName,
		zvecdb.FieldSchema{Name: zvecCategoriesField, DataType: zvecdb.DataTypeArrayString, Index: zvecdb.NewInvertIndexParams()},
		zvecdb.FieldSchema{Name: zvecTimestampField, DataType: zvecdb.DataTypeInt64, Index: zvecdb.NewInvertIndexParams()},
		zvecdb.NewField(zvecHiddenField, zvecdb.DataTypeBool),
		zvecdb.FieldSchema{Name: zvecVectorField, DataType: zvecdb.DataTypeVectorFP32, Dimension: uint32(dimensions), Index: index},
	)
	if err := schema.Validate(); err != nil {
		return zvecdb.CollectionSchema{}, errors.Trace(err)
	}
	return schema, nil
}

func (db *Zvec) DeleteCollection(ctx context.Context, name string) error {
	if db.closed.Load() {
		return errors.New("zvec database is closed")
	}
	value, found := db.collections.LoadAndDelete(name)
	if !found {
		return errors.NotFoundf("collection %s", name)
	}
	collection := value.(*zvecdb.Collection)
	if err := collection.Destroy(ctx); err != nil {
		if !db.closed.Load() {
			db.collections.LoadOrStore(name, collection)
		}
		return errors.Trace(err)
	}
	return nil
}

func (db *Zvec) CountVectors(ctx context.Context, name string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, errors.Trace(err)
	}
	collection, err := db.collection(name)
	if err != nil {
		return 0, err
	}
	count := collection.Stats().DocumentCount
	if count > math.MaxInt64 {
		return 0, errors.Errorf("zvec collection %s contains too many vectors", name)
	}
	return int64(count), nil
}

func (db *Zvec) AddVectors(ctx context.Context, name string, vectors []Vector) error {
	if len(vectors) == 0 {
		return nil
	}
	collection, err := db.collection(name)
	if err != nil {
		return err
	}
	schema := collection.Schema()
	documents := make([]zvecdb.Document, len(vectors))
	for i, vector := range vectors {
		document := zvecdb.Document{PrimaryKey: vector.Id, Fields: map[string]any{
			zvecVectorField:     zvecdb.VectorFP32(vector.Values),
			zvecCategoriesField: zvecdb.StringArray(vector.Categories),
			zvecTimestampField:  vector.Timestamp.UnixMilli(),
			zvecHiddenField:     vector.IsHidden,
		}}
		if err := document.Validate(schema); err != nil {
			return errors.Trace(err)
		}
		documents[i] = document
	}
	_, err = collection.Upsert(ctx, documents)
	return errors.Trace(err)
}

func (db *Zvec) DeleteVectors(ctx context.Context, name string, timestamp time.Time) error {
	collection, err := db.collection(name)
	if err != nil {
		return err
	}
	return errors.Trace(collection.DeleteByFilter(ctx, fmt.Sprintf("%s < %d", zvecTimestampField, timestamp.UnixMilli())))
}

func (db *Zvec) QueryVectors(ctx context.Context, name string, q Vector, categories []string, topK int) ([]ScoredVector, error) {
	if topK <= 0 {
		return []ScoredVector{}, nil
	}
	collection, err := db.collection(name)
	if err != nil {
		return nil, err
	}
	filter := fmt.Sprintf("%s = false", zvecHiddenField)
	if len(categories) > 0 {
		quoted := make([]string, len(categories))
		for i, category := range categories {
			escaped := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(category)
			quoted[i] = "'" + escaped + "'"
		}
		filter += fmt.Sprintf(" AND %s CONTAIN_ANY (%s)", zvecCategoriesField, strings.Join(quoted, ", "))
	}
	documents, err := collection.Query(ctx, zvecdb.VectorQuery{
		Field:       zvecVectorField,
		DenseVector: zvecdb.VectorFP32(q.Values),
		TopK:        topK,
		Filter:      filter,
		Projection: zvecdb.Projection{
			OutputFields:   []string{zvecCategoriesField, zvecTimestampField, zvecHiddenField},
			IncludeVectors: true,
		},
		Params: zvecdb.NewDiskANNQueryParams(),
	})
	if err != nil {
		return nil, errors.Trace(err)
	}
	info, err := db.DescribeCollection(ctx, name)
	if err != nil {
		return nil, err
	}
	results := make([]ScoredVector, len(documents))
	for i, document := range documents {
		result := ScoredVector{Vector: Vector{Id: document.PrimaryKey}, Score: document.Score}
		if info.Distance != Dot {
			result.Score = -result.Score
		}
		if value, found := document.Field(zvecCategoriesField); found {
			result.Categories = []string(value.(zvecdb.StringArray))
		}
		if value, found := document.Field(zvecTimestampField); found {
			result.Timestamp = time.UnixMilli(value.(int64))
		}
		if value, found := document.Field(zvecHiddenField); found {
			result.IsHidden = value.(bool)
		}
		if value, found := document.Field(zvecVectorField); found {
			result.Vector.Values = []float32(value.(zvecdb.VectorFP32))
		}
		results[i] = result
	}
	return results, nil
}

func (db *Zvec) collection(name string) (*zvecdb.Collection, error) {
	if db.closed.Load() {
		return nil, errors.New("zvec database is closed")
	}
	value, found := db.collections.Load(name)
	if !found {
		return nil, errors.NotFoundf("collection %s", name)
	}
	return value.(*zvecdb.Collection), nil
}

func distanceToZvec(distance Distance) (zvecdb.MetricType, error) {
	switch distance {
	case Cosine:
		return zvecdb.MetricTypeCosine, nil
	case Euclidean:
		return zvecdb.MetricTypeL2, nil
	case Dot:
		return zvecdb.MetricTypeIP, nil
	default:
		return zvecdb.MetricTypeUndefined, errors.NotSupportedf("distance method %v", distance)
	}
}

func zvecDistance(metric zvecdb.MetricType) (Distance, error) {
	switch metric {
	case zvecdb.MetricTypeCosine:
		return Cosine, nil
	case zvecdb.MetricTypeL2:
		return Euclidean, nil
	case zvecdb.MetricTypeIP:
		return Dot, nil
	default:
		return Cosine, errors.NotSupportedf("zvec metric %s", metric)
	}
}
