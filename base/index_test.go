package base

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndex(t *testing.T) {
	// Null indexer
	var index *Index
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
