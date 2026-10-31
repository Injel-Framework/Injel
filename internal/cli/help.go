package cli

import (
	"fmt"
	"io"
)

// PrintUsage writes the main CLI help menu to the given writer.
func PrintUsage(w io.Writer) {
	fmt.Fprintf(w, `Injel CLI — Official Command-Line Tool for the Injel Framework
Spring-inspired developer experience. Native Go simplicity.

Usage:
  injel <command> [arguments]

Available Commands:
  new         Scaffold a new Injel backend application project
  run         Run the current Injel application
  doctor      Inspect the local Go environment and toolchain
  version     Show the Injel CLI version and build metadata
  help        Show help information about Injel commands

Flags:
  -h, --help      Show help for injel or a specific command
  -v, --version   Show injel version information

Use "injel <command> --help" for more information about a specific command.
`)
}

// PrintNewUsage writes help for the "new" command.
func PrintNewUsage(w io.Writer) {
	fmt.Fprintf(w, `Usage:
  injel new --name <project-name> --module <module-path> [flags]

Flags:
  --name string            Project name and destination directory name (required)
  --module string          Go module path, e.g. github.com/user/api (required)
  --dir string             Parent directory where project will be created (default: ".")
  --injel-version string   Injel release version to require, e.g. v0.1.0
  --local-injel string     Path to local Injel framework repository (for development)

Examples:
  injel new --name user-api --module github.com/company/user-api --injel-version v0.1.0
  injel new --name demo-app --module demo-app --local-injel ~/go/src/github.com/Injel-Framework/Injel
`)
}
