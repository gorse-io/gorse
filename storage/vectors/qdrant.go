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

func (db *Qdrant) DescribeCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	info, err := db.client.GetCollectionInfo(ctx, name)
	if err != nil {
		return nil, errors.Trace(err)
	}
	params := info.GetConfig().GetParams().GetVectorsConfig().GetParams()
	if params == nil {
		paramsMap := info.GetConfig().GetParams().GetVectorsConfig().GetParamsMap()
		if paramsMap == nil || len(paramsMap.GetMap()) == 0 {
			return nil, errors.NotFoundf("vector params for collection %s", name)
		}
		for _, vectorParams := range paramsMap.GetMap() {
			params = vectorParams
			break
		}
	}
	distance, err := qdrantDistanceToDistance(params.GetDistance())
	if err != nil {
		return nil, errors.Trace(err)
	}
	quantizationConfig := params.GetQuantizationConfig()
	if quantizationConfig == nil {
		quantizationConfig = info.GetConfig().GetQuantizationConfig()
	}
	config, err := qdrantVectorConfig(quantizationConfig)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &CollectionInfo{
		Name:         name,
		Dimension:    int(params.GetSize()),
		Distance:     distance,
		VectorConfig: config,
	}, nil
}

func (db *Qdrant) AddCollection(ctx context.Context, name string, dimensions int, distance Distance, config VectorConfig) error {
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

	quantizationConfig, err := qdrantQuantizationConfig(config)
	if err != nil {
		return errors.Trace(err)
	}

	err = db.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     uint64(dimensions),
			Distance: qdrantDistance,
		}),
		QuantizationConfig: quantizationConfig,
		HnswConfig: &qdrant.HnswConfigDiff{
			M:           ptrUint64(defaultHNSWM),
			EfConstruct: ptrUint64(defaultHNSWEfConstruct),
		},
	})
	if err != nil {
		return errors.Trace(err)
	}

	_, err = db.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
		CollectionName: name,
		Wait:           ptrBool(true),
		FieldName:      qdrantPayloadTimestampKey,
		FieldType:      qdrant.FieldType_FieldTypeInteger.Enum(),
	})
	return errors.Trace(err)
}

func qdrantQuantizationConfig(config VectorConfig) (*qdrant.QuantizationConfig, error) {
	switch config.Quantization {
	case QuantizationNone, "":
		return nil, nil
	case QuantizationRQ:
		turbo := &qdrant.TurboQuantization{}
		if config.QuantizationBits != 0 {
			bits, err := qdrantTurboQuantBits(config.QuantizationBits)
			if err != nil {
				return nil, errors.Trace(err)
			}
			turbo.Bits = bits.Enum()
		}
		return qdrant.NewQuantizationTurbo(turbo), nil
	case QuantizationSQ:
		return nil, errors.NotSupportedf("SQ quantization for Qdrant")
	case QuantizationPQ:
		return nil, errors.NotSupportedf("PQ quantization for Qdrant")
	default:
		return nil, errors.NotSupportedf("quantization type %s for Qdrant", config.Quantization)
	}
}

func qdrantVectorConfig(config *qdrant.QuantizationConfig) (VectorConfig, error) {
	if config == nil {
		return DefaultVectorConfig(), nil
	}
	if turbo := config.GetTurboquant(); turbo != nil {
		bits := 0
		if turbo.Bits != nil {
			var err error
			bits, err = qdrantTurboQuantBitSizeToBits(turbo.GetBits())
			if err != nil {
				return VectorConfig{}, errors.Trace(err)
			}
		}
		return VectorConfig{
			Quantization:     QuantizationRQ,
			QuantizationBits: bits,
		}, nil
	}
	if config.GetScalar() != nil {
		return VectorConfig{}, errors.NotSupportedf("SQ quantization for Qdrant")
	}
	if config.GetProduct() != nil {
		return VectorConfig{}, errors.NotSupportedf("PQ quantization for Qdrant")
	}
	if config.GetBinary() != nil {
		return VectorConfig{}, errors.NotSupportedf("binary quantization for Qdrant")
	}
	return DefaultVectorConfig(), nil
}

func qdrantTurboQuantBits(bits int) (qdrant.TurboQuantBitSize, error) {
	switch bits {
	case 1:
		return qdrant.TurboQuantBitSize_Bits1, nil
	case 2:
		return qdrant.TurboQuantBitSize_Bits2, nil
	case 4:
		return qdrant.TurboQuantBitSize_Bits4, nil
	default:
		return 0, errors.NotSupportedf("RQ quantization bits %d for Qdrant", bits)
	}
}

func qdrantTurboQuantBitSizeToBits(bits qdrant.TurboQuantBitSize) (int, error) {
	switch bits {
	case qdrant.TurboQuantBitSize_Bits1:
		return 1, nil
	case qdrant.TurboQuantBitSize_Bits2:
		return 2, nil
	case qdrant.TurboQuantBitSize_Bits4:
		return 4, nil
	default:
		return 0, errors.NotSupportedf("RQ quantization bits %s for Qdrant", bits.String())
	}
}

func qdrantDistanceToDistance(distance qdrant.Distance) (Distance, error) {
	switch distance {
	case qdrant.Distance_Cosine:
		return Cosine, nil
	case qdrant.Distance_Euclid:
		return Euclidean, nil
	case qdrant.Distance_Dot:
		return Dot, nil
	default:
		return Cosine, errors.NotSupportedf("distance method %s", distance.String())
	}
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
		Limit:          ptrUint64(uint64(topK)),
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

// Helper functions for pointer types
func ptrBool(v bool) *bool {
	return &v
}

func ptrUint64(v uint64) *uint64 {
	return &v
}
