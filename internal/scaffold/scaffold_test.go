package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldOptionsValidation(t *testing.T) {
	// Empty name
	opts := Options{Name: "", Module: "example.com/api"}
	if err := opts.Validate(); err == nil {
		t.Fatal("expected error for empty name")
	}

	// Invalid name with path traversal
	opts = Options{Name: "../bad-name", Module: "example.com/api"}
	if err := opts.Validate(); err == nil {
		t.Fatal("expected error for path traversal in name")
	}

	// Invalid name with special characters
	opts = Options{Name: "my project!", Module: "example.com/api"}
	if err := opts.Validate(); err == nil {
		t.Fatal("expected error for special characters in name")
	}

	// Empty module
	opts = Options{Name: "my-api", Module: ""}
	if err := opts.Validate(); err == nil {
		t.Fatal("expected error for empty module")
	}

	// Invalid module
	opts = Options{Name: "my-api", Module: "/invalid/module/"}
	if err := opts.Validate(); err == nil {
		t.Fatal("expected error for invalid module")
	}

	// Mutually exclusive flags
	opts = Options{
		Name:         "my-api",
		Module:       "example.com/api",
		InjelVersion: "v0.1.0",
		LocalInjel:   "/some/path",
	}
	if err := opts.Validate(); err == nil {
		t.Fatal("expected error when both --injel-version and --local-injel are set")
	}
}

func TestScaffoldGenerateWithVersion(t *testing.T) {
	tempDir := t.TempDir()

	opts := Options{
		Name:         "order-service",
		Module:       "github.com/company/order-service",
		Dir:          tempDir,
		InjelVersion: "v0.1.0",
	}

	files, err := Generate(opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	expectedFiles := []string{
		"go.mod",
		".gitignore",
		".env.example",
		"README.md",
		"main.go",
		filepath.Join("internal", "config", "config.go"),
		filepath.Join("internal", "handler", "hello.go"),
	}

	targetDir := filepath.Join(tempDir, "order-service")
	for _, expected := range expectedFiles {
		p := filepath.Join(targetDir, expected)
		if _, statErr := os.Stat(p); statErr != nil {
			t.Fatalf("expected file %s not found: %v", expected, statErr)
		}
	}

	if len(files) != len(expectedFiles) {
		t.Fatalf("expected %d files, got %d: %v", len(expectedFiles), len(files), files)
	}

	// Verify go.mod content
	goModBytes, err := os.ReadFile(filepath.Join(targetDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	goMod := string(goModBytes)
	if !strings.Contains(goMod, "module github.com/company/order-service") {
		t.Fatalf("missing module declaration in go.mod: %s", goMod)
	}
	if !strings.Contains(goMod, "require github.com/Injel-Framework/Injel v0.1.0") {
		t.Fatalf("missing dependency declaration in go.mod: %s", goMod)
	}

	// Attempting to generate again into the same directory must fail
	_, err = Generate(opts)
	if err == nil {
		t.Fatal("expected error when generating into an existing directory")
	}
}

func TestScaffoldGenerateWithLocalInjel(t *testing.T) {
	tempDir := t.TempDir()

	// Locate repo root containing go.mod
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	opts := Options{
		Name:       "payment-service",
		Module:     "github.com/company/payment-service",
		Dir:        tempDir,
		LocalInjel: repoRoot,
	}

	files, err := Generate(opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	targetDir := filepath.Join(tempDir, "payment-service")

	// Verify go.work was created
	goWorkBytes, err := os.ReadFile(filepath.Join(targetDir, "go.work"))
	if err != nil {
		t.Fatalf("expected go.work to be created: %v", err)
	}
	goWork := string(goWorkBytes)
	if !strings.Contains(goWork, repoRoot) {
		t.Fatalf("go.work does not contain repoRoot %s: %s", repoRoot, goWork)
	}

	// Verify go.work is in returned files
	foundGoWork := false
	for _, f := range files {
		if f == "go.work" {
			foundGoWork = true
			break
		}
	}
	if !foundGoWork {
		t.Fatal("go.work not reported in generated files list")
	}
}

func TestIntegrationGeneratedApplicationBuilds(t *testing.T) {
	tempDir := t.TempDir()

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	opts := Options{
		Name:       "sample-app",
		Module:     "github.com/test/sample-app",
		Dir:        tempDir,
		LocalInjel: repoRoot,
	}

	_, err = Generate(opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	appDir := filepath.Join(tempDir, "sample-app")

	// Run go build in generated project using the local workspace
	cmd := exec.Command("go", "build", "-v", "./...")
	cmd.Dir = appDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated project failed to build:\nOutput:\n%s\nError: %v", string(out), err)
	}
}
