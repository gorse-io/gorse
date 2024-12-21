package dataset

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLoadIris(t *testing.T) {
	data, target, err := LoadIris()
	assert.NoError(t, err)
	assert.Len(t, data, 150)
	assert.Len(t, data[0], 4)
	assert.Len(t, target, 150)
}
