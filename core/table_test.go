package core

import (
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
)

const tableEpsilon = 0.00001

func TestDataTable(t *testing.T) {
	table := NewDataTable(
		[]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		[]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		[]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	// Test len
	assert.Equal(t, 10, table.Len())
	// Test mean
	assert.Equal(t, 4.5, table.Mean())
	// Test std
	assert.True(t, math.Abs(table.StdDev()-2.872281) < tableEpsilon)
	// Test min
	assert.Equal(t, 0.0, table.Min())
	// Test max
	assert.Equal(t, 9.0, table.Max())
}

func TestVirtualTable(t *testing.T) {
	table := NewDataTable(
		[]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		[]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		[]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	vt := NewVirtualTable(table, []int{0, 2, 4, 6, 8})
	// Test len
	assert.Equal(t, 5, vt.Len())
	// Test mean
	assert.Equal(t, 4.0, vt.Mean())
	// Test std
	assert.True(t, math.Abs(vt.StdDev()-2.828427) < tableEpsilon)
	// Test min
	assert.Equal(t, 0.0, vt.Min())
	// Test max
	assert.Equal(t, 8.0, vt.Max())
	// Test subset
	st := vt.SubSet([]int{0, 2, 4})
	assert.Equal(t, 3, st.Len())
	u := make([]int, 0, 3)
	i := make([]int, 0, 3)
	r := make([]float64, 0, 3)
	st.ForEach(func(userId, itemId int, rating float64) {
		u = append(u, userId)
		i = append(i, itemId)
		r = append(r, rating)
	})
	assert.Equal(t, []int{0, 4, 8}, u)
	assert.Equal(t, []int{0, 4, 8}, i)
	assert.Equal(t, []float64{0, 4, 8}, r)
}
