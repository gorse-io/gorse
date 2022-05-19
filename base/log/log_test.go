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

package log

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestSetDevelopmentLogger(t *testing.T) {
	temp, err := os.MkdirTemp("", "test_gorse")
	assert.NoError(t, err)
	// set existed path
	SetDevelopmentLogger(temp + "/gorse.log")
	_, err = os.Stat(temp + "/gorse.log")
	assert.NoError(t, err)
	// set non-existed path
	SetDevelopmentLogger(temp + "/gorse/gorse.log")
	_, err = os.Stat(temp + "/gorse/gorse.log")
	assert.NoError(t, err)
	// permission denied
	assert.Panics(t, func() {
		SetDevelopmentLogger("/gorse.log")
	})
	assert.Panics(t, func() {
		SetDevelopmentLogger("/gorse/gorse.log")
	})
}

func TestSetProductionLogger(t *testing.T) {
	temp, err := os.MkdirTemp("", "test_gorse")
	assert.NoError(t, err)
	// set existed path
	SetProductionLogger(temp + "/gorse.log")
	_, err = os.Stat(temp + "/gorse.log")
	assert.NoError(t, err)
	// set non-existed path
	SetProductionLogger(temp + "/gorse/gorse.log")
	_, err = os.Stat(temp + "/gorse/gorse.log")
	assert.NoError(t, err)
	// permission denied
	assert.Panics(t, func() {
		SetProductionLogger("/gorse.log")
	})
	assert.Panics(t, func() {
		SetProductionLogger("/gorse/gorse.log")
	})
}

func TestRedactDBURL(t *testing.T) {
	assert.Equal(t, "mysql://xxxxx:xxxxxxxxxx@tcp(localhost:3306)/gorse?parseTime=true", RedactDBURL("mysql://gorse:gorse_pass@tcp(localhost:3306)/gorse?parseTime=true"))
	assert.Equal(t, "postgres://xxx:xxxxxx@1.2.3.4:5432/mydb?sslmode=verify-full", RedactDBURL("postgres://bob:secret@1.2.3.4:5432/mydb?sslmode=verify-full"))
	assert.Equal(t, "mysql://gorse:gorse_pass@tcp(localhost:3306) gorse?parseTime=true", RedactDBURL("mysql://gorse:gorse_pass@tcp(localhost:3306) gorse?parseTime=true"))
	assert.Equal(t, "postgres://bob:secret@1.2.3.4:5432 mydb?sslmode=verify-full", RedactDBURL("postgres://bob:secret@1.2.3.4:5432 mydb?sslmode=verify-full"))
}
