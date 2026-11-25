// Copyright 2022 gorse Project Authors
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

// Package jsoncodec preserves the JSON representation used by storage records and cursors.
package jsoncodec

import (
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
)

// Keep stored null collections, deterministic map order, and the escaping of existing cursors.
var marshalOptions = json.JoinOptions(json.FormatNilMapAsNull(true), json.FormatNilSliceAsNull(true), json.Deterministic(true), jsontext.EscapeForHTML(true), jsontext.EscapeForJS(true))

func Marshal(v any) ([]byte, error) { return json.Marshal(v, marshalOptions) }

// Numbers in untyped labels and parameters must not pass through float64.
var exactNumbers = json.WithUnmarshalers(json.UnmarshalFromFunc(func(d *jsontext.Decoder, v *any) error {
	if d.PeekKind() == '0' {
		*v = jsonv1.Number("")
	}
	return errors.ErrUnsupported
}))

func Unmarshal(b []byte, v any) error {
	return json.Unmarshal(b, v, exactNumbers, json.MatchCaseInsensitiveNames(true))
}
