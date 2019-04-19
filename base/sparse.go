package base

import (
	"container/heap"
	"sort"
)

// Indexer manages the map between sparse IDs and dense indices. A sparse ID is
// a user ID or item ID. The dense index is the internal user index or item index
// optimized for faster parameter access and less memory usage.
type Indexer struct {
	Indices map[int]int // sparse ID -> dense index
	IDs     []int       // dense index -> sparse ID
}

// NotId represents an ID doesn't exist.
const NotId = -1

// NewIndexer creates a Indexer.
func NewIndexer() *Indexer {
	set := new(Indexer)
	set.Indices = make(map[int]int)
	set.IDs = make([]int, 0)
	return set
}

// Len returns the number of indexed IDs.
func (set *Indexer) Len() int {
	return len(set.IDs)
}

// Add adds a new ID to the indexer.
func (set *Indexer) Add(ID int) {
	if _, exist := set.Indices[ID]; !exist {
		set.Indices[ID] = len(set.IDs)
		set.IDs = append(set.IDs, ID)
	}
}

// ToIndex converts a sparse ID to a dense index.
func (set *Indexer) ToIndex(ID int) int {
	if denseId, exist := set.Indices[ID]; exist {
		return denseId
	}
	return NotId
}

// ToID converts a dense index to a sparse ID.
func (set *Indexer) ToID(index int) int {
	return set.IDs[index]
}

// MarginalSubSet constructs a subset over a list of IDs, indices and values.
type MarginalSubSet struct {
	IDs     []int     // the full list of IDs
	Indices []int     // the full list of indices
	Values  []float64 // the full list of values
	SubSet  []int     // indices of the subset
}

// NewMarginalSubSet creates a MarginalSubSet.
func NewMarginalSubSet(id, indices []int, values []float64, subset []int) *MarginalSubSet {
	set := new(MarginalSubSet)
	set.IDs = id
	set.Indices = indices
	set.Values = values
	set.SubSet = subset
	sort.Sort(set)
	return set
}

// Len returns the number of items.
func (set *MarginalSubSet) Len() int {
	return len(set.SubSet)
}

// Swap two items.
func (set *MarginalSubSet) Swap(i, j int) {
	set.SubSet[i], set.SubSet[j] = set.SubSet[j], set.SubSet[i]
}

// Less compares two items.
func (set *MarginalSubSet) Less(i, j int) bool {
	return set.IDs[set.SubSet[i]] < set.IDs[set.SubSet[j]]
}

// Count gets the size of marginal subset.
func (set *MarginalSubSet) Count() int {
	return len(set.SubSet)
}

// GetIndex returns the index of i-th item.
func (set *MarginalSubSet) GetIndex(i int) int {
	return set.Indices[set.SubSet[i]]
}

// Mean of ratings in the subset.
func (set *MarginalSubSet) Mean() float64 {
	sum := 0.0
	for _, i := range set.SubSet {
		sum += set.Values[i]
	}
	return sum / float64(set.Len())
}

// Contain returns true am ID existed in the subset.
func (set *MarginalSubSet) Contain(id int) bool {
	// if id is out of range
	if id < set.IDs[set.SubSet[0]] || id > set.IDs[set.SubSet[set.Len()-1]] {
		return false
	}
	// binary search
	low, high := 0, set.Len()-1
	for low <= high {
		// in bound
		if set.IDs[set.SubSet[low]] == id || set.IDs[set.SubSet[high]] == id {
			return true
		}
		mid := (low + high) / 2
		// in mid
		if id == set.IDs[set.SubSet[mid]] {
			return true
		} else if id < set.IDs[set.SubSet[mid]] {
			low = low + 1
			high = mid - 1
		} else if id > set.IDs[set.SubSet[mid]] {
			low = mid + 1
			high = high - 1
		}
	}
	return false
}

// ForIntersection iterates items in the intersection of two subsets.
// The method find items with common indices in linear time.
func (set *MarginalSubSet) ForIntersection(other *MarginalSubSet, f func(id int, a, b float64)) {
	// Iterate
	i, j := 0, 0
	for i < set.Len() && j < other.Len() {
		if set.IDs[set.SubSet[i]] == other.IDs[other.SubSet[j]] {
			f(set.IDs[set.SubSet[i]], set.Values[set.SubSet[i]], other.Values[other.SubSet[j]])
			i++
			j++
		} else if set.IDs[set.SubSet[i]] < other.IDs[other.SubSet[j]] {
			i++
		} else {
			j++
		}
	}
}

// ForEach iterates items in the subset with IDs.
func (set *MarginalSubSet) ForEach(f func(i, id int, value float64)) {
	for i, offset := range set.SubSet {
		f(i, set.IDs[offset], set.Values[offset])
	}
}

// ForEachIndex iterates items in the subset with indices.
func (set *MarginalSubSet) ForEachIndex(f func(i, index int, value float64)) {
	for i, offset := range set.SubSet {
		f(i, set.Indices[offset], set.Values[offset])
	}
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
