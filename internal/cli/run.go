package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

const runShutdownTimeout = 5 * time.Second

// RunCommand executes "go run . [args...]" in the current directory.
//
// The go command is started in its own process group. This is important because
// "go run" starts the user's application as a child process. Forwarding a
// signal only to the go command can leave the application running. Forwarding
// it to the process group lets both processes perform their normal shutdown.
func RunCommand(args []string, stdout, stderr io.Writer) int {
	if _, err := os.Stat("go.mod"); err != nil {
		fmt.Fprintln(stderr, "error: injel run must be executed inside a Go module (no go.mod found in current directory)")
		return 1
	}

	goPath, err := exec.LookPath("go")
	if err != nil {
		fmt.Fprintln(stderr, "error: Go executable not found in $PATH. Please install Go to run applications.")
		return 1
	}

	cmdArgs := append([]string{"run", "."}, args...)
	cmd := exec.Command(goPath, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Give "go run" and the application it starts a separate process group.
	// This implementation targets Unix-like systems (Linux/macOS).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(stderr, "error: failed to start application: %v\n", err)
		return 1
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		return runCommandExitCode(err, stderr)

	case received := <-sigCh:
		sig, ok := received.(syscall.Signal)
		if !ok {
			sig = syscall.SIGINT
		}

		// Forward the signal to the entire process group, including the
		// application spawned internally by "go run".
		if err := syscall.Kill(-cmd.Process.Pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
			fmt.Fprintf(stderr, "warning: failed to forward signal %v: %v\n", sig, err)
		}

		// Wait for the go tool to exit. It may exit before its child, so
		// separately wait for the process group to disappear as well.
		waitTimer := time.NewTimer(runShutdownTimeout)
		select {
		case <-waitCh:
			if !waitTimer.Stop() {
				select {
				case <-waitTimer.C:
				default:
				}
			}
		case <-waitTimer.C:
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-waitCh
		}

		waitForProcessGroupExit(cmd.Process.Pid, runShutdownTimeout)
		return 128 + int(sig)
	}
}

func runCommandExitCode(err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if code := exitErr.ExitCode(); code >= 0 {
			return code
		}
	}

	fmt.Fprintf(stderr, "error: failed to run application: %v\n", err)
	return 1
}

// waitForProcessGroupExit avoids returning while a descendant of "go run"
// is still alive. If graceful shutdown takes too long, it force-stops the
// remaining processes in that group.
func waitForProcessGroupExit(pgid int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		err := syscall.Kill(-pgid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
			return
		}
		<-ticker.C
	}
}
