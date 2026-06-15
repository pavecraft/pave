package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/xoai/pave/internal/project"
)

var (
	_ Provider = (*Copilot)(nil)
	_ Provider = (*Fallback)(nil)
)

func TestByName(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"claude", "copilot"} {
		p, err := ByName(name)
		if err != nil {
			t.Errorf("ByName(%q) error = %v", name, err)
			continue
		}
		if p.Name() != name {
			t.Errorf("ByName(%q).Name() = %q", name, p.Name())
		}
	}
	if _, err := ByName("bogus"); err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestCopilotName(t *testing.T) {
	t.Parallel()
	if NewCopilot().Name() != "copilot" {
		t.Error("Name() != copilot")
	}
}

// stubProvider is a configurable in-memory Provider for registry tests.
type stubProvider struct {
	name      string
	result    Result
	runs      int
	available error
}

func (s *stubProvider) Name() string { return s.name }
func (s *stubProvider) Available(context.Context) error {
	return s.available
}
func (s *stubProvider) CheckLimit(context.Context) (LimitStatus, error) {
	return LimitStatus{}, nil
}
func (s *stubProvider) Run(context.Context, Task) (Result, error) {
	s.runs++
	return s.result, nil
}

func TestFallbackUsesSecondaryOnLimit(t *testing.T) {
	t.Parallel()
	primary := &stubProvider{name: "claude", result: Result{Success: false, Stderr: "rate limit hit"}}
	secondary := &stubProvider{name: "copilot", result: Result{Success: true, Output: "done"}}
	fb := &Fallback{Primary: primary, Secondary: secondary}

	res, err := fb.Run(context.Background(), Task{Feature: project.Feature{Title: "X"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Success {
		t.Errorf("expected secondary success, got %+v", res)
	}
	if primary.runs != 1 || secondary.runs != 1 {
		t.Errorf("runs: primary=%d secondary=%d, want 1/1", primary.runs, secondary.runs)
	}
}

func TestFallbackStaysOnPrimaryWhenNotLimited(t *testing.T) {
	t.Parallel()
	primary := &stubProvider{name: "claude", result: Result{Success: true}}
	secondary := &stubProvider{name: "copilot", result: Result{Success: true}}
	fb := &Fallback{Primary: primary, Secondary: secondary}

	if _, err := fb.Run(context.Background(), Task{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if secondary.runs != 0 {
		t.Errorf("secondary should not run, ran %d", secondary.runs)
	}
}

func TestFallbackNoSecondary(t *testing.T) {
	t.Parallel()
	primary := &stubProvider{name: "claude", result: Result{Success: false, Stderr: "rate limit"}}
	fb := &Fallback{Primary: primary}

	res, err := fb.Run(context.Background(), Task{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Success {
		t.Error("expected primary's limited result to pass through")
	}
}

func TestFallbackSecondaryUnavailable(t *testing.T) {
	t.Parallel()
	primary := &stubProvider{name: "claude", result: Result{Success: false, Stderr: "429"}}
	secondary := &stubProvider{name: "copilot", available: errors.New("not installed")}
	fb := &Fallback{Primary: primary, Secondary: secondary}

	res, err := fb.Run(context.Background(), Task{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if secondary.runs != 0 {
		t.Errorf("unavailable secondary should not run, ran %d", secondary.runs)
	}
	if res.Success {
		t.Error("expected primary limited result to pass through")
	}
}
