package vector

import (
	"time"

	"github.com/gorse-io/gorse/storage"
)

type Vector struct {
	Id         string
	Value      []float32
	IsHidden   bool      `json:"-"`
	Categories []string  `json:"-" gorm:"type:text;serializer:json"`
	Timestamp  time.Time `json:"-"`
}

type Database interface {
	Close() error
	List() ([]string, error)
	Create(name string, dimension int) error
	Drop(name string) error

	Add(name string, vectors []Vector) error
	Search(name string, query []float32, topK int, filter map[string]any) ([]Vector, []float32, error)
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
