package core

// RuntimeOptions defined options used in run time.
type RuntimeOptions struct {
	Verbose bool // Verbose switch
	NJobs   int  // Number of jobs
}

// NewRuntimeOptions creates a RuntimeOptions from RuntimeOptionSetters.
func NewRuntimeOptions(setters ...RuntimeOptionSetter) *RuntimeOptions {
	options := new(RuntimeOptions)
	for _, setter := range setters {
		setter(options)
	}
	return options
}

// RuntimeOptionSetter changes options.
type RuntimeOptionSetter func(options *RuntimeOptions)

// WithVerbose sets the verbose switch
func WithVerbose(verbose bool) RuntimeOptionSetter {
	return func(options *RuntimeOptions) {
		options.Verbose = verbose
	}
}

// WithNJobs sets the number of jobs.
func WithNJobs(nJobs int) RuntimeOptionSetter {
	return func(options *RuntimeOptions) {
		options.NJobs = nJobs
	}
}
