package planner

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paveforge/pave/internal/config"
	"github.com/paveforge/pave/internal/interactive"
	"github.com/paveforge/pave/internal/project"
	"github.com/paveforge/pave/internal/provider"
	"github.com/paveforge/pave/internal/state"
)

// --- mocks ---

type fakeControls struct {
	pauses, resumes, stops int32
}

func (c *fakeControls) Pause() error  { atomic.AddInt32(&c.pauses, 1); return nil }
func (c *fakeControls) Resume() error { atomic.AddInt32(&c.resumes, 1); return nil }
func (c *fakeControls) Stop() error   { atomic.AddInt32(&c.stops, 1); return nil }

type mockProvider struct {
	name     string
	results  []provider.Result // consumed per call; last repeats
	errs     []error
	block    bool // block until ctx cancelled, then return result
	runs     int32
	controls *fakeControls
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Available(context.Context) error { return nil }

func (m *mockProvider) CheckLimit(context.Context) (provider.LimitStatus, error) {
	return provider.LimitStatus{}, nil
}

func (m *mockProvider) Run(ctx context.Context, t provider.Task) (provider.Result, error) {
	n := int(atomic.AddInt32(&m.runs, 1)) - 1
	if t.OnStart != nil {
		if m.controls == nil {
			m.controls = &fakeControls{}
		}
		t.OnStart(m.controls)
	}
	if m.block {
		<-ctx.Done()
	}
	return m.at(m.results, n), m.errAt(n)
}

func (m *mockProvider) at(rs []provider.Result, n int) provider.Result {
	if len(rs) == 0 {
		return provider.Result{}
	}
	if n >= len(rs) {
		n = len(rs) - 1
	}
	return rs[n]
}

func (m *mockProvider) errAt(n int) error {
	if n < len(m.errs) {
		return m.errs[n]
	}
	return nil
}

func newStore(t *testing.T) state.Store {
	t.Helper()
	st, err := state.New(context.Background(), config.Database{Driver: config.DriverSQLite, DSN: ":memory:"})
	if err != nil {
		t.Fatalf("state.New: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func seed(t *testing.T, st state.Store, runID string, rows ...state.FeatureRow) state.Run {
	t.Helper()
	ctx := context.Background()
	run := state.Run{ID: runID, Project: "/p", Provider: "claude", StartedAt: time.Now(), Status: state.RunRunning}
	if err := st.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	for _, r := range rows {
		r.RunID = runID
		r.UpdatedAt = time.Now()
		if err := st.UpsertFeature(ctx, r); err != nil {
			t.Fatalf("UpsertFeature: %v", err)
		}
	}
	return run
}

func engine(st state.Store, p provider.Provider, ev <-chan interactive.Event) *Engine {
	return &Engine{
		Store:    st,
		Provider: p,
		Limiter:  NopLimiter{},
		// Retry.BackoffInitial/Max left as zero so retries fire immediately in tests.
		Cfg:    config.Config{ProjectPath: ".", MaxRetries: 1, TaskTimeout: 0},
		Events: ev,
	}
}

// --- tests ---

func TestProcessImplementsFeature(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newStore(t)
	run := seed(t, st, "r1", state.FeatureRow{ID: "f1", Title: "One", Status: project.StatusPending})

	p := &mockProvider{name: "claude", results: []provider.Result{{Success: true, RawExit: 0, Output: "done"}}}
	rows, _ := st.ListFeatures(ctx, "r1")

	sum, err := engine(st, p, nil).Process(ctx, run, rows)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if sum.Implemented != 1 {
		t.Errorf("summary = %+v, want 1 implemented", sum)
	}
	got, _ := st.GetFeature(ctx, "r1", "f1")
	if got.Status != project.StatusImplemented {
		t.Errorf("status = %q, want implemented", got.Status)
	}
	atts, _ := st.ListAttempts(ctx, "r1")
	if len(atts) != 1 || !atts[0].Success {
		t.Errorf("attempts = %+v", atts)
	}
}

func TestProcessFailureWithRetry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newStore(t)
	run := seed(t, st, "r1", state.FeatureRow{ID: "f1", Title: "One", Status: project.StatusPending})

	p := &mockProvider{name: "claude", results: []provider.Result{{Success: false, RawExit: 1}}}
	rows, _ := st.ListFeatures(ctx, "r1")

	sum, err := engine(st, p, nil).Process(ctx, run, rows)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if sum.Failed != 1 {
		t.Errorf("summary = %+v, want 1 failed", sum)
	}
	// MaxRetries=1 ⇒ 2 attempts total.
	if got := atomic.LoadInt32(&p.runs); got != 2 {
		t.Errorf("provider runs = %d, want 2", got)
	}
	got, _ := st.GetFeature(ctx, "r1", "f1")
	if got.Status != project.StatusFailed {
		t.Errorf("status = %q, want failed", got.Status)
	}
}

func TestProcessRetrySucceeds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newStore(t)
	run := seed(t, st, "r1", state.FeatureRow{ID: "f1", Title: "One", Status: project.StatusPending})

	p := &mockProvider{name: "claude", results: []provider.Result{
		{Success: false, RawExit: 1},
		{Success: true, RawExit: 0},
	}}
	rows, _ := st.ListFeatures(ctx, "r1")

	sum, err := engine(st, p, nil).Process(ctx, run, rows)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if sum.Implemented != 1 {
		t.Errorf("summary = %+v, want 1 implemented", sum)
	}
}

func TestProcessSkipsImplemented(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newStore(t)
	run := seed(t, st, "r1", state.FeatureRow{ID: "f1", Title: "One", Status: project.StatusImplemented})

	p := &mockProvider{name: "claude", results: []provider.Result{{Success: true}}}
	rows, _ := st.ListFeatures(ctx, "r1")

	_, err := engine(st, p, nil).Process(ctx, run, rows)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if got := atomic.LoadInt32(&p.runs); got != 0 {
		t.Errorf("provider should not run for implemented feature, ran %d", got)
	}
}

func TestProcessSkipsUnmetDependency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newStore(t)
	run := seed(t, st,
		"r1",
		state.FeatureRow{ID: "dep", Title: "Dep", Status: project.StatusPending, Priority: 0},
		state.FeatureRow{ID: "f1", Title: "One", Status: project.StatusPending, Priority: 1, DependsOn: []string{"dep"}},
	)

	// Dep fails so f1's dependency is never satisfied.
	p := &mockProvider{name: "claude", results: []provider.Result{{Success: false, RawExit: 1}}}
	rows, _ := st.ListFeatures(ctx, "r1")

	sum, err := engine(st, p, nil).Process(ctx, run, rows)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if sum.Skipped < 1 {
		t.Errorf("expected f1 skipped, summary = %+v", sum)
	}
	f1, _ := st.GetFeature(ctx, "r1", "f1")
	if f1.Status != project.StatusPending {
		t.Errorf("f1 status = %q, want pending", f1.Status)
	}
}

func TestProcessDependencyChainSucceeds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newStore(t)
	run := seed(t, st,
		"r1",
		state.FeatureRow{ID: "dep", Title: "Dep", Status: project.StatusPending, Priority: 0},
		state.FeatureRow{ID: "f1", Title: "One", Status: project.StatusPending, Priority: 1, DependsOn: []string{"dep"}},
	)
	p := &mockProvider{name: "claude", results: []provider.Result{{Success: true}}}
	rows, _ := st.ListFeatures(ctx, "r1")

	sum, err := engine(st, p, nil).Process(ctx, run, rows)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if sum.Implemented != 2 {
		t.Errorf("summary = %+v, want 2 implemented", sum)
	}
}

func TestProcessQuitLeavesPending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newStore(t)
	run := seed(t, st, "r1", state.FeatureRow{ID: "f1", Title: "One", Status: project.StatusPending})

	events := make(chan interactive.Event, 1)
	events <- interactive.Event{Kind: interactive.EventQuit}

	p := &mockProvider{name: "claude", block: true, results: []provider.Result{{Success: false}}}
	rows, _ := st.ListFeatures(ctx, "r1")

	sum, err := engine(st, p, events).Process(ctx, run, rows)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if sum.Skipped != 1 {
		t.Errorf("summary = %+v, want 1 skipped", sum)
	}
	got, _ := st.GetFeature(ctx, "r1", "f1")
	if got.Status != project.StatusPending {
		t.Errorf("status = %q, want pending after quit", got.Status)
	}
	if p.controls == nil || atomic.LoadInt32(&p.controls.stops) == 0 {
		t.Error("expected Stop to be called on quit")
	}
}

func TestRetryBackoff(t *testing.T) {
	t.Parallel()
	initial := 30 * time.Second
	max := 10 * time.Minute
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 30 * time.Second},
		{2, 60 * time.Second},
		{3, 120 * time.Second},
		{7, max}, // would overflow; capped
	}
	for _, tc := range cases {
		got := retryBackoff(initial, max, tc.attempt)
		if got != tc.want {
			t.Errorf("retryBackoff(attempt=%d) = %s, want %s", tc.attempt, got, tc.want)
		}
	}
}

func TestProcessFailTwiceThenSucceed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newStore(t)
	run := seed(t, st, "r1", state.FeatureRow{ID: "f1", Title: "One", Status: project.StatusPending})

	p := &mockProvider{name: "claude", results: []provider.Result{
		{Success: false, RawExit: 1},
		{Success: false, RawExit: 1},
		{Success: true, RawExit: 0},
	}}
	rows, _ := st.ListFeatures(ctx, "r1")

	eng := &Engine{
		Store:    st,
		Provider: p,
		Limiter:  NopLimiter{},
		Cfg:      config.Config{ProjectPath: ".", MaxRetries: 2, TaskTimeout: 0},
	}
	sum, err := eng.Process(ctx, run, rows)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if sum.Implemented != 1 {
		t.Errorf("summary = %+v, want 1 implemented", sum)
	}
	if got := atomic.LoadInt32(&p.runs); got != 3 {
		t.Errorf("provider runs = %d, want 3", got)
	}
	got, _ := st.GetFeature(ctx, "r1", "f1")
	if got.Status != project.StatusImplemented {
		t.Errorf("status = %q, want implemented", got.Status)
	}
}

func TestProcessPauseResume(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newStore(t)
	run := seed(t, st, "r1", state.FeatureRow{ID: "f1", Title: "One", Status: project.StatusPending})

	events := make(chan interactive.Event, 3)
	events <- interactive.Event{Kind: interactive.EventPause}
	events <- interactive.Event{Kind: interactive.EventResume}
	events <- interactive.Event{Kind: interactive.EventTerminate}

	p := &mockProvider{name: "claude", block: true, results: []provider.Result{{Success: false}}}
	rows, _ := st.ListFeatures(ctx, "r1")

	_, err := engine(st, p, events).Process(ctx, run, rows)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if p.controls == nil {
		t.Fatal("controls never set")
	}
	if atomic.LoadInt32(&p.controls.pauses) != 1 {
		t.Errorf("pauses = %d, want 1", p.controls.pauses)
	}
	if atomic.LoadInt32(&p.controls.resumes) != 1 {
		t.Errorf("resumes = %d, want 1", p.controls.resumes)
	}
	// Terminated leaves the feature pending.
	got, _ := st.GetFeature(ctx, "r1", "f1")
	if got.Status != project.StatusPending {
		t.Errorf("status = %q, want pending after terminate", got.Status)
	}
}
