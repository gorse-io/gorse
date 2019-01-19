package core

import (
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"math"
)

/* Table */

// DataTable is an array of (userId, itemId, rating).
type DataTable struct {
	Ratings []float64
	Users   []int
	Items   []int
}

// NewDataTable creates a new raw Data set.
func NewDataTable(users, items []int, ratings []float64) *DataTable {
	table := new(DataTable)
	table.Users = users
	table.Items = items
	table.Ratings = ratings
	return table
}

// Len returns the length of the DataTable.
func (dataTable *DataTable) Len() int {
	return len(dataTable.Ratings)
}

// Get the i-th rating in the DataTable.
func (dataTable *DataTable) Get(i int) (int, int, float64) {
	return dataTable.Users[i], dataTable.Items[i], dataTable.Ratings[i]
}

// ForEach iterates ratings in the DataTable.
func (dataTable *DataTable) ForEach(f func(userId, itemId int, rating float64)) {
	for i := 0; i < dataTable.Len(); i++ {
		f(dataTable.Users[i], dataTable.Items[i], dataTable.Ratings[i])
	}
}

// Mean returns the mean of ratings in the DataTable.
func (dataTable *DataTable) Mean() float64 {
	return stat.Mean(dataTable.Ratings, nil)
}

// StdDev returns the standard deviation of ratings in the DataTable.
func (dataTable *DataTable) StdDev() float64 {
	mean := dataTable.Mean()
	sum := 0.0
	dataTable.ForEach(func(userId, itemId int, rating float64) {
		sum += (rating - mean) * (rating - mean)
	})
	return math.Sqrt(sum / float64(dataTable.Len()))
}

// Min returns the minimal ratings in the DataTable.
func (dataTable *DataTable) Min() float64 {
	return floats.Min(dataTable.Ratings)
}

// Max returns the maximal ratings in the DataTable.
func (dataTable *DataTable) Max() float64 {
	return floats.Max(dataTable.Ratings)
}

// SubSet returns a subset of the DataTable.
func (dataTable *DataTable) SubSet(indices []int) Table {
	return NewVirtualTable(dataTable, indices)
}

// VirtualTable is a virtual subset of DataTable, which saves indices pointed to a DataTable.
type VirtualTable struct {
	Data  *DataTable
	Index []int
}

// NewVirtualTable creates a new virtual table.
func NewVirtualTable(dataSet *DataTable, index []int) *VirtualTable {
	table := new(VirtualTable)
	table.Data = dataSet
	table.Index = index
	return table
}

// Len returns the length of VirtualTable.
func (vTable *VirtualTable) Len() int {
	return len(vTable.Index)
}

// Get the i-th ratings in the VirtualTable.
func (vTable *VirtualTable) Get(i int) (int, int, float64) {
	indexInData := vTable.Index[i]
	return vTable.Data.Get(indexInData)
}

// ForEach iterates ratings in the VirtualTable.
func (vTable *VirtualTable) ForEach(f func(userId, itemId int, rating float64)) {
	for i := 0; i < vTable.Len(); i++ {
		userId, itemId, rating := vTable.Get(i)
		f(userId, itemId, rating)
	}
}

// Mean returns the mean of ratings in the VirtualTable.
func (vTable *VirtualTable) Mean() float64 {
	mean := 0.0
	vTable.ForEach(func(userId, itemId int, rating float64) {
		mean += rating
	})
	return mean / float64(vTable.Len())
}

// StdDev returns the standard deviation of ratings in the VirtualTable.
func (vTable *VirtualTable) StdDev() float64 {
	mean := vTable.Mean()
	sum := 0.0
	vTable.ForEach(func(userId, itemId int, rating float64) {
		sum += (rating - mean) * (rating - mean)
	})
	return math.Sqrt(sum / float64(vTable.Len()))
}

// Min returns the minimal ratings in the VirtualTable.
func (vTable *VirtualTable) Min() float64 {
	_, _, min := vTable.Get(0)
	vTable.ForEach(func(userId, itemId int, rating float64) {
		if rating < min {
			min = rating
		}
	})
	return min
}

// Max returns the maximal ratings in the VirtualTable.
func (vTable *VirtualTable) Max() float64 {
	_, _, max := vTable.Get(0)
	vTable.ForEach(func(userId, itemId int, rating float64) {
		if rating > max {
			max = rating
		}
	})
	return max
}

// SubSet returns a subset of ratings in the VirtualTable.
func (vTable *VirtualTable) SubSet(indices []int) Table {
	rawIndices := make([]int, len(indices))
	for i, index := range indices {
		rawIndices[i] = vTable.Index[index]
	}
	return NewVirtualTable(vTable.Data, rawIndices)
}
