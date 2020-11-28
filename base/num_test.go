package base

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewMatrix(t *testing.T) {
	a := NewMatrix(3, 4)
	assert.Equal(t, 3, len(a))
	assert.Equal(t, 4, len(a[0]))
	assert.Equal(t, 4, len(a[0]))
	assert.Equal(t, 4, len(a[0]))
}
