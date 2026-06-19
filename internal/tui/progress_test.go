package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/paveforge/pave/internal/project"
)

func TestProgressLifecycle(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := New(&buf, false, 0) // plain text: no ANSI, no redraw

	p.FeatureStarted("feat-a")
	p.FeatureFinished("feat-a", project.StatusImplemented)
	p.FeatureStarted("feat-b")
	p.FeatureSkipped("feat-b")
	p.FeatureStarted("feat-c")
	p.FeatureRetry("feat-c", 1)
	p.FeatureFinished("feat-c", project.StatusFailed)
	p.Stop()

	out := buf.String()
	if !strings.Contains(out, "✓") {
		t.Error("missing ✓ for implemented feature")
	}
	if !strings.Contains(out, "✗") {
		t.Error("missing ✗ for failed feature")
	}
	if !strings.Contains(out, "feat-a") || !strings.Contains(out, "feat-c") {
		t.Error("missing feature IDs in output")
	}
}

func TestProgressNonTTY(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := New(&buf, false, 0)

	p.FeatureStarted("feat-a")
	p.FeatureFinished("feat-a", project.StatusImplemented)
	p.FeatureSkipped("feat-b")
	p.Stop()

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	// Expect: "> feat-a", "✓ feat-a", "- feat-b" — 3 lines
	if len(lines) < 3 {
		t.Errorf("expected at least 3 lines, got %d: %q", len(lines), buf.String())
	}
	for _, line := range lines {
		if strings.Contains(line, "\033") {
			t.Errorf("unexpected ANSI escape in non-TTY output: %q", line)
		}
	}
}

func TestProgressRetry(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := New(&buf, false, 0)

	p.FeatureStarted("feat-a")
	p.FeatureRetry("feat-a", 2)
	p.FeatureFinished("feat-a", project.StatusFailed)
	p.Stop()

	if !strings.Contains(buf.String(), "retry") {
		t.Error("expected 'retry' in output after FeatureRetry")
	}
}

func TestProgressANSIDoesNotPanic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := New(&buf, true, 0)
	p.FeatureStarted("feat-x")
	time.Sleep(200 * time.Millisecond) // let the spinner tick a few times
	p.FeatureFinished("feat-x", project.StatusImplemented)
	p.FeatureSkipped("feat-y")
	p.Stop()
	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

func TestProgressStopWithActiveFeature(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := New(&buf, false, 0)
	p.FeatureStarted("feat-interrupted")
	// Stop without calling FeatureFinished — simulates mid-run interruption.
	p.Stop()
	// Should not deadlock or panic.
}

func TestProgressEmptyRun(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := New(&buf, false, 0)
	p.Stop() // must not panic or deadlock
}

func TestProgressCounter(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := New(&buf, true, 3) // 3 total features

	// First feature: counter should show (1/3)
	if got := p.counter(); got != "(1/3)" {
		t.Errorf("counter before any done = %q, want (1/3)", got)
	}

	p.FeatureFinished("feat-a", project.StatusImplemented)

	p.mu.Lock()
	if got := p.counter(); got != "(2/3)" {
		t.Errorf("counter after 1 done = %q, want (2/3)", got)
	}
	p.mu.Unlock()

	p.FeatureSkipped("feat-b")

	p.mu.Lock()
	if got := p.counter(); got != "(3/3)" {
		t.Errorf("counter after 2 done = %q, want (3/3)", got)
	}
	p.mu.Unlock()

	p.Stop()
}
