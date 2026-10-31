package cli

import (
	"fmt"
	"io"
	"runtime"
)

// Build information injected via ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// PrintVersion writes formatted version information to the provided writer.
func PrintVersion(w io.Writer) {
	fmt.Fprintf(w, "injel version %s (commit: %s, built at: %s, %s/%s, runtime: %s)\n",
		Version, Commit, Date, runtime.GOOS, runtime.GOARCH, runtime.Version())
}
