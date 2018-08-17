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
	svd := algo.NewSVDRecommend()
	set := data.LoadDataSet()
	var start time.Time
	start = time.Now()
	fmt.Println(cv.CrossValidate(random, set, []string{"RMSE"}, 5))
	fmt.Println(time.Since(start))
	start = time.Now()
	fmt.Println(cv.CrossValidate(baseline, set, []string{"RMSE"}, 5))
	fmt.Println(time.Since(start))
	start = time.Now()
	fmt.Println(cv.CrossValidate(svd, set, []string{"RMSE"}, 5))
	fmt.Println(time.Since(start))
}
