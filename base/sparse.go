package base

import "gonum.org/v1/gonum/stat"

type SparseIdSet struct {
	DenseIds  map[int]int
	SparseIds []int
}

// An ID not existed in the data set.
const NewId = -1

func MakeSparseIdSet() SparseIdSet {
	return SparseIdSet{
		DenseIds:  make(map[int]int),
		SparseIds: make([]int, 0),
	}
}

func (set *SparseIdSet) Length() int {
	return len(set.SparseIds)
}

func (set *SparseIdSet) Add(sparseId int) {
	if _, exist := set.DenseIds[sparseId]; !exist {
		set.DenseIds[sparseId] = len(set.SparseIds)
		set.SparseIds = append(set.SparseIds, len(set.SparseIds))
	}
}

func (set *SparseIdSet) ToDenseId(sparseId int) int {
	if denseId, exist := set.DenseIds[sparseId]; exist {
		return denseId
	}
	return NewId
}

func (set *SparseIdSet) ToSparseId(denseId int) int {
	return set.SparseIds[denseId]
}

type SparseVector struct {
	Indices []int
	Values  []float64
}

func MakeSparseVector() SparseVector {
	return SparseVector{
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

func (vec *SparseVector) Length() int {
	return len(vec.Values)
}

func (vec *SparseVector) Add(index int, value float64) {
	vec.Indices = append(vec.Indices, index)
	vec.Values = append(vec.Values, value)
}

func (vec *SparseVector) SortIndex() {

}

func (vec *SparseVector) ForEach(f func(i, index int, value float64)) {
	for i := range vec.Indices {
		f(i, vec.Indices[i], vec.Values[i])
	}
}

func (vec *SparseVector) ForIntersection(other *SparseVector, f func(index int, a, b float64)) {

}

type AdjacentVector struct {
	Similarities []float64
	Peers        []int
}

func MakeAdjacentVector(k int) AdjacentVector {
	return AdjacentVector{
		Similarities: make([]float64, k),
		Peers:        make([]int, k),
	}
}

func (mat *AdjacentVector) Put(i, j int, similarity float64) {

}

func (mat *AdjacentVector) Get(i int) SparseVector {
	return SparseVector{}
}
