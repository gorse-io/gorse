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
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestValidateNotEmpty(t *testing.T) {
	assert.Panics(t, func() { validateNotEmpty("", nil) })
	assert.Panics(t, func() { validateNotEmpty("", []string{}) })
	assert.NotPanics(t, func() { validateNotEmpty("", []string{"a"}) })
}

func TestValidateNotNegative(t *testing.T) {
	assert.Panics(t, func() { validateNotNegative("", -1) })
	assert.NotPanics(t, func() { validateNotNegative("", 1) })
}

func TestValidatePositive(t *testing.T) {
	assert.Panics(t, func() { validatePositive("", 0) })
	assert.NotPanics(t, func() { validatePositive("", 1) })
}

func TestValidateIn(t *testing.T) {
	assert.Panics(t, func() { validateIn("", "d", []string{"a", "b", "c"}) })
	assert.NotPanics(t, func() { validateIn("", "a", []string{"a", "b", "c"}) })
}

func TestValidateSubset(t *testing.T) {
	assert.Panics(t, func() { validateSubset("", []string{"d"}, []string{"a", "b", "c"}) })
	assert.NotPanics(t, func() { validateSubset("", []string{"a", "b"}, []string{"a", "b", "c"}) })
}
