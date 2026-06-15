package planner

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// levelString renders a slog.Level as a lowercase token for storage.
func levelString(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "error"
	case l >= slog.LevelWarn:
		return "warn"
	case l >= slog.LevelInfo:
		return "info"
	default:
		return "debug"
	}
}

// attrsJSON turns a flat key/value arg slice (as passed to slog) into a JSON
// object string. Odd trailing args are ignored. Returns "" when empty.
func attrsJSON(args []any) string {
	if len(args) == 0 {
		return ""
	}
	m := make(map[string]any, len(args)/2)
	for i := 0; i+1 < len(args); i += 2 {
		key := fmt.Sprint(args[i])
		m[key] = args[i+1]
	}
	if len(m) == 0 {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}
