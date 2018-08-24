package core

//import (
//	"testing"
//	"time"
//	"github.com/olekukonko/tablewriter"
//	"os"
//	"fmt"
//	"github.com/gonum/stat"
//)
//
//func TestBenchmark(t *testing.T) {
//	random := NewRandom()
//	set := LoadDataSet()
//	var start time.Time
//	table := tablewriter.NewWriter(os.Stdout)
//	table.SetHeader([]string{"Name", "RMSE", "MAE", "Time"})
//	start = time.Now()
//	out := CrossValidate(random, set, []string{"RMSE"}, 5, 0)
//	tm := time.Since(start)
//	table.Append([]string{"Random",
//		fmt.Sprintf("%f", stat.Mean(out[0], nil)),
//		fmt.Sprintf("%f", stat.Mean(out[1], nil)),
//		fmt.Sprint(tm),
//	})
//	table.Render() // Send output
//	nmf := NewNMF()
//	start = time.Now()
//	out = CrossValidate(nmf, set, []string{"RMSE"}, 5, 0)
//	tm = time.Since(start)
//	table.Append([]string{"NMF",
//		fmt.Sprintf("%f", stat.Mean(out[0],nil)),
//		fmt.Sprintf("%f", stat.Mean(out[1],nil)),
//		fmt.Sprint(tm),
//	})
//	table.Render() // Send output
//	baseline := NewBaseLine()
//	start = time.Now()
//	out = CrossValidate(baseline, set, []string{"RMSE"}, 5, 0)
//	tm = time.Since(start)
//	table.Append([]string{"BaseLine",
//		fmt.Sprintf("%f", stat.Mean(out[0], nil)),
//		fmt.Sprintf("%f", stat.Mean(out[1], nil)),
//		fmt.Sprint(tm),
//	})
//	table.Render() // Send output
//	svd := NewSVD()
//	start = time.Now()
//	out = CrossValidate(svd, set, []string{"RMSE"}, 5, 0)
//	tm = time.Since(start)
//	table.Append([]string{"SVD",
//		fmt.Sprintf("%f", stat.Mean(out[0], nil)),
//		fmt.Sprintf("%f", stat.Mean(out[1], nil)),
//		fmt.Sprint(tm),
//	})
//	table.Render() // Send output
//	pp := NewSVDPP()
//	start = time.Now()
//	out = CrossValidate(pp, set, []string{"RMSE"}, 5, 0)
//	tm = time.Since(start)
//	table.Append([]string{"SVD++",
//		fmt.Sprintf("%f", stat.Mean(out[0], nil)),
//		fmt.Sprintf("%f", stat.Mean(out[1], nil)),
//		fmt.Sprint(tm),
//	})
//	table.Render() // Send output
//}
