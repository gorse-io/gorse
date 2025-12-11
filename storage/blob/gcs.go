// Copyright 2025 gorse Project Authors
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

package blob

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	"github.com/gorse-io/gorse/config"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type GCS struct {
	client *storage.Client
	bucket string
	prefix string
}

func NewGCS(cfg config.GCSConfig) (*GCS, error) {
	var opts []option.ClientOption
	if os.Getenv("GCS_EMULATOR_EDNPOINT") != "" {
		opts = append(opts, option.WithEndpoint(os.Getenv("GCS_EMULATOR_EDNPOINT")))
		opts = append(opts, option.WithoutAuthentication())
	}
	if cfg.CredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(cfg.CredentialsFile))
	}
	client, err := storage.NewClient(context.Background(), opts...)
	if err != nil {
		return nil, err
	}
	return &GCS{
		client: client,
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
	}, nil
}

func (g *GCS) Open(name string) (io.ReadCloser, error) {
	path := filepath.Join(g.prefix, name)
	return g.client.Bucket(g.bucket).Object(path).NewReader(context.Background())
}

func (g *GCS) Create(name string) (io.WriteCloser, chan struct{}, error) {
	path := filepath.Join(g.prefix, name)
	wc := g.client.Bucket(g.bucket).Object(path).NewWriter(context.Background())
	done := make(chan struct{})
	return &gcsWriter{wc, done}, done, nil
}

type gcsWriter struct {
	*storage.Writer
	done chan struct{}
}

func (w *gcsWriter) Close() error {
	err := w.Writer.Close()
	close(w.done)
	return err
}

func (g *GCS) List() ([]string, error) {
	var names []string
	it := g.client.Bucket(g.bucket).Objects(context.Background(), &storage.Query{
		Prefix: g.prefix,
	})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		name := attrs.Name[len(g.prefix):]
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
		names = append(names, name)
	}
	return names, nil
}

func (g *GCS) Remove(name string) error {
	path := filepath.Join(g.prefix, name)
	return g.client.Bucket(g.bucket).Object(path).Delete(context.Background())
}
