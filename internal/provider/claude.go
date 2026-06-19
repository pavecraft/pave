package provider

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/paveforge/pave/internal/proc"
)

// Claude drives the `claude` CLI in headless print mode (`claude -p`).
type Claude struct {
	// Bin is the executable name or path. Defaults to "claude".
	Bin string
}

// NewClaude returns a Claude provider with default settings.
func NewClaude() *Claude { return &Claude{Bin: "claude"} }

func (c *Claude) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "claude"
}

// Name implements Provider.
func (c *Claude) Name() string { return "claude" }

// Available implements Provider by checking that the CLI is on PATH.
func (c *Claude) Available(ctx context.Context) error {
	if _, err := exec.LookPath(c.bin()); err != nil {
		return fmt.Errorf("claude CLI not found (%q): %w", c.bin(), err)
	}
	return nil
}

// buildArgs assembles the argument vector for a task.
func (c *Claude) buildArgs(t Task) []string {
	// --dangerously-skip-permissions is required for headless operation: without it
	// the claude CLI pauses on every tool call waiting for interactive approval,
	// which hangs the subprocess indefinitely in non-TTY mode.
	args := []string{"-p", BuildPrompt(t), "--dangerously-skip-permissions"}
	if t.Model != "" {
		args = append(args, "--model", t.Model)
	}
	if t.Effort != "" {
		args = append(args, "--effort", t.Effort)
	}
	return args
}

// Run implements Provider. It shells out to `claude -p`, capturing output and
// exit status. The working directory is the task's project path.
func (c *Claude) Run(ctx context.Context, t Task) (Result, error) {
	start := time.Now()
	p, err := proc.Start(ctx, c.bin(), c.buildArgs(t), t.ProjectPath)
	if err != nil {
		return Result{}, err
	}
	if t.OnStart != nil {
		t.OnStart(p)
	}
	code, waitErr := p.Wait()
	res := Result{
		Output:   p.StdoutString(),
		Stderr:   p.StderrString(),
		Success:  code == 0 && waitErr == nil,
		Duration: time.Since(start),
		RawExit:  code,
		// TODO(session): parse --output-format json to capture session_id.
	}
	if waitErr != nil {
		return res, fmt.Errorf("claude run: %w", waitErr)
	}
	return res, nil
}

// CheckLimit implements Provider. The Claude CLI does not expose a stable
// machine-readable usage endpoint, so limits are detected reactively by the
// limiter from run output rather than proactively here.
func (c *Claude) CheckLimit(ctx context.Context) (LimitStatus, error) {
	// TODO(limit-detection): query a usage source if a stable one becomes available.
	return LimitStatus{Limited: false}, nil
}
