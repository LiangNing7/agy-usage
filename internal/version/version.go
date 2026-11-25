package version

import (
	"fmt"
	"runtime"
)

var (
	GitVersion = "v0.1.0"
	GitCommit  = "dev"
	BuildDate  = "unknown"
)

func String() string {
	return fmt.Sprintf("agy-usage %s (commit: %s, built: %s, %s/%s)",
		GitVersion, GitCommit, BuildDate, runtime.GOOS, runtime.GOARCH)
}
