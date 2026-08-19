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
	"time"

	"github.com/google/uuid"
	"github.com/gorse-io/gorse/storage"
	"github.com/pkg/errors"
	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	qdrantPayloadCategoriesKey = "categories"
	qdrantPayloadHiddenKey     = "hidden"
	qdrantPayloadIdKey         = "id"
	qdrantPayloadTimestampKey  = "timestamp"
	qdrantVectorName           = "vector"
)

func init() {
	Register([]string{storage.QdrantPrefix}, func(path, tablePrefix string, opts ...storage.Option) (Database, error) {
		database := new(Qdrant)
		u, err := url.Parse(path)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		host := u.Hostname()
		port := u.Port()
		portInt, err := strconv.Atoi(port)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		database.client, err = qdrant.NewClient(&qdrant.Config{
			Host: host,
			Port: portInt,
		})
		if err != nil {
			return nil, errors.WithStack(err)
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

func (db *Qdrant) Optimize(_ context.Context, _ string) error {
	return nil
}

func (db *Qdrant) Close() error {
	return db.client.Close()
}

func (db *Qdrant) ListCollections(ctx context.Context) ([]string, error) {
	return db.client.ListCollections(ctx)
}

func (db *Qdrant) DescribeCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	info, err := db.client.GetCollectionInfo(ctx, name)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, fmt.Errorf("collection %s: %w", name, storage.ErrNotFound)
		}
		return nil, errors.WithStack(err)
	}
	collectionParams := info.GetConfig().GetParams()
	if _, ok := collectionParams.GetSparseVectorsConfig().GetMap()[qdrantVectorName]; ok {
		return &CollectionInfo{Name: name, Dimension: 0, Distance: Dot}, nil
	}
	params, ok := collectionParams.GetVectorsConfig().GetParamsMap().GetMap()[qdrantVectorName]
	if !ok {
		return nil, fmt.Errorf("vector field %s: %w", qdrantVectorName, storage.ErrNotFound)
	}
	var distance Distance
	switch params.GetDistance() {
	case qdrant.Distance_Cosine:
		distance = Cosine
	case qdrant.Distance_Euclid:
		distance = Euclidean
	case qdrant.Distance_Dot:
		distance = Dot
	default:
		return nil, fmt.Errorf("distance method %s %w", params.GetDistance().String(), storage.ErrNotSupported)
	}
	quantizationConfig := info.GetConfig().GetQuantizationConfig()
	config, err := qdrantVectorConfig(quantizationConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &CollectionInfo{
		Name:         name,
		Dimension:    int(params.GetSize()),
		Distance:     distance,
		VectorConfig: config,
	}, nil
}

func (db *Qdrant) AddCollection(ctx context.Context, name string, dimensions int, distance Distance, config VectorConfig) error {
	if dimensions == 0 {
		if distance != Dot {
			return fmt.Errorf("distance method for sparse vector %w", storage.ErrNotSupported)
		}
		if config != (VectorConfig{}) {
			return fmt.Errorf("quantization for sparse vector %w", storage.ErrNotSupported)
		}
		return db.createCollection(ctx, name, &qdrant.CreateCollection{
			CollectionName: name,
			SparseVectorsConfig: qdrant.NewSparseVectorsConfig(map[string]*qdrant.SparseVectorParams{
				qdrantVectorName: {},
			}),
		})
	}
	var qdrantDistance qdrant.Distance
	switch distance {
	case Cosine:
		qdrantDistance = qdrant.Distance_Cosine
	case Euclidean:
		qdrantDistance = qdrant.Distance_Euclid
	case Dot:
		qdrantDistance = qdrant.Distance_Dot
	default:
		return fmt.Errorf("distance method %w", storage.ErrNotSupported)
	}

	quantizationConfig, err := qdrantQuantizationConfig(config)
	if err != nil {
		return errors.WithStack(err)
	}

	return db.createCollection(ctx, name, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
			qdrantVectorName: {
				Size:     uint64(dimensions),
				Distance: qdrantDistance,
			},
		}),
		QuantizationConfig: quantizationConfig,
	})
}

func (db *Qdrant) createCollection(ctx context.Context, name string, request *qdrant.CreateCollection) error {
	err := db.client.CreateCollection(ctx, request)
	if err != nil {
		return errors.WithStack(err)
	}

	_, err = db.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
		CollectionName: name,
		Wait:           new(true),
		FieldName:      qdrantPayloadTimestampKey,
		FieldType:      qdrant.FieldType_FieldTypeInteger.Enum(),
	})
	if err != nil {
		return errors.WithStack(err)
	}
	_, err = db.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
		CollectionName: name,
		Wait:           new(true),
		FieldName:      qdrantPayloadHiddenKey,
		FieldType:      qdrant.FieldType_FieldTypeBool.Enum(),
	})
	return errors.WithStack(err)
}

func qdrantQuantizationConfig(config VectorConfig) (*qdrant.QuantizationConfig, error) {
	switch config.Type {
	case QuantizationNone:
		return nil, nil
	case QuantizationRQ:
		turbo := &qdrant.TurboQuantization{}
		if config.Bits != 0 {
			switch config.Bits {
			case 1:
				turbo.Bits = qdrant.TurboQuantBitSize_Bits1.Enum()
			case 2:
				turbo.Bits = qdrant.TurboQuantBitSize_Bits2.Enum()
			case 4:
				turbo.Bits = qdrant.TurboQuantBitSize_Bits4.Enum()
			default:
				return nil, fmt.Errorf("RQ quantization bits %d for Qdrant %w", config.Bits, storage.ErrNotSupported)
			}
		}
		return qdrant.NewQuantizationTurbo(turbo), nil
	case QuantizationSQ:
		if config.Bits != 0 && config.Bits != 8 {
			return nil, fmt.Errorf("SQ quantization bits for Qdrant %w", storage.ErrNotSupported)
		}
		return qdrant.NewQuantizationScalar(&qdrant.ScalarQuantization{
			Type: qdrant.QuantizationType_Int8,
		}), nil
	case QuantizationPQ:
		product := &qdrant.ProductQuantization{}
		if config.Bits != 0 {
			switch config.Bits {
			case 8:
				product.Compression = qdrant.CompressionRatio_x4
			case 4:
				product.Compression = qdrant.CompressionRatio_x8
			case 2:
				product.Compression = qdrant.CompressionRatio_x16
			case 1:
				product.Compression = qdrant.CompressionRatio_x32
			default:
				return nil, fmt.Errorf("PQ quantization bits %d for Qdrant %w", config.Bits, storage.ErrNotSupported)
			}
		}
		return qdrant.NewQuantizationProduct(product), nil
	default:
		return nil, fmt.Errorf("quantization type %s for Qdrant %w", config.Type, storage.ErrNotSupported)
	}
}

func qdrantVectorConfig(config *qdrant.QuantizationConfig) (VectorConfig, error) {
	if config == nil {
		return VectorConfig{}, nil
	}
	if turbo := config.GetTurboquant(); turbo != nil {
		bits := 0
		if turbo.Bits != nil {
			switch turbo.GetBits() {
			case qdrant.TurboQuantBitSize_Bits1:
				bits = 1
			case qdrant.TurboQuantBitSize_Bits2:
				bits = 2
			case qdrant.TurboQuantBitSize_Bits4:
				bits = 4
			default:
				return VectorConfig{}, fmt.Errorf("RQ quantization bits %s for Qdrant %w", turbo.GetBits().String(), storage.ErrNotSupported)
			}
		}
		return VectorConfig{
			Type: QuantizationRQ,
			Bits: bits,
		}, nil
	}
	if scalar := config.GetScalar(); scalar != nil {
		if scalar.GetType() != qdrant.QuantizationType_Int8 {
			return VectorConfig{}, fmt.Errorf("SQ quantization type %s for Qdrant %w", scalar.GetType().String(), storage.ErrNotSupported)
		}
		return VectorConfig{
			Type: QuantizationSQ,
			Bits: 8,
		}, nil
	}
	if product := config.GetProduct(); product != nil {
		var bits int
		switch product.GetCompression() {
		case qdrant.CompressionRatio_x4:
			bits = 8
		case qdrant.CompressionRatio_x8:
			bits = 4
		case qdrant.CompressionRatio_x16:
			bits = 2
		case qdrant.CompressionRatio_x32:
			bits = 1
		default:
			return VectorConfig{}, fmt.Errorf("PQ quantization compression %s for Qdrant %w", product.GetCompression().String(), storage.ErrNotSupported)
		}
		return VectorConfig{
			Type: QuantizationPQ,
			Bits: bits,
		}, nil
	}
	if config.GetBinary() != nil {
		return VectorConfig{}, fmt.Errorf("binary quantization for Qdrant %w", storage.ErrNotSupported)
	}
	return VectorConfig{}, nil
}

func (db *Qdrant) DeleteCollection(ctx context.Context, name string) error {
	return db.client.DeleteCollection(ctx, name)
}

func (db *Qdrant) CountVectors(ctx context.Context, collection string) (int64, error) {
	count, err := db.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: collection,
		Exact:          new(true),
	})
	return int64(count), errors.WithStack(err)
}

func (db *Qdrant) AddVectors(ctx context.Context, collection string, vectors []Vector) error {
	if len(vectors) == 0 {
		return nil
	}
	points := make([]*qdrant.PointStruct, 0, len(vectors))
	for _, vector := range vectors {
		var value *qdrant.Vector
		if len(vector.Indices) > 0 {
			value = qdrant.NewVectorSparse(vector.Indices, vector.Values)
		} else {
			value = qdrant.NewVectorDense(vector.Values)
		}
		values := qdrant.NewVectorsMap(map[string]*qdrant.Vector{qdrantVectorName: value})
		points = append(points, &qdrant.PointStruct{
			Id: qdrant.NewID(uuid.NewMD5(uuid.NameSpaceURL, []byte(vector.Id)).String()),
			Payload: map[string]*qdrant.Value{
				qdrantPayloadCategoriesKey: qdrantListValue(vector.Categories),
				qdrantPayloadHiddenKey:     qdrant.NewValueBool(vector.IsHidden),
				qdrantPayloadIdKey:         qdrant.NewValueString(vector.Id),
				qdrantPayloadTimestampKey:  qdrant.NewValueInt(vector.Timestamp.UnixMilli()),
			},
			Vectors: values,
		})
	}
	_, err := db.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Wait:           new(true),
		Points:         points,
	})
	return errors.WithStack(err)
}

func (db *Qdrant) GetVectors(ctx context.Context, collection string, ids []string) ([]Vector, error) {
	if len(ids) == 0 {
		return []Vector{}, nil
	}
	pointIDs := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		pointIDs[i] = qdrant.NewID(uuid.NewMD5(uuid.NameSpaceURL, []byte(id)).String())
	}
	points, err := db.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: collection,
		Ids:            pointIDs,
		WithPayload:    qdrant.NewWithPayloadEnable(true),
		WithVectors:    qdrant.NewWithVectorsInclude(qdrantVectorName),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	vectors := make([]Vector, 0, len(points))
	for _, point := range points {
		value := qdrantVector(point.GetVectors())
		vectors = append(vectors, Vector{
			Id:         qdrantId(point.GetPayload()),
			Values:     value.Values,
			Indices:    value.Indices,
			IsHidden:   qdrantHidden(point.GetPayload()),
			Categories: qdrantCategories(point.GetPayload()),
			Timestamp:  qdrantTimestamp(point.GetPayload()),
		})
	}
	return orderVectors(ids, vectors), nil
}

func (db *Qdrant) DeleteVectors(ctx context.Context, collection string, timestamp time.Time) error {
	lt := float64(timestamp.UnixMilli())
	_, err := db.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collection,
		Wait:           new(true),
		Points: qdrant.NewPointsSelectorFilter(&qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewRange(qdrantPayloadTimestampKey, &qdrant.Range{Lt: &lt}),
			},
		}),
	})
	return errors.WithStack(err)
}

func (db *Qdrant) QueryVectors(ctx context.Context, collection string, q Vector, categories []string, topK int) ([]ScoredVector, error) {
	if topK <= 0 {
		return []ScoredVector{}, nil
	}
	request := &qdrant.QueryPoints{
		CollectionName: collection,
		Limit:          new(uint64(topK)),
		WithPayload:    qdrant.NewWithPayloadEnable(true),
		WithVectors:    qdrant.NewWithVectorsEnable(true),
		Filter: &qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewMatchBool(qdrantPayloadHiddenKey, false),
		}},
	}
	if len(q.Indices) > 0 {
		request.Query = qdrant.NewQuerySparse(q.Indices, q.Values)
	} else {
		request.Query = qdrant.NewQueryDense(q.Values)
	}
	request.Using = new(qdrantVectorName)
	request.WithVectors = qdrant.NewWithVectorsInclude(qdrantVectorName)
	if len(categories) > 0 {
		request.Filter.Must = append(request.Filter.Must,
			qdrant.NewMatchKeywords(qdrantPayloadCategoriesKey, categories...))
	}
	response, err := db.client.Query(ctx, request)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	results := make([]ScoredVector, 0, len(response))
	for _, scored := range response {
		vector := qdrantVector(scored.GetVectors())
		results = append(results, ScoredVector{
			Vector: Vector{
				Id:         qdrantId(scored.GetPayload()),
				Values:     vector.Values,
				Indices:    vector.Indices,
				IsHidden:   qdrantHidden(scored.GetPayload()),
				Categories: qdrantCategories(scored.GetPayload()),
			},
			Score: scored.GetScore(),
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

func qdrantHidden(payload map[string]*qdrant.Value) bool {
	if value, ok := payload[qdrantPayloadHiddenKey]; ok {
		return value.GetBoolValue()
	}
	return false
}

func qdrantTimestamp(payload map[string]*qdrant.Value) time.Time {
	if value, ok := payload[qdrantPayloadTimestampKey]; ok {
		return time.UnixMilli(value.GetIntegerValue()).UTC()
	}
	return time.Time{}
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

func qdrantVector(output *qdrant.VectorsOutput) Vector {
	if output != nil {
		if named := output.GetVectors(); named != nil {
			if vector := named.GetVectors()[qdrantVectorName]; vector != nil {
				if sparse := vector.GetSparseVector(); sparse != nil {
					return Vector{Indices: sparse.GetIndices(), Values: sparse.GetValues()}
				}
				if dense := vector.GetDenseVector(); dense != nil {
					return Vector{Values: dense.GetData()}
				}
			}
		}
	}
	return Vector{}
}
