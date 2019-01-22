package core

import (
	"runtime"
)

/* Options for Model.Fit()  */

// FitOptions defines options used when model fitting.
type FitOptions struct {
	Verbose bool // Verbose switch
	NJobs   int  // Number of jobs
}

// NewFitOptions creates a (FitOptions) object from (FitOption)s.
func NewFitOptions(setters []FitOption) *FitOptions {
	options := new(FitOptions)
	options.NJobs = runtime.NumCPU()
	options.Verbose = true
	for _, setter := range setters {
		setter(options)
	}
	return options
}

// FitOption is used to change (FitOptions).
type FitOption func(options *FitOptions)

// WithVerbose sets the verbose switch
func WithVerbose(verbose bool) FitOption {
	return func(options *FitOptions) {
		options.Verbose = verbose
	}
}

// WithNJobs sets the number of jobs.
func WithNJobs(nJobs int) FitOption {
	return func(options *FitOptions) {
		options.NJobs = nJobs
	}
}

/* Options for CrossValidate() */

// CVOptions defines options used when cross validation.
type CVOptions struct {
	FitOptions       // Options for model fitting
	Seed       int64 // Random seed
}

// CVOption is used to change (CVOptions).
type CVOption func(options *CVOptions)

// NewCVOptions creates a (FitOptions) object from (FitOption)s.
func NewCVOptions(setters []CVOption) *CVOptions {
	options := new(CVOptions)
	options.NJobs = runtime.NumCPU()
	options.Verbose = true
	for _, setter := range setters {
		setter(options)
	}
	return options
}

/* Options for Evaluator */

// EvaluatorOptions defines options used by evaluator.
type EvaluatorOptions struct {
	trainSet *DataSet
	nJobs    int
}

// EvaluatorOption is used to change EvaluatorOptions.
type EvaluatorOption func(*EvaluatorOptions)

// NewEvaluatorOptions create a EvaluatorOptions from EvaluatorOption.
func NewEvaluatorOptions(isRanking bool, option []EvaluatorOption) *EvaluatorOptions {
	options := new(EvaluatorOptions)
	for _, opt := range option {
		opt(options)
	}
	return options
}

// WithTrainSet passes the training set to the evaluator.
func WithTrainSet(trainSet *DataSet) EvaluatorOption {
	return func(options *EvaluatorOptions) {
		options.trainSet = trainSet
	}
}

// WithJobs sets the number of jobs.
func WithJobs(n int) EvaluatorOption {
	return func(options *EvaluatorOptions) {
		options.nJobs = n
	}
}
