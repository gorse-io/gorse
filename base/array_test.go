package base

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIntegers(t *testing.T) {
	var a Integers
	assert.Zero(t, a.Len())
	for i := 0; i < 123; i++ {
		a.Append(int32(i))
	}
	assert.Equal(t, 123, a.Len())
	for i := 0; i < 123; i++ {
		assert.Equal(t, int32(i), a.Get(i))
	}
}

func TestFloats(t *testing.T) {
	var a Floats
	assert.Zero(t, a.Len())
	for i := 0; i < 123; i++ {
		a.Append(float32(i))
	}
	assert.Equal(t, 123, a.Len())
	for i := 0; i < 123; i++ {
		assert.Equal(t, float32(i), a.Get(i))
	}
}
