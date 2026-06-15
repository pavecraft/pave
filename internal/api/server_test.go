package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/pavecraft/pave/internal/state"
)

// mockStore implements state.Store for testing.
type mockStore struct {
	runs           []state.Run
	features       []state.FeatureRow
	attempts       []state.Attempt
	logLines       []state.LogLine
	featureHistory []state.FeatureHistoryRow
}

func (m *mockStore) CreateRun(_ context.Context, r state.Run) error { return nil }
func (m *mockStore) UpdateRunStatus(_ context.Context, _ string, _ state.RunStatus, _ *time.Time) error {
	return nil
}
func (m *mockStore) GetRun(_ context.Context, id string) (state.Run, error) {
	for _, r := range m.runs {
		if r.ID == id {
			return r, nil
		}
	}
	return state.Run{}, state.ErrNotFound{What: "run " + id}
}
func (m *mockStore) ListRuns(_ context.Context, limit int) ([]state.Run, error) {
	if limit > len(m.runs) {
		return m.runs, nil
	}
	return m.runs[:limit], nil
}
func (m *mockStore) UpsertFeature(_ context.Context, _ state.FeatureRow) error { return nil }
func (m *mockStore) ListFeatures(_ context.Context, runID string) ([]state.FeatureRow, error) {
	var out []state.FeatureRow
	for _, f := range m.features {
		if f.RunID == runID {
			out = append(out, f)
		}
	}
	return out, nil
}
func (m *mockStore) GetFeature(_ context.Context, runID, id string) (state.FeatureRow, error) {
	for _, f := range m.features {
		if f.RunID == runID && f.ID == id {
			return f, nil
		}
	}
	return state.FeatureRow{}, state.ErrNotFound{What: "feature " + id}
}
func (m *mockStore) CreateAttempt(_ context.Context, _ state.Attempt) error { return nil }
func (m *mockStore) FinishAttempt(_ context.Context, _ state.Attempt) error { return nil }
func (m *mockStore) ListAttempts(_ context.Context, runID string) ([]state.Attempt, error) {
	var out []state.Attempt
	for _, a := range m.attempts {
		if a.RunID == runID {
			out = append(out, a)
		}
	}
	return out, nil
}
func (m *mockStore) GetAttempt(_ context.Context, id string) (state.Attempt, error) {
	for _, a := range m.attempts {
		if a.ID == id {
			return a, nil
		}
	}
	return state.Attempt{}, state.ErrNotFound{What: "attempt " + id}
}
func (m *mockStore) AppendLogLine(_ context.Context, _ state.LogLine) error { return nil }
func (m *mockStore) ListLogLines(_ context.Context, runID string, afterID int64) ([]state.LogLine, error) {
	var out []state.LogLine
	for _, l := range m.logLines {
		if l.RunID == runID && l.ID > afterID {
			out = append(out, l)
		}
	}
	return out, nil
}
func (m *mockStore) FeatureHistory(_ context.Context) ([]state.FeatureHistoryRow, error) {
	return m.featureHistory, nil
}
func (m *mockStore) SetLimiterWindow(_ context.Context, _ state.LimiterWindow) error { return nil }
func (m *mockStore) GetLimiterWindow(_ context.Context, _ string) (state.LimiterWindow, error) {
	return state.LimiterWindow{}, state.ErrNotFound{What: "limiter"}
}
func (m *mockStore) Close() error { return nil }

// minimalFS returns a tiny fs.FS with just an index.html for static serving tests.
func minimalFS() fs.FS {
	return fstest.MapFS{
		"index.html":       &fstest.MapFile{Data: []byte("<html>pave</html>")},
		"assets/index.js":  &fstest.MapFile{Data: []byte("console.log('pave')")},
		"assets/index.css": &fstest.MapFile{Data: []byte("body{}")},
	}
}

func newTestServer(store state.Store) http.Handler {
	return NewServer(store, minimalFS())
}

func TestListRunsEmpty(t *testing.T) {
	t.Parallel()
	srv := newTestServer(&mockStore{})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/api/runs", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body == "null" {
		t.Fatal("body is null; want []")
	}
	var runs []state.Run
	if err := json.Unmarshal(rec.Body.Bytes(), &runs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if runs == nil || len(runs) != 0 {
		t.Errorf("expected empty slice, got %v", runs)
	}
}

func TestListRuns(t *testing.T) {
	t.Parallel()
	now := time.Now()
	store := &mockStore{runs: []state.Run{
		{ID: "r1", Project: "/proj", Provider: "claude", StartedAt: now, Status: state.RunCompleted},
	}}
	srv := newTestServer(store)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/api/runs", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var runs []state.Run
	if err := json.Unmarshal(rec.Body.Bytes(), &runs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != "r1" {
		t.Errorf("runs = %+v", runs)
	}
}

func TestListRunsLimit(t *testing.T) {
	t.Parallel()
	now := time.Now()
	store := &mockStore{runs: []state.Run{
		{ID: "r1", StartedAt: now, Status: state.RunCompleted},
		{ID: "r2", StartedAt: now, Status: state.RunCompleted},
		{ID: "r3", StartedAt: now, Status: state.RunCompleted},
	}}
	srv := newTestServer(store)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/api/runs?limit=2", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var runs []state.Run
	json.Unmarshal(rec.Body.Bytes(), &runs)
	if len(runs) != 2 {
		t.Errorf("expected 2 runs with limit=2, got %d", len(runs))
	}
}

func TestGetRun(t *testing.T) {
	t.Parallel()
	store := &mockStore{runs: []state.Run{
		{ID: "abc", Project: "/p", Provider: "claude", StartedAt: time.Now(), Status: state.RunRunning},
	}}
	srv := newTestServer(store)

	tests := []struct {
		path string
		code int
	}{
		{"/api/runs/abc", http.StatusOK},
		{"/api/runs/missing", http.StatusNotFound},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest("GET", tt.path, nil))
		if rec.Code != tt.code {
			t.Errorf("%s: status = %d, want %d", tt.path, rec.Code, tt.code)
		}
	}
}

func TestListFeatures(t *testing.T) {
	t.Parallel()
	store := &mockStore{
		runs: []state.Run{{ID: "r1", StartedAt: time.Now(), Status: state.RunRunning}},
		features: []state.FeatureRow{
			{ID: "f1", RunID: "r1", Title: "Auth"},
			{ID: "f2", RunID: "r1", Title: "API"},
		},
	}
	srv := newTestServer(store)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/api/runs/r1/features", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var features []state.FeatureRow
	json.Unmarshal(rec.Body.Bytes(), &features)
	if len(features) != 2 {
		t.Errorf("expected 2 features, got %d", len(features))
	}
}

func TestListFeaturesEmpty(t *testing.T) {
	t.Parallel()
	srv := newTestServer(&mockStore{})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/api/runs/no-such-run/features", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body == "null" {
		t.Fatal("body is null; want []")
	}
	var features []state.FeatureRow
	json.Unmarshal(rec.Body.Bytes(), &features)
	if len(features) != 0 {
		t.Errorf("expected empty slice, got %v", features)
	}
}

func TestListAttempts(t *testing.T) {
	t.Parallel()
	store := &mockStore{attempts: []state.Attempt{
		{ID: "a1", RunID: "r1", FeatureID: "f1", Provider: "claude", StartedAt: time.Now()},
	}}
	srv := newTestServer(store)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/api/runs/r1/attempts", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var attempts []state.Attempt
	json.Unmarshal(rec.Body.Bytes(), &attempts)
	if len(attempts) != 1 || attempts[0].ID != "a1" {
		t.Errorf("attempts = %+v", attempts)
	}
}

func TestListAttemptsEmpty(t *testing.T) {
	t.Parallel()
	srv := newTestServer(&mockStore{})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/api/runs/no-such-run/attempts", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body == "null" {
		t.Fatal("body is null; want []")
	}
	var attempts []state.Attempt
	json.Unmarshal(rec.Body.Bytes(), &attempts)
	if len(attempts) != 0 {
		t.Errorf("expected empty slice, got %v", attempts)
	}
}

func TestGetAttempt(t *testing.T) {
	t.Parallel()
	store := &mockStore{attempts: []state.Attempt{
		{ID: "a1", RunID: "r1", FeatureID: "f1", Provider: "claude", StartedAt: time.Now()},
	}}
	srv := newTestServer(store)

	tests := []struct {
		path string
		code int
	}{
		{"/api/attempts/a1", http.StatusOK},
		{"/api/attempts/missing", http.StatusNotFound},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest("GET", tt.path, nil))
		if rec.Code != tt.code {
			t.Errorf("%s: status = %d, want %d", tt.path, rec.Code, tt.code)
		}
	}
}

func TestFeatureHistoryEmpty(t *testing.T) {
	t.Parallel()
	srv := newTestServer(&mockStore{})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/api/features/history", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body == "null" {
		t.Fatal("body is null; want []")
	}
	var rows []state.FeatureHistoryRow
	json.Unmarshal(rec.Body.Bytes(), &rows)
	if len(rows) != 0 {
		t.Errorf("expected empty slice, got %v", rows)
	}
}

func TestFeatureHistory(t *testing.T) {
	t.Parallel()
	store := &mockStore{featureHistory: []state.FeatureHistoryRow{
		{FeatureID: "f1", Attempts: 3, Successes: 2},
	}}
	srv := newTestServer(store)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/api/features/history", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var rows []state.FeatureHistoryRow
	json.Unmarshal(rec.Body.Bytes(), &rows)
	if len(rows) != 1 || rows[0].FeatureID != "f1" {
		t.Errorf("rows = %+v", rows)
	}
}

func TestSSEStreamUnknownRun(t *testing.T) {
	t.Parallel()
	srv := newTestServer(&mockStore{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/runs/no-such-run/stream", nil)
	ctx, cancel := context.WithTimeout(req.Context(), time.Second)
	defer cancel()
	srv.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for unknown run", rec.Code)
	}
}

func TestContentTypeJSON(t *testing.T) {
	t.Parallel()
	now := time.Now()
	store := &mockStore{runs: []state.Run{
		{ID: "r1", StartedAt: now, Status: state.RunCompleted},
	}}
	srv := newTestServer(store)

	endpoints := []string{
		"/api/runs",
		"/api/runs/r1",
		"/api/runs/r1/features",
		"/api/runs/r1/attempts",
		"/api/attempts/missing",
		"/api/features/history",
	}
	for _, path := range endpoints {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		ct := rec.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("GET %s: Content-Type = %q, want application/json", path, ct)
		}
	}
}

func TestStaticAssetServed(t *testing.T) {
	t.Parallel()
	srv := newTestServer(&mockStore{})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/assets/index.js", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for known asset", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "pave") {
		t.Errorf("expected JS asset content, got: %q", rec.Body.String())
	}
}

func TestStaticSPAFallback(t *testing.T) {
	t.Parallel()
	srv := newTestServer(&mockStore{})

	tests := []struct{ path string }{
		{"/"},
		{"/runs/abc123"},
		{"/features"},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest("GET", tt.path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", tt.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "pave") {
			t.Errorf("GET %s: body does not contain expected content", tt.path)
		}
	}
}

func TestSSEStream(t *testing.T) {
	t.Parallel()
	now := time.Now()
	store := &mockStore{
		runs: []state.Run{{ID: "r1", StartedAt: now, Status: state.RunCompleted}},
		logLines: []state.LogLine{
			{ID: 1, RunID: "r1", TS: now, Level: "info", Msg: "hello"},
		},
	}
	srv := newTestServer(store)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/runs/r1/stream", nil)
	// Cancel context quickly so the stream loop exits.
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()
	srv.ServeHTTP(rec, req.WithContext(ctx))

	body := rec.Body.String()
	if !strings.Contains(body, "data:") {
		t.Errorf("SSE response missing data frame; body = %q", body)
	}
}
