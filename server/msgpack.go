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
	"errors"
	"io"
	"strconv"

	"github.com/emicklei/go-restful/v3"
	"github.com/tinylib/msgp/msgp"
)

const (
	mimeMsgpack  = "application/msgpack"
	mimeXMsgpack = "application/x-msgpack"
)

type msgpackAccessor string

func init() {
	for _, mime := range []string{mimeMsgpack, mimeXMsgpack} {
		restful.RegisterEntityAccessor(mime, msgpackAccessor(mime))
	}
}

// Use the same JSON mapping as the public DTOs rather than maintaining a
// parallel set of tags, timestamp encodings and custom marshalers.
func (msgpackAccessor) Read(req *restful.Request, dst any) error {
	body, err := io.ReadAll(req.Request.Body)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if _, err := msgp.UnmarshalAsJSON(&buf, body); err != nil {
		return err
	}
	dec := json.NewDecoder(&buf)
	dec.UseNumber() // data.ValidateLabels expects json.Number, including in arrays.
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("expected one MessagePack entity")
	}
	return nil
}

func (mime msgpackAccessor) Write(resp *restful.Response, status int, v any) error {
	if v == nil { // Match go-restful's empty entity response.
		resp.WriteHeader(status)
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var wire any
	if err := dec.Decode(&wire); err != nil {
		return err
	}
	wire, err = msgpackNumbers(wire)
	if err != nil {
		return err
	}
	body, err := msgp.AppendIntf(nil, wire)
	if err != nil {
		return err
	}
	resp.Header().Set("Content-Type", string(mime))
	resp.WriteHeader(status)
	_, err = resp.Write(body)
	return err
}

// JSON numbers must become MessagePack numbers, not strings. Try both
// integer ranges before floating point to preserve large IDs and labels.
func msgpackNumbers(v any) (any, error) {
	var err error
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i, nil
		}
		if u, err := strconv.ParseUint(string(x), 10, 64); err == nil {
			return u, nil
		}
		return x.Float64()
	case []any:
		for i := range x {
			if x[i], err = msgpackNumbers(x[i]); err != nil {
				return nil, err
			}
		}
	case map[string]any:
		for k := range x {
			if x[k], err = msgpackNumbers(x[k]); err != nil {
				return nil, err
			}
		}
	}
	return v, nil
}
