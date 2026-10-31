package main

import (
	"os"

	"github.com/Injel-Framework/Injel/internal/cli"
)

func main() {
	exitCode := cli.Run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}
