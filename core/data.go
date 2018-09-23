package core

import (
	"archive/zip"
	"bufio"
	"fmt"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

/* Built-in */

// Built-in data set
type _BuiltInDataSet struct {
	url  string
	path string
	sep  string
}

var builtInDataSets = map[string]_BuiltInDataSet{
	"ml-100k": {
		url:  "https://cdn.sine-x.com/datasets/movielens/ml-100k.zip",
		path: "ml-100k/u.data",
		sep:  "\t",
	},
	"ml-1m": {
		url:  "https://cdn.sine-x.com/datasets/movielens/ml-1m.zip",
		path: "ml-1m/ratings.dat",
		sep:  "::",
	},
	"ml-10m": {
		url:  "https://cdn.sine-x.com/datasets/movielens/ml-10m.zip",
		path: "ml-10M100K/ratings.dat",
		sep:  "::",
	},
	"ml-20m": {
		url:  "https://cdn.sine-x.com/datasets/movielens/ml-20m.zip",
		path: "ml-20m/ratings.csv",
		sep:  ",",
	},
}

// The data directories
var (
	downloadDir string
	datasetDir  string
)

func init() {
	usr, _ := user.Current()
	gorseDir := usr.HomeDir + "/.gorse"
	downloadDir = gorseDir + "/download"
	datasetDir = gorseDir + "/datasets"
}

/* Data Set */

// Raw data set. An array of (userId, itemId, rating).
type DataSet struct {
	Ratings []float64
	Users   []int
	Items   []int
}

// Create a new raw data set.
func NewRawSet(users, items []int, ratings []float64) DataSet {
	return DataSet{
		Users:   users,
		Items:   items,
		Ratings: ratings,
	}
}

// Get the number of ratings in the data set.
func (dataSet *DataSet) Length() int {
	return len(dataSet.Ratings)
}

// Get the i-th <userId, itemId, rating>.
func (dataSet *DataSet) Index(i int) (int, int, float64) {
	return dataSet.Users[i], dataSet.Items[i], dataSet.Ratings[i]
}

// Get a subset of the data set.
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
		// Test Data
		testIndex := perm[begin:end]
		testFolds[i] = dataSet.SubSet(testIndex)
		// Train Data
		trainIndex := concatenate(perm[0:begin], perm[end:dataSet.Length()])
		trainFolds[i] = NewTrainSet(dataSet.SubSet(trainIndex))
		begin = end
	}
	return trainFolds, testFolds
}

// Train test split. Return train set and test set.
func (dataSet *DataSet) Split(testSize float64, seed int) (TrainSet, DataSet) {
	rand.Seed(0)
	perm := rand.Perm(dataSet.Length())
	mid := int(float64(dataSet.Length()) * testSize)
	testSet := dataSet.SubSet(perm[:mid])
	trainSet := dataSet.SubSet(perm[mid:])
	return NewTrainSet(trainSet), testSet
}

// Save data set to csv.
func (dataSet *DataSet) ToCSV(fileName string, sep string) error {
	file, err := os.Create(fileName)
	defer file.Close()
	if err == nil {
		writer := bufio.NewWriter(file)
		for i := range dataSet.Ratings {
			writer.WriteString(fmt.Sprintf("%v%s%v%s%v\n",
				dataSet.Users[i], sep,
				dataSet.Items[i], sep,
				dataSet.Ratings[i]))
		}
	}
	return err
}

// Predict ratings for a set of <userId, itemId>s.
func (dataSet *DataSet) Predict(estimator Estimator) []float64 {
	predictions := make([]float64, dataSet.Length())
	for j := 0; j < dataSet.Length(); j++ {
		userId, itemId, _ := dataSet.Index(j)
		predictions[j] = estimator.Predict(userId, itemId)
	}
	return predictions
}

// Train data set.
type TrainSet struct {
	DataSet
	GlobalMean   float64
	UserCount    int
	ItemCount    int
	InnerUserIds map[int]int // userId -> innerUserId
	InnerItemIds map[int]int // itemId -> innerItemId
	userRatings  [][]IdRating
	itemRatings  [][]IdRating
}

// <userId, rating> or <itemId, rating>
type IdRating struct {
	Id     int
	Rating float64
}

// An ID not existed in the data set.
const NewId = -1

// Create a train set from a raw data set.
func NewTrainSet(rawSet DataSet) TrainSet {
	set := TrainSet{}
	set.DataSet = rawSet
	set.GlobalMean = stat.Mean(rawSet.Ratings, nil)
	// Create userId -> innerUserId map
	set.InnerUserIds = make(map[int]int)
	for _, userId := range set.Users {
		if _, exist := set.InnerUserIds[userId]; !exist {
			set.InnerUserIds[userId] = set.UserCount
			set.UserCount++
		}
	}
	// Create itemId -> innerItemId map
	set.InnerItemIds = make(map[int]int)
	for _, itemId := range set.Items {
		if _, exist := set.InnerItemIds[itemId]; !exist {
			set.InnerItemIds[itemId] = set.ItemCount
			set.ItemCount++
		}
	}
	return set
}

// Get the range of LeftRatings. Return minimum and maximum.
func (trainSet *TrainSet) RatingRange() (float64, float64) {
	return floats.Min(trainSet.Ratings), floats.Max(trainSet.Ratings)
}

// Convert user ID to inner user ID
func (trainSet *TrainSet) ConvertUserId(userId int) int {
	if innerUserId, exist := trainSet.InnerUserIds[userId]; exist {
		return innerUserId
	}
	return NewId
}

// Convert item ID to inner item ID
func (trainSet *TrainSet) ConvertItemId(itemId int) int {
	if innerItemId, exist := trainSet.InnerItemIds[itemId]; exist {
		return innerItemId
	}
	return NewId
}

// Get users' LeftRatings: an array of <itemId, rating> for each user.
func (trainSet *TrainSet) UserRatings() [][]IdRating {
	if trainSet.userRatings == nil {
		trainSet.userRatings = make([][]IdRating, trainSet.UserCount)
		for innerUserId := range trainSet.userRatings {
			trainSet.userRatings[innerUserId] = make([]IdRating, 0)
		}
		for i := 0; i < len(trainSet.Users); i++ {
			innerUserId := trainSet.ConvertUserId(trainSet.Users[i])
			innerItemId := trainSet.ConvertItemId(trainSet.Items[i])
			trainSet.userRatings[innerUserId] = append(trainSet.userRatings[innerUserId], IdRating{innerItemId, trainSet.Ratings[i]})
		}
	}
	return trainSet.userRatings
}

// Get items' LeftRatings: an array of <userId, Rating> for each item.
func (trainSet *TrainSet) ItemRatings() [][]IdRating {
	if trainSet.itemRatings == nil {
		trainSet.itemRatings = make([][]IdRating, trainSet.ItemCount)
		for innerItemId := range trainSet.itemRatings {
			trainSet.itemRatings[innerItemId] = make([]IdRating, 0)
		}
		for i := 0; i < len(trainSet.Items); i++ {
			innerUserId := trainSet.ConvertUserId(trainSet.Users[i])
			innerItemId := trainSet.ConvertItemId(trainSet.Items[i])
			trainSet.itemRatings[innerItemId] = append(trainSet.itemRatings[innerItemId], IdRating{innerUserId, trainSet.Ratings[i]})
		}
	}
	return trainSet.itemRatings
}

/* Loader */

// Load build in data set. Now support:
//   ml-100k	- MovieLens 100K
//   ml-1m		- MovieLens 1M
//   ml-10m		- MovieLens 10M
//   ml-20m		- MovieLens 20M
func LoadDataFromBuiltIn(dataSetName string) DataSet {
	// Extract data set information
	dataSet, exist := builtInDataSets[dataSetName]
	if !exist {
		log.Fatal("no such data set ", dataSetName)
	}
	dataFileName := filepath.Join(datasetDir, dataSet.path)
	if _, err := os.Stat(dataFileName); os.IsNotExist(err) {
		zipFileName, _ := downloadFromUrl(dataSet.url, downloadDir)
		unzip(zipFileName, datasetDir)
	}
	return LoadDataFromFile(dataFileName, dataSet.sep, false)
}

// Load data from text file. The text file should be:
//
//   [optional header]
// 	 <userId 1> <sep> <itemId 1> <sep> <rating 1> <sep> <extras>
// 	 <userId 2> <sep> <itemId 2> <sep> <rating 2> <sep> <extras>
// 	 <userId 3> <sep> <itemId 3> <sep> <rating 3> <sep> <extras>
//	 ...
//
// For example, the `u.data` from MovieLens 100K is:
//
//  196\t242\t3\t881250949
//  186\t302\t3\t891717742
//  22\t377\t1\t878887116
//
func LoadDataFromFile(fileName string, sep string, hasHeader bool) DataSet {
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
		// Ignore header
		if hasHeader {
			hasHeader = false
			continue
		}
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

// Download file from URL.
func downloadFromUrl(src string, dst string) (string, error) {
	// Extract file name
	tokens := strings.Split(src, "/")
	fileName := filepath.Join(dst, tokens[len(tokens)-1])
	// Create file
	if err := os.MkdirAll(filepath.Dir(fileName), os.ModePerm); err != nil {
		return fileName, err
	}
	output, err := os.Create(fileName)
	if err != nil {
		fmt.Println("Error while creating", fileName, "-", err)
		return fileName, err
	}
	defer output.Close()
	// Download file
	response, err := http.Get(src)
	if err != nil {
		fmt.Println("Error while downloading", src, "-", err)
		return fileName, err
	}
	defer response.Body.Close()
	// Save file
	_, err = io.Copy(output, response.Body)
	if err != nil {
		fmt.Println("Error while downloading", src, "-", err)
		return fileName, err
	}
	return fileName, nil
}

// Unzip zip file.
func unzip(src string, dst string) ([]string, error) {
	var fileNames []string
	// Open zip file
	r, err := zip.OpenReader(src)
	if err != nil {
		return fileNames, err
	}
	defer r.Close()
	// Extract files
	for _, f := range r.File {
		// Open file
		rc, err := f.Open()
		if err != nil {
			return fileNames, err
		}
		// Store filename/path for returning and using later on
		filePath := filepath.Join(dst, f.Name)
		// Check for ZipSlip. More Info: http://bit.ly/2MsjAWE
		if !strings.HasPrefix(filePath, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fileNames, fmt.Errorf("%s: illegal file path", filePath)
		}
		// Add filename
		fileNames = append(fileNames, filePath)

		if f.FileInfo().IsDir() {
			// Create folder
			os.MkdirAll(filePath, os.ModePerm)
		} else {
			// Create all folders
			if err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
				return fileNames, err
			}
			// Create file
			outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return fileNames, err
			}
			// Save file
			_, err = io.Copy(outFile, rc)
			// Close the file without defer to close before next iteration of loop
			outFile.Close()
			if err != nil {
				return fileNames, err
			}
		}
		// Close file
		rc.Close()
	}
	return fileNames, nil
}

/* Utils */

// Get the mean of ratings for each user (item).
func means(a [][]IdRating) []float64 {
	m := make([]float64, len(a))
	for i := range a {
		sum, count := 0.0, 0.0
		for _, ir := range a[i] {
			sum += ir.Rating
			count++
		}
		m[i] = sum / count
	}
	return m
}

// Sort users' (items') ratings.
func sorts(idRatings [][]IdRating) []SortedIdRatings {
	a := make([]SortedIdRatings, len(idRatings))
	for i := range idRatings {
		a[i] = NewSortedIdRatings(idRatings[i])
	}
	return a
}

// An array of <id, rating> sorted by id.
type SortedIdRatings struct {
	data []IdRating
}

// Create a sorted array of <id, rating>
func NewSortedIdRatings(a []IdRating) SortedIdRatings {
	b := SortedIdRatings{a}
	sort.Sort(b)
	return b
}

func (sir SortedIdRatings) Len() int {
	return len(sir.data)
}

func (sir SortedIdRatings) Swap(i, j int) {
	sir.data[i], sir.data[j] = sir.data[j], sir.data[i]
}

func (sir SortedIdRatings) Less(i, j int) bool {
	return sir.data[i].Id < sir.data[j].Id
}
