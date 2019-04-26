package base

import "log"

// RuntimeOptions defines options used for runtime.
type RuntimeOptions struct {
	Verbose bool // Verbose switch
	NJobs   int  // Number of jobs
}

// GetJobs returns the number of concurrent jobs.
func (options *RuntimeOptions) GetJobs() int {
	if options == nil || options.NJobs < 1 {
		return 1
	}
	return options.NJobs
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
