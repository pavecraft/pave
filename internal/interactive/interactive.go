// Package interactive reads single keypresses from the terminal during a run
// and emits control Events (pause/resume/terminate/quit) on a channel consumed
// by the planner. Key handling is split from terminal setup so the mapping can
// be tested without a TTY.
package interactive

import (
	"context"
	"io"
	"os"

	"golang.org/x/term"
)

// EventKind identifies a control action requested by the user.
type EventKind string

const (
	EventPause     EventKind = "pause"
	EventResume    EventKind = "resume"
	EventTerminate EventKind = "terminate"
	EventQuit      EventKind = "quit"
)

// Event is a single control action.
type Event struct{ Kind EventKind }

// Hint is the one-line help shown at the start of an interactive run.
const Hint = "[P]ause  [R]esume  [T]erminate task  [Q]uit"

// MapKey maps a single input byte to an Event. The second return is false if
// the key is not bound to any action.
func MapKey(b byte) (Event, bool) {
	switch b {
	case 'p', 'P':
		return Event{EventPause}, true
	case 'r', 'R':
		return Event{EventResume}, true
	case 't', 'T':
		return Event{EventTerminate}, true
	case 'q', 'Q', 0x03: // q or Ctrl-C
		return Event{EventQuit}, true
	default:
		return Event{}, false
	}
}

// Listen puts stdin into raw mode (if it is a terminal) and returns a channel of
// Events. The terminal is restored when ctx is cancelled. If stdin is not a
// terminal, it returns a channel that emits nothing until ctx is cancelled, so
// callers work uniformly in non-interactive environments.
func Listen(ctx context.Context) (<-chan Event, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		ch := make(chan Event)
		go func() {
			<-ctx.Done()
			close(ch)
		}()
		return ch, nil
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	go func() {
		<-ctx.Done()
		_ = term.Restore(fd, oldState)
	}()

	return ListenReader(ctx, os.Stdin), nil
}

// ListenReader reads bytes from r and emits mapped Events until ctx is cancelled
// or r reaches EOF. It is the testable core of Listen.
//
// Reading happens in a dedicated goroutine so that cancellation is prompt even
// while a blocking Read is outstanding (a raw terminal Read cannot otherwise be
// interrupted). The reader goroutine unblocks on the next byte or on EOF.
func ListenReader(ctx context.Context, r io.Reader) <-chan Event {
	ch := make(chan Event)
	bytesCh := make(chan byte)

	// Reader goroutine: turn blocking reads into channel sends.
	go func() {
		defer close(bytesCh)
		buf := make([]byte, 1)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				select {
				case bytesCh <- buf[0]:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Mapper goroutine: map bytes to events; stop promptly on cancel.
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case b, ok := <-bytesCh:
				if !ok {
					return
				}
				if ev, ok := MapKey(b); ok {
					select {
					case ch <- ev:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return ch
}
