package vectors

import (
	"context"
	"net/url"
	"strconv"

	"github.com/gorse-io/gorse/storage"
	"github.com/juju/errors"
	"github.com/qdrant/go-client/qdrant"
)

const (
	defaultVectorSize = 100
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

func (db *Qdrant) Close() error {
	return db.client.Close()
}

func (db *Qdrant) ListCollections(ctx context.Context) ([]string, error) {
	collections, err := db.client.ListCollections(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var names []string
	for _, collection := range collections {
		names = append(names, collection)
	}
	return names, nil
}

func (db *Qdrant) AddCollection(ctx context.Context, name string) error {
	err := db.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     defaultVectorSize,
			Distance: qdrant.Distance_Cosine,
		}),
	})
	return errors.Trace(err)
}

func (db *Qdrant) DeleteCollection(ctx context.Context, name string) error {
	return db.client.DeleteCollection(ctx, name)
}
