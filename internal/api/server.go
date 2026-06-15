// Package api provides the HTTP server that backs the pave UI. It exposes a
// REST API over the state.Store and serves the embedded React/Vite SPA.
package api

import (
	"io/fs"
	"net/http"

	"github.com/pavecraft/pave/internal/state"
)

// Server wires API routes and static file serving onto a single http.Handler.
type Server struct {
	store state.Store
	files fs.FS // root of the compiled Vite output (index.html at the top level)
}

// NewServer returns an http.Handler that serves the pave REST API at /api/*
// and the static SPA for all other paths.
func NewServer(store state.Store, files fs.FS) http.Handler {
	s := &Server{store: store, files: files}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/runs", s.listRuns)
	mux.HandleFunc("GET /api/runs/{id}", s.getRun)
	mux.HandleFunc("GET /api/runs/{id}/features", s.listFeatures)
	mux.HandleFunc("GET /api/runs/{id}/attempts", s.listAttempts)
	mux.HandleFunc("GET /api/runs/{id}/stream", s.streamLogs)
	mux.HandleFunc("GET /api/attempts/{id}", s.getAttempt)
	mux.HandleFunc("GET /api/features/history", s.featureHistory)
	mux.HandleFunc("/", s.staticHandler)

	return mux
}
