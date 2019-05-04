package test

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/model"
	"log"
	"os"
	"strings"
)

func init() {
	modelNames := make([]string, 0)
	for name := range Models {
		modelNames = append(modelNames, name)
	}
	CmdTest.Long = fmt.Sprintf("Test a model with cross validation. Models includes %s.",
		strings.Join(modelNames, ", "))
	// Data loaders
	CmdTest.PersistentFlags().String("load-builtin", "", "load data from built-in")
	CmdTest.PersistentFlags().String("load-csv", "", "load data from CSV file")
	CmdTest.PersistentFlags().String("csv-sep", "\t", "load CSV file with separator")
	CmdTest.PersistentFlags().Bool("csv-header", false, "load CSV file with header")
	// Splitter
	CmdTest.PersistentFlags().Int("split-fold", 5, "split data by k fold")
	// Evaluators
	CmdTest.PersistentFlags().Int("top", 10, "evaluate the model in top N ranking")
	for _, evalFlag := range EvalFlags {
		CmdTest.PersistentFlags().Bool(evalFlag.Name, false, evalFlag.Help)
	}
	// Hyper-parameters
	for _, paramFlag := range ParamFlags {
		switch paramFlag.Type {
		case INT:
			CmdTest.PersistentFlags().Int(paramFlag.Name, 0, paramFlag.Help)
		case FLOAT64:
			CmdTest.PersistentFlags().Float64(paramFlag.Name, 0, paramFlag.Help)
		case STRING:
			CmdTest.PersistentFlags().String(paramFlag.Name, "", paramFlag.Help)
		case BOOL:
			CmdTest.PersistentFlags().Bool(paramFlag.Name, false, paramFlag.Help)
		}
	}
}

var CmdTest = &cobra.Command{
	Use:   "test [model]",
	Short: "Test a model by cross validation",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		modelName := args[0]
		var model core.ModelInterface
		var exist bool
		if model, exist = Models[modelName]; !exist {
			log.Fatalf("Unknown model %s\n", modelName)
		}
		// Load data
		var data *core.DataSet
		if cmd.PersistentFlags().Changed("load-builtin") {
			name, _ := cmd.PersistentFlags().GetString("load-builtin")
			data = core.LoadDataFromBuiltIn(name)
			log.Printf("Load built-in dataset %s\n", name)
		} else if cmd.PersistentFlags().Changed("load-csv") {
			name, _ := cmd.PersistentFlags().GetString("load-csv")
			sep, _ := cmd.PersistentFlags().GetString("csv-sep")
			header, _ := cmd.PersistentFlags().GetBool("csv-header")
			data = core.LoadDataFromCSV(name, sep, header)
		} else {
			data = core.LoadDataFromBuiltIn("ml-100k")
			log.Println("Load default dataset ml-100k")
		}
		// Load hyper-parameters
		params := make(base.Params)
		for _, paramFlag := range ParamFlags {
			if cmd.PersistentFlags().Changed(paramFlag.Name) {
				switch paramFlag.Type {
				case INT:
					value, _ := cmd.PersistentFlags().GetInt(paramFlag.Name)
					params[paramFlag.Key] = value
				case FLOAT64:
					value, _ := cmd.PersistentFlags().GetFloat64(paramFlag.Name)
					params[paramFlag.Key] = value
				case STRING:
					value, _ := cmd.PersistentFlags().GetString(paramFlag.Name)
					params[paramFlag.Key] = value
				case BOOL:
					value, _ := cmd.PersistentFlags().GetBool(paramFlag.Name)
					params[paramFlag.Key] = value
				}
			}
		}
		log.Printf("Load hyper-parameters %v\n", params)
		model.SetParams(params)
		// Load splitter
		k, _ := cmd.PersistentFlags().GetInt("split-fold")
		log.Printf("Use %d-fold splitter\n", k)
		// Load evaluators
		evaluatorNames := make([]string, 0)
		n, _ := cmd.PersistentFlags().GetInt("top")
		rankMetrics := make([]core.RankMetric, 0)
		ratingMetrics := make([]core.RatingMetric, 0)
		evalChanged := false
		for _, evalFlag := range EvalFlags {
			if cmd.PersistentFlags().Changed(evalFlag.Name) {
				evalChanged = true
				if evalFlag.Rank {
					rankMetrics = append(rankMetrics, evalFlag.RankMetric)
					evaluatorNames = append(evaluatorNames, fmt.Sprintf("%s@%d", evalFlag.Print, n))
				} else {
					ratingMetrics = append(ratingMetrics, evalFlag.RatingMetric)
					evaluatorNames = append(evaluatorNames, evalFlag.Print)
				}
			}
		}
		if !evalChanged {
			ratingMetrics = append(ratingMetrics, core.RMSE)
			evaluatorNames = append(evaluatorNames, "RMSE")
		}
		log.Printf("Use evaluators %v\n", evaluatorNames)
		evaluators := make([]core.CVEvaluator, 0)
		if len(ratingMetrics) > 0 {
			evaluators = append(evaluators, core.NewRatingEvaluator(ratingMetrics...))
		}
		if len(rankMetrics) > 0 {
			evaluators = append(evaluators, core.NewRankEvaluator(n, rankMetrics...))
		}
		// Cross validation
		out := core.CrossValidate(model, data, core.NewKFoldSplitter(5), 0, nil, evaluators...)
		// Render table
		header := make([]string, k+2)
		header[k+1] = "Mean"
		for i := 1; i <= k; i++ {
			header[i] = fmt.Sprintf("Fold %d", i)
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader(header)
		for i, v := range out {
			row := make([]string, k+2)
			row[0] = evaluatorNames[i]
			for i, value := range v.TestScore {
				row[i+1] = fmt.Sprintf("%f", value)
			}
			mean, margin := v.MeanAndMargin()
			row[k+1] = fmt.Sprintf("%f(±%f)", mean, margin)
			table.Append(row)
		}
		table.Render()
		log.Printf("Complete cross validation:\n")
	},
}

/* Models */

var Models = map[string]core.ModelInterface{
	"svd":           model.NewSVD(nil),
	"knn":           model.NewKNN(nil),
	"wrmf":          model.NewWRMF(nil),
	"baseline":      model.NewBaseLine(nil),
	"item-pop":      model.NewItemPop(nil),
	"slope-one":     model.NewSlopOne(nil),
	"co-clustering": model.NewCoClustering(nil),
}

/* Flags for evaluators */

type EvalFlag struct {
	Rank         bool
	Print        string
	Name         string
	Help         string
	RatingMetric core.RatingMetric
	RankMetric   core.RankMetric
}

var EvalFlags = []EvalFlag{
	{Rank: false, RatingMetric: core.RMSE, Print: "RMSE", Name: "eval-rmse", Help: "evaluate the model by RMSE"},
	{Rank: false, RatingMetric: core.MAE, Print: "MAE", Name: "eval-mae", Help: "evaluate the model by MAE"},
	{Rank: true, RankMetric: core.Precision, Print: "Precision", Name: "eval-precision", Help: "evaluate the model by Precision@N"},
	{Rank: true, RankMetric: core.Recall, Print: "Recall", Name: "eval-recall", Help: "evaluate the model by Recall@N"},
	{Rank: true, RankMetric: core.NDCG, Print: "NDCG", Name: "eval-ndcg", Help: "evaluate the model by NDCG@N"},
	{Rank: true, RankMetric: core.MAP, Print: "MAP", Name: "eval-map", Help: "evaluate the model by MAP@N"},
	{Rank: true, RankMetric: core.MRR, Print: "MRR", Name: "eval-mrr", Help: "evaluate the model by MRR@N"},
}

/* Flags for hyper-parameters */

const (
	INT     = 0
	FLOAT64 = 1
	BOOL    = 2
	STRING  = 3
)

type ParamFlag struct {
	Type int
	Key  base.ParamName
	Name string
	Help string
}

var ParamFlags = []ParamFlag{
	{FLOAT64, base.Lr, "set-lr", "set learning rate"},
	{FLOAT64, base.Reg, "set-reg", "set regularization strength"},
	{INT, base.NEpochs, "set-n-epochs", "set number of epochs"},
	{INT, base.NFactors, "set-n-factors", "set number of factors"},
	{INT, base.RandomState, "set-random-state", "set random state (seed)"},
	{BOOL, base.UseBias, "set-use-bias", "set use bias"},
	{FLOAT64, base.InitMean, "set-init-mean", "set mean of gaussian initial parameter"},
	{FLOAT64, base.InitStdDev, "set-init-std", "set standard deviation of gaussian initial parameter"},
	{FLOAT64, base.InitLow, "set-init-low", "set lower bound of uniform initial parameter"},
	{FLOAT64, base.InitHigh, "set-init-high", "set upper bound of uniform initial parameter"},
	{INT, base.NUserClusters, "set-user-clusters", "set number of user cluster"},
	{INT, base.NItemClusters, "set-item-clusters", "set number of item cluster"},
	{STRING, base.Type, "set-type", "set type for KNN"},
	{BOOL, base.UserBased, "set-user-based", "set user based if true. otherwise item based."},
	{STRING, base.Similarity, "set-similarity", "set similarity metrics"},
	{STRING, base.K, "set-k", "set number of neighbors"},
	{STRING, base.MinK, "set-mink", "set least number of neighbors"},
	{STRING, base.Optimizer, "set-optimizer", "set optimizer for optimization (sgd/bpr)"},
	{INT, base.Shrinkage, "set-shrinkage", "set shrinkage strength of similarity"},
	{FLOAT64, base.Alpha, "set-alpha", "set alpha value, depend on context"},
}
