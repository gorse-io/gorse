package version

import (
	"fmt"
	"runtime"
)

// Default build-time variable.
// These values are overridden via ldflags
var (
	Version    = "unknown-version"
	GitCommit  = "unknown-commit"
	BuildTime  = "unknown-buildtime"
	APIVersion = "v0.2.7"
)

func BuildInfo() string {
	var buildInfo string
	buildInfo += fmt.Sprintln("Version:\t", Version)
	buildInfo += fmt.Sprintln("API version:\t", APIVersion)
	buildInfo += fmt.Sprintln("Go version:\t", runtime.Version())
	buildInfo += fmt.Sprintln("Git commit:\t", GitCommit)
	buildInfo += fmt.Sprintln("Built:\t\t", BuildTime)
	buildInfo += fmt.Sprintf("OS/Arch:\t %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return buildInfo
}
