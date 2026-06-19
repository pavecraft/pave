// Package limiter detects provider rate-limit conditions, tracks a backoff
// window, and gates further provider use until the window clears. It treats the
// provider as a black box, relying on observed output signals rather than
// assumptions, and persists the active window so a restart respects an ongoing
// cooldown.
package limiter

import (
	"context"
	"log/slog"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/paveforge/pave/internal/config"
	"github.com/paveforge/pave/internal/provider"
	"github.com/paveforge/pave/internal/state"
)

// Limiter implements planner.Limiter.
type Limiter struct {
	store    state.Store
	provider string
	cfg      config.Limiter
	log      *slog.Logger

	mu      sync.Mutex
	strikes int
	resetAt time.Time
	reason  string

	// Injectable for testing.
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
	jit   func() float64
}

// New constructs a Limiter, loading any persisted active window for provider so
// an ongoing cooldown survives restarts.
func New(ctx context.Context, store state.Store, providerName string, cfg config.Limiter, log *slog.Logger) *Limiter {
	l := &Limiter{
		store:    store,
		provider: providerName,
		cfg:      cfg,
		log:      orDefault(log),
		now:      time.Now,
		jit:      rand.Float64,
	}
	l.sleep = realSleep

	if w, err := store.GetLimiterWindow(ctx, providerName); err == nil && w.ResetAt != nil {
		l.resetAt = *w.ResetAt
		l.reason = w.Reason
	}
	return l
}

// Wait blocks until the provider is clear to use again, or ctx is cancelled.
func (l *Limiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	reset, reason := l.resetAt, l.reason
	l.mu.Unlock()

	d := reset.Sub(l.now())
	if d <= 0 {
		return nil
	}
	l.log.Warn("rate limit active; waiting", "provider", l.provider, "reason", reason,
		"reset_at", reset.Format(time.RFC3339), "wait", d.Round(time.Second).String())
	return l.sleep(ctx, d)
}

// Observe inspects a completed run for limit signals and updates the window.
func (l *Limiter) Observe(res provider.Result, _ error) {
	limited, reason := provider.DetectLimit(res)

	l.mu.Lock()
	defer l.mu.Unlock()

	if !limited {
		if l.strikes != 0 {
			l.strikes = 0
			l.resetAt = time.Time{}
			l.reason = ""
		}
		return
	}

	l.strikes++

	// If the provider told us exactly when the limit resets, use that directly
	// instead of computing an exponential backoff guess.
	if explicit := provider.ParseResetTime(res.Output + "\n" + res.Stderr); !explicit.IsZero() {
		l.resetAt = explicit.Add(l.cfg.ResetBuffer)
		l.reason = reason
		wait := time.Until(l.resetAt).Round(time.Second)
		l.log.Warn("rate limit detected; using explicit reset time", "provider", l.provider,
			"reason", reason, "reset_at", l.resetAt.Format(time.RFC3339), "wait", wait.String())
	} else {
		d := l.backoffWithJitter(l.strikes)
		l.resetAt = l.now().Add(d)
		l.reason = reason
		l.log.Warn("rate limit detected", "provider", l.provider, "reason", reason,
			"strikes", l.strikes, "cooldown", d.Round(time.Second).String())
	}

	_ = l.store.SetLimiterWindow(context.Background(), state.LimiterWindow{
		Provider:  l.provider,
		LimitedAt: l.now(),
		ResetAt:   &l.resetAt,
		Reason:    reason,
	})
}

// Status returns the current window's reset time and reason. The bool reports
// whether a cooldown is currently active.
func (l *Limiter) Status() (active bool, resetAt time.Time, reason string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.resetAt.IsZero() || !l.resetAt.After(l.now()) {
		return false, l.resetAt, l.reason
	}
	return true, l.resetAt, l.reason
}

// backoffWithJitter computes the cooldown for the given strike count: an
// exponential backoff from BackoffInitial, capped at BackoffMax, with up to
// ±25% jitter.
func (l *Limiter) backoffWithJitter(strikes int) time.Duration {
	base := backoff(l.cfg.BackoffInitial, l.cfg.BackoffMax, strikes)
	// Jitter factor in [0.75, 1.25).
	factor := 0.75 + 0.5*l.jit()
	d := time.Duration(float64(base) * factor)
	if d > l.cfg.BackoffMax {
		d = l.cfg.BackoffMax
	}
	return d
}

// backoff returns initial * 2^(strikes-1), capped at max.
func backoff(initial, max time.Duration, strikes int) time.Duration {
	if strikes < 1 {
		strikes = 1
	}
	mult := math.Pow(2, float64(strikes-1))
	d := time.Duration(float64(initial) * mult)
	if d > max || d <= 0 {
		return max
	}
	return d
}

func realSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func orDefault(l *slog.Logger) *slog.Logger {
	if l != nil {
		return l
	}
	return slog.Default()
}
