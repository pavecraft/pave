// Package state provides durable, crash-safe persistence for pave behind a
// single Store interface backed by pluggable drivers (SQLite, PostgreSQL,
// Turso/libSQL). All timestamps are stored as RFC3339Nano strings in UTC so the
// representation is identical across every backend.
package state

import (
	"context"
	"time"

	"github.com/pavecraft/pave/internal/project"
)

// RunStatus is the lifecycle state of a single `pave run` invocation.
type RunStatus string

const (
	RunRunning     RunStatus = "running"
	RunCompleted   RunStatus = "completed"
	RunFailed      RunStatus = "failed"
	RunInterrupted RunStatus = "interrupted"
)

// Run is one `pave run` invocation.
type Run struct {
	ID        string
	Project   string
	Provider  string
	StartedAt time.Time
	EndedAt   *time.Time
	Status    RunStatus
}

// FeatureRow is the persisted snapshot of a feature within a run.
type FeatureRow struct {
	ID          string
	RunID       string
	Title       string
	Description string
	Status      project.Status
	Priority    int
	DependsOn   []string
	UpdatedAt   time.Time
}

// Attempt is one provider invocation for a feature.
type Attempt struct {
	ID         string
	RunID      string
	FeatureID  string
	Provider   string
	Prompt     string
	Output     string
	Stderr     string
	ExitCode   int
	Success    bool
	SessionID  string
	StartedAt  time.Time
	EndedAt    *time.Time
	DurationMs int64
}

// LogLine is a structured log event, streamed to the UI in real time.
type LogLine struct {
	ID        int64
	RunID     string
	AttemptID string // empty if not tied to a specific attempt
	TS        time.Time
	Level     string
	Msg       string
	Attrs     string // JSON object of key-value pairs
}

// LimiterWindow is the persisted rate-limit backoff state for a provider.
type LimiterWindow struct {
	Provider  string
	LimitedAt time.Time
	ResetAt   *time.Time
	Reason    string
}

// Store is the persistence contract. It is the only seam to the database; no
// driver-specific SQL lives outside this package.
type Store interface {
	// Runs.
	CreateRun(ctx context.Context, r Run) error
	UpdateRunStatus(ctx context.Context, id string, status RunStatus, endedAt *time.Time) error
	GetRun(ctx context.Context, id string) (Run, error)
	ListRuns(ctx context.Context, limit int) ([]Run, error)

	// Features.
	UpsertFeature(ctx context.Context, f FeatureRow) error
	ListFeatures(ctx context.Context, runID string) ([]FeatureRow, error)
	GetFeature(ctx context.Context, runID, featureID string) (FeatureRow, error)

	// Attempts.
	CreateAttempt(ctx context.Context, a Attempt) error
	FinishAttempt(ctx context.Context, a Attempt) error
	ListAttempts(ctx context.Context, runID string) ([]Attempt, error)

	// Log lines.
	AppendLogLine(ctx context.Context, l LogLine) error
	ListLogLines(ctx context.Context, runID string, afterID int64) ([]LogLine, error)

	// Limiter windows.
	SetLimiterWindow(ctx context.Context, w LimiterWindow) error
	GetLimiterWindow(ctx context.Context, provider string) (LimiterWindow, error)

	// Close releases the underlying database resources.
	Close() error
}

// ErrNotFound is returned by Get* methods when no row matches.
type ErrNotFound struct{ What string }

func (e ErrNotFound) Error() string { return e.What + ": not found" }
