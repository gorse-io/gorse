package base

import (
	"gonum.org/v1/gonum/stat"
	"sort"
)

// SparseIdSet manages the map between dense IDs and sparse IDs.
type SparseIdSet struct {
	DenseIds  map[int]int
	SparseIds []int
}

// NotId represents the ID not existed in the data set.
const NotId = -1

// MakeSparseIdSet makes a SparseIdSet.
func MakeSparseIdSet() SparseIdSet {
	return SparseIdSet{
		DenseIds:  make(map[int]int),
		SparseIds: make([]int, 0),
	}
}

// Len returns the number of IDs.
func (set *SparseIdSet) Len() int {
	return len(set.SparseIds)
}

// Add adds a new ID to the ID set.
func (set *SparseIdSet) Add(sparseId int) {
	if _, exist := set.DenseIds[sparseId]; !exist {
		set.DenseIds[sparseId] = len(set.SparseIds)
		set.SparseIds = append(set.SparseIds, sparseId)
	}
}

// ToDenseId converts a sparse ID to a dense ID.
func (set *SparseIdSet) ToDenseId(sparseId int) int {
	if denseId, exist := set.DenseIds[sparseId]; exist {
		return denseId
	}
	return NotId
}

// ToSparseId converts a dense ID to a sparse ID.
func (set *SparseIdSet) ToSparseId(denseId int) int {
	return set.SparseIds[denseId]
}

type SparseVector struct {
	Indices []int
	Values  []float64
	Sorted  bool
}

func MakeSparseVector() SparseVector {
	return SparseVector{
		Indices: make([]int, 0),
		Values:  make([]float64, 0),
	}
}

func NewSparseVector() *SparseVector {
	return &SparseVector{
		Indices: make([]int, 0),
		Values:  make([]float64, 0),
	}
}

func MakeDenseSparseMatrix(row int) []SparseVector {
	mat := make([]SparseVector, row)
	for i := range mat {
		mat[i] = MakeSparseVector()
	}
	return mat
}

func Means(a []SparseVector) []float64 {
	m := make([]float64, len(a))
	for i := range a {
		m[i] = stat.Mean(a[i].Values, nil)
	}
	return m
}

func (vec *SparseVector) Add(index int, value float64) {
	vec.Indices = append(vec.Indices, index)
	vec.Values = append(vec.Values, value)
	vec.Sorted = false
}

func (vec *SparseVector) Len() int {
	return len(vec.Values)
}

func (vec *SparseVector) Less(i, j int) bool {
	return vec.Indices[i] < vec.Indices[j]
}

func (vec *SparseVector) Swap(i, j int) {
	vec.Indices[i], vec.Indices[j] = vec.Indices[j], vec.Indices[i]
	vec.Values[i], vec.Values[j] = vec.Values[j], vec.Values[i]
}

func (vec *SparseVector) ForEach(f func(i, index int, value float64)) {
	for i := range vec.Indices {
		f(i, vec.Indices[i], vec.Values[i])
	}
}

func (vec *SparseVector) ForIntersection(other *SparseVector, f func(index int, a, b float64)) {
	// Sort indices of the left vector
	if !vec.Sorted {
		sort.Sort(vec)
		vec.Sorted = true
	}
	// Sort indices of the right vector
	if !other.Sorted {
		sort.Sort(other)
		other.Sorted = true
	}
	// Iterate
	i, j := 0, 0
	for i < vec.Len() && j < other.Len() {
		if vec.Indices[i] == other.Indices[j] {
			f(vec.Indices[i], vec.Values[i], other.Values[j])
			i++
			j++
		} else if vec.Indices[i] < other.Indices[j] {
			i++
		} else {
			j++
		}
	}
}

// AdjacentVector is designed for neighbor-based models to store K nearest neighborhoods.
type AdjacentVector struct {
	Similarities []float64
	Peers        []int
}

// MakeAdjacentVector makes a AdjacentVector with k size.
func MakeAdjacentVector(k int) AdjacentVector {
	return AdjacentVector{
		Similarities: make([]float64, k),
		Peers:        make([]int, k),
	}
}

// Put puts a new neighbor to the adjacent vector.
func (vec *AdjacentVector) Put(i, j int, similarity float64) {
	// Find minimum
	minIndex := Argmin(vec.Similarities)
	// Replace minimum
	vec.Similarities[minIndex] = similarity
	vec.Peers[minIndex] = i
}

// ToSparseVector returns all non-zero items.
func (vec *AdjacentVector) ToSparseVector() *SparseVector {
	neighbors := NewSparseVector()
	for i := range vec.Peers {
		if vec.Similarities[i] > 0 {
			neighbors.Add(vec.Peers[i], vec.Similarities[i])
		}
	}
	return neighbors
}
