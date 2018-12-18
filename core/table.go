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

// NewDataTable creates a new raw data set.
func NewDataTable(users, items []int, ratings []float64) *DataTable {
	return &DataTable{
		Users:   users,
		Items:   items,
		Ratings: ratings,
	}
}

func (dataSet *DataTable) Len() int {
	return len(dataSet.Ratings)
}

func (dataSet *DataTable) Get(i int) (int, int, float64) {
	return dataSet.Users[i], dataSet.Items[i], dataSet.Ratings[i]
}

func (dataSet *DataTable) ForEach(f func(userId, itemId int, rating float64)) {
	for i := 0; i < dataSet.Len(); i++ {
		f(dataSet.Users[i], dataSet.Items[i], dataSet.Ratings[i])
	}
}

func (dataSet *DataTable) Mean() float64 {
	return stat.Mean(dataSet.Ratings, nil)
}

func (dataSet *DataTable) StdDev() float64 {
	return stat.StdDev(dataSet.Ratings, nil)
}

func (dataSet *DataTable) Min() float64 {
	return floats.Min(dataSet.Ratings)
}

func (dataSet *DataTable) Max() float64 {
	return floats.Max(dataSet.Ratings)
}

// Subset returns a subset of the data set.
func (dataSet *DataTable) SubSet(indices []int) Table {
	return NewVirtualTable(dataSet, indices)
}

// VirtualTable is a virtual subset of DataTable.
type VirtualTable struct {
	data  *DataTable
	index []int
}

// NewVirtualTable creates a new virtual data set.
func NewVirtualTable(dataSet *DataTable, index []int) *VirtualTable {
	return &VirtualTable{
		data:  dataSet,
		index: index,
	}
}

func (dataSet *VirtualTable) Len() int {
	return len(dataSet.index)
}

func (dataSet *VirtualTable) Get(i int) (int, int, float64) {
	indexInData := dataSet.index[i]
	return dataSet.data.Get(indexInData)
}

func (dataSet *VirtualTable) ForEach(f func(userId, itemId int, rating float64)) {
	for i := 0; i < dataSet.Len(); i++ {
		userId, itemId, rating := dataSet.Get(i)
		f(userId, itemId, rating)
	}
}

func (dataSet *VirtualTable) Mean() float64 {
	mean := 0.0
	dataSet.ForEach(func(userId, itemId int, rating float64) {
		mean += rating
	})
	return mean / float64(dataSet.Len())
}

func (dataSet *VirtualTable) StdDev() float64 {
	mean := dataSet.Mean()
	sum := 0.0
	dataSet.ForEach(func(userId, itemId int, rating float64) {
		sum += (rating - mean) * (rating - mean)
	})
	return math.Sqrt(mean / float64(dataSet.Len()))
}

func (dataSet *VirtualTable) Min() float64 {
	_, _, min := dataSet.Get(0)
	dataSet.ForEach(func(userId, itemId int, rating float64) {
		if rating < min {
			rating = min
		}
	})
	return min
}

func (dataSet *VirtualTable) Max() float64 {
	_, _, max := dataSet.Get(0)
	dataSet.ForEach(func(userId, itemId int, rating float64) {
		if rating > max {
			max = rating
		}
	})
	return max
}

func (dataSet *VirtualTable) SubSet(indices []int) Table {
	rawIndices := make([]int, len(indices))
	for i, index := range indices {
		rawIndices[i] = dataSet.index[index]
	}
	return NewVirtualTable(dataSet.data, rawIndices)
}
