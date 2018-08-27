package core

import (
	"bufio"
	"gonum.org/v1/gonum/floats"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Set map[int]interface{}

type TrainSet struct {
	interactionRatings []float64
	interactionUsers   []int
	interactionItems   []int
}

// Get data set size.
func (set *TrainSet) Length() int {
	return len(set.interactionRatings)
}

// Get all ratings. Return three slice of user IDs, item IDs and ratings.
func (set *TrainSet) Interactions() ([]int, []int, []float64) {
	return set.interactionUsers, set.interactionItems, set.interactionRatings
}

// Get all users. Return a slice of unique user IDs.
func (set *TrainSet) Users() Set {
	return unique(set.interactionUsers)
}

// Get all items. Return a slice of unique item IDs.
func (set *TrainSet) Items() Set {
	return unique(set.interactionItems)
}

// Get the range of ratings. Return minimum and maximum.
func (set *TrainSet) RatingRange() (float64, float64) {
	return floats.Min(set.interactionRatings), floats.Max(set.interactionRatings)
}

func (set *TrainSet) UserHistory() map[int][]int {
	histories := make(map[int][]int)
	users, items, _ := set.Interactions()
	for i := 0; i < len(users); i++ {
		userId := users[i]
		itemId := items[i]
		// Create slice at first time
		if _, exist := histories[userId]; !exist {
			histories[userId] = make([]int, 0)
		}
		// Insert item
		histories[userId] = append(histories[userId], itemId)
	}
	return histories
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
		testFolds[i].interactionUsers = selectInt(set.interactionUsers, testIndex)
		testFolds[i].interactionItems = selectInt(set.interactionItems, testIndex)
		testFolds[i].interactionRatings = selectFloat(set.interactionRatings, testIndex)
		// Train set
		trainTest := concatenate(perm[0:begin], perm[end:set.Length()])
		trainFolds[i].interactionUsers = selectInt(set.interactionUsers, trainTest)
		trainFolds[i].interactionItems = selectInt(set.interactionItems, trainTest)
		trainFolds[i].interactionRatings = selectFloat(set.interactionRatings, trainTest)
		begin = end
	}
	return trainFolds, testFolds
}

func LoadDataFromBuiltIn(dataSetName string) TrainSet {
	// Extract data set information
	dataSet, exist := buildInDataSet[dataSetName]
	if !exist {
		log.Fatal("no such data set", dataSetName)
	}
	const dataFolder = "data"
	const tempFolder = "temp"
	dataFileName := filepath.Join(dataFolder, dataSet.path)
	if _, err := os.Stat(dataFileName); os.IsNotExist(err) {
		zipFileName, _ := DownloadFromUrl(dataSet.url, tempFolder)
		Unzip(zipFileName, dataFolder)
	}
	return LoadDataFromFile(dataFileName, dataSet.sep)
}

func LoadDataFromFile(fileName string, sep string) TrainSet {
	set := TrainSet{}
	set.interactionUsers = make([]int, 0)
	set.interactionItems = make([]int, 0)
	set.interactionRatings = make([]float64, 0)
	// Open file
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	// Read CSV file
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		user, _ := strconv.Atoi(fields[0])
		item, _ := strconv.Atoi(fields[1])
		rating, _ := strconv.Atoi(fields[2])
		set.interactionUsers = append(set.interactionUsers, user)
		set.interactionItems = append(set.interactionItems, item)
		set.interactionRatings = append(set.interactionRatings, float64(rating))
	}
	return set
}

// Utils
