package base

import (
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

// SparseVector handles the sparse vector.
type SparseVector struct {
	Indices []int
	Values  []float64
	Sorted  bool
}

// MakeSparseVector makes a SparseVector.
func MakeSparseVector() SparseVector {
	return SparseVector{
		Indices: make([]int, 0),
		Values:  make([]float64, 0),
	}
}

// NewSparseVector creates a SparseVector.
func NewSparseVector() *SparseVector {
	return &SparseVector{
		Indices: make([]int, 0),
		Values:  make([]float64, 0),
	}
}

// MakeDenseSparseMatrix makes an array of SparseVectors.
func MakeDenseSparseMatrix(row int) []SparseVector {
	mat := make([]SparseVector, row)
	for i := range mat {
		mat[i] = MakeSparseVector()
	}
	return mat
}

// Add a new item.
func (vec *SparseVector) Add(index int, value float64) {
	vec.Indices = append(vec.Indices, index)
	vec.Values = append(vec.Values, value)
	vec.Sorted = false
}

// Len returns the number of items.
func (vec *SparseVector) Len() int {
	return len(vec.Values)
}

// Less compares indices of two items.
func (vec *SparseVector) Less(i, j int) bool {
	return vec.Indices[i] < vec.Indices[j]
}

// Swap two items.
func (vec *SparseVector) Swap(i, j int) {
	vec.Indices[i], vec.Indices[j] = vec.Indices[j], vec.Indices[i]
	vec.Values[i], vec.Values[j] = vec.Values[j], vec.Values[i]
}

// ForEach iterates items in the vector.
func (vec *SparseVector) ForEach(f func(i, index int, value float64)) {
	for i := range vec.Indices {
		f(i, vec.Indices[i], vec.Values[i])
	}
}

// ForIntersection iterates items in the intersection of two vectors.
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

// Add a new neighbor to the adjacent vector.
func (vec *AdjacentVector) Add(i int, similarity float64) {
	// Deprecate zero items
	if similarity == 0 {
		return
	}
	// Find minimum
	minIndex := ArgMin(vec.Similarities)
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
