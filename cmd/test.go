package cmd

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
	"time"
)

func init() {
	modelNames := make([]string, 0)
	for name := range models {
		modelNames = append(modelNames, name)
	}
	commandTest.Long = fmt.Sprintf("Test a model with cross validation. \n\nModels: \n  %s",
		strings.Join(modelNames, " | "))
	// Data loaders
	commandTest.PersistentFlags().String("load-builtin", "", "load data from built-in")
	commandTest.PersistentFlags().String("load-csv", "", "load data from CSV file")
	commandTest.PersistentFlags().String("csv-sep", "\t", "load CSV file with separator")
	commandTest.PersistentFlags().Bool("csv-header", false, "load CSV file with header")
	// Splitter
	commandTest.PersistentFlags().Int("split-fold", 5, "split data by k fold")
	// Evaluators
	commandTest.PersistentFlags().Int("top", 10, "evaluate the model in top N ranking")
	for _, evalFlag := range evalFlags {
		commandTest.PersistentFlags().Bool(evalFlag.Name, false, evalFlag.Help)
	}
	// Hyper-parameters
	for _, paramFlag := range paramFlags {
		switch paramFlag.Type {
		case intFlag:
			commandTest.PersistentFlags().Int(paramFlag.Name, 0, paramFlag.Help)
		case float64Flag:
			commandTest.PersistentFlags().Float64(paramFlag.Name, 0, paramFlag.Help)
		case stringFlag:
			commandTest.PersistentFlags().String(paramFlag.Name, "", paramFlag.Help)
		case boolFlag:
			commandTest.PersistentFlags().Bool(paramFlag.Name, false, paramFlag.Help)
		}
	}
	// Runtime options
	commandTest.PersistentFlags().BoolP("verbose", "v", true, "verbose")
	commandTest.PersistentFlags().Int("fit-jobs", 1, "number of jobs for model fitting")
	commandTest.PersistentFlags().IntP("cv-jobs", "j", 1, "number of jobs for cross validation")
}

var commandTest = &cobra.Command{
	Use:   "test [model]",
	Short: "Test a model by cross validation",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		modelName := args[0]
		var model core.ModelInterface
		var exist bool
		if model, exist = models[modelName]; !exist {
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
		for _, paramFlag := range paramFlags {
			if cmd.PersistentFlags().Changed(paramFlag.Name) {
				switch paramFlag.Type {
				case intFlag:
					value, _ := cmd.PersistentFlags().GetInt(paramFlag.Name)
					params[paramFlag.Key] = value
				case float64Flag:
					value, _ := cmd.PersistentFlags().GetFloat64(paramFlag.Name)
					params[paramFlag.Key] = value
				case stringFlag:
					value, _ := cmd.PersistentFlags().GetString(paramFlag.Name)
					params[paramFlag.Key] = value
				case boolFlag:
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
		for _, evalFlag := range evalFlags {
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
		evaluators := make([]core.CrossValidationEvaluator, 0)
		if len(ratingMetrics) > 0 {
			evaluators = append(evaluators, core.NewRatingEvaluator(ratingMetrics...))
		}
		if len(rankMetrics) > 0 {
			evaluators = append(evaluators, core.NewRankEvaluator(n, rankMetrics...))
		}
		// Load runtime options
		verbose, _ := cmd.PersistentFlags().GetBool("verbose")
		fitJobs, _ := cmd.PersistentFlags().GetInt("fit-jobs")
		cvJobs, _ := cmd.PersistentFlags().GetInt("cv-jobs")
		options := &base.RuntimeOptions{verbose, fitJobs, cvJobs}
		log.Printf("Runtime options: verbose = %v, fit_jobs = %v, cv_jobs = %v\n",
			options.GetVerbose(), options.GetFitJobs(), options.GetCVJobs())
		// Cross validation
		start := time.Now()
		out := core.CrossValidate(model, data, core.NewKFoldSplitter(5), 0, options, evaluators...)
		elapsed := time.Since(start)
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
		log.Printf("Complete cross validation (%v7.0.0.)\n", elapsed)
	},
}

/* Models */

var models = map[string]core.ModelInterface{
	"svd":           model.NewSVD(nil),
	"knn":           model.NewKNN(nil),
	"wrmf":          model.NewWRMF(nil),
	"baseline":      model.NewBaseLine(nil),
	"item-pop":      model.NewItemPop(nil),
	"slope-one":     model.NewSlopOne(nil),
	"co-clustering": model.NewCoClustering(nil),
	"bpr":           model.NewBPR(nil),
	"knn-implicit":  model.NewKNNImplicit(nil),
}

/* Flags for evaluators */

type _EvalFlag struct {
	Rank         bool
	Print        string
	Name         string
	Help         string
	RatingMetric core.RatingMetric
	RankMetric   core.RankMetric
}

var evalFlags = []_EvalFlag{
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
	intFlag     = 0
	float64Flag = 1
	boolFlag    = 2
	stringFlag  = 3
)

type _ParamFlag struct {
	Type int
	Key  base.ParamName
	Name string
	Help string
}

var paramFlags = []_ParamFlag{
	{float64Flag, base.Lr, "set-lr", "set learning rate"},
	{float64Flag, base.Reg, "set-reg", "set regularization strength"},
	{intFlag, base.NEpochs, "set-n-epochs", "set number of epochs"},
	{intFlag, base.NFactors, "set-n-factors", "set number of factors"},
	{intFlag, base.RandomState, "set-random-state", "set random state (seed)"},
	{boolFlag, base.UseBias, "set-use-bias", "set use bias"},
	{float64Flag, base.InitMean, "set-init-mean", "set mean of gaussian initial parameter"},
	{float64Flag, base.InitStdDev, "set-init-std", "set standard deviation of gaussian initial parameter"},
	{float64Flag, base.InitLow, "set-init-low", "set lower bound of uniform initial parameter"},
	{float64Flag, base.InitHigh, "set-init-high", "set upper bound of uniform initial parameter"},
	{intFlag, base.NUserClusters, "set-user-clusters", "set number of user cluster"},
	{intFlag, base.NItemClusters, "set-item-clusters", "set number of item cluster"},
	{stringFlag, base.Type, "set-type", "set type for KNN"},
	{boolFlag, base.UserBased, "set-user-based", "set user based if true. otherwise item based."},
	{stringFlag, base.Similarity, "set-similarity", "set similarity metrics"},
	{stringFlag, base.K, "set-k", "set number of neighbors"},
	{stringFlag, base.MinK, "set-mink", "set least number of neighbors"},
	{stringFlag, base.Optimizer, "set-optimizer", "set optimizer for optimization (sgd/bpr)"},
	{intFlag, base.Shrinkage, "set-shrinkage", "set shrinkage strength of similarity"},
	{float64Flag, base.Alpha, "set-alpha", "set alpha value, depend on context"},
}
