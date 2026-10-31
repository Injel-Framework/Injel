package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIRunHelpAndVersion(t *testing.T) {
	// 1. No arguments -> usage
	var stdout, stderr bytes.Buffer
	code := Run([]string{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Injel CLI") {
		t.Fatalf("expected Injel CLI in help output: %s", stdout.String())
	}

	// 2. --help flag
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Available Commands:") {
		t.Fatalf("missing commands in help output: %s", stdout.String())
	}

	// 3. help new
	stdout.Reset()
	code = Run([]string{"help", "new"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "injel new --name") {
		t.Fatalf("missing new usage: %s", stdout.String())
	}

	// 4. version command & flags
	for _, vArg := range []string{"version", "--version", "-v"} {
		stdout.Reset()
		code = Run([]string{vArg}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("expected code 0 for %s, got %d", vArg, code)
		}
		if !strings.Contains(stdout.String(), "injel version") {
			t.Fatalf("missing version in output for %s: %s", vArg, stdout.String())
		}
	}
}

func TestCLIUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"unknown-cmd"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2 for unknown command, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("missing unknown command error in stderr: %s", stderr.String())
	}
}

func TestCLIDoctor(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"doctor"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected doctor exit code 0, got %d. stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Injel Doctor") {
		t.Fatalf("missing Injel Doctor in output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "[✓] Go executable") {
		t.Fatalf("missing Go executable check: %s", stdout.String())
	}
}

func TestCLINewCommand(t *testing.T) {
	tempDir := t.TempDir()
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	// 1. Missing required flags
	var stdout, stderr bytes.Buffer
	code := Run([]string{"new"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1 for missing flags, got %d", code)
	}

	// 2. Valid generation with --local-injel
	stdout.Reset()
	stderr.Reset()
	args := []string{
		"new",
		"--name", "test-project",
		"--module", "github.com/company/test-project",
		"--dir", tempDir,
		"--local-injel", repoRoot,
	}
	code = Run(args, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d. stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Successfully created Injel project") {
		t.Fatalf("missing success message: %s", stdout.String())
	}
}
