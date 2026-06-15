package state

import (
	"context"
	"testing"
	"time"

	"github.com/xoai/pave/internal/project"
)

// newTestStore returns an in-memory SQLite store for contract testing.
func newTestStore(t *testing.T) Store {
	t.Helper()
	st, err := openSQLite(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("openSQLite(:memory:) error = %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestRunLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newTestStore(t)

	start := time.Now().Truncate(time.Millisecond)
	run := Run{ID: "run1", Project: "/proj", Provider: "claude", StartedAt: start, Status: RunRunning}
	if err := st.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	got, err := st.GetRun(ctx, "run1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Provider != "claude" || got.Status != RunRunning {
		t.Errorf("GetRun = %+v", got)
	}
	if !got.StartedAt.Equal(start) {
		t.Errorf("StartedAt = %v, want %v", got.StartedAt, start)
	}

	end := start.Add(time.Minute)
	if err := st.UpdateRunStatus(ctx, "run1", RunCompleted, &end); err != nil {
		t.Fatalf("UpdateRunStatus: %v", err)
	}
	got, _ = st.GetRun(ctx, "run1")
	if got.Status != RunCompleted || got.EndedAt == nil || !got.EndedAt.Equal(end) {
		t.Errorf("after update = %+v", got)
	}

	runs, err := st.ListRuns(ctx, 10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("ListRuns = %v, err = %v", runs, err)
	}
}

func TestGetRunNotFound(t *testing.T) {
	t.Parallel()
	st := newTestStore(t)
	_, err := st.GetRun(context.Background(), "missing")
	if _, ok := err.(ErrNotFound); !ok {
		t.Fatalf("expected ErrNotFound, got %T (%v)", err, err)
	}
}

func TestFeatureUpsert(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newTestStore(t)

	must(t, st.CreateRun(ctx, Run{ID: "r", Project: "/p", Provider: "claude", StartedAt: time.Now(), Status: RunRunning}))

	f := FeatureRow{
		ID: "f01", RunID: "r", Title: "Config", Description: "load",
		Status: project.StatusPending, Priority: 2, DependsOn: []string{"f00"},
		UpdatedAt: time.Now(),
	}
	must(t, st.UpsertFeature(ctx, f))

	// Upsert again with a new status; should update, not duplicate.
	f.Status = project.StatusImplemented
	must(t, st.UpsertFeature(ctx, f))

	got, err := st.GetFeature(ctx, "r", "f01")
	if err != nil {
		t.Fatalf("GetFeature: %v", err)
	}
	if got.Status != project.StatusImplemented {
		t.Errorf("status = %q, want implemented", got.Status)
	}
	if len(got.DependsOn) != 1 || got.DependsOn[0] != "f00" {
		t.Errorf("DependsOn = %v", got.DependsOn)
	}

	list, err := st.ListFeatures(ctx, "r")
	if err != nil || len(list) != 1 {
		t.Fatalf("ListFeatures = %v (err %v), want 1", list, err)
	}
}

func TestFeatureOrdering(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newTestStore(t)
	must(t, st.CreateRun(ctx, Run{ID: "r", Project: "/p", Provider: "claude", StartedAt: time.Now(), Status: RunRunning}))

	now := time.Now()
	must(t, st.UpsertFeature(ctx, FeatureRow{ID: "b", RunID: "r", Title: "B", Status: project.StatusPending, Priority: 5, UpdatedAt: now}))
	must(t, st.UpsertFeature(ctx, FeatureRow{ID: "a", RunID: "r", Title: "A", Status: project.StatusPending, Priority: 1, UpdatedAt: now}))

	list, _ := st.ListFeatures(ctx, "r")
	if len(list) != 2 || list[0].ID != "a" {
		t.Errorf("expected priority ordering, got %v", list)
	}
}

func TestAttemptLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newTestStore(t)
	must(t, st.CreateRun(ctx, Run{ID: "r", Project: "/p", Provider: "claude", StartedAt: time.Now(), Status: RunRunning}))

	start := time.Now().Truncate(time.Millisecond)
	a := Attempt{
		ID: "a1", RunID: "r", FeatureID: "f01", Provider: "claude",
		Prompt: "do the thing", StartedAt: start,
	}
	must(t, st.CreateAttempt(ctx, a))

	end := start.Add(2 * time.Second)
	a.Output = "done"
	a.Stderr = "warn"
	a.ExitCode = 0
	a.Success = true
	a.SessionID = "sess-123"
	a.EndedAt = &end
	a.DurationMs = 2000
	must(t, st.FinishAttempt(ctx, a))

	list, err := st.ListAttempts(ctx, "r")
	if err != nil || len(list) != 1 {
		t.Fatalf("ListAttempts = %v (err %v)", list, err)
	}
	got := list[0]
	if !got.Success || got.Output != "done" || got.SessionID != "sess-123" || got.DurationMs != 2000 {
		t.Errorf("finished attempt = %+v", got)
	}
}

func TestLogLines(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newTestStore(t)
	must(t, st.CreateRun(ctx, Run{ID: "r", Project: "/p", Provider: "claude", StartedAt: time.Now(), Status: RunRunning}))

	for i := 0; i < 3; i++ {
		must(t, st.AppendLogLine(ctx, LogLine{RunID: "r", TS: time.Now(), Level: "info", Msg: "line"}))
	}

	all, err := st.ListLogLines(ctx, "r", 0)
	if err != nil || len(all) != 3 {
		t.Fatalf("ListLogLines(0) = %v (err %v)", all, err)
	}
	// IDs must be monotonically increasing.
	if !(all[0].ID < all[1].ID && all[1].ID < all[2].ID) {
		t.Errorf("ids not increasing: %d %d %d", all[0].ID, all[1].ID, all[2].ID)
	}

	tail, err := st.ListLogLines(ctx, "r", all[0].ID)
	if err != nil || len(tail) != 2 {
		t.Fatalf("ListLogLines(after first) = %v (err %v), want 2", tail, err)
	}
}

func TestLimiterWindow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newTestStore(t)

	limited := time.Now().Truncate(time.Millisecond)
	reset := limited.Add(time.Hour)
	w := LimiterWindow{Provider: "claude", LimitedAt: limited, ResetAt: &reset, Reason: "429"}
	must(t, st.SetLimiterWindow(ctx, w))

	// Overwrite (upsert) with a new reason.
	w.Reason = "usage limit"
	must(t, st.SetLimiterWindow(ctx, w))

	got, err := st.GetLimiterWindow(ctx, "claude")
	if err != nil {
		t.Fatalf("GetLimiterWindow: %v", err)
	}
	if got.Reason != "usage limit" || got.ResetAt == nil || !got.ResetAt.Equal(reset) {
		t.Errorf("limiter window = %+v", got)
	}

	if _, err := st.GetLimiterWindow(ctx, "copilot"); err == nil {
		t.Error("expected ErrNotFound for unknown provider")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
