// Copyright 2025 gorse Project Authors
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

package expression

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeedbackTypeExpression_MarshalJSON(t *testing.T) {
	var f FeedbackTypeExpression
	err := f.UnmarshalJSON([]byte(`test`))
	assert.NoError(t, err)
	assert.Equal(t, "test", f.FeedbackType)
	assert.Equal(t, None, f.ExprType)

	err = f.UnmarshalJSON([]byte(`1a`))
	assert.Error(t, err)

	err = f.UnmarshalJSON([]byte(`test<16`))
	assert.NoError(t, err)
	assert.Equal(t, "test", f.FeedbackType)
	assert.Equal(t, Less, f.ExprType)
	assert.Equal(t, 16.0, f.Value)

	err = f.UnmarshalJSON([]byte(`test<=16`))
	assert.NoError(t, err)
	assert.Equal(t, "test", f.FeedbackType)
	assert.Equal(t, LessOrEqual, f.ExprType)
	assert.Equal(t, 16.0, f.Value)

	err = f.UnmarshalJSON([]byte(`test>16`))
	assert.NoError(t, err)
	assert.Equal(t, "test", f.FeedbackType)
	assert.Equal(t, Greater, f.ExprType)
	assert.Equal(t, 16.0, f.Value)

	err = f.UnmarshalJSON([]byte(`test>=16`))
	assert.NoError(t, err)
	assert.Equal(t, "test", f.FeedbackType)
	assert.Equal(t, GreaterOrEqual, f.ExprType)
	assert.Equal(t, 16.0, f.Value)
}

func TestFeedbackTypeExpression_UnmarshalJSON(t *testing.T) {
	f := FeedbackTypeExpression{FeedbackType: "test", Value: 16}
	buf, err := f.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, `test`, string(buf))

	f.ExprType = Less
	buf, err = f.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, `test<16`, string(buf))

	f.ExprType = LessOrEqual
	buf, err = f.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, `test<=16`, string(buf))

	f.ExprType = Greater
	buf, err = f.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, `test>16`, string(buf))

	f.ExprType = GreaterOrEqual
	buf, err = f.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, `test>=16`, string(buf))
}

func TestFeedbackTypeExpression_Match(t *testing.T) {
	f := FeedbackTypeExpression{FeedbackType: "test", Value: 6}
	assert.True(t, f.Match("test", 0))
	assert.False(t, f.Match("a", 1))

	f.ExprType = Less
	assert.True(t, f.Match("test", 5))
	assert.False(t, f.Match("test", 6))

	f.ExprType = LessOrEqual
	assert.True(t, f.Match("test", 6))
	assert.False(t, f.Match("test", 7))

	f.ExprType = Greater
	assert.True(t, f.Match("test", 7))
	assert.False(t, f.Match("test", 6))

	f.ExprType = GreaterOrEqual
	assert.True(t, f.Match("test", 6))
	assert.False(t, f.Match("test", 5))
}
