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

package config

import (
	"encoding/json/jsontext"

	"github.com/gorse-io/gorse/common/jsonutil"
)

func (v Config) MarshalJSONTo(enc *jsontext.Encoder) error {
	type wire Config
	return jsonutil.MarshalDurationFields(enc, (*wire)(&v))
}

func (v *Config) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	type wire Config
	return jsonutil.UnmarshalDurationFields(dec, (*wire)(v))
}

func (v RecommendConfig) MarshalJSONTo(enc *jsontext.Encoder) error {
	type wire RecommendConfig
	return jsonutil.MarshalDurationFields(enc, (*wire)(&v))
}

func (v *RecommendConfig) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	type wire RecommendConfig
	return jsonutil.UnmarshalDurationFields(dec, (*wire)(v))
}
