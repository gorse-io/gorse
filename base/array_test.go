package base

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArray(t *testing.T) {
	var a Array[int32]
	assert.Zero(t, a.Len())
	for i := 0; i < 123; i++ {
		a.Append(int32(i))
	}
	assert.Equal(t, 123, a.Len())
	for i := 0; i < 123; i++ {
		assert.Equal(t, int32(i), a.Get(i))
	}
}
