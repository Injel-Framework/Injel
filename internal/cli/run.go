package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// RunCommand executes "go run . [args...]" in the current directory with signal handling.
func RunCommand(args []string, stdout, stderr io.Writer) int {
	// 1. Verify go.mod exists in current directory
	if _, err := os.Stat("go.mod"); err != nil {
		fmt.Fprintln(stderr, "error: injel run must be executed inside a Go module (no go.mod found in current directory)")
		return 1
	}

	// 2. Locate Go executable
	goPath, err := exec.LookPath("go")
	if err != nil {
		fmt.Fprintln(stderr, "error: Go executable not found in $PATH. Please install Go to run applications.")
		return 1
	}

	// 3. Prepare command arguments
	cmdArgs := append([]string{"run", "."}, args...)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := exec.CommandContext(ctx, goPath, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		if ctx.Err() != nil {
			return 130 // 128 + SIGINT
		}
		fmt.Fprintf(stderr, "error: failed to run application: %v\n", err)
		return 1
	}

	return 0
}
