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
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	mimeMsgpack  = "application/msgpack"
	mimeXMsgpack = "application/x-msgpack"
)

type msgpackAccessor string

func init() {
	// REST timestamps use RFC3339 text rather than MessagePack extensions.
	msgpack.Register(time.Time{}, func(enc *msgpack.Encoder, v reflect.Value) error {
		text, err := v.Interface().(time.Time).MarshalText()
		if err != nil {
			return err
		}
		return enc.EncodeString(string(text))
	}, func(dec *msgpack.Decoder, v reflect.Value) error {
		text, err := dec.DecodeString()
		if err != nil {
			return err
		}
		return v.Addr().Interface().(*time.Time).UnmarshalText([]byte(text))
	})
	msgpack.Register(json.Number(""), encodeMsgpackNumber, nil)
	for _, mime := range []string{mimeMsgpack, mimeXMsgpack} {
		restful.RegisterEntityAccessor(mime, msgpackAccessor(mime))
	}
}

func encodeMsgpackNumber(enc *msgpack.Encoder, v reflect.Value) error {
	n := v.String()
	if n == "" {
		return enc.EncodeInt(0)
	} // json.Number's zero value
	if !json.Valid([]byte(n)) {
		return fmt.Errorf("invalid number %q", n)
	}
	if i, err := strconv.ParseInt(n, 10, 64); err == nil {
		return enc.EncodeInt(i)
	}
	if u, err := strconv.ParseUint(n, 10, 64); err == nil {
		return enc.EncodeUint(u)
	}
	if !strings.ContainsAny(n, ".eE") {
		return fmt.Errorf("integer outside MessagePack range: %s", n)
	}
	f, err := strconv.ParseFloat(n, 64)
	if err != nil {
		return err
	}
	return enc.EncodeFloat64(f)
}

func (msgpackAccessor) Read(req *restful.Request, dst any) error {
	dec := msgpack.NewDecoder(req.Request.Body)
	dec.SetCustomStructTag("json")
	dec.UseLooseInterfaceDecoding(true)
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if _, err := dec.PeekCode(); err != io.EOF {
		if err != nil {
			return err
		}
		return errors.New("expected one MessagePack entity")
	}
	return normalizeMsgpackLabels(dst)
}

// Only dynamically typed labels need conversion: validation and storage use
// json.Number for numbers in both scalar labels and nested vectors.
func normalizeMsgpackLabels(dst any) error {
	var labels *any
	switch v := dst.(type) {
	case *data.User:
		labels = &v.Labels
	case *data.UserPatch:
		labels = &v.Labels
	case *Item:
		labels = &v.Labels
	case *data.ItemPatch:
		labels = &v.Labels
	case *Feedback:
		labels = &v.Labels
	case *[]data.User:
		return normalizeMsgpackBatch(*v)
	case *[]Item:
		return normalizeMsgpackBatch(*v)
	case *[]Feedback:
		return normalizeMsgpackBatch(*v)
	default:
		return nil
	}
	var err error
	*labels, err = labelNumbers(*labels)
	return err
}

func normalizeMsgpackBatch[T any](batch []T) error {
	for i := range batch {
		if err := normalizeMsgpackLabels(&batch[i]); err != nil {
			return err
		}
	}
	return nil
}

func labelNumbers(v any) (any, error) {
	var err error
	switch x := v.(type) {
	case int64:
		return json.Number(strconv.FormatInt(x, 10)), nil
	case uint64:
		return json.Number(strconv.FormatUint(x, 10)), nil
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, fmt.Errorf("invalid label number %v", x)
		}
		return json.Number(strconv.FormatFloat(x, 'g', -1, 64)), nil
	case []any:
		for i := range x {
			if x[i], err = labelNumbers(x[i]); err != nil {
				return nil, err
			}
		}
	case map[string]any:
		for k := range x {
			if x[k], err = labelNumbers(x[k]); err != nil {
				return nil, err
			}
		}
	}
	return v, nil
}

func (mime msgpackAccessor) Write(resp *restful.Response, status int, v any) error {
	if v == nil {
		resp.WriteHeader(status)
		return nil
	}
	var body bytes.Buffer
	enc := msgpack.NewEncoder(&body)
	enc.SetCustomStructTag("json")
	if err := enc.EncodeValue(reflect.ValueOf(v)); err != nil {
		return err
	}
	resp.Header().Set("Content-Type", string(mime))
	resp.WriteHeader(status)
	_, err := resp.Write(body.Bytes())
	return err
}
