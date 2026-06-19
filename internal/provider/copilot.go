package provider

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/paveforge/pave/internal/proc"
)

// Copilot drives the GitHub `copilot` CLI in headless print mode (`copilot -p`).
//
// TODO(copilot-sdk): evaluate the Copilot JSON-RPC server mode for richer
// control; the subprocess keeps both providers symmetric for now.
type Copilot struct {
	// Bin is the executable name or path. Defaults to "copilot".
	Bin string
}

// NewCopilot returns a Copilot provider with default settings.
func NewCopilot() *Copilot { return &Copilot{Bin: "copilot"} }

func (c *Copilot) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "copilot"
}

// Name implements Provider.
func (c *Copilot) Name() string { return "copilot" }

// Available implements Provider by checking that the CLI is on PATH.
func (c *Copilot) Available(ctx context.Context) error {
	if _, err := exec.LookPath(c.bin()); err != nil {
		return fmt.Errorf("copilot CLI not found (%q): %w", c.bin(), err)
	}
	return nil
}

// buildArgs assembles the argument vector for a task.
func (c *Copilot) buildArgs(t Task) []string {
	args := []string{"-p", BuildPrompt(t)}
	if t.Model != "" {
		args = append(args, "--model", t.Model)
	}
	return args
}

// Run implements Provider. It shells out to `copilot -p`, capturing output and
// exit status with the project path as the working directory.
func (c *Copilot) Run(ctx context.Context, t Task) (Result, error) {
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
	}
	if waitErr != nil {
		return res, fmt.Errorf("copilot run: %w", waitErr)
	}
	return res, nil
}

// CheckLimit implements Provider. Limits are detected reactively from run output
// by the limiter rather than proactively here.
func (c *Copilot) CheckLimit(ctx context.Context) (LimitStatus, error) {
	return LimitStatus{Limited: false}, nil
}
