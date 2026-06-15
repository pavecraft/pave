// Package planner is the orchestration loop. It pulls pending features, asks the
// limiter for clearance, drives the provider, applies interactive controls
// (pause/resume/terminate/quit), and persists every state transition.
package planner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/pavecraft/pave/internal/config"
	"github.com/pavecraft/pave/internal/interactive"
	"github.com/pavecraft/pave/internal/project"
	"github.com/pavecraft/pave/internal/provider"
	"github.com/pavecraft/pave/internal/state"
)

// Limiter gates provider usage around rate limits. The planner depends only on
// this interface; the concrete implementation lives in internal/limiter.
type Limiter interface {
	// Wait blocks until the provider is clear to use, or ctx is cancelled.
	Wait(ctx context.Context) error
	// Observe inspects a completed run for limit signals and updates state.
	Observe(res provider.Result, runErr error)
}

// NopLimiter never limits. Used until the real limiter is wired in.
type NopLimiter struct{}

func (NopLimiter) Wait(context.Context) error     { return nil }
func (NopLimiter) Observe(provider.Result, error) {}

// Progress receives feature lifecycle events so the caller can render a live UI.
// All methods must be safe to call concurrently.
type Progress interface {
	FeatureStarted(id string)
	FeatureFinished(id string, status project.Status)
	FeatureSkipped(id string)
	FeatureRetry(id string, attempt int)
	// FeatureTail delivers the last line of provider output while a feature is
	// running. It is called periodically and may be called with an empty line.
	FeatureTail(id string, line string)
}

// outputTailer is optionally implemented by Controls to expose live output.
type outputTailer interface {
	LastOutput() string
}

// Engine runs features for a single run.
type Engine struct {
	Store    state.Store
	Provider provider.Provider
	Limiter  Limiter
	Cfg      config.Config
	Log      *slog.Logger

	// Out receives plain-text control messages (pause/resume/quit). If nil,
	// os.Stdout is used.
	Out io.Writer

	// Progress receives feature lifecycle callbacks for live rendering.
	// If nil the engine falls back to printing plain lines to Out.
	Progress Progress

	// Events is the interactive control channel. May be nil (non-interactive).
	Events <-chan interactive.Event
}

// Summary reports the outcome of a Process call.
type Summary struct {
	Implemented int
	Failed      int
	Skipped     int // left pending (terminated, quit, or unmet deps)
}

// attemptResult records how a single attempt ended relative to user controls.
type attemptResult int

const (
	attemptNormal attemptResult = iota
	attemptTerminated
	attemptQuit
)

// Process runs the given features in order, persisting each transition. It stops
// early if the user quits or ctx is cancelled. Already-implemented features are
// skipped. Features whose dependencies are not yet implemented are left pending.
func (e *Engine) Process(ctx context.Context, run state.Run, features []state.FeatureRow) (Summary, error) {
	if e.Limiter == nil {
		e.Limiter = NopLimiter{}
	}
	var sum Summary
	done := implementedSet(features)

	for i := range features {
		f := features[i]
		if f.Status == project.StatusImplemented {
			continue
		}
		if !depsSatisfied(f, done) {
			e.notifySkipped(f.ID)
			e.log(ctx, run, "", slog.LevelInfo, "skipping feature; dependencies not met", "feature", f.ID)
			sum.Skipped++
			continue
		}

		result, ares, err := e.runFeature(ctx, run, &f)
		if err != nil {
			return sum, err
		}

		switch {
		case result.Success:
			f.Status = project.StatusImplemented
			done[f.ID] = true
			sum.Implemented++
		case ares == attemptTerminated || ares == attemptQuit:
			f.Status = project.StatusPending
			sum.Skipped++
		default:
			f.Status = project.StatusFailed
			sum.Failed++
		}
		f.UpdatedAt = time.Now()
		if err := e.Store.UpsertFeature(ctx, f); err != nil {
			return sum, err
		}
		e.notifyFinished(f.ID, f.Status)
		e.log(ctx, run, "", slog.LevelInfo, "feature finished", "feature", f.ID, "status", string(f.Status))

		if ares == attemptQuit || ctx.Err() != nil {
			break
		}
	}
	return sum, nil
}

// runFeature marks the feature in progress and runs it, retrying on failure up
// to Cfg.MaxRetries times. It returns the last result and how it ended.
func (e *Engine) runFeature(ctx context.Context, run state.Run, f *state.FeatureRow) (provider.Result, attemptResult, error) {
	f.Status = project.StatusInProgress
	f.UpdatedAt = time.Now()
	if err := e.Store.UpsertFeature(ctx, *f); err != nil {
		return provider.Result{}, attemptNormal, err
	}
	e.notifyStarted(f.ID)
	e.log(ctx, run, "", slog.LevelInfo, "feature started", "feature", f.ID)

	var (
		result   provider.Result
		ares     attemptResult
		err      error
		failures int // count only non-limit failures against MaxRetries
	)
	for attempt := 0; ; attempt++ {
		if werr := e.Limiter.Wait(ctx); werr != nil {
			return result, ares, werr
		}
		result, ares, err = e.executeAttempt(ctx, run, *f)
		if err != nil {
			return result, ares, err
		}
		e.Limiter.Observe(result, nil)

		if result.Success || ares == attemptTerminated || ares == attemptQuit {
			break
		}

		// If a limit was detected, the limiter has set a backoff window.
		// Don't count this against MaxRetries — loop again so Wait() blocks
		// until the cooldown expires, then retry the same feature.
		if limited, _ := provider.DetectLimit(result); limited {
			e.notifyRetry(f.ID, attempt+1)
			e.log(ctx, run, "", slog.LevelWarn, "usage limit hit; waiting for reset", "feature", f.ID, "attempt", attempt+1)
			continue
		}

		failures++
		if failures > e.Cfg.MaxRetries {
			break
		}
		e.notifyRetry(f.ID, attempt+1)
		e.log(ctx, run, "", slog.LevelWarn, "retrying feature", "feature", f.ID, "attempt", attempt+1)
	}
	return result, ares, nil
}

// executeAttempt records an attempt, runs the provider, and concurrently applies
// interactive control events to the in-flight subprocess.
func (e *Engine) executeAttempt(ctx context.Context, run state.Run, f state.FeatureRow) (provider.Result, attemptResult, error) {
	attemptCtx, cancel := context.WithCancel(ctx)
	if e.Cfg.TaskTimeout > 0 {
		attemptCtx, cancel = context.WithTimeout(ctx, e.Cfg.TaskTimeout)
	}
	defer cancel()

	attemptID := uuid.NewString()
	now := time.Now()
	att := state.Attempt{
		ID: attemptID, RunID: run.ID, FeatureID: f.ID, Provider: e.Provider.Name(),
		Prompt: provider.BuildPrompt(e.taskFor(f, nil)), StartedAt: now,
	}
	if err := e.Store.CreateAttempt(ctx, att); err != nil {
		return provider.Result{}, attemptNormal, err
	}

	ctrlCh := make(chan provider.Controls, 1)
	task := e.taskFor(f, func(c provider.Controls) { ctrlCh <- c })

	type runRes struct {
		r   provider.Result
		err error
	}
	resCh := make(chan runRes, 1)
	go func() {
		r, err := e.Provider.Run(attemptCtx, task)
		resCh <- runRes{r, err}
	}()

	// Phase 1: wait for the subprocess to start (yielding controls), or for it
	// to finish/cancel before it ever started.
	var controls provider.Controls
	select {
	case controls = <-ctrlCh:
	case rr := <-resCh:
		e.finishAttempt(ctx, att, rr.r, time.Since(now))
		return rr.r, attemptNormal, nil
	case <-ctx.Done():
		cancel()
		rr := <-resCh
		e.finishAttempt(ctx, att, rr.r, time.Since(now))
		return rr.r, attemptQuit, nil
	}

	// Phase 2: event loop with controls available.
	events := e.Events
	status := attemptNormal

	// Poll provider output every 200ms for live tail display.
	tailTicker := time.NewTicker(200 * time.Millisecond)
	defer tailTicker.Stop()
	tailer, _ := controls.(outputTailer)

	for {
		select {
		case <-tailTicker.C:
			if tailer != nil {
				e.notifyTail(f.ID, tailer.LastOutput())
			}

		case ev, ok := <-events:
			if !ok {
				events = nil // channel closed; stop selecting on it
				continue
			}
			switch ev.Kind {
			case interactive.EventPause:
				e.applyControl(ctx, run, "pausing", controls.Pause)
			case interactive.EventResume:
				e.applyControl(ctx, run, "resuming", controls.Resume)
			case interactive.EventTerminate:
				status = attemptTerminated
				_ = controls.Stop()
				cancel()
			case interactive.EventQuit:
				status = attemptQuit
				_ = controls.Stop()
				cancel()
			}

		case rr := <-resCh:
			e.finishAttempt(ctx, att, rr.r, time.Since(now))
			return rr.r, status, nil

		case <-ctx.Done():
			// Parent cancelled (e.g. signal). Stop and drain.
			status = attemptQuit
			_ = controls.Stop()
			cancel()
			rr := <-resCh
			e.finishAttempt(ctx, att, rr.r, time.Since(now))
			return rr.r, status, nil
		}
	}
}

func (e *Engine) applyControl(ctx context.Context, run state.Run, msg string, fn func() error) {
	if err := fn(); err != nil {
		e.logger().Warn("control failed", "action", msg, "err", err)
		return
	}
	if e.Progress == nil {
		e.print("  %s\n", msg)
	}
	_ = e.Store.AppendLogLine(ctx, state.LogLine{
		RunID: run.ID, TS: time.Now(), Level: "info", Msg: msg,
	})
}

func (e *Engine) finishAttempt(ctx context.Context, att state.Attempt, r provider.Result, dur time.Duration) {
	end := time.Now()
	att.Output = r.Output
	att.Stderr = r.Stderr
	att.ExitCode = r.RawExit
	att.Success = r.Success
	att.SessionID = r.SessionID
	att.EndedAt = &end
	att.DurationMs = dur.Milliseconds()
	if err := e.Store.FinishAttempt(ctx, att); err != nil {
		e.logger().Warn("failed to persist attempt", "err", err)
	}
	// Write provider output to log_lines so the UI live log shows it.
	level := "info"
	if !r.Success {
		level = "error"
	}
	if r.Output != "" {
		_ = e.Store.AppendLogLine(ctx, state.LogLine{
			RunID: att.RunID, AttemptID: att.ID, TS: end,
			Level: level, Msg: "provider output", Attrs: attrsJSON([]any{"output", r.Output}),
		})
	}
	if r.Stderr != "" {
		_ = e.Store.AppendLogLine(ctx, state.LogLine{
			RunID: att.RunID, AttemptID: att.ID, TS: end,
			Level: "warn", Msg: "provider stderr", Attrs: attrsJSON([]any{"stderr", r.Stderr}),
		})
	}
}

// taskFor builds a provider.Task for a feature row.
func (e *Engine) taskFor(f state.FeatureRow, onStart func(provider.Controls)) provider.Task {
	return provider.Task{
		Feature: project.Feature{
			ID:          f.ID,
			Title:       f.Title,
			Description: f.Description,
			Status:      f.Status,
			DependsOn:   f.DependsOn,
			Priority:    f.Priority,
		},
		ProjectPath: e.Cfg.ProjectPath,
		Model:       e.Cfg.Model,
		OnStart:     onStart,
	}
}

// log writes a structured line to both slog and the store (for the UI).
func (e *Engine) log(ctx context.Context, run state.Run, attemptID string, level slog.Level, msg string, args ...any) {
	e.logger().Log(ctx, level, msg, args...)
	_ = e.Store.AppendLogLine(ctx, state.LogLine{
		RunID:     run.ID,
		AttemptID: attemptID,
		TS:        time.Now(),
		Level:     levelString(level),
		Msg:       msg,
		Attrs:     attrsJSON(args),
	})
}

func (e *Engine) notifyStarted(id string) {
	if e.Progress != nil {
		e.Progress.FeatureStarted(id)
	} else {
		e.print("  >  %s\n", id)
	}
}

func (e *Engine) notifyFinished(id string, status project.Status) {
	if e.Progress != nil {
		e.Progress.FeatureFinished(id, status)
	} else {
		icon := "✓"
		if status == project.StatusFailed {
			icon = "✗"
		}
		e.print("  %s  %s → %s\n", icon, id, status)
	}
}

func (e *Engine) notifySkipped(id string) {
	if e.Progress != nil {
		e.Progress.FeatureSkipped(id)
	} else {
		e.print("  -  %s (skipped)\n", id)
	}
}

func (e *Engine) notifyRetry(id string, attempt int) {
	if e.Progress != nil {
		e.Progress.FeatureRetry(id, attempt)
	} else {
		e.print("  >  %s (retry %d)\n", id, attempt)
	}
}

func (e *Engine) notifyTail(id, line string) {
	if e.Progress != nil {
		e.Progress.FeatureTail(id, line)
	}
}

// print writes a plain-text line to Out (defaults to os.Stdout).
func (e *Engine) print(format string, args ...any) {
	w := e.Out
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintf(w, format, args...)
}

func (e *Engine) logger() *slog.Logger {
	if e.Log != nil {
		return e.Log
	}
	return slog.Default()
}

func implementedSet(features []state.FeatureRow) map[string]bool {
	done := make(map[string]bool)
	for _, f := range features {
		if f.Status == project.StatusImplemented {
			done[f.ID] = true
		}
	}
	return done
}

func depsSatisfied(f state.FeatureRow, done map[string]bool) bool {
	for _, dep := range f.DependsOn {
		if !done[dep] {
			return false
		}
	}
	return true
}
