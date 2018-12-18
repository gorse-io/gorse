package base

import (
	"container/heap"
	"gonum.org/v1/gonum/stat"
	"sort"
)

// SparseIdSet manages the map between dense IDs and sparse IDs.
type SparseIdSet struct {
	DenseIds  map[int]int
	SparseIds []int
}

// NotId represents an ID not existed in the data set.
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

func SparseVectorsMean(a []SparseVector) []float64 {
	m := make([]float64, len(a))
	for i := range a {
		m[i] = stat.Mean(a[i].Values, nil)
	}
	return m
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

// SortIndex sorts items by indices.
func (vec *SparseVector) SortIndex() {
	if !vec.Sorted {
		sort.Sort(vec)
		vec.Sorted = true
	}
}

// ForIntersection iterates items in the intersection of two vectors.
func (vec *SparseVector) ForIntersection(other *SparseVector, f func(index int, a, b float64)) {
	// Sort indices of the left vector
	vec.SortIndex()
	// Sort indices of the right vector
	other.SortIndex()
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

// KNNHeap is designed for neighbor-based models to store K nearest neighborhoods.
type KNNHeap struct {
	SparseVector
	Similarities []float64
	K            int
}

// MakeKNNHeap makes a KNNHeap with k size.
func MakeKNNHeap(k int) KNNHeap {
	return KNNHeap{
		SparseVector: SparseVector{},
		Similarities: make([]float64, 0),
		K:            k,
	}
}

func (vec *KNNHeap) Less(i, j int) bool {
	return vec.Similarities[i] < vec.Similarities[j]
}

func (vec *KNNHeap) Swap(i, j int) {
	vec.SparseVector.Swap(i, j)
	vec.Similarities[i], vec.Similarities[j] = vec.Similarities[j], vec.Similarities[i]
}

type KNNHeapItem struct {
	Index      int
	Value      float64
	Similarity float64
}

func (vec *KNNHeap) Push(x interface{}) {
	item := x.(KNNHeapItem)
	vec.Indices = append(vec.Indices, item.Index)
	vec.Values = append(vec.Values, item.Value)
	vec.Similarities = append(vec.Similarities, item.Similarity)
}

func (vec *KNNHeap) Pop() interface{} {
	// Extract the minimum
	n := vec.Len()
	item := KNNHeapItem{
		Index:      vec.Indices[n-1],
		Value:      vec.Values[n-1],
		Similarity: vec.Similarities[n-1],
	}
	// Remove last element
	vec.Indices = vec.Indices[0 : n-1]
	vec.Values = vec.Values[0 : n-1]
	vec.Similarities = vec.Similarities[0 : n-1]
	// We dont' expect return
	return item
}

// Add a new neighbor to the adjacent vector.
func (vec *KNNHeap) Add(index int, value float64, similarity float64) {
	// Deprecate zero items
	if similarity == 0 {
		return
	}
	// Insert item
	heap.Push(vec, KNNHeapItem{index, value, similarity})
	// Remove minimum
	if vec.Len() > vec.K {
		heap.Pop(vec)
	}
}
