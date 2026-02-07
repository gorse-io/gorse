package vectors

import (
	"context"
	"strings"

	"github.com/gorse-io/gorse/storage"
	"github.com/juju/errors"
)

type Database interface {
	Init() error
	Close() error
	ListCollections(ctx context.Context) ([]string, error)
	AddCollection(ctx context.Context, name string) error
	DeleteCollection(ctx context.Context, name string) error
}

// Creator creates a database instance.
type Creator func(path, tablePrefix string, opts ...storage.Option) (Database, error)

var creators = make(map[string]Creator)

// Register a database creator.
func Register(prefixes []string, creator Creator) {
	for _, p := range prefixes {
		creators[p] = creator
	}
}

// Open a connection to a database.
func Open(path, tablePrefix string, opts ...storage.Option) (Database, error) {
	for prefix, creator := range creators {
		if strings.HasPrefix(path, prefix) {
			return creator(path, tablePrefix, opts...)
		}
	}
	return nil, errors.Errorf("Unknown database: %s", path)
}
