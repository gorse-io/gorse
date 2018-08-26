package core

import (
	"bufio"
	"github.com/kniren/gota/dataframe"
	"gonum.org/v1/gonum/floats"
	"math/rand"
	"os"
	"path/filepath"
)

type TrainSet struct {
	ratings []float64
	users   []int
	items   []int
}

func (set *TrainSet) Length() int {
	return len(set.ratings)
}

func (set *TrainSet) Interactions() ([]int, []int, []float64) {
	return set.users, set.items, set.ratings
}

func (set *TrainSet) Users() []int {
	return set.users
}

func (set *TrainSet) Items() []int {
	return set.items
}

func (set *TrainSet) Ratings() []float64 {
	return set.ratings
}

func (set *TrainSet) RatingRange() (float64, float64) {
	ratings := set.Ratings()
	return floats.Min(ratings), floats.Max(ratings)
}

func (set *TrainSet) KFold(k int, seed int64) ([]TrainSet, []TrainSet) {
	trainFolds := make([]TrainSet, k)
	testFolds := make([]TrainSet, k)
	rand.Seed(0)
	perm := rand.Perm(set.Length())
	foldSize := set.Length() / k
	begin, end := 0, 0
	for i := 0; i < k; i++ {
		end += foldSize
		if i < set.Length()%k {
			end++
		}
		// Test set
		testIndex := perm[begin:end]
		testFolds[i].users = SelectInt(set.users, testIndex)
		testFolds[i].items = SelectInt(set.items, testIndex)
		testFolds[i].ratings = SelectFloat(set.ratings, testIndex)
		// Train set
		trainTest := Concatenate(perm[0:begin], perm[end:set.Length()])
		trainFolds[i].users = SelectInt(set.users, trainTest)
		trainFolds[i].items = SelectInt(set.items, trainTest)
		trainFolds[i].ratings = SelectFloat(set.ratings, trainTest)
		begin = end
	}
	return trainFolds, testFolds
}

func NewDataSet(df dataframe.DataFrame) TrainSet {
	set := TrainSet{}
	set.users, _ = df.Col("X0").Int()
	set.items, _ = df.Col("X1").Int()
	set.ratings = df.Col("X2").Float()
	return set
}

func LoadDataFromBuiltIn() TrainSet {
	const dataFolder = "data"
	const tempFolder = "temp"
	dataFileName := filepath.Join(dataFolder, "ml-100k\\u.data")
	zipFileName := filepath.Join(tempFolder, "ml-100k.zip")
	if _, err := os.Stat(dataFileName); os.IsNotExist(err) {
		DownloadFromUrl("http://files.grouplens.org/datasets/movielens/ml-100k.zip", tempFolder)
		Unzip(zipFileName, dataFolder)
	}
	df := readCSV(dataFileName)
	return NewDataSet(df)
}

func readCSV(fileName string) dataframe.DataFrame {
	// Read CSV file
	file, _ := os.Open(fileName)
	df := dataframe.ReadCSV(bufio.NewReader(file),
		dataframe.WithDelimiter('\t'),
		dataframe.HasHeader(false))
	return df
}

// Utils
