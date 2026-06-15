// Package provider defines the Provider seam that makes pave agnostic to the
// underlying AI coding CLI, plus concrete implementations (Claude, Copilot).
// The planner depends only on the Provider interface.
package provider

import (
	"context"
	"time"

	"github.com/pavecraft/pave/internal/project"
)

// Controls is the subset of subprocess lifecycle operations the planner uses to
// steer an in-flight run under interactive control. *proc.Process satisfies it.
type Controls interface {
	Pause() error
	Resume() error
	Stop() error
}

// Task is a single unit of work handed to a provider.
type Task struct {
	Feature     project.Feature
	ProjectPath string
	Context     string // additional prompt context
	Model       string // provider-specific model; empty = provider default

	// OnStart, if set, is called once the underlying subprocess has started,
	// handing back controls so the caller can pause/resume/stop it.
	OnStart func(Controls)
}

// Result is the outcome of running a Task.
type Result struct {
	Output    string        // captured stdout
	Stderr    string        // captured stderr
	Success   bool          // true if the provider exited 0
	SessionID string        // resumable session id, if the provider exposes one
	Duration  time.Duration // wall-clock execution time
	RawExit   int           // raw process exit code
}

// LimitStatus describes a provider's current usage/limit state.
type LimitStatus struct {
	Limited bool
	ResetAt time.Time // zero value if unknown
	Reason  string
}

// Provider abstracts an AI coding CLI driven as a subprocess.
type Provider interface {
	// Name returns a stable identifier, e.g. "claude" or "copilot".
	Name() string

	// Run executes a single coding task by shelling out to the provider CLI in
	// headless mode. It must honor ctx cancellation.
	Run(ctx context.Context, t Task) (Result, error)

	// CheckLimit reports the current usage/limit status if known. When unknown,
	// it returns Limited=false with a zero ResetAt.
	CheckLimit(ctx context.Context) (LimitStatus, error)

	// Available reports whether the underlying CLI is installed and usable.
	Available(ctx context.Context) error
}
