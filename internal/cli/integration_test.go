package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// repositoryRoot returns the root directory of the Injel repository.
func repositoryRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine caller info")
	}

	root := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	root, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

// buildInjelBinary builds the CLI into a test-owned temporary directory.
// Tests must not depend on a pre-existing /tmp/injel binary.
func buildInjelBinary(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "injel")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/injel")
	cmd.Dir = repositoryRoot(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build Injel CLI: %v\n%s", err, output)
	}
	return binary
}

// runInjel runs the supplied CLI binary and captures its output and exit code.
func runInjel(t *testing.T, binary string, args []string, env []string, workDir string) (int, string, string) {
	t.Helper()

	cmd := exec.Command(binary, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("failed to run injel: %v", err)
		}
		exitCode = exitErr.ExitCode()
	}

	return exitCode, stdout.String(), stderr.String()
}

// killProcessGroup terminates the test process and any child processes it
// started. The integration tests use a separate process group on Unix.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	_ = cmd.Process.Kill()
}

func TestScaffoldEdgeCases(t *testing.T) {
	binary := buildInjelBinary(t)
	tmp := t.TempDir()

	cases := []struct {
		name string
		args []string
		want int
	}{
		{"space name", []string{"new", "--name", "my app", "--module", "github.com/user/myapp"}, 1},
		{"special chars", []string{"new", "--name", "my$app!", "--module", "github.com/user/myapp"}, 1},
		{"path traversal", []string{"new", "--name", "bad", "--module", "github.com/user/bad", "--dir", "../outside"}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exit, _, stderr := runInjel(t, binary, tc.args, nil, tmp)
			if exit != tc.want {
				t.Fatalf("expected exit %d, got %d; stderr: %s", tc.want, exit, stderr)
			}

			entries, err := os.ReadDir(tmp)
			if err != nil {
				t.Fatalf("read temporary directory: %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("expected no files created, found %d entries", len(entries))
			}
		})
	}
}

// TestGenerateAndRunApp clones the current repository, scaffolds an application
// against that clone, builds it, exercises its HTTP endpoints, and checks that
// SIGTERM causes a graceful shutdown.
func TestGenerateAndRunApp(t *testing.T) {
	work := t.TempDir()
	binary := buildInjelBinary(t)
	repoRoot := repositoryRoot(t)

	repoCopy := filepath.Join(work, "repoCopy")
	repoURL := (&url.URL{Scheme: "file", Path: repoRoot}).String()
	cloneCmd := exec.Command("git", "clone", "--depth", "1", "--", repoURL, repoCopy)
	if output, err := cloneCmd.CombinedOutput(); err != nil {
		t.Fatalf("clone repository: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(repoCopy, "go.mod")); err != nil {
		t.Fatalf("cloned Injel repository has no go.mod: %v", err)
	}

	exit, _, stderr := runInjel(t, binary, []string{
		"new", "--name", "testapp",
		"--module", "github.com/example/testapp",
		"--local-injel", repoCopy,
	}, nil, work)
	if exit != 0 {
		t.Fatalf("scaffold failed: exit %d; stderr: %s", exit, stderr)
	}

	projectPath := filepath.Join(work, "testapp")
	appBinary := filepath.Join(projectPath, "testapp")
	buildApp := exec.Command("go", "build", "-o", appBinary, ".")
	buildApp.Dir = projectPath
	if output, err := buildApp.CombinedOutput(); err != nil {
		t.Fatalf("build generated application: %v\n%s", err, output)
	}
	if info, err := os.Stat(appBinary); err != nil {
		t.Fatalf("generated application binary was not created: %v", err)
	} else if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("generated application is not an executable regular file: mode %v", info.Mode())
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve HTTP port: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release reserved HTTP port: %v", err)
	}

	app := exec.Command(appBinary)
	app.Env = append(os.Environ(), "HTTP_ADDR="+addr)
	app.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var appStdout, appStderr bytes.Buffer
	app.Stdout = &appStdout
	app.Stderr = &appStderr
	if err := app.Start(); err != nil {
		t.Fatalf("start generated application: %v", err)
	}

	waited := false
	defer func() {
		if !waited {
			killProcessGroup(app)
			_ = app.Wait()
		}
	}()

	client := &http.Client{Timeout: time.Second}
	healthURL := "http://" + addr + "/health"
	helloURL := "http://" + addr + "/api/v1/hello"
	ready := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, requestErr := client.Get(healthURL)
		if requestErr == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		killProcessGroup(app)
		_ = app.Wait()
		waited = true
		t.Fatalf("generated application did not become healthy; stdout: %s; stderr: %s", appStdout.String(), appStderr.String())
	}

	healthResp, err := client.Get(healthURL)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	var health map[string]string
	decodeErr := json.NewDecoder(healthResp.Body).Decode(&health)
	_ = healthResp.Body.Close()
	if decodeErr != nil {
		t.Fatalf("decode /health response: %v", decodeErr)
	}
	if healthResp.StatusCode != http.StatusOK || health["status"] != "ok" {
		t.Fatalf("unexpected /health response: status=%d payload=%v", healthResp.StatusCode, health)
	}

	helloResp, err := client.Get(helloURL)
	if err != nil {
		t.Fatalf("GET /api/v1/hello: %v", err)
	}
	var hello map[string]any
	decodeErr = json.NewDecoder(helloResp.Body).Decode(&hello)
	_ = helloResp.Body.Close()
	if decodeErr != nil {
		t.Fatalf("decode /api/v1/hello response: %v", decodeErr)
	}
	if helloResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected /api/v1/hello status: %d", helloResp.StatusCode)
	}
	if _, ok := hello["message"]; !ok {
		t.Fatalf("/api/v1/hello response has no message field: %v", hello)
	}

	if err := app.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM to generated application: %v", err)
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- app.Wait() }()
	select {
	case waitErr := <-waitCh:
		waited = true
		if waitErr != nil {
			t.Fatalf("generated application did not shut down gracefully: %v; stderr: %s", waitErr, appStderr.String())
		}
	case <-time.After(15 * time.Second):
		killProcessGroup(app)
		<-waitCh
		waited = true
		t.Fatal("generated application did not exit after SIGTERM")
	}
	if app.ProcessState == nil || !app.ProcessState.Exited() {
		t.Fatal("generated application process did not reach an exited state")
	}
	if app.ProcessState.ExitCode() != 0 {
		t.Fatalf("expected graceful exit code 0, got %d", app.ProcessState.ExitCode())
	}
}

// TestInjelRunSignalForwarding verifies that injel run forwards SIGINT to its
// child process and returns the conventional exit code 130.
func TestInjelRunSignalForwarding(t *testing.T) {
	src := `package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Println("started")
	<-ctx.Done()
	fmt.Println("exiting")
}`

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatalf("write child program: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module tmpmodule\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write child go.mod: %v", err)
	}

	binary := buildInjelBinary(t)
	cmd := exec.Command(binary, "run")
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("create CLI stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start injel run: %v", err)
	}

	waited := false
	defer func() {
		if !waited {
			killProcessGroup(cmd)
			_ = cmd.Wait()
		}
	}()

	lines := make(chan string, 32)
	scanDone := make(chan error, 1)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		defer close(lines)
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		scanDone <- scanner.Err()
	}()
	defer func() {
		if !waited {
			killProcessGroup(cmd)
			select {
			case <-readerDone:
			case <-time.After(2 * time.Second):
			}
			_ = cmd.Wait()
		}
	}()

	var output strings.Builder
	started := false
	startTimer := time.NewTimer(5 * time.Second)
	defer startTimer.Stop()
	for !started {
		select {
		case line, ok := <-lines:
			if !ok {
				scanErr := <-scanDone
				t.Fatalf("injel run exited before child printed started (scanner error: %v); output: %s; stderr: %s", scanErr, output.String(), stderr.String())
			}
			output.WriteString(line)
			output.WriteByte('\n')
			if strings.Contains(line, "started") {
				started = true
			}
		case <-startTimer.C:
			t.Fatalf("timeout waiting for child to print started; output: %s", output.String())
		}
	}

	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT to injel CLI: %v", err)
	}

	readTimer := time.NewTimer(5 * time.Second)
	defer readTimer.Stop()
	readerFinished := false
	var scanErr error
	for !readerFinished {
		select {
		case line, ok := <-lines:
			if !ok {
				readerFinished = true
				scanErr = <-scanDone
				continue
			}
			output.WriteString(line)
			output.WriteByte('\n')
		case <-readTimer.C:
			t.Fatalf("timeout waiting for child process to exit; output: %s", output.String())
		}
	}
	if scanErr != nil {
		t.Fatalf("read injel output: %v", scanErr)
	}

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	var waitErr error
	select {
	case waitErr = <-waitCh:
		waited = true
	case <-time.After(5 * time.Second):
		killProcessGroup(cmd)
		waitErr = <-waitCh
		waited = true
		t.Fatalf("injel CLI did not exit after child received SIGINT (wait error: %v)", waitErr)
	}

	exitErr, ok := waitErr.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected CLI exit code 130, got error %v", waitErr)
	}
	if exitErr.ExitCode() != 130 {
		t.Fatalf("expected CLI exit code 130, got %d; output: %s; stderr: %s", exitErr.ExitCode(), output.String(), stderr.String())
	}
	if !strings.Contains(output.String(), "exiting") {
		t.Fatalf("child did not report signal handling; output: %s; stderr: %s", output.String(), stderr.String())
	}
	if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
		t.Fatal("injel CLI process did not reach an exited state")
	}
}

func TestDoctorMissingGo(t *testing.T) {
	binary := buildInjelBinary(t)
	exit, _, stderr := runInjel(t, binary, []string{"doctor"}, []string{"PATH="}, "")
	if exit == 0 {
		t.Fatalf("doctor should fail when Go is missing; stderr: %s", stderr)
	}
}

func TestDoctorOldVersion(t *testing.T) {
	binary := buildInjelBinary(t)
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "go")
	script := "#!/bin/sh\nprintf '%s\\n' 'go version go1.23 linux/amd64'\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake go executable: %v", err)
	}

	path := dir + string(os.PathListSeparator) + os.Getenv("PATH")
	exit, _, stderr := runInjel(t, binary, []string{"doctor"}, []string{"PATH=" + path}, "")
	if exit == 0 {
		t.Fatalf("doctor should reject an old Go version; stderr: %s", stderr)
	}
}

func TestBinarySizeReporting(t *testing.T) {
	binary := buildInjelBinary(t)
	info, err := os.Stat(binary)
	if err != nil {
		t.Fatalf("stat built Injel binary: %v", err)
	}
	t.Logf("Injel CLI binary size: %d bytes", info.Size())
}
