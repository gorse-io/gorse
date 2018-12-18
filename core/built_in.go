package core

import "os/user"

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
	TempDir     string
)

func init() {
	usr, _ := user.Current()
	gorseDir := usr.HomeDir + "/.gorse"
	downloadDir = gorseDir + "/download"
	dataSetDir = gorseDir + "/datasets"
	TempDir = gorseDir + "/temp"
}
