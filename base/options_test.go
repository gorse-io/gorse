package base

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRuntimeOptions(t *testing.T) {
	// Check default
	var opt1 *RuntimeOptions
	assert.Equal(t, true, opt1.GetVerbose())
	assert.Equal(t, 1, opt1.GetJobs())
	// Check options
	opt2 := &RuntimeOptions{false, 10}
	assert.Equal(t, false, opt2.GetVerbose())
	assert.Equal(t, 10, opt2.GetJobs())
}
