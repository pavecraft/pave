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
	ansiReset = "\033[0m"
	ansiGreen = "\033[32m"
	ansiRed   = "\033[31m"
	ansiCyan  = "\033[36m"
	ansiDim   = "\033[2m"
	ansiUp    = "\033[%dA"  // move cursor up N lines (stays on same column)
	ansiClear = "\r\033[2K" // carriage return + erase entire line → always at col 0
)

type rowState int

const (
	rowPending rowState = iota
	rowRunning
	rowDone
	rowFailed
	rowSkipped
)

type row struct {
	id    string
	state rowState
	retry int
}

// Progress renders a live feature list with an animated spinner for the
// currently running item. It redraws in-place using ANSI escape codes when
// writing to a real terminal, and falls back to plain text otherwise.
type Progress struct {
	out   io.Writer
	ansi  bool
	rows  []*row
	index map[string]*row

	mu    sync.Mutex
	frame int

	stopCh chan struct{}
	doneCh chan struct{}
}

// New creates a Progress renderer for the given feature IDs and immediately
// prints the initial (all-pending) list. Call Stop when the run finishes.
// Set ansi=true when out is a real terminal that supports escape codes.
func New(out io.Writer, featureIDs []string, ansi bool) *Progress {
	p := &Progress{
		out:    out,
		ansi:   ansi,
		index:  make(map[string]*row, len(featureIDs)),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	for _, id := range featureIDs {
		if _, exists := p.index[id]; exists {
			continue // skip duplicate IDs
		}
		r := &row{id: id, state: rowPending}
		p.rows = append(p.rows, r)
		p.index[id] = r
	}
	p.printAll(false) // initial render — no "move up" yet
	go p.animate()
	return p
}

// FeatureStarted marks a feature as running.
func (p *Progress) FeatureStarted(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if r := p.index[id]; r != nil {
		r.state = rowRunning
	}
}

// FeatureFinished marks a feature with its final status.
func (p *Progress) FeatureFinished(id string, status project.Status) {
	p.mu.Lock()
	defer p.mu.Unlock()
	r := p.index[id]
	if r == nil {
		return
	}
	switch status {
	case project.StatusImplemented:
		r.state = rowDone
	case project.StatusFailed:
		r.state = rowFailed
	default:
		r.state = rowSkipped
	}
}

// FeatureSkipped marks a feature as skipped (unmet deps or user terminated).
func (p *Progress) FeatureSkipped(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if r := p.index[id]; r != nil {
		r.state = rowSkipped
	}
}

// FeatureRetry records the current retry attempt number for display.
func (p *Progress) FeatureRetry(id string, attempt int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if r := p.index[id]; r != nil {
		r.retry = attempt
	}
}

// Stop halts the animation and does one final redraw so all icons settle.
func (p *Progress) Stop() {
	close(p.stopCh)
	<-p.doneCh
	p.mu.Lock()
	defer p.mu.Unlock()
	p.printAll(true)
	fmt.Fprintln(p.out)
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
			p.printAll(true)
			p.mu.Unlock()
		case <-p.stopCh:
			return
		}
	}
}

// printAll renders every row. When redraw=true and ANSI is available it moves
// the cursor up first so the block is overwritten in-place.
func (p *Progress) printAll(redraw bool) {
	if redraw && p.ansi && len(p.rows) > 0 {
		fmt.Fprintf(p.out, ansiUp, len(p.rows))
	}
	for _, r := range p.rows {
		line := p.formatRow(r)
		if p.ansi {
			fmt.Fprintf(p.out, "%s%s\n", ansiClear, line)
		} else if !redraw {
			// plain text: only print once (initial), never redraw
			fmt.Fprintf(p.out, "%s\n", line)
		}
	}
}

func (p *Progress) formatRow(r *row) string {
	if p.ansi {
		return p.formatRowANSI(r)
	}
	return p.formatRowPlain(r)
}

func (p *Progress) formatRowANSI(r *row) string {
	switch r.state {
	case rowPending:
		return fmt.Sprintf("  %s·%s  %s%s%s", ansiDim, ansiReset, ansiDim, r.id, ansiReset)
	case rowRunning:
		spinner := spinnerFrames[p.frame]
		label := r.id
		if r.retry > 0 {
			label += fmt.Sprintf(" %s(retry %d)%s", ansiDim, r.retry, ansiReset)
		}
		return fmt.Sprintf("  %s%s%s  %s", ansiCyan, spinner, ansiReset, label)
	case rowDone:
		return fmt.Sprintf("  %s✓%s  %s%s%s", ansiGreen, ansiReset, ansiGreen, r.id, ansiReset)
	case rowFailed:
		return fmt.Sprintf("  %s✗%s  %s%s%s", ansiRed, ansiReset, ansiRed, r.id, ansiReset)
	case rowSkipped:
		return fmt.Sprintf("  %s-%s  %s%s%s", ansiDim, ansiReset, ansiDim, r.id, ansiReset)
	}
	return ""
}

func (p *Progress) formatRowPlain(r *row) string {
	switch r.state {
	case rowPending:
		return fmt.Sprintf("  ·  %s", r.id)
	case rowRunning:
		if r.retry > 0 {
			return fmt.Sprintf("  >  %s (retry %d)", r.id, r.retry)
		}
		return fmt.Sprintf("  >  %s", r.id)
	case rowDone:
		return fmt.Sprintf("  ✓  %s", r.id)
	case rowFailed:
		return fmt.Sprintf("  ✗  %s", r.id)
	case rowSkipped:
		return fmt.Sprintf("  -  %s", r.id)
	}
	return ""
}
