package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var goVersionRegex = regexp.MustCompile(`go version go(\d+)\.(\d+)`)

// RunDoctor performs diagnostics on the local Go development environment.
func RunDoctor(stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, "Injel Doctor — Diagnosing Go development environment...")
	fmt.Fprintln(stdout, "")

	hasErrors := false

	// 1. Check Go executable
	goPath, err := exec.LookPath("go")
	if err != nil {
		fmt.Fprintln(stdout, "  [✗] Go executable: Not found in $PATH. Please install Go (https://go.dev/dl/).")
		hasErrors = true
	} else {
		fmt.Fprintf(stdout, "  [✓] Go executable: %s\n", goPath)

		// 2. Check Go version
		cmd := exec.Command(goPath, "version")
		out, err := cmd.Output()
		if err != nil {
			fmt.Fprintf(stdout, "  [✗] Go version: Failed to execute %s version: %v\n", goPath, err)
			hasErrors = true
		} else {
			rawVer := strings.TrimSpace(string(out))
			matches := goVersionRegex.FindStringSubmatch(rawVer)
			if len(matches) >= 3 {
				major, _ := strconv.Atoi(matches[1])
				minor, _ := strconv.Atoi(matches[2])
				if major > 1 || (major == 1 && minor >= 24) {
					fmt.Fprintf(stdout, "  [✓] Go version: %s (meets requirement >= 1.24)\n", rawVer)
				} else {
					fmt.Fprintf(stdout, "  [✗] Go version: %s (Injel requires Go 1.24 or higher)\n", rawVer)
					hasErrors = true
				}
			} else {
				// Non-standard version string (e.g. devel / tip), print as-is
				fmt.Fprintf(stdout, "  [?] Go version: %s (ensure Go 1.24+ compatibility)\n", rawVer)
			}
		}
	}

	// 3. Check Go module status in current directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stdout, "  [?] Go module: Unable to determine current working directory: %v\n", err)
	} else {
		goModPath := filepath.Join(cwd, "go.mod")
		content, err := os.ReadFile(goModPath)
		if err != nil {
			fmt.Fprintf(stdout, "  [-] Go module: No go.mod found in current directory (%s)\n", cwd)
		} else {
			moduleName := "unknown"
			for _, line := range strings.Split(string(content), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					moduleName = strings.TrimSpace(strings.TrimPrefix(line, "module"))
					break
				}
			}
			fmt.Fprintf(stdout, "  [✓] Go module: %s (in %s)\n", moduleName, cwd)
		}
	}

	fmt.Fprintln(stdout, "")
	if hasErrors {
		fmt.Fprintln(stderr, "Doctor detected issues that may prevent building Injel projects.")
		return 1
	}

	fmt.Fprintln(stdout, "All checks passed! Your environment is ready for Injel development.")
	return 0
}
