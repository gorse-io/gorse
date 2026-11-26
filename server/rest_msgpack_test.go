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
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
)

func unpackEntity(t *testing.T, body []byte) any {
	t.Helper()
	dec := msgpack.NewDecoder(bytes.NewReader(body))
	var value any
	require.NoError(t, dec.Decode(&value))
	_, err := dec.PeekCode()
	require.ErrorIs(t, err, io.EOF)
	return value
}

func readPackedEntity(body string, dst any) error {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	return msgpackAccessor(mimeMsgpack).Read(restful.NewRequest(req), dst)
}

func writePackedEntity(t *testing.T, value any) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	require.NoError(t, msgpackAccessor(mimeMsgpack).Write(restful.NewResponse(w), http.StatusOK, value))
	return unpackEntity(t, w.Body.Bytes()).(map[string]any)
}

func TestMsgpackFeedbackFields(t *testing.T) {
	const stamp = "2026-01-02T03:04:05.123456789-07:00"
	const fields = "\x84\xacFeedbackType\xa4like\xa6UserId\xa1u\xa6ItemId\xa1i\xa9Timestamp\xd9\x23" + stamp
	key := data.FeedbackKey{FeedbackType: "like", UserId: "u", ItemId: "i"}
	var incoming Feedback
	require.NoError(t, readPackedEntity(fields, &incoming))
	require.Equal(t, key, incoming.FeedbackKey)
	require.Equal(t, stamp, incoming.Timestamp)

	ts, err := time.Parse(time.RFC3339Nano, stamp)
	require.NoError(t, err)
	wire := writePackedEntity(t, data.Feedback{FeedbackKey: key, Timestamp: ts})
	require.Equal(t, "like", wire["FeedbackType"])
	require.Equal(t, "u", wire["UserId"])
	require.Equal(t, "i", wire["ItemId"])
	require.NotContains(t, wire, "FeedbackKey")
	require.Equal(t, stamp, wire["Timestamp"])
}

func TestMsgpackNumericLabels(t *testing.T) {
	// A map containing an array with fixint, int64, uint64, float32 and float64 values.
	const labels = "\x81\xa6Labels\x81\xa6vector\x95\x2a" +
		"\xd3\xff\xdf\xff\xff\xff\xff\xff\xff" + // -(2^53 + 1)
		"\xcf\xff\xff\xff\xff\xff\xff\xff\xff" + // max uint64
		"\xca\x3f\xc0\x00\x00\xcb\xc0\x02\x00\x00\x00\x00\x00\x00" // 1.5, -2.25
	var user data.User
	require.NoError(t, readPackedEntity(labels, &user))
	require.Equal(t, map[string]any{"vector": []any{json.Number("42"), json.Number("-9007199254740993"),
		json.Number("18446744073709551615"), json.Number("1.5"), json.Number("-2.25")}}, user.Labels)
	wire := writePackedEntity(t, user)
	require.NotContains(t, wire, "UpdateAt") // json:"-" on the storage DTO
	// JSONEq converts numbers to float64; compare digits and numeric (unquoted) forms instead.
	b, err := json.Marshal(wire["Labels"])
	require.NoError(t, err)
	require.Equal(t, `{"vector":[42,-9007199254740993,18446744073709551615,1.5,-2.25]}`, string(b))
}

func TestMsgpackTrailingData(t *testing.T) {
	for name, suffix := range map[string]string{"second_object": "\x80", "truncated_string": "\xd9", "reserved_byte": "\xc1"} {
		t.Run(name, func(t *testing.T) {
			var user data.User
			require.Error(t, readPackedEntity("\x81\xa6UserId\xa1u"+suffix, &user))
		})
	}
}

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
			payload, err = json.Marshal(unpackEntity(t, payload))
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
