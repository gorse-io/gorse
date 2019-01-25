package core

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewRuntimeOptions(t *testing.T) {
	options := NewRuntimeOptions([]RuntimeOption{
		WithVerbose(false),
		WithNJobs(3),
	})
	assert.Equal(t, options.Verbose, false)
	assert.Equal(t, options.NJobs, 3)
}
