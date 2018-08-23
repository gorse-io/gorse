package main

import (
	"core"
	"core/algo"
	"core/data"
	"fmt"
	"github.com/gonum/stat"
	"github.com/olekukonko/tablewriter"
	"os"
	"time"
)

func main() {
	random := algo.NewRandom()
	set := data.LoadDataSet()
	var start time.Time
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "RMSE", "MAE", "Time"})
	start = time.Now()
	out := core.CrossValidate(random, set, []string{"RMSE"}, 5, 0)
	t := time.Since(start)
	table.Append([]string{"Random",
		fmt.Sprintf("%f", stat.Mean(out[0], nil)),
		fmt.Sprintf("%f", stat.Mean(out[1], nil)),
		fmt.Sprint(t),
	})
	table.Render() // Send output
	//nmf := algo.NewNMF()
	//start = time.Now()
	//out = core.CrossValidate(nmf, set, []string{"RMSE"}, 5, 0)
	//t = time.Since(start)
	//table.Append([]string{"NMF",
	//	fmt.Sprintf("%f", stat.Mean(out[0],nil)),
	//	fmt.Sprintf("%f", stat.Mean(out[1],nil)),
	//	fmt.Sprint(t),
	//})
	//table.Render() // Send output
	baseline := algo.NewBaseLine()
	start = time.Now()
	out = core.CrossValidate(baseline, set, []string{"RMSE"}, 5, 0)
	t = time.Since(start)
	table.Append([]string{"BaseLine",
		fmt.Sprintf("%f", stat.Mean(out[0], nil)),
		fmt.Sprintf("%f", stat.Mean(out[1], nil)),
		fmt.Sprint(t),
	})
	table.Render() // Send output
	svd := algo.NewSVD()
	start = time.Now()
	out = core.CrossValidate(svd, set, []string{"RMSE"}, 5, 0)
	t = time.Since(start)
	table.Append([]string{"SVD",
		fmt.Sprintf("%f", stat.Mean(out[0], nil)),
		fmt.Sprintf("%f", stat.Mean(out[1], nil)),
		fmt.Sprint(t),
	})
	table.Render() // Send output
	pp := algo.NewSVDPP()
	start = time.Now()
	out = core.CrossValidate(pp, set, []string{"RMSE"}, 5, 0)
	t = time.Since(start)
	table.Append([]string{"SVD++",
		fmt.Sprintf("%f", stat.Mean(out[0], nil)),
		fmt.Sprintf("%f", stat.Mean(out[1], nil)),
		fmt.Sprint(t),
	})
	table.Render() // Send output
}
