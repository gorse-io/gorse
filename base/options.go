package base

import "runtime"

// FitOptions defined options used in fitting.
type FitOptions struct {
	Verbose  bool // Verbose switch
	Diagnose bool
	NJobs    int // Number of jobs
}

// NewCVOptions creates a FitOptions from FitOption.
func NewFitOptions(setters []FitOption) *FitOptions {
	options := new(FitOptions)
	options.NJobs = runtime.NumCPU()
	options.Diagnose = true
	options.Verbose = true
	for _, setter := range setters {
		setter(options)
	}
	return options
}

// FitOption changes options.
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

type CVOptions struct {
	FitOptions
	Seed int64
}

type CVOption func(options *CVOptions)

// NewCVOptions creates a FitOptions from FitOption.
func NewCVOptions(setters []CVOption) *CVOptions {
	options := new(CVOptions)
	options.NJobs = 1
	options.Diagnose = true
	options.Verbose = true
	for _, setter := range setters {
		setter(options)
	}
	return options
}
