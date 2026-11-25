// Copyright 2021 gorse Project Authors
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

package jsonutil

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"time"
)

// MarshalDurationFields encodes durations as Go duration strings within v only.
func MarshalDurationFields(enc *jsontext.Encoder, v any) error {
	return json.MarshalEncode(enc, v, json.WithMarshalers(durationMarshaler))
}

var durationMarshaler = json.MarshalToFunc(func(enc *jsontext.Encoder, d time.Duration) error {
	return enc.WriteToken(jsontext.String(d.String()))
})

// UnmarshalDurationFields accepts duration strings, not integer nanoseconds.
func UnmarshalDurationFields(dec *jsontext.Decoder, v any) error {
	return json.UnmarshalDecode(dec, v, json.WithUnmarshalers(durationUnmarshaler))
}

var durationUnmarshaler = json.UnmarshalFromFunc(func(dec *jsontext.Decoder, d *time.Duration) error {
	var s string
	if err := json.UnmarshalDecode(dec, &s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err == nil {
		*d = parsed
	}
	return err
})
