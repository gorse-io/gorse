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
	"path/filepath"

	"github.com/gorse-io/gorse/base/log"
	"github.com/gorse-io/gorse/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

type S3 struct {
	*minio.Client
	bucket string
	prefix string
}

func NewS3(cfg config.S3Config) (*S3, error) {
	minioClient, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
	})
	if err != nil {
		return nil, err
	}
	return &S3{
		Client: minioClient,
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
	}, nil
}

// Open a file in S3 for reading. This function returns an io.Reader that can be used to read the file's content.
func (s *S3) Open(name string) (io.ReadCloser, error) {
	object, err := s.Client.GetObject(context.Background(), s.bucket, filepath.Join(s.prefix, name), minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return object, nil
}

// Create a new file in S3 for writing. This function returns an io.WriteCloser that can be used to write data to the
// file. It also returns a done channel that is closed when the writing is complete.
func (s *S3) Create(name string) (io.WriteCloser, chan struct{}, error) {
	fullPath := filepath.Join(s.prefix, name)
	pr, pw := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := s.Client.PutObject(context.Background(), s.bucket, fullPath, pr, -1, minio.PutObjectOptions{})
		if err != nil {
			log.Logger().Error("failed to upload file to S3", zap.String("file", fullPath), zap.Error(err))
		}
	}()
	return pw, done, nil
}
