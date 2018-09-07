package core

import (
	"bufio"
	"github.com/gonum/stat"
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
	userCount          int
	itemCount          int
	innerUserIds       map[int]int // userId -> innerUserId
	innerItemIds       map[int]int // itemId -> innerItemId
	userRatings        [][]float64
	itemRatings        [][]float64
}

const newId = -1

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

func (set *TrainSet) GlobalMean() float64 {
	return stat.Mean(set.interactionRatings, nil)
}

func (set *TrainSet) UserCount() int {
	return set.userCount
}

func (set *TrainSet) ItemCount() int {
	return set.itemCount
}

func (set *TrainSet) ConvertUserId(userId int) int {
	if innerUserId, exist := set.innerUserIds[userId]; exist {
		return innerUserId
	}
	return newId
}

func (set *TrainSet) ConvertItemId(itemId int) int {
	if innerItemId, exist := set.innerItemIds[itemId]; exist {
		return innerItemId
	}
	return newId
}

func (set *TrainSet) UserRatings() [][]float64 {
	if set.userRatings == nil {
		set.userRatings = newNanMatrix(set.userCount, set.itemCount)
		users, items, ratings := set.Interactions()
		for i := 0; i < len(users); i++ {
			innerUserId := set.ConvertUserId(users[i])
			innerItemId := set.ConvertItemId(items[i])
			set.userRatings[innerUserId][innerItemId] = ratings[i]
		}
	}
	return set.userRatings
}

func (set *TrainSet) ItemRatings() [][]float64 {
	if set.itemRatings == nil {
		set.itemRatings = newNanMatrix(set.itemCount, set.userCount)
		users, items, ratings := set.Interactions()
		for i := 0; i < len(users); i++ {
			innerUserId := set.ConvertUserId(users[i])
			innerItemId := set.ConvertItemId(items[i])
			set.itemRatings[innerItemId][innerUserId] = ratings[i]
		}
	}
	return set.itemRatings
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
		testFolds[i] = NewTrainSet(selectInt(set.interactionUsers, testIndex),
			selectInt(set.interactionItems, testIndex),
			selectFloat(set.interactionRatings, testIndex))
		// Train set
		trainIndex := concatenate(perm[0:begin], perm[end:set.Length()])
		trainFolds[i] = NewTrainSet(selectInt(set.interactionUsers, trainIndex),
			selectInt(set.interactionItems, trainIndex),
			selectFloat(set.interactionRatings, trainIndex))
		begin = end
	}
	return trainFolds, testFolds
}

// TODO: Train test split. Return train set and test set.
func (set *TrainSet) Split(testSize int) (TrainSet, TrainSet) {
	return TrainSet{}, TrainSet{}
}

func NewTrainSet(users, items []int, ratings []float64) TrainSet {
	set := TrainSet{}
	set.interactionUsers = users
	set.interactionItems = items
	set.interactionRatings = ratings
	// Create userId -> innerUserId map
	set.innerUserIds = make(map[int]int)
	for _, userId := range set.interactionUsers {
		if _, exist := set.innerUserIds[userId]; !exist {
			set.innerUserIds[userId] = set.userCount
			set.userCount++
		}
	}
	// Create itemId -> innerItemId map
	set.innerItemIds = make(map[int]int)
	for _, itemId := range set.interactionItems {
		if _, exist := set.innerItemIds[itemId]; !exist {
			set.innerItemIds[itemId] = set.itemCount
			set.itemCount++
		}
	}
	return set
}

// Load build in data set
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

// Load data from file
func LoadDataFromFile(fileName string, sep string) TrainSet {
	interactionUsers := make([]int, 0)
	interactionItems := make([]int, 0)
	interactionRatings := make([]float64, 0)
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
		fields := strings.Split(line, sep)
		user, _ := strconv.Atoi(fields[0])
		item, _ := strconv.Atoi(fields[1])
		rating, _ := strconv.Atoi(fields[2])
		interactionUsers = append(interactionUsers, user)
		interactionItems = append(interactionItems, item)
		interactionRatings = append(interactionRatings, float64(rating))
	}
	return NewTrainSet(interactionUsers, interactionItems, interactionRatings)
}
