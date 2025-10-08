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

package logics

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"modernc.org/quickjs"
)

func TestEnv(t *testing.T) {
	t.Setenv("TEST_ENV", "test_value")

	external, err := NewExternal()
	assert.NoError(t, err)
	defer external.Close()

	value, err := external.vm.Eval(`env.TEST_ENV`, quickjs.EvalGlobal)
	assert.NoError(t, err)
	assert.Equal(t, "test_value", value)
}

func TestFetch(t *testing.T) {
	var (
		req  *http.Request
		body string
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req = r
		b, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		body = string(b)
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	external, err := NewExternal()
	assert.NoError(t, err)
	defer external.Close()

	response, err := external.vm.Eval(`fetch("`+ts.URL+`")`, quickjs.EvalGlobal)
	assert.NoError(t, err)
	if assert.NotNil(t, req) {
		assert.Equal(t, "GET", req.Method)
	}
	if assert.IsType(t, &quickjs.Object{}, response) {
		var resp map[string]any
		err = json.Unmarshal([]byte(response.(*quickjs.Object).String()), &resp)
		assert.NoError(t, err)
		assert.Equal(t, true, resp["ok"])
		assert.Equal(t, float64(200), resp["status"])
		assert.Equal(t, "200 OK", resp["statusText"])
		assert.Equal(t, "Hello, client\n", resp["body"])
		headers := resp["headers"].(map[string]any)
		assert.Contains(t, headers, "Content-Length")
		assert.Contains(t, headers, "Date")
	}

	_, err = external.vm.Eval(`fetch({method: "POST", url: "`+ts.URL+`"})`, quickjs.EvalGlobal)
	assert.NoError(t, err)
	if assert.NotNil(t, req) {
		assert.Equal(t, "POST", req.Method)
	}

	_, err = external.vm.Eval(`fetch("`+ts.URL+`", {
	method: "PUT",
	headers: {
		"Content-Type": "application/json"
	},
	body: JSON.stringify({message: "Hello, server"})
	})`, quickjs.EvalGlobal)
	assert.NoError(t, err)
	if assert.NotNil(t, req) {
		assert.Equal(t, "PUT", req.Method)
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		assert.Equal(t, `{"message":"Hello, server"}`, body)
	}
}
