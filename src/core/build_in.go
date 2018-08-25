package core

type BuildInDataSet struct {
	url string
	sep string
}

var buildInDataSet = map[string]BuildInDataSet{
	"ml-100k": {url: "http://files.grouplens.org/datasets/movielens/ml-100k.zip", sep: "\t"},
	"ml-1m":   {url: "http://files.grouplens.org/datasets/movielens/ml-1m.zip"},
	"jester":  {url: "http://eigentaste.berkeley.edu/dataset/jester_dataset_2.zip"},
}
