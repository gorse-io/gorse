package main

import (
	"./algo"
	"./cv"
	"./data"
	"fmt"
	"golang.org/x/exp/rand"
	"time"
)

func main() {
	rand.Seed(uint64(time.Now().UTC().UnixNano()))
	random := algo.NewRandomRecommend()
	baseline := algo.NewBaseLineRecommend()
	set := data.LoadDataSet()
	fmt.Println(cv.CrossValidate(random, set, []string{"RMSE"}, 5))
	fmt.Println(cv.CrossValidate(baseline, set, []string{"RMSE"}, 5))
}
