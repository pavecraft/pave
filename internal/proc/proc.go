// Package proc owns all subprocess lifecycle management for pave. Business
// logic (providers, planner) must spawn processes through this package rather
// than calling os/exec directly, so that runs can be paused, resumed, and
// stopped under interactive control.
package proc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// stopGrace is how long Stop waits after SIGTERM before sending SIGKILL.
var stopGrace = 5 * time.Second

// Process is a running (or finished) subprocess with lifecycle controls.
type Process struct {
	cmd    *exec.Cmd
	stdout *syncBuffer
	stderr *syncBuffer

	done chan struct{} // closed when the process has been reaped

	mu       sync.Mutex
	finished bool
	exitCode int
	waitErr  error
}

// Start launches name with args in directory dir. If ctx is cancelled, the
// process is stopped (SIGTERM then SIGKILL) automatically. Stdout and stderr are
// captured and readable via Stdout/Stderr.
func Start(ctx context.Context, name string, args []string, dir string) (*Process, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	p := newProcess(cmd)
	cmd.Stdout = p.stdout
	cmd.Stderr = p.stderr
	return p.launch(ctx, name)
}

// IOOptions configures stdio and environment for StartIO.
type IOOptions struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Env    []string // extra "KEY=VALUE" entries appended to the current environment
}

// StartIO launches name with args in dir, streaming stdio to the provided
// writers/reader instead of capturing it. It is intended for long-running
// foreground subprocesses such as the UI dev server. Lifecycle control (Stop on
// ctx cancel) works the same as Start.
func StartIO(ctx context.Context, name string, args []string, dir string, opts IOOptions) (*Process, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	p := newProcess(cmd)
	return p.launch(ctx, name)
}

func newProcess(cmd *exec.Cmd) *Process {
	return &Process{
		cmd:    cmd,
		stdout: &syncBuffer{},
		stderr: &syncBuffer{},
		done:   make(chan struct{}),
	}
}

// launch starts the process group, reaper, and ctx watcher.
func (p *Process) launch(ctx context.Context, name string) (*Process, error) {
	setPgid(p.cmd) // platform-specific: run in its own process group

	if err := p.cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", name, err)
	}

	go p.reap()

	if ctx != nil {
		go func() {
			select {
			case <-ctx.Done():
				_ = p.Stop()
			case <-p.done:
			}
		}()
	}

	return p, nil
}

// reap waits for the process to exit and records the result.
func (p *Process) reap() {
	err := p.cmd.Wait()
	p.mu.Lock()
	p.finished = true
	p.exitCode = exitCodeFrom(err)
	if !isExitError(err) {
		// Only surface non-ExitError problems (e.g. signal kill); a normal
		// nonzero exit is conveyed through the exit code, not the error.
		p.waitErr = err
	}
	p.mu.Unlock()
	close(p.done)
}

// Wait blocks until the process exits and returns its exit code. The error is
// non-nil only for abnormal termination (e.g. killed by a signal); a normal
// nonzero exit is reported via the exit code with a nil error.
func (p *Process) Wait() (int, error) {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exitCode, p.waitErr
}

// Done returns a channel closed when the process has exited.
func (p *Process) Done() <-chan struct{} { return p.done }

// exited reports whether the process has already been reaped.
func (p *Process) exited() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// Pause suspends the process (and its group). On platforms without job-control
// signals this returns an error.
func (p *Process) Pause() error {
	if p.exited() {
		return errors.New("process already exited")
	}
	return p.pause()
}

// Resume continues a paused process.
func (p *Process) Resume() error {
	if p.exited() {
		return errors.New("process already exited")
	}
	return p.resume()
}

// Stop terminates the process gracefully: SIGTERM, then SIGKILL after a grace
// period if it has not exited. It returns nil if the process is already gone.
func (p *Process) Stop() error {
	if p.exited() {
		return nil
	}
	// Resume first so a paused process can handle the termination signal.
	_ = p.resume()
	if err := p.terminate(); err != nil {
		// Best-effort terminate failed; fall through to kill.
		_ = p.kill()
	}
	select {
	case <-p.done:
		return nil
	case <-time.After(stopGrace):
		return p.kill()
	}
}

// Stdout returns a reader over the captured standard output so far.
func (p *Process) Stdout() io.Reader { return bytes.NewReader(p.stdout.Bytes()) }

// Stderr returns a reader over the captured standard error so far.
func (p *Process) Stderr() io.Reader { return bytes.NewReader(p.stderr.Bytes()) }

// StdoutString returns the captured standard output as a string.
func (p *Process) StdoutString() string { return p.stdout.String() }

// StderrString returns the captured standard error as a string.
func (p *Process) StderrString() string { return p.stderr.String() }

// exitCodeFrom extracts an exit code from a cmd.Wait error.
func exitCodeFrom(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode() // -1 if terminated by signal
	}
	return -1
}

func isExitError(err error) bool {
	var ee *exec.ExitError
	return errors.As(err, &ee)
}

// syncBuffer is a goroutine-safe bytes.Buffer.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf.Bytes()...)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
