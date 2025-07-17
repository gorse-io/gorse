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
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/zhenghaoz/gorse/config"
	"path/filepath"
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

func (s *S3) UploadBlob(name, path string) error {
	_, err := s.Client.FPutObject(context.Background(), s.bucket, filepath.Join(s.prefix, name), path, minio.PutObjectOptions{})
	return err
}

func (s *S3) DownloadBlob(name, path string) error {
	err := s.Client.FGetObject(context.Background(), s.bucket, filepath.Join(s.prefix, name), path, minio.GetObjectOptions{})
	return err
}
