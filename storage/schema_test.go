package storage

import (
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAppendURLParams(t *testing.T) {
	// test windows path
	url, err := AppendURLParams(`c:\\sqlite.db`, []lo.Tuple2[string, string]{{"a", "b"}})
	assert.NoError(t, err)
	assert.Equal(t, `c:\\sqlite.db?a=b`, url)
	// test no scheme
	url, err = AppendURLParams(`sqlite.db`, []lo.Tuple2[string, string]{{"a", "b"}})
	assert.NoError(t, err)
	assert.Equal(t, `sqlite.db?a=b`, url)
}
