package interactive

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMapKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   byte
		want EventKind
		ok   bool
	}{
		{'p', EventPause, true},
		{'P', EventPause, true},
		{'r', EventResume, true},
		{'R', EventResume, true},
		{'t', EventTerminate, true},
		{'T', EventTerminate, true},
		{'q', EventQuit, true},
		{'Q', EventQuit, true},
		{0x03, EventQuit, true}, // Ctrl-C
		{'x', "", false},
		{'\n', "", false},
	}
	for _, tt := range tests {
		ev, ok := MapKey(tt.in)
		if ok != tt.ok {
			t.Errorf("MapKey(%q) ok = %v, want %v", tt.in, ok, tt.ok)
			continue
		}
		if ok && ev.Kind != tt.want {
			t.Errorf("MapKey(%q) = %v, want %v", tt.in, ev.Kind, tt.want)
		}
	}
}

func TestListenReaderEmitsEvents(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// "x" is ignored; the rest map to events in order.
	ch := ListenReader(ctx, strings.NewReader("pxrtq"))

	want := []EventKind{EventPause, EventResume, EventTerminate, EventQuit}
	var got []EventKind
	timeout := time.After(2 * time.Second)
	for range want {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed early; got %v", got)
			}
			got = append(got, ev.Kind)
		case <-timeout:
			t.Fatalf("timed out; got %v", got)
		}
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestListenReaderStopsOnContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	// A reader that blocks forever.
	pr, _ := blockingPipe()
	ch := ListenReader(ctx, pr)
	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to close, got an event")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not close after context cancel")
	}
}

// blockingPipe returns a reader whose Read blocks until the write end is closed.
func blockingPipe() (r *blockReader, cancel func()) {
	br := &blockReader{done: make(chan struct{})}
	return br, func() { close(br.done) }
}

type blockReader struct{ done chan struct{} }

func (b *blockReader) Read(p []byte) (int, error) {
	<-b.done
	return 0, context.Canceled
}
