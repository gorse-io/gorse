// Copyright 2026 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"encoding/json/v2"

	"github.com/emicklei/go-restful/v3"
	"github.com/gorse-io/gorse/common/jsonutil"
)

type jsonEntity struct{}

func init() {
	restful.RegisterEntityAccessor(restful.MIME_JSON, jsonEntity{})
}

func (jsonEntity) Read(req *restful.Request, v any) error {
	return json.UnmarshalRead(req.Request.Body, v, jsonutil.ExactNumbers, json.MatchCaseInsensitiveNames(true))
}

func (jsonEntity) Write(resp *restful.Response, status int, v any) error {
	if v == nil {
		resp.WriteHeader(status)
		return nil
	}
	resp.Header().Set("Content-Type", restful.MIME_JSON)
	resp.WriteHeader(status)
	return json.MarshalWrite(resp, v, jsonutil.DurationAsNano)
}
