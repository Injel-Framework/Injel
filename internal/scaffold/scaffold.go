package scaffold

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

//go:embed templates/*
var templateFS embed.FS

var (
	projectNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
	modulePathRegex  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*(/[a-zA-Z0-9_.-]+)*$`)
)

// Options holds configuration parameters for scaffolding a new Injel application.
type Options struct {
	Name         string
	Module       string
	Dir          string
	InjelVersion string
	LocalInjel   string
}

// TemplateData holds data passed into project templates.
type TemplateData struct {
	ProjectName  string
	ModulePath   string
	InjelVersion string
}

// Validate checks whether the given options are valid.
func (o *Options) Validate() error {
	o.Name = strings.TrimSpace(o.Name)
	if o.Name == "" {
		return errors.New("project name cannot be empty (use --name)")
	}
	if !projectNameRegex.MatchString(o.Name) {
		return fmt.Errorf("invalid project name %q: must contain only letters, numbers, hyphens, and underscores", o.Name)
	}

	o.Module = strings.TrimSpace(o.Module)
	if o.Module == "" {
		return errors.New("module path cannot be empty (use --module)")
	}
	if !modulePathRegex.MatchString(o.Module) {
		return fmt.Errorf("invalid module path %q: must follow Go module conventions", o.Module)
	}

	if o.InjelVersion != "" && o.LocalInjel != "" {
		return errors.New("flags --injel-version and --local-injel are mutually exclusive")
	}

	if o.LocalInjel != "" {
		absLocal, err := filepath.Abs(o.LocalInjel)
		if err != nil {
			return fmt.Errorf("invalid local Injel path: %w", err)
		}
		goModPath := filepath.Join(absLocal, "go.mod")
		content, err := os.ReadFile(goModPath)
		if err != nil {
			return fmt.Errorf("local Injel path %q does not contain a valid go.mod: %w", absLocal, err)
		}
		if !strings.Contains(string(content), "module github.com/Injel-Framework/Injel") {
			return fmt.Errorf("local path %q is not the Injel framework module", absLocal)
		}
		o.LocalInjel = absLocal
	}

	return nil
}

// Generate creates a new Injel project based on the provided options.
// It returns a list of created files relative to the target directory.
func Generate(opts Options) (createdFiles []string, retErr error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	targetDir := filepath.Join(opts.Dir, opts.Name)
	if targetDir == "" {
		targetDir = opts.Name
	}

	// 1. Check if destination already exists
	if _, err := os.Stat(targetDir); err == nil {
		return nil, fmt.Errorf("destination directory already exists: %s", targetDir)
	}

	// Track created items for rollback on failure
	var rollbackPaths []string
	defer func() {
		if retErr != nil {
			// Rollback created files in reverse order
			for i := len(rollbackPaths) - 1; i >= 0; i-- {
				_ = os.RemoveAll(rollbackPaths[i])
			}
		}
	}()

	// 2. Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}
	rollbackPaths = append(rollbackPaths, targetDir)

	injelVer := opts.InjelVersion
	if injelVer == "" {
		if opts.LocalInjel != "" {
			injelVer = "v0.0.0-dev"
		} else {
			return nil, errors.New("Injel has not released a stable public version yet; please specify --local-injel <path> for development or --injel-version <version>")
		}
	}

	data := TemplateData{
		ProjectName:  opts.Name,
		ModulePath:   opts.Module,
		InjelVersion: injelVer,
	}

	// File definitions to render
	filesToRender := []struct {
		relPath  string
		tmplName string
	}{
		{"go.mod", "templates/gomod.tmpl"},
		{".gitignore", "templates/gitignore.tmpl"},
		{".env.example", "templates/envexample.tmpl"},
		{"README.md", "templates/readme.tmpl"},
		{"main.go", "templates/main.tmpl"},
		{filepath.Join("internal", "config", "config.go"), "templates/config.tmpl"},
		{filepath.Join("internal", "handler", "hello.go"), "templates/hello_handler.tmpl"},
	}

	for _, f := range filesToRender {
		destPath := filepath.Join(targetDir, f.relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory for %s: %w", f.relPath, err)
		}

		tmplContent, err := templateFS.ReadFile(f.tmplName)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded template %s: %w", f.tmplName, err)
		}

		tmpl, err := template.New(f.relPath).Parse(string(tmplContent))
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", f.tmplName, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("failed to execute template %s: %w", f.tmplName, err)
		}

		if err := os.WriteFile(destPath, buf.Bytes(), 0644); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", f.relPath, err)
		}
		createdFiles = append(createdFiles, f.relPath)
	}

	// If local Injel development mode, create go.work
	if opts.LocalInjel != "" {
		goWorkPath := filepath.Join(targetDir, "go.work")
		goWorkContent := fmt.Sprintf("go 1.24.0\n\nuse (\n\t.\n\t%s\n)\n", opts.LocalInjel)
		if err := os.WriteFile(goWorkPath, []byte(goWorkContent), 0644); err != nil {
			return nil, fmt.Errorf("failed to write go.work: %w", err)
		}
		createdFiles = append(createdFiles, "go.work")
	}

	return createdFiles, nil
}
