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
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/config"
	"github.com/juju/errors"
	"go.uber.org/zap"
)

type AzureBlob struct {
	client    *azblob.Client
	container string
	prefix    string
}

func NewAzureBlob(cfg config.AzureBlobConfig, container string, prefix string) (*AzureBlob, error) {
	var (
		client *azblob.Client
		err    error
	)
	if cfg.ConnectionString != "" {
		client, err = azblob.NewClientFromConnectionString(cfg.ConnectionString, nil)
		if err != nil {
			return nil, errors.Trace(err)
		}
	} else {
		if cfg.AccountName == "" || cfg.AccountKey == "" {
			return nil, errors.New("azure blob requires account_name and account_key or connection_string")
		}
		endpoint := cfg.Endpoint
		if endpoint == "" {
			endpoint = fmt.Sprintf("https://%s.blob.core.windows.net/", cfg.AccountName)
		}
		cred, err := azblob.NewSharedKeyCredential(cfg.AccountName, cfg.AccountKey)
		if err != nil {
			return nil, errors.Trace(err)
		}
		client, err = azblob.NewClientWithSharedKeyCredential(endpoint, cred, nil)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return &AzureBlob{
		client:    client,
		container: container,
		prefix:    strings.TrimPrefix(prefix, "/"),
	}, nil
}

func (a *AzureBlob) Open(name string) (io.ReadCloser, error) {
	fullPath := filepath.Join(a.prefix, name)
	resp, err := a.client.DownloadStream(context.Background(), a.container, fullPath, nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (a *AzureBlob) Create(name string) (io.WriteCloser, chan struct{}, error) {
	fullPath := filepath.Join(a.prefix, name)
	pr, pw := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := a.client.UploadStream(context.Background(), a.container, fullPath, pr, nil)
		if err != nil {
			log.Logger().Error("failed to upload file to Azure Blob", zap.String("file", fullPath), zap.Error(err))
		}
	}()
	return pw, done, nil
}

func (a *AzureBlob) List() ([]string, error) {
	var (
		prefix *string
		names  []string
	)
	if a.prefix != "" {
		prefix = &a.prefix
	}
	pager := a.client.NewListBlobsFlatPager(a.container, &azblob.ListBlobsFlatOptions{Prefix: prefix})
	for pager.More() {
		resp, err := pager.NextPage(context.Background())
		if err != nil {
			return nil, err
		}
		for _, item := range resp.Segment.BlobItems {
			name := ""
			if item.Name != nil {
				name = *item.Name
			}
			if a.prefix != "" {
				name = strings.TrimPrefix(name, a.prefix)
				if len(name) > 0 && name[0] == '/' {
					name = name[1:]
				}
			}
			if name != "" {
				names = append(names, name)
			}
		}
	}
	return names, nil
}

func (a *AzureBlob) Remove(name string) error {
	fullPath := filepath.Join(a.prefix, name)
	_, err := a.client.DeleteBlob(context.Background(), a.container, fullPath, nil)
	if err != nil {
		log.Logger().Error("failed to remove file from Azure Blob", zap.String("file", fullPath), zap.Error(err))
		return err
	}
	return nil
}
