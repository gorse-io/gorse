package base

import (
	"container/heap"
	"gonum.org/v1/gonum/stat"
	"sort"
)

// SparseIdSet manages the map between sparse IDs and dense IDs. The sparse ID is the ID
// in raw user ID or item ID. The dense ID is the internal user ID or item ID optimized
// for faster parameter access and less memory usage.
type SparseIdSet struct {
	DenseIds  map[int]int // sparse ID -> dense ID
	SparseIds []int       // dense ID -> sparse ID
}

// NotId represents an ID doesn't exist.
const NotId = -1

// NewSparseIdSet creates a SparseIdSet.
func NewSparseIdSet() *SparseIdSet {
	set := new(SparseIdSet)
	set.DenseIds = make(map[int]int)
	set.SparseIds = make([]int, 0)
	return set
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
	// Return NotId if set is null
	if set == nil {
		return NotId
	}
	if denseId, exist := set.DenseIds[sparseId]; exist {
		return denseId
	}
	return NotId
}

// ToSparseId converts a dense ID to a sparse ID.
func (set *SparseIdSet) ToSparseId(denseId int) int {
	return set.SparseIds[denseId]
}

// SparseVector is the data structure for the sparse vector.
type SparseVector struct {
	Indices []int
	Values  []float64
	Sorted  bool
}

// NewSparseVector creates a SparseVector.
func NewSparseVector() *SparseVector {
	return &SparseVector{
		Indices: make([]int, 0),
		Values:  make([]float64, 0),
	}
}

// NewDenseSparseMatrix creates an array of SparseVectors.
func NewDenseSparseMatrix(row int) []*SparseVector {
	mat := make([]*SparseVector, row)
	for i := range mat {
		mat[i] = NewSparseVector()
	}
	return mat
}

// SparseVectorsMean computes the mean of each sparse vector.
func SparseVectorsMean(a []*SparseVector) []float64 {
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

// Less returns true if the index of i-th item is less than the index of j-th item.
func (vec *SparseVector) Less(i, j int) bool {
	return vec.Indices[i] < vec.Indices[j]
}

// Swap two items.
func (vec *SparseVector) Swap(i, j int) {
	vec.Indices[i], vec.Indices[j] = vec.Indices[j], vec.Indices[i]
	vec.Values[i], vec.Values[j] = vec.Values[j], vec.Values[i]
}

// ForEach iterates items in the sparse vector.
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

// ForIntersection iterates items in the intersection of two vectors. The method sorts two vectors
// by indices first, then find common indices in linear time.
func (vec *SparseVector) ForIntersection(other *SparseVector, f func(index int, a, b float64)) {
	// Sort indices of the left vec
	vec.SortIndex()
	// Sort indices of the right vec
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

// KNNHeap is designed for neighbor-based models to store K nearest neighbors.
// Heap is used to reduce time complexity and memory complexity in neighbors
// searching.
type KNNHeap struct {
	SparseVector           // Store neighbor IDs and ratings.
	Similarities []float64 // Store similarity.
	K            int       // Number of required neighbors.
}

// NewKNNHeap creates a KNNHeap.
func NewKNNHeap(k int) *KNNHeap {
	knnHeap := new(KNNHeap)
	knnHeap.SparseVector = SparseVector{}
	knnHeap.Similarities = make([]float64, 0)
	knnHeap.K = k
	return knnHeap
}

// Less returns true if the similarity of i-th item is less than the similarity of j-th item.
// It is a method of heap.Interface.
func (knnHeap *KNNHeap) Less(i, j int) bool {
	return knnHeap.Similarities[i] < knnHeap.Similarities[j]
}

// Swap the i-th item with the j-th item. It is a method of heap.Interface.
func (knnHeap *KNNHeap) Swap(i, j int) {
	knnHeap.SparseVector.Swap(i, j)
	knnHeap.Similarities[i], knnHeap.Similarities[j] = knnHeap.Similarities[j], knnHeap.Similarities[i]
}

// _KNNHeapItem is designed for heap.Interface to pass neighborhoods.
type _KNNHeapItem struct {
	Id         int
	Rating     float64
	Similarity float64
}

// Push a neighbors into the KNNHeap. It is a method of heap.Interface.
func (knnHeap *KNNHeap) Push(x interface{}) {
	item := x.(_KNNHeapItem)
	knnHeap.Indices = append(knnHeap.Indices, item.Id)
	knnHeap.Values = append(knnHeap.Values, item.Rating)
	knnHeap.Similarities = append(knnHeap.Similarities, item.Similarity)
}

// Pop the last item (the neighbor with minimum similarity) in the KNNHeap.
// It is a method of heap.Interface.
func (knnHeap *KNNHeap) Pop() interface{} {
	// Extract the minimum
	n := knnHeap.Len()
	item := _KNNHeapItem{
		Id:         knnHeap.Indices[n-1],
		Rating:     knnHeap.Values[n-1],
		Similarity: knnHeap.Similarities[n-1],
	}
	// Remove last element
	knnHeap.Indices = knnHeap.Indices[0 : n-1]
	knnHeap.Values = knnHeap.Values[0 : n-1]
	knnHeap.Similarities = knnHeap.Similarities[0 : n-1]
	// We never use returned item
	return item
}

// Add a new neighbor to the KNNHeap. Neighbors with zero similarity will be ignored.
func (knnHeap *KNNHeap) Add(id int, rating float64, similarity float64) {
	// Deprecate zero items
	if similarity == 0 {
		return
	}
	// Insert item
	heap.Push(knnHeap, _KNNHeapItem{id, rating, similarity})
	// Remove minimum
	if knnHeap.Len() > knnHeap.K {
		heap.Pop(knnHeap)
	}
}
