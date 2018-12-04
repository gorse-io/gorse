package core

import (
	"archive/zip"
	"bufio"
	"fmt"
	. "github.com/zhenghaoz/gorse/core/base"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"io"
	"log"
	"math"
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
	url    string
	path   string
	sep    string
	loader func(string, string, bool) DataSet
}

var builtInDataSets = map[string]_BuiltInDataSet{
	"ml-100k": {
		url:    "https://cdn.sine-x.com/datasets/movielens/ml-100k.zip",
		path:   "ml-100k/u.data",
		sep:    "\t",
		loader: LoadDataFromFile,
	},
	"ml-1m": {
		url:    "https://cdn.sine-x.com/datasets/movielens/ml-1m.zip",
		path:   "ml-1m/ratings.dat",
		sep:    "::",
		loader: LoadDataFromFile,
	},
	"ml-10m": {
		url:    "https://cdn.sine-x.com/datasets/movielens/ml-10m.zip",
		path:   "ml-10M100K/ratings.dat",
		sep:    "::",
		loader: LoadDataFromFile,
	},
	"ml-20m": {
		url:    "https://cdn.sine-x.com/datasets/movielens/ml-20m.zip",
		path:   "ml-20m/ratings.csv",
		sep:    ",",
		loader: LoadDataFromFile,
	},
	"netflix": {
		url:    "https://cdn.sine-x.com/datasets/netflix/netflix-prize-data.zip",
		path:   "netflix/training_set.txt",
		sep:    ",",
		loader: LoadDataFromNetflix,
	},
}

// The data directories
var (
	downloadDir string
	dataSetDir  string
	tempDir     string
)

func init() {
	usr, _ := user.Current()
	gorseDir := usr.HomeDir + "/.gorse"
	downloadDir = gorseDir + "/download"
	dataSetDir = gorseDir + "/datasets"
	tempDir = gorseDir + "/_userCluster"
}

/* Data Set */

type DataSet interface {
	Length() int
	Index(i int) (int, int, float64)
	Mean() float64
	StdDev() float64
	Min() float64
	Max() float64
	ForEach(f func(userId, itemId int, rating float64))
	SubSet(indices []int) DataSet
}

// RawDataSet is an array of (userId, itemId, rating).
type RawDataSet struct {
	Ratings []float64
	Users   []int
	Items   []int
}

// NewRawDataSet creates a new raw data set.
func NewRawDataSet(users, items []int, ratings []float64) *RawDataSet {
	return &RawDataSet{
		Users:   users,
		Items:   items,
		Ratings: ratings,
	}
}

// Length returns the number of ratings in the data set.
func (dataSet *RawDataSet) Length() int {
	return len(dataSet.Ratings)
}

// Index returns the i-th <userId, itemId, rating>.
func (dataSet *RawDataSet) Index(i int) (int, int, float64) {
	return dataSet.Users[i], dataSet.Items[i], dataSet.Ratings[i]
}

func (dataSet *RawDataSet) ForEach(f func(userId, itemId int, rating float64)) {
	for i := 0; i < dataSet.Length(); i++ {
		f(dataSet.Users[i], dataSet.Items[i], dataSet.Ratings[i])
	}
}

func (dataSet *RawDataSet) Mean() float64 {
	return stat.Mean(dataSet.Ratings, nil)
}

func (dataSet *RawDataSet) StdDev() float64 {
	return stat.StdDev(dataSet.Ratings, nil)
}

func (dataSet *RawDataSet) Min() float64 {
	return floats.Min(dataSet.Ratings)
}

func (dataSet *RawDataSet) Max() float64 {
	return floats.Max(dataSet.Ratings)
}

// Subset returns a subset of the data set.
func (dataSet *RawDataSet) SubSet(indices []int) DataSet {
	return NewVirtualDataSet(dataSet, indices)
}

// VirtualDataSet is a virtual subset of RawDataSet.
type VirtualDataSet struct {
	data  *RawDataSet
	index []int
}

// NewVirtualDataSet creates a new virtual data set.
func NewVirtualDataSet(dataSet *RawDataSet, index []int) *VirtualDataSet {
	return &VirtualDataSet{
		data:  dataSet,
		index: index,
	}
}

func (dataSet *VirtualDataSet) Length() int {
	return len(dataSet.index)
}

func (dataSet *VirtualDataSet) Index(i int) (int, int, float64) {
	indexInData := dataSet.index[i]
	return dataSet.data.Index(indexInData)
}

func (dataSet *VirtualDataSet) ForEach(f func(userId, itemId int, rating float64)) {
	for i := 0; i < dataSet.Length(); i++ {
		userId, itemId, rating := dataSet.Index(i)
		f(userId, itemId, rating)
	}
}

func (dataSet *VirtualDataSet) Mean() float64 {
	mean := 0.0
	dataSet.ForEach(func(userId, itemId int, rating float64) {
		mean += rating
	})
	return mean / float64(dataSet.Length())
}

func (dataSet *VirtualDataSet) StdDev() float64 {
	mean := dataSet.Mean()
	sum := 0.0
	dataSet.ForEach(func(userId, itemId int, rating float64) {
		sum += (rating - mean) * (rating - mean)
	})
	return math.Sqrt(mean / float64(dataSet.Length()))
}

func (dataSet *VirtualDataSet) Min() float64 {
	_, _, min := dataSet.Index(0)
	dataSet.ForEach(func(userId, itemId int, rating float64) {
		if rating < min {
			rating = min
		}
	})
	return min
}

func (dataSet *VirtualDataSet) Max() float64 {
	_, _, max := dataSet.Index(0)
	dataSet.ForEach(func(userId, itemId int, rating float64) {
		if rating > max {
			max = rating
		}
	})
	return max
}

// Subset returns a subset of the data set.
func (dataSet *VirtualDataSet) SubSet(indices []int) DataSet {
	rawIndices := make([]int, len(indices))
	for i, index := range indices {
		rawIndices[i] = dataSet.index[index]
	}
	return NewVirtualDataSet(dataSet.data, rawIndices)
}

// Train test split. Return train set and test set.
func Split(dataSet DataSet, testSize float64, seed int) (DataSet, DataSet) {
	rand.Seed(0)
	perm := rand.Perm(dataSet.Length())
	mid := int(float64(dataSet.Length()) * testSize)
	testSet := dataSet.SubSet(perm[:mid])
	trainSet := dataSet.SubSet(perm[mid:])
	return trainSet, testSet
}

// Save data set to csv.
func (dataSet *RawDataSet) ToCSV(fileName string, sep string) error {
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
func Predict(dataSet DataSet, estimator Model) []float64 {
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
	GlobalMean  float64
	UserRatings []SparseVector
	ItemRatings []SparseVector
	UserIdSet   SparseIdSet // Users' ID set
	ItemIdSet   SparseIdSet // Items' ID set
}

// <userId, rating> or <itemId, rating>
type IdRating struct {
	Id     int
	Rating float64
}

// NewTrainSet creates a train set from a raw data set.
func NewTrainSet(rawSet DataSet) TrainSet {
	set := TrainSet{}
	set.DataSet = rawSet
	set.GlobalMean = rawSet.Mean()
	// Create IdSet
	set.UserIdSet = MakeSparseIdSet()
	set.ItemIdSet = MakeSparseIdSet()
	rawSet.ForEach(func(userId, itemId int, rating float64) {
		set.UserIdSet.Add(userId)
		set.ItemIdSet.Add(itemId)
	})
	set.UserRatings = MakeDenseSparseMatrix(set.UserCount())
	set.ItemRatings = MakeDenseSparseMatrix(set.ItemCount())
	rawSet.ForEach(func(userId, itemId int, rating float64) {
		userDenseId := set.UserIdSet.ToDenseId(userId)
		itemDenseId := set.ItemIdSet.ToDenseId(itemId)
		set.UserRatings[userDenseId].Add(itemDenseId, rating)
		set.ItemRatings[itemDenseId].Add(userDenseId, rating)
	})
	return set
}

func (trainSet *TrainSet) UserCount() int {
	return trainSet.UserIdSet.Length()
}

func (trainSet *TrainSet) ItemCount() int {
	return trainSet.ItemIdSet.Length()
}

// Range gets the range of ratings. Return minimum and maximum.
func (trainSet *TrainSet) Range() (float64, float64) {
	return trainSet.Min(), trainSet.Max()
}

func (trainSet *TrainSet) Stat() map[string]float64 {
	ret := make(map[string]float64)
	////
	//sum := 0
	//for _, irs := range trainSet.UserRatings() {
	//	sum += len(irs)
	//}
	//ret["n_rating_per_user"] = float64(sum) / float64(trainSet.UserCount())
	return ret
}

/* Loader */

// LoadDataFromBuiltIn loads a built-in data set. Now support:
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
	dataFileName := filepath.Join(dataSetDir, dataSet.path)
	if _, err := os.Stat(dataFileName); os.IsNotExist(err) {
		zipFileName, _ := downloadFromUrl(dataSet.url, downloadDir)
		unzip(zipFileName, dataSetDir)
	}
	return dataSet.loader(dataFileName, dataSet.sep, false)
}

// LoadDataFromFile loads data from a text file. The text file should be:
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
		rating, _ := strconv.ParseFloat(fields[2], 32)
		users = append(users, user)
		items = append(items, item)
		ratings = append(ratings, rating)
	}
	return NewRawDataSet(users, items, ratings)
}

func LoadDataFromNetflix(fileName string, sep string, hasHeader bool) DataSet {
	users := make([]int, 0)
	items := make([]int, 0)
	ratings := make([]float64, 0)
	// Open file
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	// Read file
	scanner := bufio.NewScanner(file)
	itemId := -1
	for scanner.Scan() {
		line := scanner.Text()
		if line[len(line)-1] == ':' {
			// <itemId>:
			if itemId, err = strconv.Atoi(line[0 : len(line)-1]); err != nil {
				log.Fatal(err)
			}
		} else {
			// <userId>, <rating>, <date>
			fields := strings.Split(line, ",")
			userId, _ := strconv.Atoi(fields[0])
			rating, _ := strconv.Atoi(fields[1])
			users = append(users, userId)
			items = append(items, itemId)
			ratings = append(ratings, float64(rating))
		}
	}
	return NewRawDataSet(users, items, ratings)
}

// Download file from URL.
func downloadFromUrl(src string, dst string) (string, error) {
	fmt.Printf("Download dataset from %s\n", src)
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
	fmt.Printf("Unzip dataset %s\n", src)
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
func means(a []SparseVector) []float64 {
	m := make([]float64, len(a))
	for i := range a {
		m[i] = stat.Mean(a[i].Values, nil)
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

// SortedIdRatings is an array of <id, rating> sorted by id.
type SortedIdRatings struct {
	data []IdRating
}

// NewSortedIdRatings creates a sorted array of <id, rating>
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
