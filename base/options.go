package base

import "log"

// RuntimeOptions defines options used for runtime.
type RuntimeOptions struct {
	Verbose bool // Verbose switch
	FitJobs int  // Number of jobs for model fitting
	CVJobs  int  // Number of jobs for cross validation
}

// GetFitJobs returns the number of concurrent jobs for model fitting.
func (options *RuntimeOptions) GetFitJobs() int {
	if options == nil || options.FitJobs < 1 {
		return 1
	}
	return options.FitJobs
}

// GetCVJobs returns the number of concurrent jobs for cross validation.
func (options *RuntimeOptions) GetCVJobs() int {
	if options == nil || options.CVJobs < 1 {
		return 1
	}
	return options.CVJobs
}

// GetVerbose returns the indicator of verbose.
func (options *RuntimeOptions) GetVerbose() bool {
	if options == nil {
		return true
	}
	return options.Verbose
}

// Log to logs.
func (options *RuntimeOptions) Log(v ...interface{}) {
	if options.GetVerbose() {
		log.Print(v...)
	}
}

// Logln to logs with newline.
func (options *RuntimeOptions) Logln(v ...interface{}) {
	if options.GetVerbose() {
		log.Println(v...)
	}
}

// Logf to logs with format.
func (options *RuntimeOptions) Logf(format string, v ...interface{}) {
	if options.GetVerbose() {
		log.Printf(format, v...)
	}
}
