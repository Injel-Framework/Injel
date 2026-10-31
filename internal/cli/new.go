package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/Injel-Framework/Injel/internal/scaffold"
)

// RunNew handles the "injel new" command.
func RunNew(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		name         string
		module       string
		dir          string
		injelVersion string
		localInjel   string
	)

	fs.StringVar(&name, "name", "", "Project name and directory name (required)")
	fs.StringVar(&module, "module", "", "Go module path (required)")
	fs.StringVar(&dir, "dir", ".", "Parent directory where project will be created")
	fs.StringVar(&injelVersion, "injel-version", "", "Injel release version to require")
	fs.StringVar(&localInjel, "local-injel", "", "Path to local Injel framework repository")

	fs.Usage = func() {
		PrintNewUsage(stderr)
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	opts := scaffold.Options{
		Name:         name,
		Module:       module,
		Dir:          dir,
		InjelVersion: injelVersion,
		LocalInjel:   localInjel,
	}

	createdFiles, err := scaffold.Generate(opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	targetDir := filepath.Join(opts.Dir, opts.Name)

	fmt.Fprintf(stdout, "Successfully created Injel project %q at %s!\n\n", opts.Name, targetDir)
	fmt.Fprintln(stdout, "Created files:")
	for _, f := range createdFiles {
		fmt.Fprintf(stdout, "  - %s\n", f)
	}

	fmt.Fprintln(stdout, "\nNext steps:")
	fmt.Fprintf(stdout, "  cd %s\n", targetDir)
	fmt.Fprintln(stdout, "  cp .env.example .env")
	fmt.Fprintln(stdout, "  go run .")
	fmt.Fprintln(stdout, "\nHappy coding with Injel!")

	return 0
}
