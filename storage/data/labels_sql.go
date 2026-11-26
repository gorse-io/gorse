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

package data

import (
	"context"
	"encoding/json/v2"
	"reflect"

	"gorm.io/gorm/schema"
)

func init() { schema.RegisterSerializer("gorse_labels", labelSerializer{}) }

type labelSerializer struct{}

func (labelSerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue any) error {
	var raw []byte
	switch v := dbValue.(type) {
	case nil:
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		var err error
		raw, err = json.Marshal(v)
		if err != nil {
			return err
		}
	}
	var labels any
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &labels); err != nil {
			return err
		}
	}
	return field.Set(ctx, dst, labels)
}

func (labelSerializer) Value(ctx context.Context, field *schema.Field, dst reflect.Value, value any) (any, error) {
	b, err := json.Marshal(value)
	if err != nil || string(b) == "null" {
		return nil, err
	}
	return string(b), nil
}
