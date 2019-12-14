package base

import (
	"container/heap"
	"gonum.org/v1/gonum/floats"
	"sort"
)

// Indexer manages the map between sparse IDs and dense indices. A sparse ID is
// a user ID or item ID. The dense index is the internal user index or item index
// optimized for faster parameter access and less memory usage.
type Indexer struct {
	Indices map[string]int // sparse ID -> dense index
	IDs     []string       // dense index -> sparse ID
}

// NotId represents an ID doesn't exist.
const NotId = -1

// NewIndexer creates a Indexer.
func NewIndexer() *Indexer {
	set := new(Indexer)
	set.Indices = make(map[string]int)
	set.IDs = make([]string, 0)
	return set
}

// Len returns the number of indexed IDs.
func (set *Indexer) Len() int {
	return len(set.IDs)
}

// Add adds a new ID to the indexer.
func (set *Indexer) Add(ID string) {
	if _, exist := set.Indices[ID]; !exist {
		set.Indices[ID] = len(set.IDs)
		set.IDs = append(set.IDs, ID)
	}
}

// ToIndex converts a sparse ID to a dense index.
func (set *Indexer) ToIndex(ID string) int {
	if denseId, exist := set.Indices[ID]; exist {
		return denseId
	}
	return NotId
}

// ToID converts a dense index to a sparse ID.
func (set *Indexer) ToID(index int) string {
	return set.IDs[index]
}

// StringIndexer manages the map between names and indices. The index is the internal index
// optimized for faster parameter access and less memory usage.
type StringIndexer struct {
	Indices map[string]int
	Names   []string
}

// NewStringIndexer creates a StringIndexer.
func NewStringIndexer() *StringIndexer {
	set := new(StringIndexer)
	set.Indices = make(map[string]int)
	set.Names = make([]string, 0)
	return set
}

// Len returns the number of indexed IDs.
func (set *StringIndexer) Len() int {
	return len(set.Names)
}

// Add adds a new ID to the indexer.
func (set *StringIndexer) Add(name string) {
	if _, exist := set.Indices[name]; !exist {
		set.Indices[name] = len(set.Names)
		set.Names = append(set.Names, name)
	}
}

// ToIndex converts a sparse ID to a dense index.
func (set *StringIndexer) ToIndex(name string) int {
	if denseId, exist := set.Indices[name]; exist {
		return denseId
	}
	return NotId
}

// ToName converts an index to a name.
func (set *StringIndexer) ToName(index int) string {
	return set.Names[index]
}

// MarginalSubSet constructs a subset over a list of IDs, indices and values.
type MarginalSubSet struct {
	Indexer *Indexer  // the indexer
	Indices []int     // the full list of indices
	Values  []float64 // the full list of values
	SubSet  []int     // indices of the subset
}

// NewMarginalSubSet creates a MarginalSubSet.
func NewMarginalSubSet(indexer *Indexer, indices []int, values []float64, subset []int) *MarginalSubSet {
	set := new(MarginalSubSet)
	set.Indexer = indexer
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
	return set.GetID(i) < set.GetID(j)
}

// Count gets the size of marginal subset.
func (set *MarginalSubSet) Count() int {
	return len(set.SubSet)
}

// GetIndex returns the index of i-th item.
func (set *MarginalSubSet) GetIndex(i int) int {
	return set.Indices[set.SubSet[i]]
}

// GetID returns the ID of i-th item.
func (set *MarginalSubSet) GetID(i int) string {
	index := set.GetIndex(i)
	return set.Indexer.ToID(index)
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
func (set *MarginalSubSet) Contain(id string) bool {
	// if id is out of range
	if set.Len() == 0 || id < set.GetID(0) || id > set.GetID(set.Len()-1) {
		return false
	}
	// binary search
	low, high := 0, set.Len()-1
	for low <= high {
		// in bound
		if set.GetID(low) == id || set.GetID(high) == id {
			return true
		}
		mid := (low + high) / 2
		// in mid
		if id == set.GetID(mid) {
			return true
		} else if id < set.GetID(mid) {
			low = low + 1
			high = mid - 1
		} else if id > set.GetID(mid) {
			low = mid + 1
			high = high - 1
		}
	}
	return false
}

// ForIntersection iterates items in the intersection of two subsets.
// The method find items with common indices in linear time.
func (set *MarginalSubSet) ForIntersection(other *MarginalSubSet, f func(id string, a, b float64)) {
	// Iterate
	i, j := 0, 0
	for i < set.Len() && j < other.Len() {
		if set.GetID(i) == other.GetID(j) {
			f(set.GetID(i), set.Values[set.SubSet[i]], other.Values[other.SubSet[j]])
			i++
			j++
		} else if set.GetID(i) < other.GetID(j) {
			i++
		} else {
			j++
		}
	}
}

// ForEach iterates items in the subset with IDs.
func (set *MarginalSubSet) ForEach(f func(i int, id string, value float64)) {
	for i, offset := range set.SubSet {
		f(i, set.GetID(i), set.Values[offset])
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
	if vec == nil {
		return 0
	}
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
	if vec != nil {
		for i := range vec.Indices {
			f(i, vec.Indices[i], vec.Values[i])
		}
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

// MaxHeap is designed for store K maximal elements. Heap is used to reduce time complexity
// and memory complexity in top-K searching.
type MaxHeap struct {
	Elem  []interface{} // store elements
	Score []float64     // store scores
	K     int           // the size of heap
}

// NewMaxHeap creates a MaxHeap.
func NewMaxHeap(k int) *MaxHeap {
	knnHeap := new(MaxHeap)
	knnHeap.Elem = make([]interface{}, 0)
	knnHeap.Score = make([]float64, 0)
	knnHeap.K = k
	return knnHeap
}

// Less returns true if the score of i-th item is less than the score of j-th item.
// It is a method of heap.Interface.
func (maxHeap *MaxHeap) Less(i, j int) bool {
	return maxHeap.Score[i] < maxHeap.Score[j]
}

// Swap the i-th item with the j-th item. It is a method of heap.Interface.
func (maxHeap *MaxHeap) Swap(i, j int) {
	maxHeap.Elem[i], maxHeap.Elem[j] = maxHeap.Elem[j], maxHeap.Elem[i]
	maxHeap.Score[i], maxHeap.Score[j] = maxHeap.Score[j], maxHeap.Score[i]
}

// Len returns the size of heap. It is a method of heap.Interface.
func (maxHeap *MaxHeap) Len() int {
	return len(maxHeap.Elem)
}

// _HeapItem is designed for heap.Interface to pass neighborhoods.
type _HeapItem struct {
	Elem  interface{}
	Score float64
}

// Push a neighbors into the MaxHeap. It is a method of heap.Interface.
func (maxHeap *MaxHeap) Push(x interface{}) {
	item := x.(_HeapItem)
	maxHeap.Elem = append(maxHeap.Elem, item.Elem)
	maxHeap.Score = append(maxHeap.Score, item.Score)
}

// Pop the last item (the element with minimal score) in the MaxHeap.
// It is a method of heap.Interface.
func (maxHeap *MaxHeap) Pop() interface{} {
	// Extract the minimum
	n := maxHeap.Len()
	item := _HeapItem{
		Elem:  maxHeap.Elem[n-1],
		Score: maxHeap.Score[n-1],
	}
	// Remove last element
	maxHeap.Elem = maxHeap.Elem[0 : n-1]
	maxHeap.Score = maxHeap.Score[0 : n-1]
	// We never use returned item
	return item
}

// Add a new element to the MaxHeap.
func (maxHeap *MaxHeap) Add(elem interface{}, score float64) {
	// Insert item
	heap.Push(maxHeap, _HeapItem{elem, score})
	// Remove minimum
	if maxHeap.Len() > maxHeap.K {
		heap.Pop(maxHeap)
	}
}

// ToSorted returns a sorted slice od elements in the heap.
func (maxHeap *MaxHeap) ToSorted() ([]interface{}, []float64) {
	// sort indices
	scores := make([]float64, maxHeap.Len())
	indices := make([]int, maxHeap.Len())
	copy(scores, maxHeap.Score)
	floats.Argsort(scores, indices)
	// make output
	sorted := make([]interface{}, maxHeap.Len())
	for i := range indices {
		sorted[i] = maxHeap.Elem[indices[maxHeap.Len()-1-i]]
		scores[i] = maxHeap.Score[indices[maxHeap.Len()-1-i]]
	}
	return sorted, scores
}
