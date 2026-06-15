package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/xoai/pave/internal/config"
	"github.com/xoai/pave/internal/provider"
	"github.com/xoai/pave/internal/state"
)

func TestBackoff(t *testing.T) {
	t.Parallel()
	initial := time.Minute
	max := time.Hour
	tests := []struct {
		strikes int
		want    time.Duration
	}{
		{1, time.Minute},
		{2, 2 * time.Minute},
		{3, 4 * time.Minute},
		{4, 8 * time.Minute},
		{10, time.Hour},  // capped
		{100, time.Hour}, // overflow-safe cap
	}
	for _, tt := range tests {
		if got := backoff(initial, max, tt.strikes); got != tt.want {
			t.Errorf("backoff(strikes=%d) = %s, want %s", tt.strikes, got, tt.want)
		}
	}
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

func testCfg() config.Limiter {
	return config.Limiter{Window: 5 * time.Hour, BackoffInitial: time.Minute, BackoffMax: time.Hour}
}

func TestObserveSetsAndClearsWindow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newStore(t)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := New(ctx, st, "claude", testCfg(), nil)
	l.now = func() time.Time { return base }
	l.jit = func() float64 { return 0.5 } // factor 1.0, no jitter

	// A limited result opens a window.
	l.Observe(provider.Result{Success: false, Stderr: "rate limit"}, nil)
	active, reset, _ := l.Status()
	if !active {
		t.Fatal("expected active window after limit")
	}
	if want := base.Add(time.Minute); !reset.Equal(want) {
		t.Errorf("resetAt = %v, want %v", reset, want)
	}

	// Persisted to the store.
	w, err := st.GetLimiterWindow(ctx, "claude")
	if err != nil {
		t.Fatalf("GetLimiterWindow: %v", err)
	}
	if w.ResetAt == nil {
		t.Fatal("persisted window missing reset")
	}

	// A successful result clears it.
	l.Observe(provider.Result{Success: true}, nil)
	if active, _, _ := l.Status(); active {
		t.Error("expected window cleared after success")
	}
}

func TestObserveExponentialStrikes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newStore(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := New(ctx, st, "claude", testCfg(), nil)
	l.now = func() time.Time { return base }
	l.jit = func() float64 { return 0.5 } // factor 1.0

	l.Observe(provider.Result{Success: false, Stderr: "429"}, nil)
	_, r1, _ := l.Status()
	l.Observe(provider.Result{Success: false, Stderr: "429"}, nil)
	_, r2, _ := l.Status()

	if r1.Sub(base) != time.Minute {
		t.Errorf("first cooldown = %s, want 1m", r1.Sub(base))
	}
	if r2.Sub(base) != 2*time.Minute {
		t.Errorf("second cooldown = %s, want 2m", r2.Sub(base))
	}
}

func TestWaitClearWhenNoWindow(t *testing.T) {
	t.Parallel()
	st := newStore(t)
	l := New(context.Background(), st, "claude", testCfg(), nil)
	if err := l.Wait(context.Background()); err != nil {
		t.Errorf("Wait with no window = %v, want nil", err)
	}
}

func TestWaitSleepsUntilReset(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newStore(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := New(ctx, st, "claude", testCfg(), nil)
	l.now = func() time.Time { return base }
	l.jit = func() float64 { return 0.5 }

	var sleptFor time.Duration
	l.sleep = func(ctx context.Context, d time.Duration) error {
		sleptFor = d
		return nil
	}

	l.Observe(provider.Result{Success: false, Stderr: "rate limit"}, nil)
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if sleptFor != time.Minute {
		t.Errorf("slept for %s, want 1m", sleptFor)
	}
}

func TestWaitHonorsContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	st := newStore(t)
	l := New(ctx, st, "claude", testCfg(), nil)
	l.now = func() time.Time { return time.Now() }
	l.resetAt = time.Now().Add(time.Hour)

	cancel()
	if err := l.Wait(ctx); err == nil {
		t.Error("expected Wait to return ctx error after cancel")
	}
}

func TestNewLoadsPersistedWindow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newStore(t)

	reset := time.Now().Add(time.Hour).UTC()
	must(t, st.SetLimiterWindow(ctx, state.LimiterWindow{
		Provider: "claude", LimitedAt: time.Now().UTC(), ResetAt: &reset, Reason: "429",
	}))

	l := New(ctx, st, "claude", testCfg(), nil)
	active, gotReset, reason := l.Status()
	if !active {
		t.Fatal("expected loaded window to be active")
	}
	if !gotReset.Equal(reset) {
		t.Errorf("resetAt = %v, want %v", gotReset, reset)
	}
	if reason != "429" {
		t.Errorf("reason = %q, want 429", reason)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
