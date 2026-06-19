package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/paveforge/pave/internal/state"
)

// streamLogs streams new log lines for a run as Server-Sent Events. It polls
// the store every 500ms, flushing rows with id > lastId. The stream closes
// ~2s after the run reaches a terminal state with no new lines.
func (s *Server) streamLogs(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")

	if _, err := s.store.GetRun(r.Context(), runID); err != nil {
		handleStoreError(w, err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	var lastID int64
	if v := r.URL.Query().Get("after"); v != "" {
		lastID, _ = strconv.ParseInt(v, 10, 64)
	}

	idleTerminalTicks := 0
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		lines, err := s.store.ListLogLines(ctx, runID, lastID)
		if err != nil {
			sendSSE(w, flusher, []map[string]any{{
				"id": -1, "ts": time.Now().UTC().Format(time.RFC3339),
				"level": "error", "msg": err.Error(), "attrs": "",
			}})
			return
		}

		if len(lines) > 0 {
			lastID = lines[len(lines)-1].ID
			sendSSE(w, flusher, lines)
			idleTerminalTicks = 0
		} else {
			run, err := s.store.GetRun(ctx, runID)
			if err == nil && run.Status != state.RunRunning {
				idleTerminalTicks++
				if idleTerminalTicks > 4 { // ~2s grace
					return
				}
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func sendSSE(w http.ResponseWriter, f http.Flusher, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}
