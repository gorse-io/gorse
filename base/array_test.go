package base

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
	assert.Equal(t, 48+4*batchSize, a.Bytes())
}
