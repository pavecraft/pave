package provider

import (
	"context"
	"fmt"
	"log/slog"
)

// ByName constructs a provider by its stable name.
func ByName(name string) (Provider, error) {
	switch name {
	case "claude":
		return NewClaude(), nil
	case "copilot":
		return NewCopilot(), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", name)
	}
}

// Fallback is a Provider that runs Primary and, if a run is rejected due to a
// rate/usage limit, transparently retries the same task on Secondary.
type Fallback struct {
	Primary   Provider
	Secondary Provider
	Log       *slog.Logger
}

// Name implements Provider, reporting the primary's name.
func (f *Fallback) Name() string { return f.Primary.Name() }

// Available requires the primary to be usable; the secondary is best-effort.
func (f *Fallback) Available(ctx context.Context) error { return f.Primary.Available(ctx) }

// CheckLimit reports the primary's limit status.
func (f *Fallback) CheckLimit(ctx context.Context) (LimitStatus, error) {
	return f.Primary.CheckLimit(ctx)
}

// Run executes the task on the primary; on a detected limit it switches to the
// secondary (if configured and available) and runs there instead.
func (f *Fallback) Run(ctx context.Context, t Task) (Result, error) {
	res, err := f.Primary.Run(ctx, t)
	if err != nil {
		return res, err
	}
	if limited, reason := DetectLimit(res); limited && f.Secondary != nil {
		if avErr := f.Secondary.Available(ctx); avErr != nil {
			f.logger().Warn("primary limited but fallback unavailable",
				"primary", f.Primary.Name(), "secondary", f.Secondary.Name(), "err", avErr)
			return res, nil
		}
		f.logger().Warn("primary limited; falling back",
			"primary", f.Primary.Name(), "secondary", f.Secondary.Name(), "reason", reason)
		return f.Secondary.Run(ctx, t)
	}
	return res, err
}

func (f *Fallback) logger() *slog.Logger {
	if f.Log != nil {
		return f.Log
	}
	return slog.Default()
}
