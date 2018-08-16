package main

import (
	"./algo"
	"./data"
	"./cv"
	"golang.org/x/exp/rand"
)

func main() {
	rand.Seed(100)
	random := &algo.Random{}
	set := data.LoadDataSet()
	cv.CrossValidate(random, set, []string{"RMSE"}, 5)
}
