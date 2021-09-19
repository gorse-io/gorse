package base

import (
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

func TestMapIndex(t *testing.T) {
	// Null indexer
	var set *MapIndex
	assert.Zero(t, set.Len())
	// Create a indexer
	set = NewMapIndex()
	assert.Zero(t, set.Len())
	// Add Names
	set.Add("1")
	set.Add("2")
	set.Add("4")
	set.Add("8")
	assert.Equal(t, int32(4), set.Len())
	assert.Equal(t, int32(0), set.ToNumber("1"))
	assert.Equal(t, int32(1), set.ToNumber("2"))
	assert.Equal(t, int32(2), set.ToNumber("4"))
	assert.Equal(t, int32(3), set.ToNumber("8"))
	assert.Equal(t, NotId, set.ToNumber("1000"))
	assert.Equal(t, "1", set.ToName(0))
	assert.Equal(t, "2", set.ToName(1))
	assert.Equal(t, "4", set.ToName(2))
	assert.Equal(t, "8", set.ToName(3))
	// Get names
	assert.Equal(t, []string{"1", "2", "4", "8"}, set.GetNames())
}

func TestDirectIndex(t *testing.T) {
	// Create a indexer
	set := NewDirectIndex()
	assert.Zero(t, set.Len())
	// Add Names
	set.Add("1")
	set.Add("2")
	set.Add("4")
	set.Add("8")
	assert.Panics(t, func() { set.Add("abc") })
	assert.Equal(t, int32(9), set.Len())
	assert.Equal(t, int32(1), set.ToNumber("1"))
	assert.Equal(t, int32(2), set.ToNumber("2"))
	assert.Equal(t, int32(4), set.ToNumber("4"))
	assert.Equal(t, int32(8), set.ToNumber("8"))
	assert.Equal(t, NotId, set.ToNumber("1000"))
	assert.Panics(t, func() { set.ToNumber("abc") })
	assert.Equal(t, "0", set.ToName(0))
	assert.Equal(t, "1", set.ToName(1))
	assert.Equal(t, "2", set.ToName(2))
	assert.Equal(t, "3", set.ToName(3))
	assert.Panics(t, func() { set.ToName(10) })
	// Get names
	names := set.GetNames()
	assert.Equal(t, 9, len(names))
	for i := range names {
		assert.Equal(t, strconv.Itoa(i), names[i])
	}
}
