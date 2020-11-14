package model

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIndex(t *testing.T) {
	// Create a indexer
	set := NewIndex()
	assert.Equal(t, set.Len(), 0)
	// Add Names
	set.Add("1")
	set.Add("2")
	set.Add("4")
	set.Add("8")
	assert.Equal(t, 4, set.Len())
	assert.Equal(t, 0, set.ToNumber("1"))
	assert.Equal(t, 1, set.ToNumber("2"))
	assert.Equal(t, 2, set.ToNumber("4"))
	assert.Equal(t, 3, set.ToNumber("8"))
	assert.Equal(t, NotId, set.ToNumber("1000"))
	assert.Equal(t, "1", set.ToName(0))
	assert.Equal(t, "2", set.ToName(1))
	assert.Equal(t, "4", set.ToName(2))
	assert.Equal(t, "8", set.ToName(3))
}
