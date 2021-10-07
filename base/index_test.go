package base

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

func TestMapIndex(t *testing.T) {
	// Null indexer
	var index *MapIndex
	assert.Zero(t, index.Len())
	// Create a indexer
	index = NewMapIndex()
	assert.Zero(t, index.Len())
	// Add Names
	index.Add("1")
	index.Add("2")
	index.Add("4")
	index.Add("8")
	assert.Equal(t, int32(4), index.Len())
	assert.Equal(t, int32(0), index.ToNumber("1"))
	assert.Equal(t, int32(1), index.ToNumber("2"))
	assert.Equal(t, int32(2), index.ToNumber("4"))
	assert.Equal(t, int32(3), index.ToNumber("8"))
	assert.Equal(t, NotId, index.ToNumber("1000"))
	assert.Equal(t, "1", index.ToName(0))
	assert.Equal(t, "2", index.ToName(1))
	assert.Equal(t, "4", index.ToName(2))
	assert.Equal(t, "8", index.ToName(3))
	// Get names
	assert.Equal(t, []string{"1", "2", "4", "8"}, index.GetNames())
	// Encode and decode
	buf := bytes.NewBuffer(nil)
	err := MarshalIndex(buf, index)
	assert.NoError(t, err)
	indexCopy, err := UnmarshalIndex(buf)
	assert.NoError(t, err)
	assert.Equal(t, index, indexCopy)
}

func TestDirectIndex(t *testing.T) {
	// Create a indexer
	index := NewDirectIndex()
	assert.Zero(t, index.Len())
	// Add Names
	index.Add("1")
	index.Add("2")
	index.Add("4")
	index.Add("8")
	assert.Panics(t, func() { index.Add("abc") })
	assert.Equal(t, int32(9), index.Len())
	assert.Equal(t, int32(1), index.ToNumber("1"))
	assert.Equal(t, int32(2), index.ToNumber("2"))
	assert.Equal(t, int32(4), index.ToNumber("4"))
	assert.Equal(t, int32(8), index.ToNumber("8"))
	assert.Equal(t, NotId, index.ToNumber("1000"))
	assert.Panics(t, func() { index.ToNumber("abc") })
	assert.Equal(t, "0", index.ToName(0))
	assert.Equal(t, "1", index.ToName(1))
	assert.Equal(t, "2", index.ToName(2))
	assert.Equal(t, "3", index.ToName(3))
	assert.Panics(t, func() { index.ToName(10) })
	// Get names
	names := index.GetNames()
	assert.Equal(t, 9, len(names))
	for i := range names {
		assert.Equal(t, strconv.Itoa(i), names[i])
	}
	// Encode and decode
	buf := bytes.NewBuffer(nil)
	err := MarshalIndex(buf, index)
	assert.NoError(t, err)
	indexCopy, err := UnmarshalIndex(buf)
	assert.NoError(t, err)
	assert.Equal(t, index, indexCopy)
}
