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
	commandTest.PersistentFlags().Int("n-split", 5, "number of splits")
	commandTest.PersistentFlags().String("splitter", "loo", "the splitter: loo or k-fold")
	// Evaluators
	commandTest.PersistentFlags().Int("top", 10, "evaluate the model in top N ranking")
	commandTest.PersistentFlags().Int("n-negative", 100, "number of negative samples when evaluating")
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
		k, _ := cmd.PersistentFlags().GetInt("n-split")
		splitterName, _ := cmd.PersistentFlags().GetString("splitter")
		var splitter core.Splitter
		switch splitterName {
		case "loo":
			log.Printf("Use loo splitter\n")
			splitter = core.NewUserLOOSplitter(k)
			break
		case "k-fold":
			log.Printf("Use %d-fold splitter\n", k)
			splitter = core.NewKFoldSplitter(k)
			break
		default:
			log.Fatalf("Unkown splitter %s\n", splitterName)
		}
		// Load evaluators
		evaluatorNames := make([]string, 0)
		n, _ := cmd.PersistentFlags().GetInt("top")
		numSample, _ := cmd.PersistentFlags().GetInt("n-negative")
		scorers := make([]core.Scorer, 0)
		for _, evalFlag := range evalFlags {
			if cmd.PersistentFlags().Changed(evalFlag.Name) {
				scorers = append(scorers, evalFlag.Scorer)
				evaluatorNames = append(evaluatorNames, fmt.Sprintf("%s@%d", evalFlag.Print, n))
			}
		}
		if len(scorers) == 0 {
			scorers = append(scorers, core.NDCG)
			evaluatorNames = append(evaluatorNames, fmt.Sprintf("NDCG@%d", n))
		}
		log.Printf("Use evaluators %v\n", evaluatorNames)
		// Load runtime options
		verbose, _ := cmd.PersistentFlags().GetBool("verbose")
		fitJobs, _ := cmd.PersistentFlags().GetInt("fit-jobs")
		cvJobs, _ := cmd.PersistentFlags().GetInt("cv-jobs")
		options := &base.RuntimeOptions{verbose, fitJobs, cvJobs}
		log.Printf("Runtime options: verbose = %v, fit_jobs = %v, cv_jobs = %v\n",
			options.GetVerbose(), options.GetFitJobs(), options.GetCVJobs())
		// Cross validation
		start := time.Now()
		out := core.CrossValidate(model, data, splitter, 0, options, core.NewEvaluator(n, numSample, scorers...))
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
		log.Printf("Complete cross validation (%v)\n", elapsed)
	},
}

/* Models */

var models = map[string]core.ModelInterface{
	"als":      model.NewALS(nil),
	"item-pop": model.NewItemPop(nil),
	"bpr":      model.NewBPR(nil),
	"knn":      model.NewKNN(nil),
}

/* Flags for evaluators */

type _EvalFlag struct {
	Print  string
	Name   string
	Help   string
	Scorer core.Scorer
}

var evalFlags = []_EvalFlag{
	{Scorer: core.Precision, Print: "Precision", Name: "eval-precision", Help: "evaluate the model by Precision@N"},
	{Scorer: core.Recall, Print: "Recall", Name: "eval-recall", Help: "evaluate the model by Recall@N"},
	{Scorer: core.HR, Print: "HR", Name: "eval-hr", Help: "evaluate the model by HR@N"},
	{Scorer: core.NDCG, Print: "NDCG", Name: "eval-ndcg", Help: "evaluate the model by NDCG@N"},
	{Scorer: core.MAP, Print: "MAP", Name: "eval-map", Help: "evaluate the model by MAP@N"},
	{Scorer: core.MRR, Print: "MRR", Name: "eval-mrr", Help: "evaluate the model by MRR@N"},
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
	{float64Flag, base.InitMean, "set-init-mean", "set mean of gaussian initial parameter"},
	{float64Flag, base.InitStdDev, "set-init-std", "set standard deviation of gaussian initial parameter"},
	{float64Flag, base.Alpha, "set-alpha", "set alpha value, depend on context"},
}
