package base

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestValidateId(t *testing.T) {
	assert.NotNil(t, ValidateId(""))
	assert.NotNil(t, ValidateId("/"))
	assert.Nil(t, ValidateId("abc"))
}

func TestValidateLabel(t *testing.T) {
	assert.NotNil(t, ValidateLabel(""))
	assert.NotNil(t, ValidateLabel("/"))
	assert.NotNil(t, ValidateLabel("|"))
	assert.Nil(t, ValidateLabel("abc"))
}

func TestEscape(t *testing.T) {
	assert.Equal(t, "123", Escape("123"))
	assert.Equal(t, "\"\"\"123\"\"\"", Escape("\"123\""))
	assert.Equal(t, "\"1,2,3\"", Escape("1,2,3"))
	assert.Equal(t, "\"\"\",\"\"\"", Escape("\",\""))
}

func TestSplit(t *testing.T) {
	assert.Equal(t, []string{"1", "2", "3"}, Split("1,2,3"))
	assert.Equal(t, []string{"1,2", "3,4", "5,6"}, Split("\"1,2\",\"3,4\",\"5,6\""))
	assert.Equal(t, []string{"\"1,2\",\"3,4\",\"5,6\""}, Split("\"\"\"1,2\"\",\"\"3,4\"\",\"\"5,6\"\"\""))
}
