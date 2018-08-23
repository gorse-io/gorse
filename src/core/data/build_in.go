package data

import (
	"bufio"
	"github.com/kniren/gota/dataframe"
	"gonum.org/v1/gonum/floats"
	"math/rand"
	"os"
)

type Set struct {
	Interactions dataframe.DataFrame
	NRatings     int
	users        []int
	items        []int
	UserMap      map[int]int
	ItemMap      map[int]int
}

func Unique(a []int) []int {
	memo := make(map[int]bool)
	ret := make([]int, 0, len(a))
	for _, val := range a {
		if _, exist := memo[val]; !exist {
			memo[val] = true
			ret = append(ret, val)
		}
	}
	return ret
}

func (set *Set) NRow() int {
	return set.Interactions.Nrow()
}

func (set *Set) AllInteraction() ([]int, []int, []float64) {
	users, _ := set.Interactions.Col("X0").Int()
	items, _ := set.Interactions.Col("X1").Int()
	ratings := set.Interactions.Col("X2").Float()
	return users, items, ratings
}

func (set *Set) AllUsers() []int {
	return set.users
}

func (set *Set) AllItems() []int {
	return set.items
}

func (set *Set) AllRatings() []float64 {
	return set.Interactions.Col("X2").Float()
}

func (set *Set) RatingRange() (float64, float64) {
	ratings := set.AllRatings()
	return floats.Min(ratings), floats.Max(ratings)
}

func (set *Set) KFold(k int, seed int64) ([]dataframe.DataFrame, []dataframe.DataFrame) {
	trainFolds := make([]dataframe.DataFrame, k)
	testFolds := make([]dataframe.DataFrame, k)
	rand.Seed(seed)
	perm := rand.Perm(set.NRatings)
	foldSize := set.NRatings / k
	for i := 0; i < k; i++ {
		begin := i * foldSize
		end := begin + foldSize
		if i+1 == k {
			end = set.NRatings
		}
		testFolds[i] = set.Interactions.Subset(perm[begin:end]).Copy()
		trainFolds[i] = set.Interactions.Subset(append(perm[0:begin], perm[end:set.NRatings]...)).Copy()
	}
	return trainFolds, testFolds
}

func NewDataSet(df dataframe.DataFrame) Set {
	set := Set{}
	set.Interactions = df
	set.NRatings = set.Interactions.Nrow()
	// Create user table
	users, _ := set.Interactions.Col("X0").Int()
	set.users = Unique(users)
	set.UserMap = make(map[int]int)
	for innerUserId, userId := range set.users {
		set.UserMap[userId] = innerUserId
	}
	// Create item table
	items, _ := set.Interactions.Col("X1").Int()
	set.items = Unique(items)
	set.ItemMap = make(map[int]int)
	for innerItemId, itemId := range set.items {
		set.ItemMap[itemId] = innerItemId
	}
	return set
}

func LoadDataSet() Set {
	return NewDataSet(readCSV("data/ml-100k/u.data"))
}

func readCSV(fileName string) dataframe.DataFrame {
	// Read CSV file
	file, _ := os.Open(fileName)
	df := dataframe.ReadCSV(bufio.NewReader(file),
		dataframe.WithDelimiter('\t'),
		dataframe.HasHeader(false))
	return df
}
