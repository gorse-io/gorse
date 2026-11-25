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
package server

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tinylib/msgp/msgp"
)

func (suite *ServerTestSuite) TestRESTWireFormats() {
	server := httptest.NewServer(suite.handler)
	defer server.Close()
	const jsonType, packType = "application/json", "application/msgpack"
	// Hand-encoded fixmap and fixstr fields, independent of the server codec.
	const packedUser = "\x82\xa6UserId\xa4wire\xa6Labels\x91\xa3tag"
	const jsonUser = `{"UserId":"wire","Labels":["tag"]}`

	request := func(t *testing.T, method, path, contentType, accept, body string, status int) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), method, server.URL+path, strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("X-API-Key", apiKey)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		resp, err := server.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		payload, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, status, resp.StatusCode, "%s %s: %q", method, path, payload)
		return resp, payload
	}
	check := func(t *testing.T, resp *http.Response, payload []byte, wantType, wantJSON string) {
		t.Helper()
		media, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		require.NoError(t, err)
		require.Equal(t, wantType, media)
		if wantType == packType {
			value, rest, err := msgp.ReadIntfBytes(payload)
			require.NoError(t, err)
			require.Empty(t, rest, "unexpected trailing MessagePack bytes")
			payload, err = json.Marshal(value)
			require.NoError(t, err)
		}
		require.JSONEq(t, wantJSON, string(payload))
	}
	for _, tc := range []struct{ name, contentType, accept, body, wantType string }{
		{"json_default", jsonType, "", jsonUser, jsonType},
		{"msgpack_default", packType, "", packedUser, jsonType},
		{"json_to_msgpack", jsonType, packType, jsonUser, packType},
		{"msgpack_to_json", packType, jsonType, packedUser, jsonType},
		{"msgpack_to_msgpack", packType, packType, packedUser, packType},
	} {
		suite.T().Run(tc.name, func(t *testing.T) {
			resp, body := request(t, http.MethodPost, "/api/user", tc.contentType, tc.accept, tc.body, http.StatusOK)
			check(t, resp, body, tc.wantType, `{"RowAffected":1}`)
			resp, body = request(t, http.MethodGet, "/api/user/wire", "", tc.accept, "", http.StatusOK)
			check(t, resp, body, tc.wantType, `{"UserId":"wire","Labels":["tag"],"Comment":""}`)
			// Don't let a previous successful insertion mask a broken decoder.
			request(t, http.MethodDelete, "/api/user/wire", "", "", "", http.StatusOK)
		})
	}
	suite.T().Run("truncated_msgpack", func(t *testing.T) {
		request(t, http.MethodPost, "/api/user", packType, jsonType, packedUser[:len(packedUser)-1], http.StatusBadRequest)
	})
}
