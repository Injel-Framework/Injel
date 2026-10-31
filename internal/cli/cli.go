package cli

import (
	"fmt"
	"io"
)

// Run parses CLI arguments, dispatches to the corresponding command,
// and returns the appropriate process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		PrintUsage(stdout)
		return 0
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "help", "-h", "--help":
		if len(cmdArgs) > 0 && cmdArgs[0] == "new" {
			PrintNewUsage(stdout)
			return 0
		}
		PrintUsage(stdout)
		return 0

	case "version", "-v", "--version":
		PrintVersion(stdout)
		return 0

	case "doctor":
		return RunDoctor(stdout, stderr)

	case "run":
		return RunCommand(cmdArgs, stdout, stderr)

	case "new":
		return RunNew(cmdArgs, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "unknown command %q for \"injel\"\nRun 'injel --help' for usage.\n", cmd)
		return 2
	}
}
