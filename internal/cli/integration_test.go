package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"bufio"
	"strings"
	"time"
)

// helper to run the built injel binary with args and capture exit code, stdout, stderr
var injelBinary = "/tmp/injel"
func runInjel(t *testing.T, args []string, env []string, workDir string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(injelBinary, args...)
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
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run injel: %v", err)
		}
	}
	return exitCode, stdout.String(), stderr.String()
}


func TestScaffoldEdgeCases(t *testing.T) {
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
	for _, c := range cases {
		exit, _, errout := runInjel(t, c.args, nil, tmp)
		if exit != c.want {
			t.Fatalf("case %s: expected exit %d, got %d, stderr: %s", c.name, c.want, exit, errout)
		}
		entries, _ := os.ReadDir(tmp)
		if len(entries) != 0 {
			t.Fatalf("case %s: expected no files created, found %d entries", c.name, len(entries))
		}
	}
}

// TestGenerateAndRunApp creates a temporary copy of the repository (including go.mod) and scaffolds a project using that copy.
func TestGenerateAndRunApp(t *testing.T) {
	work := t.TempDir()
	// Determine repository root for local injel path
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("cannot determine caller info")
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename))) // three levels up
	// Clone repository into a temporary directory to avoid using the original source directly
	repoCopy := filepath.Join(work, "repoCopy")
	if err := os.MkdirAll(repoCopy, 0o755); err != nil {
		t.Fatalf("cannot create repo copy dir: %v", err)
	}
	cloneCmd := exec.Command("git", "clone", "--depth", "1", repoRoot, repoCopy)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone failed: %v, output: %s", err, string(out))
	}
	// Scaffold a new project using the copied repository
	exit, _, errout := runInjel(t, []string{"new", "--name", "testapp", "--module", "github.com/example/testapp", "--local-injel", repoCopy}, nil, work)
	if exit != 0 {
		t.Fatalf("scaffold failed: exit %d, stderr %s", exit, errout)
	}
	projPath := filepath.Join(work, "testapp")
	// Build the generated project with explicit output
	cmd := exec.Command("go", "build", "-o", "testapp", ".")
	cmd.Dir = projPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v, output: %s", err, string(out))
	}
	// Find a free port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("unable to get free port: %v", err)
	}
	addr := l.Addr().String()
	l.Close()
	// Run the built binary
	bin := filepath.Join(projPath, "testapp")
	cmd = exec.Command(bin)
	cmd.Env = append(os.Environ(), "HTTP_ADDR="+addr)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start generated app: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	// Wait for server ready
	client := &http.Client{Timeout: 2 * time.Second}
	healthURL := "http://" + addr + "/health"
	helloURL := "http://" + addr + "/api/v1/hello"
	wait := time.Now().Add(5 * time.Second)
	for time.Now().Before(wait) {
		if _, err := client.Get(healthURL); err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	// Verify /health
	resp, err := client.Get(healthURL)
	if err != nil {
		t.Fatalf("health endpoint error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status %d", resp.StatusCode)
	}
	var health map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if health["status"] != "UP" {
		t.Fatalf("unexpected health payload: %v", health)
	}
	// Verify /api/v1/hello
	resp, err = client.Get(helloURL)
	if err != nil {
		t.Fatalf("hello endpoint error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("hello status %d", resp.StatusCode)
	}
	var hello map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&hello); err != nil {
		t.Fatalf("decode hello: %v", err)
	}
	if _, ok := hello["message"]; !ok {
		t.Fatalf("hello payload missing 'message'")
	}
	// Graceful shutdown via SIGTERM and verify process exits
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}
	done := make(chan error)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
				t.Fatalf("app exited with code %d", exitErr.ExitCode())
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("app did not exit after SIGTERM")
	}
	// Ensure the process has terminated
	if _, err := os.FindProcess(cmd.Process.Pid); err != nil {
		t.Fatalf("process still exists after termination: %v", err)
	}
}

// TestInjelRunSignalForwarding verifies that the CLI forwards signals correctly.
func TestInjelRunSignalForwarding(t *testing.T) {
src := `
import (
    "context"
    "fmt"
    "os/signal"
    "syscall"
    "time"
)
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    fmt.Println("started")
    <-ctx.Done()
    fmt.Println("exiting")
    time.Sleep(100 * time.Millisecond)
}`
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	// Ensure go.mod exists (minimal)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module tmpmodule\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// Build the injel CLI binary locally
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("cannot determine caller info")
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename))) // three levels up
	injelBin := filepath.Join(dir, "injel")
	buildCmd := exec.Command("go", "build", "-o", injelBin, "./cmd/injel")
	buildCmd.Dir = repoRoot
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build injel binary: %v, output: %s", err, string(out))
	}

	cmd := exec.Command(injelBin, "run")
	cmd.Dir = dir
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		t.Fatalf("run injel run: %v", err)
	}

	// Capture output and wait for "started"
	var outBuf bytes.Buffer
	startedCh := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			outBuf.WriteString(line + "\n")
			if strings.Contains(line, "started") {
				close(startedCh)
			}
		}
	}()

	select {
	case <-startedCh:
		// proceed
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for child to print 'started'")
	}

	// Send SIGINT to injel process
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}

	// Wait for termination
	err = cmd.Wait()
	if err == nil {
		t.Fatalf("expected non-zero exit due to signal")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 130 {
			t.Fatalf("expected exit code 130, got %d", exitErr.ExitCode())
		}
	} else {
		t.Fatalf("unexpected error type: %v", err)
	}

	// Verify child printed "exiting"
	if !bytes.Contains(outBuf.Bytes(), []byte("exiting")) {
		t.Fatalf("child program did not receive signal termination (no 'exiting' output)")
	}
}

src := `
import (
    "context"
    "fmt"
    "os/signal"
    "syscall"
    "time"
)
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    fmt.Println("started")
    <-ctx.Done()
    fmt.Println("exiting")
    time.Sleep(100 * time.Millisecond)
}`
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	// Ensure go.mod exists (minimal)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module tmpmodule\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	cmd := exec.Command("/tmp/injel", "run")
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("run injel run: %v", err)
	}
	// Wait for the program to print "started"
	time.Sleep(200 * time.Millisecond)
	if !bytes.Contains(out.Bytes(), []byte("started")) {
		t.Fatalf("child program did not output 'started' before signal")
	}
	// Send SIGINT to injel process
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}
	// Wait for termination
	err := cmd.Wait()
	if err == nil {
		t.Fatalf("expected non-zero exit due to signal")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 130 {
			t.Fatalf("expected exit code 130, got %d", exitErr.ExitCode())
		}
	} else {
		t.Fatalf("unexpected error type: %v", err)
	}
	// Verify child received SIGINT (its output should contain "exiting")
	if !bytes.Contains(out.Bytes(), []byte("exiting")) {
		t.Fatalf("child program did not receive signal termination (no 'exiting' output)")
	}
}

func TestDoctorMissingGo(t *testing.T) {
	exit, _, errout := runInjel(t, []string{"doctor"}, []string{"PATH="}, "")
	if exit == 0 {
		t.Fatalf("doctor should fail when go missing, stderr: %s", errout)
	}
}

func TestDoctorOldVersion(t *testing.T) {
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "go")
	script := "#!/bin/sh\n echo 'go version go1.23 linux/amd64'\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatalf("write wrapper: %v", err)
	}
	env := []string{"PATH=" + dir + ":" + os.Getenv("PATH")}
	exit, _, errout := runInjel(t, []string{"doctor"}, env, "")
	if exit == 0 {
		t.Fatalf("doctor should reject old version, stderr: %s", errout)
	}
}

func TestBinarySizeReporting(t *testing.T) {
	info, err := os.Stat("/tmp/injel")
	if err != nil {
		t.Fatalf("binary not found: %v", err)
	}
	t.Logf("injel binary size: %d bytes", info.Size())
}
