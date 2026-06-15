package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/xoai/pave/internal/project"
)

func TestProgressLifecycle(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	ids := []string{"feat-a", "feat-b", "feat-c"}
	p := New(&buf, ids, false) // plain text: no ANSI, no redraw

	p.FeatureStarted("feat-a")
	time.Sleep(10 * time.Millisecond)
	p.FeatureFinished("feat-a", project.StatusImplemented)
	p.FeatureSkipped("feat-b")
	p.FeatureStarted("feat-c")
	p.FeatureRetry("feat-c", 1)
	p.FeatureFinished("feat-c", project.StatusFailed)
	p.Stop()

	out := buf.String()
	// Initial render must contain all IDs.
	for _, id := range ids {
		if !strings.Contains(out, id) {
			t.Errorf("output missing %q", id)
		}
	}
}

func TestProgressANSIDoesNotPanic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := New(&buf, []string{"feat-x", "feat-y"}, true)
	p.FeatureStarted("feat-x")
	time.Sleep(200 * time.Millisecond) // let the spinner tick a few times
	p.FeatureFinished("feat-x", project.StatusImplemented)
	p.FeatureSkipped("feat-y")
	p.Stop()
	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

func TestProgressEmptyList(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := New(&buf, nil, false)
	p.Stop() // must not panic or deadlock
}
