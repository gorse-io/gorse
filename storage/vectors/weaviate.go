package vectors

import (
	"context"
	"net/url"
	"strings"

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

func (db *Weaviate) AddCollection(ctx context.Context, name string, dimensions int, distance Distance) error {
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
		},
		VectorIndexConfig: map[string]interface{}{
			"distance": weaviateDistance,
		},
	}
	err := db.client.Schema().ClassCreator().WithClass(class).Do(ctx)
	return errors.Trace(err)
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
			Properties: map[string]interface{}{
				"originalId":                 vector.Id,
				weaviatePayloadCategoriesKey: vector.Categories,
			},
			Vector: models.C11yVector(vector.Vector),
		})
	}
	_, err := db.client.Batch().ObjectsBatcher().WithObjects(objects...).Do(ctx)
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

	data := result.Data["Get"].(map[string]interface{})
	items := data[capitalize(collection)].([]interface{})
	results := make([]Vector, 0, len(items))
	for _, item := range items {
		m := item.(map[string]interface{})
		id := m["originalId"].(string)
		var cats []string
		if m[weaviatePayloadCategoriesKey] != nil {
			for _, c := range m[weaviatePayloadCategoriesKey].([]interface{}) {
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
