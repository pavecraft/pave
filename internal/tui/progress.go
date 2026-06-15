// Package tui provides terminal progress rendering for pave run.
package tui

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/pavecraft/pave/internal/project"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const (
	ansiReset     = "\033[0m"
	ansiGreen     = "\033[32m"
	ansiRed       = "\033[31m"
	ansiCyan      = "\033[36m"
	ansiDim       = "\033[2m"
	ansiEraseLine = "\r\033[K" // carriage return + erase to end of line
)

// Progress renders a live feature list by animating a single "active" line in-place
// with \r. Completed and skipped features are appended once and never redrawn.
// This eliminates all cursor-up math and is robust against any interleaved output.
type Progress struct {
	out         io.Writer
	ansi        bool
	activeID    string
	activeRetry int
	frame       int

	mu     sync.Mutex
	stopCh chan struct{}
	doneCh chan struct{}
}

// New creates a Progress renderer. Set ansi=true when out is a real terminal.
// Call Stop when the run finishes.
func New(out io.Writer, ansi bool) *Progress {
	p := &Progress{
		out:    out,
		ansi:   ansi,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	go p.animate()
	return p
}

// FeatureStarted marks a feature as the currently running item.
// In ANSI mode we set state only; the next animator tick renders the spinner line.
func (p *Progress) FeatureStarted(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activeID = id
	p.activeRetry = 0
	if !p.ansi {
		fmt.Fprintf(p.out, "  >  %s\n", id)
	}
}

// FeatureFinished terminates the active line with the final status icon.
func (p *Progress) FeatureFinished(id string, status project.Status) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activeID = ""
	if p.ansi {
		switch status {
		case project.StatusImplemented:
			fmt.Fprintf(p.out, "%s  %s✓%s  %s%s%s\n", ansiEraseLine, ansiGreen, ansiReset, ansiGreen, id, ansiReset)
		case project.StatusFailed:
			fmt.Fprintf(p.out, "%s  %s✗%s  %s%s%s\n", ansiEraseLine, ansiRed, ansiReset, ansiRed, id, ansiReset)
		default:
			fmt.Fprintf(p.out, "%s  %s-%s  %s%s%s\n", ansiEraseLine, ansiDim, ansiReset, ansiDim, id, ansiReset)
		}
	} else {
		switch status {
		case project.StatusImplemented:
			fmt.Fprintf(p.out, "  ✓  %s\n", id)
		case project.StatusFailed:
			fmt.Fprintf(p.out, "  ✗  %s\n", id)
		default:
			fmt.Fprintf(p.out, "  -  %s\n", id)
		}
	}
}

// FeatureSkipped appends a skipped line immediately (no animation needed).
func (p *Progress) FeatureSkipped(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ansi {
		fmt.Fprintf(p.out, "  %s-%s  %s%s%s\n", ansiDim, ansiReset, ansiDim, id, ansiReset)
	} else {
		fmt.Fprintf(p.out, "  -  %s\n", id)
	}
}

// FeatureRetry records the retry attempt number; the animator picks it up.
func (p *Progress) FeatureRetry(id string, attempt int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ansi {
		p.activeRetry = attempt
	} else {
		fmt.Fprintf(p.out, "  >  %s (retry %d)\n", id, attempt)
	}
}

// Stop halts the animation and flushes any in-progress active line.
func (p *Progress) Stop() {
	close(p.stopCh)
	<-p.doneCh
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ansi && p.activeID != "" {
		// Flush the active line as pending (run was interrupted mid-feature).
		fmt.Fprintf(p.out, "%s  %s-%s  %s%s%s\n", ansiEraseLine, ansiDim, ansiReset, ansiDim, p.activeID, ansiReset)
		p.activeID = ""
	}
}

func (p *Progress) animate() {
	defer close(p.doneCh)
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.mu.Lock()
			p.frame = (p.frame + 1) % len(spinnerFrames)
			if p.ansi && p.activeID != "" {
				spin := spinnerFrames[p.frame]
				if p.activeRetry > 0 {
					fmt.Fprintf(p.out, "%s  %s%s%s  %s %s(retry %d)%s",
						ansiEraseLine, ansiCyan, spin, ansiReset, p.activeID,
						ansiDim, p.activeRetry, ansiReset)
				} else {
					fmt.Fprintf(p.out, "%s  %s%s%s  %s",
						ansiEraseLine, ansiCyan, spin, ansiReset, p.activeID)
				}
			}
			p.mu.Unlock()
		case <-p.stopCh:
			return
		}
	}
}
