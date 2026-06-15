package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/pavecraft/pave/internal/project"
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

func TestProgressTail(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := New(&buf, true, 1)
	p.FeatureStarted("feat-a")

	p.FeatureTail("feat-a", "Writing src/index.ts")
	p.mu.Lock()
	if p.activeTail != "Writing src/index.ts" {
		t.Errorf("activeTail = %q, want %q", p.activeTail, "Writing src/index.ts")
	}
	p.mu.Unlock()

	// Tail for a different ID should be ignored.
	p.FeatureTail("feat-other", "should be ignored")
	p.mu.Lock()
	if p.activeTail != "Writing src/index.ts" {
		t.Errorf("activeTail changed on wrong ID: %q", p.activeTail)
	}
	p.mu.Unlock()

	// Finishing clears the tail.
	p.FeatureFinished("feat-a", project.StatusImplemented)
	p.mu.Lock()
	if p.activeTail != "" {
		t.Errorf("activeTail not cleared after finish: %q", p.activeTail)
	}
	p.mu.Unlock()
	p.Stop()
}

func TestTruncateTail(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 70)
	got := truncateTail(long)
	if len([]rune(got)) > 60 {
		t.Errorf("truncateTail did not truncate: len=%d", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Error("truncateTail missing ellipsis")
	}
	// ANSI codes should be stripped.
	ansi := "\033[32mhello\033[0m"
	if got := truncateTail(ansi); got != "hello" {
		t.Errorf("truncateTail(%q) = %q, want %q", ansi, got, "hello")
	}
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
