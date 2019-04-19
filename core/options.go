package core

/* Options for Model.Fit()  */

// RuntimeOptions defines options used for runtime.
type RuntimeOptions struct {
	Verbose bool // Verbose switch
	NJobs   int  // Number of jobs
}

// NewRuntimeOptions creates a (RuntimeOptions) object from (RuntimeOption)s.
func NewRuntimeOptions(setters []RuntimeOption) *RuntimeOptions {
	options := new(RuntimeOptions)
	options.NJobs = 1 //runtime.NumCPU()
	options.Verbose = true
	for _, setter := range setters {
		setter(options)
	}
	return options
}

// RuntimeOption is used to change (RuntimeOptions).
type RuntimeOption func(options *RuntimeOptions)

// WithVerbose sets the verbose switch
func WithVerbose(verbose bool) RuntimeOption {
	return func(options *RuntimeOptions) {
		options.Verbose = verbose
	}
}

// WithNJobs sets the number of jobs.
func WithNJobs(nJobs int) RuntimeOption {
	return func(options *RuntimeOptions) {
		options.NJobs = nJobs
	}
}
