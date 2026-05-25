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

package main

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/gorse-io/gorse/master"
)

// AdminClient is a client for the Gorse admin API.
type AdminClient struct {
	client *resty.Client
}

// NewAdminClient creates a new client for the Gorse admin API.
func NewAdminClient(endpoint, apiKey string) *AdminClient {
	client := resty.New()
	client.SetBaseURL(strings.TrimRight(endpoint, "/") + "/api")
	client.SetHeader("X-Api-Key", apiKey)
	return &AdminClient{client: client}
}

func (c *AdminClient) Get(path string, query url.Values) ([]byte, error) {
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	resp, err := c.client.R().Get(path)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.IsError() {
		return nil, newAdminAPIError(resp)
	}
	return resp.Body(), nil
}

func (c *AdminClient) PostJSON(path string, body any) ([]byte, error) {
	resp, err := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(path)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.IsError() {
		return nil, newAdminAPIError(resp)
	}
	return resp.Body(), nil
}

func (c *AdminClient) Restore(path, contentType string, reader io.Reader) (*master.DumpStats, error) {
	stats := new(master.DumpStats)
	resp, err := c.client.R().
		SetHeader("Content-Type", contentType).
		SetBody(reader).
		SetResult(stats).
		Post(path)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.IsError() {
		return nil, newAdminAPIError(resp)
	}
	return stats, nil
}

func (c *AdminClient) Delete(path string) ([]byte, error) {
	resp, err := c.client.R().Delete(path)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.IsError() {
		return nil, newAdminAPIError(resp)
	}
	return resp.Body(), nil
}

func (c *AdminClient) Download(path string, output io.Writer) error {
	resp, err := c.client.R().
		SetDoNotParseResponse(true).
		Get(path)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.RawBody().Close()
	if resp.IsError() {
		body, _ := io.ReadAll(resp.RawBody())
		return fmt.Errorf("API request failed: status=%d body=%s", resp.StatusCode(), string(body))
	}
	if _, err = io.Copy(output, resp.RawBody()); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	return nil
}

func newAdminAPIError(resp *resty.Response) error {
	return fmt.Errorf("API request failed: status=%d body=%s", resp.StatusCode(), resp.String())
}
