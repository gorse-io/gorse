package data

import (
	"os"
	"github.com/kniren/gota/dataframe"
	"bufio"
	"math/rand"
)

type Set struct {
	Interactions dataframe.DataFrame
	NRatings     int
}

func (set *Set) AllRatings() []float64 {
	return set.Interactions.Col("X2").Float()
}

func (set *Set) KFold(k int) ([]dataframe.DataFrame, []dataframe.DataFrame) {
	trainFolds := make([]dataframe.DataFrame, k)
	testFolds := make([]dataframe.DataFrame, k)
	perm := rand.Perm(set.NRatings)
	foldSize := set.NRatings / k
	for i := 0; i < set.NRatings; i += foldSize {
		j := i + foldSize
		if j >= set.NRatings {
			j = set.NRatings
		}
		testFolds[i/foldSize] = set.Interactions.Subset(perm[i:j])
		trainFolds[i/foldSize] = set.Interactions.Subset(append(perm[0:i], perm[j:set.NRatings]...))
	}
	return trainFolds, testFolds
}

func LoadDataSet() Set {
	set := Set{}
	set.Interactions = readCSV("data/ml-100k/u.data")
	set.NRatings = set.Interactions.Nrow()
	return set
}

func readCSV(fileName string) dataframe.DataFrame {
	// Read CSV file
	file, _ := os.Open(fileName)
	df := dataframe.ReadCSV(bufio.NewReader(file),
	dataframe.WithDelimiter('\t'),
	dataframe.HasHeader(false))
	return df
}