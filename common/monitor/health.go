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

package monitor

import (
	"encoding/json/v2"
	"reflect"
)

type HealthStatus struct {
	Ready               bool
	DataStoreError      error
	CacheStoreError     error
	DataStoreConnected  bool
	CacheStoreConnected bool
}

// Database drivers can return structured errors with public fields or opaque errors
// such as errors.New. Preserve the former and encode the latter as empty objects.
var healthErrors = json.WithMarshalers(json.MarshalFunc(func(err error) ([]byte, error) {
	b, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		t := reflect.TypeOf(err)
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t.Kind() == reflect.Struct {
			for _, f := range reflect.VisibleFields(t) {
				if f.IsExported() {
					return nil, marshalErr
				}
			}
			return []byte("{}"), nil
		}
	}
	return b, marshalErr
}))

func (h HealthStatus) MarshalJSON() ([]byte, error) {
	type wire HealthStatus
	return json.Marshal(wire(h), healthErrors)
}
