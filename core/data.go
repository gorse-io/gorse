package core

import (
	"archive/zip"
	"bufio"
	"fmt"
	. "github.com/zhenghaoz/gorse/base"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
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
		loader: LoadDataFromCSV,
	},
	"ml-1m": {
		url:    "https://cdn.sine-x.com/datasets/movielens/ml-1m.zip",
		path:   "ml-1m/ratings.dat",
		sep:    "::",
		loader: LoadDataFromCSV,
	},
	"ml-10m": {
		url:    "https://cdn.sine-x.com/datasets/movielens/ml-10m.zip",
		path:   "ml-10M100K/ratings.dat",
		sep:    "::",
		loader: LoadDataFromCSV,
	},
	"ml-20m": {
		url:    "https://cdn.sine-x.com/datasets/movielens/ml-20m.zip",
		path:   "ml-20m/ratings.csv",
		sep:    ",",
		loader: LoadDataFromCSV,
	},
	"netflix": {
		url:    "https://cdn.sine-x.com/datasets/netflix/netflix-prize-data.zip",
		path:   "netflix/training_set.txt",
		loader: LoadDataFromNetflixStyle,
	},
}

// The data directories
var (
	downloadDir string
	dataSetDir  string
)

func init() {
	usr, _ := user.Current()
	gorseDir := usr.HomeDir + "/.gorse"
	downloadDir = gorseDir + "/download"
	dataSetDir = gorseDir + "/datasets"
}

/* Dataset */

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

func (dataSet *RawDataSet) Len() int {
	return len(dataSet.Ratings)
}

func (dataSet *RawDataSet) Get(i int) (int, int, float64) {
	return dataSet.Users[i], dataSet.Items[i], dataSet.Ratings[i]
}

func (dataSet *RawDataSet) ForEach(f func(userId, itemId int, rating float64)) {
	for i := 0; i < dataSet.Len(); i++ {
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

func (dataSet *VirtualDataSet) Len() int {
	return len(dataSet.index)
}

func (dataSet *VirtualDataSet) Get(i int) (int, int, float64) {
	indexInData := dataSet.index[i]
	return dataSet.data.Get(indexInData)
}

func (dataSet *VirtualDataSet) ForEach(f func(userId, itemId int, rating float64)) {
	for i := 0; i < dataSet.Len(); i++ {
		userId, itemId, rating := dataSet.Get(i)
		f(userId, itemId, rating)
	}
}

func (dataSet *VirtualDataSet) Mean() float64 {
	mean := 0.0
	dataSet.ForEach(func(userId, itemId int, rating float64) {
		mean += rating
	})
	return mean / float64(dataSet.Len())
}

func (dataSet *VirtualDataSet) StdDev() float64 {
	mean := dataSet.Mean()
	sum := 0.0
	dataSet.ForEach(func(userId, itemId int, rating float64) {
		sum += (rating - mean) * (rating - mean)
	})
	return math.Sqrt(mean / float64(dataSet.Len()))
}

func (dataSet *VirtualDataSet) Min() float64 {
	_, _, min := dataSet.Get(0)
	dataSet.ForEach(func(userId, itemId int, rating float64) {
		if rating < min {
			rating = min
		}
	})
	return min
}

func (dataSet *VirtualDataSet) Max() float64 {
	_, _, max := dataSet.Get(0)
	dataSet.ForEach(func(userId, itemId int, rating float64) {
		if rating > max {
			max = rating
		}
	})
	return max
}

func (dataSet *VirtualDataSet) SubSet(indices []int) DataSet {
	rawIndices := make([]int, len(indices))
	for i, index := range indices {
		rawIndices[i] = dataSet.index[index]
	}
	return NewVirtualDataSet(dataSet.data, rawIndices)
}

// Train data set.
type TrainSet struct {
	DataSet
	GlobalMean   float64
	DenseUserIds []int
	DenseItemIds []int
	UserRatings  []SparseVector
	ItemRatings  []SparseVector
	UserIdSet    SparseIdSet // Users' ID set
	ItemIdSet    SparseIdSet // Items' ID set
}

// NewTrainSet creates a train set from a raw data set.
func NewTrainSet(rawSet DataSet) TrainSet {
	set := TrainSet{}
	set.DataSet = rawSet
	set.GlobalMean = rawSet.Mean()
	set.DenseItemIds = make([]int, 0)
	set.DenseUserIds = make([]int, 0)
	// Create IdSet
	set.UserIdSet = MakeSparseIdSet()
	set.ItemIdSet = MakeSparseIdSet()
	rawSet.ForEach(func(userId, itemId int, rating float64) {
		set.UserIdSet.Add(userId)
		set.ItemIdSet.Add(itemId)
		set.DenseUserIds = append(set.DenseUserIds, set.UserIdSet.ToDenseId(userId))
		set.DenseItemIds = append(set.DenseItemIds, set.ItemIdSet.ToDenseId(itemId))
	})
	// Create user-based and item-based ratings
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

func (trainSet *TrainSet) GetDense(i int) (int, int, float64) {
	_, _, rating := trainSet.Get(i)
	return trainSet.DenseUserIds[i], trainSet.DenseItemIds[i], rating
}

func (trainSet *TrainSet) UserCount() int {
	return trainSet.UserIdSet.Len()
}

func (trainSet *TrainSet) ItemCount() int {
	return trainSet.ItemIdSet.Len()
}

/* Loader */

// LoadDataFromBuiltIn loads a built-in data set. Now support:
//   ml-100k	- MovieLens 100K
//   ml-1m		- MovieLens 1M
//   ml-10m		- MovieLens 10M
//   ml-20m		- MovieLens 20M
//   netflix    - Netflix Prize
func LoadDataFromBuiltIn(dataSetName string) DataSet {
	// Extract data set information
	dataSet, exist := builtInDataSets[dataSetName]
	if !exist {
		log.Fatal("no such data set ", dataSetName)
	}
	dataFileName := filepath.Join(dataSetDir, dataSet.path)
	if _, err := os.Stat(dataFileName); os.IsNotExist(err) {
		zipFileName, _ := downloadFromUrl(dataSet.url, downloadDir)
		if _, err := unzip(zipFileName, dataSetDir); err != nil {
			panic(err)
		}
	}
	return dataSet.loader(dataFileName, dataSet.sep, false)
}

// LoadDataFromCSV loads data from a CSV file. The CSV file should be:
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
func LoadDataFromCSV(fileName string, sep string, hasHeader bool) DataSet {
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

// LoadDataFromNetflixStyle load data from a Netflix-style file. The CSV file should be:
//
//   <itemId 1>:
//   <userId 1>, <rating 1>, <date>
//   <userId 2>, <rating 2>, <date>
//   <userId 3>, <rating 3>, <date>
//   ...
//
func LoadDataFromNetflixStyle(fileName string, _ string, _ bool) DataSet {
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

func LoadDataFromSQL() DataSet {
	// TODO: Load data from SQL server.
	return nil
}

/* Misc */

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
			if err = os.MkdirAll(filePath, os.ModePerm); err != nil {
				return fileNames, err
			}
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
