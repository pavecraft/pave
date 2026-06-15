package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/pavecraft/pave/internal/state"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func handleStoreError(w http.ResponseWriter, err error) {
	var nf state.ErrNotFound
	if errors.As(err, &nf) {
		writeError(w, http.StatusNotFound, nf.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			limit = n
		}
	}
	runs, err := s.store.ListRuns(r.Context(), limit)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.store.GetRun(r.Context(), r.PathValue("id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) listFeatures(w http.ResponseWriter, r *http.Request) {
	features, err := s.store.ListFeatures(r.Context(), r.PathValue("id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, features)
}

func (s *Server) listAttempts(w http.ResponseWriter, r *http.Request) {
	attempts, err := s.store.ListAttempts(r.Context(), r.PathValue("id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, attempts)
}

func (s *Server) getAttempt(w http.ResponseWriter, r *http.Request) {
	attempt, err := s.store.GetAttempt(r.Context(), r.PathValue("id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, attempt)
}

func (s *Server) featureHistory(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.FeatureHistory(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// staticHandler serves the embedded Vite SPA. For unknown paths it falls back
// to index.html so React Router handles client-side navigation.
func (s *Server) staticHandler(w http.ResponseWriter, r *http.Request) {
	// Strip leading slash for fs.FS lookup.
	fsPath := strings.TrimPrefix(r.URL.Path, "/")
	if fsPath == "" {
		fsPath = "index.html"
	}

	f, err := s.files.Open(fsPath)
	if err == nil {
		stat, serr := f.Stat()
		f.Close()
		// Serve the exact file if it's not a directory.
		if serr == nil && !stat.IsDir() {
			http.ServeFileFS(w, r, s.files, fsPath)
			return
		}
	}
	// SPA fallback: serve index.html for any unresolved path.
	http.ServeFileFS(w, r, s.files, "index.html")
}
