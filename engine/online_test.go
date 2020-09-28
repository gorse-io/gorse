package engine

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var items = []RecommendedItem{
	{Item{"1", 10, time.Date(2001, 1, 1, 1, 1, 1, 1, time.UTC), []string{}}, 6},
	{Item{"2", 8, time.Date(2002, 1, 1, 1, 1, 1, 1, time.UTC), []string{}}, 8},
	{Item{"3", 4, time.Date(2003, 1, 1, 1, 1, 1, 1, time.UTC), []string{}}, 10},
	{Item{"4", 2, time.Date(2004, 1, 1, 1, 1, 1, 1, time.UTC), []string{}}, 9},
	{Item{"5", 0, time.Date(2005, 1, 1, 1, 1, 1, 1, time.UTC), []string{}}, 7},
}

func checkItems(t *testing.T, expected []int, actual []RecommendedItem) {
	for i, offset := range expected {
		assert.Equal(t, items[offset].Item, actual[i].Item)
	}
}
