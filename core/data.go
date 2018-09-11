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

// Raw data set. An array of (userId, itemId, rating).
type DataSet struct {
	Ratings []float64
	Users   []int
	Items   []int
}

func NewRawSet(users, items []int, ratings []float64) DataSet {
	return DataSet{
		Users:   users,
		Items:   items,
		Ratings: ratings,
	}
}

func (dataSet *DataSet) Length() int {
	return len(dataSet.Ratings)
}

func (dataSet *DataSet) Index(i int) (int, int, float64) {
	return dataSet.Users[i], dataSet.Items[i], dataSet.Ratings[i]
}

func (dataSet *DataSet) SubSet(indices []int) DataSet {
	return NewRawSet(selectInt(dataSet.Users, indices),
		selectInt(dataSet.Items, indices),
		selectFloat(dataSet.Ratings, indices))
}

func (dataSet *DataSet) KFold(k int, seed int64) ([]TrainSet, []DataSet) {
	trainFolds := make([]TrainSet, k)
	testFolds := make([]DataSet, k)
	rand.Seed(0)
	perm := rand.Perm(dataSet.Length())
	foldSize := dataSet.Length() / k
	begin, end := 0, 0
	for i := 0; i < k; i++ {
		end += foldSize
		if i < dataSet.Length()%k {
			end++
		}
		// Test trainSet
		testIndex := perm[begin:end]
		testFolds[i] = dataSet.SubSet(testIndex)
		// Train trainSet
		trainIndex := concatenate(perm[0:begin], perm[end:dataSet.Length()])
		trainFolds[i] = NewTrainSet(dataSet.SubSet(trainIndex))
		begin = end
	}
	return trainFolds, testFolds
}

// Train data set.
type TrainSet struct {
	DataSet
	userCount    int
	itemCount    int
	innerUserIds map[int]int // userId -> innerUserId
	innerItemIds map[int]int // itemId -> innerItemId
	userRatings  [][]float64
	itemRatings  [][]float64
}

type Set map[int]interface{}

const newId = -1

// Get all users. Return a slice of unique user IDs.
func (trainSet *TrainSet) UserSet() Set {
	return unique(trainSet.Users)
}

// Get all items. Return a slice of unique item IDs.
func (trainSet *TrainSet) ItemSet() Set {
	return unique(trainSet.Items)
}

// Get the range of ratings. Return minimum and maximum.
func (trainSet *TrainSet) RatingRange() (float64, float64) {
	return floats.Min(trainSet.Ratings), floats.Max(trainSet.Ratings)
}

func (trainSet *TrainSet) GlobalMean() float64 {
	return stat.Mean(trainSet.Ratings, nil)
}

func (trainSet *TrainSet) UserCount() int {
	return trainSet.userCount
}

func (trainSet *TrainSet) ItemCount() int {
	return trainSet.itemCount
}

func (trainSet *TrainSet) ConvertUserId(userId int) int {
	if innerUserId, exist := trainSet.innerUserIds[userId]; exist {
		return innerUserId
	}
	return newId
}

func (trainSet *TrainSet) ConvertItemId(itemId int) int {
	if innerItemId, exist := trainSet.innerItemIds[itemId]; exist {
		return innerItemId
	}
	return newId
}

func (trainSet *TrainSet) UserRatings() [][]float64 {
	if trainSet.userRatings == nil {
		trainSet.userRatings = newNanMatrix(trainSet.userCount, trainSet.itemCount)
		for i := 0; i < len(trainSet.Users); i++ {
			innerUserId := trainSet.ConvertUserId(trainSet.Users[i])
			innerItemId := trainSet.ConvertItemId(trainSet.Items[i])
			trainSet.userRatings[innerUserId][innerItemId] = trainSet.Ratings[i]
		}
	}
	return trainSet.userRatings
}

func (trainSet *TrainSet) ItemRatings() [][]float64 {
	if trainSet.itemRatings == nil {
		trainSet.itemRatings = newNanMatrix(trainSet.itemCount, trainSet.userCount)
		for i := 0; i < len(trainSet.Users); i++ {
			innerUserId := trainSet.ConvertUserId(trainSet.Users[i])
			innerItemId := trainSet.ConvertItemId(trainSet.Items[i])
			trainSet.itemRatings[innerItemId][innerUserId] = trainSet.Ratings[i]
		}
	}
	return trainSet.itemRatings
}

// TODO: Train test split. Return train set and test set.
func (trainSet *TrainSet) Split(testSize int) (TrainSet, TrainSet) {
	return TrainSet{}, TrainSet{}
}

func NewTrainSet(rawSet DataSet) TrainSet {
	set := TrainSet{}
	set.DataSet = rawSet
	// Create userId -> innerUserId map
	set.innerUserIds = make(map[int]int)
	for _, userId := range set.Users {
		if _, exist := set.innerUserIds[userId]; !exist {
			set.innerUserIds[userId] = set.userCount
			set.userCount++
		}
	}
	// Create itemId -> innerItemId map
	set.innerItemIds = make(map[int]int)
	for _, itemId := range set.Items {
		if _, exist := set.innerItemIds[itemId]; !exist {
			set.innerItemIds[itemId] = set.itemCount
			set.itemCount++
		}
	}
	return set
}

// Load build in data set
func LoadDataFromBuiltIn(dataSetName string) DataSet {
	// Extract data set information
	dataSet, exist := buildInDataSets[dataSetName]
	if !exist {
		log.Fatal("no such data set", dataSetName)
	}
	const dataFolder = "data"
	const tempFolder = "temp"
	dataFileName := filepath.Join(dataFolder, dataSet.path)
	if _, err := os.Stat(dataFileName); os.IsNotExist(err) {
		zipFileName, _ := downloadFromUrl(dataSet.url, tempFolder)
		unzip(zipFileName, dataFolder)
	}
	return LoadDataFromFile(dataFileName, dataSet.sep)
}

// Load data from file
func LoadDataFromFile(fileName string, sep string) DataSet {
	users := make([]int, 0)
	items := make([]int, 0)
	ratings := make([]float64, 0)
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
		users = append(users, user)
		items = append(items, item)
		ratings = append(ratings, float64(rating))
	}
	return NewRawSet(users, items, ratings)
}
