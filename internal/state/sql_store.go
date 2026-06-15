package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pavecraft/pave/internal/project"
)

// sqlStore implements Store over any database/sql backend, parameterized by a
// dialect. It is shared by the SQLite, Postgres, and Turso drivers.
type sqlStore struct {
	db *sql.DB
	d  dialect
}

// newSQLStore opens db, runs migrations, and returns a ready Store.
func newSQLStore(ctx context.Context, db *sql.DB, d dialect) (*sqlStore, error) {
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("pinging %s: %w", d.name, err)
	}
	if err := runMigrations(ctx, db, d); err != nil {
		return nil, err
	}
	return &sqlStore{db: db, d: d}, nil
}

func (s *sqlStore) exec(ctx context.Context, query string, args ...any) error {
	_, err := s.db.ExecContext(ctx, s.d.rebind(query), args...)
	return err
}

func (s *sqlStore) Close() error { return s.db.Close() }

// --- Runs ---

func (s *sqlStore) CreateRun(ctx context.Context, r Run) error {
	const q = `INSERT INTO runs (id, project, provider, started_at, ended_at, status)
		VALUES (?, ?, ?, ?, ?, ?)`
	if err := s.exec(ctx, q, r.ID, r.Project, r.Provider,
		fmtTime(r.StartedAt), fmtTimePtr(r.EndedAt), string(r.Status)); err != nil {
		return fmt.Errorf("creating run: %w", err)
	}
	return nil
}

func (s *sqlStore) UpdateRunStatus(ctx context.Context, id string, status RunStatus, endedAt *time.Time) error {
	const q = `UPDATE runs SET status = ?, ended_at = ? WHERE id = ?`
	if err := s.exec(ctx, q, string(status), fmtTimePtr(endedAt), id); err != nil {
		return fmt.Errorf("updating run status: %w", err)
	}
	return nil
}

func (s *sqlStore) GetRun(ctx context.Context, id string) (Run, error) {
	const q = `SELECT id, project, provider, started_at, ended_at, status FROM runs WHERE id = ?`
	row := s.db.QueryRowContext(ctx, s.d.rebind(q), id)
	r, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrNotFound{What: "run " + id}
	}
	return r, err
}

func (s *sqlStore) ListRuns(ctx context.Context, limit int) ([]Run, error) {
	const q = `SELECT id, project, provider, started_at, ended_at, status
		FROM runs ORDER BY started_at DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, s.d.rebind(q), limit)
	if err != nil {
		return nil, fmt.Errorf("listing runs: %w", err)
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- Features ---

func (s *sqlStore) UpsertFeature(ctx context.Context, f FeatureRow) error {
	deps, err := json.Marshal(f.DependsOn)
	if err != nil {
		return fmt.Errorf("marshaling depends_on: %w", err)
	}
	const q = `INSERT INTO features
		(id, run_id, title, description, status, priority, depends_on, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id, run_id) DO UPDATE SET
			title = excluded.title,
			description = excluded.description,
			status = excluded.status,
			priority = excluded.priority,
			depends_on = excluded.depends_on,
			updated_at = excluded.updated_at`
	if err := s.exec(ctx, q, f.ID, f.RunID, f.Title, f.Description,
		string(f.Status), f.Priority, string(deps), fmtTime(f.UpdatedAt)); err != nil {
		return fmt.Errorf("upserting feature: %w", err)
	}
	return nil
}

func (s *sqlStore) ListFeatures(ctx context.Context, runID string) ([]FeatureRow, error) {
	const q = `SELECT id, run_id, title, description, status, priority, depends_on, updated_at
		FROM features WHERE run_id = ? ORDER BY priority ASC, id ASC`
	rows, err := s.db.QueryContext(ctx, s.d.rebind(q), runID)
	if err != nil {
		return nil, fmt.Errorf("listing features: %w", err)
	}
	defer rows.Close()
	var out []FeatureRow
	for rows.Next() {
		f, err := scanFeature(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *sqlStore) GetFeature(ctx context.Context, runID, featureID string) (FeatureRow, error) {
	const q = `SELECT id, run_id, title, description, status, priority, depends_on, updated_at
		FROM features WHERE run_id = ? AND id = ?`
	row := s.db.QueryRowContext(ctx, s.d.rebind(q), runID, featureID)
	f, err := scanFeature(row)
	if errors.Is(err, sql.ErrNoRows) {
		return FeatureRow{}, ErrNotFound{What: "feature " + featureID}
	}
	return f, err
}

// --- Attempts ---

func (s *sqlStore) CreateAttempt(ctx context.Context, a Attempt) error {
	const q = `INSERT INTO attempts
		(id, run_id, feature_id, provider, prompt, output, stderr, exit_code,
		 success, session_id, started_at, ended_at, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if err := s.exec(ctx, q, a.ID, a.RunID, a.FeatureID, a.Provider, a.Prompt,
		a.Output, a.Stderr, a.ExitCode, b2i(a.Success), a.SessionID,
		fmtTime(a.StartedAt), fmtTimePtr(a.EndedAt), a.DurationMs); err != nil {
		return fmt.Errorf("creating attempt: %w", err)
	}
	return nil
}

func (s *sqlStore) FinishAttempt(ctx context.Context, a Attempt) error {
	const q = `UPDATE attempts SET
			output = ?, stderr = ?, exit_code = ?, success = ?,
			session_id = ?, ended_at = ?, duration_ms = ?
		WHERE id = ?`
	if err := s.exec(ctx, q, a.Output, a.Stderr, a.ExitCode, b2i(a.Success),
		a.SessionID, fmtTimePtr(a.EndedAt), a.DurationMs, a.ID); err != nil {
		return fmt.Errorf("finishing attempt: %w", err)
	}
	return nil
}

func (s *sqlStore) ListAttempts(ctx context.Context, runID string) ([]Attempt, error) {
	const q = `SELECT id, run_id, feature_id, provider, prompt, output, stderr,
			exit_code, success, session_id, started_at, ended_at, duration_ms
		FROM attempts WHERE run_id = ? ORDER BY started_at ASC`
	rows, err := s.db.QueryContext(ctx, s.d.rebind(q), runID)
	if err != nil {
		return nil, fmt.Errorf("listing attempts: %w", err)
	}
	defer rows.Close()
	var out []Attempt
	for rows.Next() {
		a, err := scanAttempt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *sqlStore) GetAttempt(ctx context.Context, id string) (Attempt, error) {
	const q = `SELECT id, run_id, feature_id, provider, prompt, output, stderr,
			exit_code, success, session_id, started_at, ended_at, duration_ms
		FROM attempts WHERE id = ?`
	row := s.db.QueryRowContext(ctx, s.d.rebind(q), id)
	a, err := scanAttempt(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Attempt{}, ErrNotFound{What: "attempt " + id}
	}
	return a, err
}

func (s *sqlStore) FeatureHistory(ctx context.Context) ([]FeatureHistoryRow, error) {
	const q = `SELECT feature_id, COUNT(*) AS attempts, SUM(success) AS successes
		FROM attempts GROUP BY feature_id ORDER BY attempts DESC`
	rows, err := s.db.QueryContext(ctx, s.d.rebind(q))
	if err != nil {
		return nil, fmt.Errorf("feature history: %w", err)
	}
	defer rows.Close()
	var out []FeatureHistoryRow
	for rows.Next() {
		var r FeatureHistoryRow
		if err := rows.Scan(&r.FeatureID, &r.Attempts, &r.Successes); err != nil {
			return nil, fmt.Errorf("scanning feature history: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- Log lines ---

func (s *sqlStore) AppendLogLine(ctx context.Context, l LogLine) error {
	const q = `INSERT INTO log_lines (run_id, attempt_id, ts, level, msg, attrs)
		VALUES (?, ?, ?, ?, ?, ?)`
	if err := s.exec(ctx, q, l.RunID, l.AttemptID, fmtTime(l.TS), l.Level, l.Msg, l.Attrs); err != nil {
		return fmt.Errorf("appending log line: %w", err)
	}
	return nil
}

func (s *sqlStore) ListLogLines(ctx context.Context, runID string, afterID int64) ([]LogLine, error) {
	const q = `SELECT id, run_id, attempt_id, ts, level, msg, attrs
		FROM log_lines WHERE run_id = ? AND id > ? ORDER BY id ASC`
	rows, err := s.db.QueryContext(ctx, s.d.rebind(q), runID, afterID)
	if err != nil {
		return nil, fmt.Errorf("listing log lines: %w", err)
	}
	defer rows.Close()
	var out []LogLine
	for rows.Next() {
		l, err := scanLogLine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// --- Limiter windows ---

func (s *sqlStore) SetLimiterWindow(ctx context.Context, w LimiterWindow) error {
	const q = `INSERT INTO limiter_windows (provider, limited_at, reset_at, reason)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (provider) DO UPDATE SET
			limited_at = excluded.limited_at,
			reset_at = excluded.reset_at,
			reason = excluded.reason`
	if err := s.exec(ctx, q, w.Provider, fmtTime(w.LimitedAt), fmtTimePtr(w.ResetAt), w.Reason); err != nil {
		return fmt.Errorf("setting limiter window: %w", err)
	}
	return nil
}

func (s *sqlStore) GetLimiterWindow(ctx context.Context, provider string) (LimiterWindow, error) {
	const q = `SELECT provider, limited_at, reset_at, reason FROM limiter_windows WHERE provider = ?`
	row := s.db.QueryRowContext(ctx, s.d.rebind(q), provider)
	var (
		w         LimiterWindow
		limitedAt string
		resetAt   sql.NullString
	)
	err := row.Scan(&w.Provider, &limitedAt, &resetAt, &w.Reason)
	if errors.Is(err, sql.ErrNoRows) {
		return LimiterWindow{}, ErrNotFound{What: "limiter window " + provider}
	}
	if err != nil {
		return LimiterWindow{}, fmt.Errorf("scanning limiter window: %w", err)
	}
	if w.LimitedAt, err = parseTime(limitedAt); err != nil {
		return LimiterWindow{}, err
	}
	if w.ResetAt, err = parseTimePtr(resetAt); err != nil {
		return LimiterWindow{}, err
	}
	return w, nil
}

// --- scan helpers ---

// scanner abstracts *sql.Row and *sql.Rows for shared scan functions.
type scanner interface {
	Scan(dest ...any) error
}

func scanRun(sc scanner) (Run, error) {
	var (
		r         Run
		startedAt string
		endedAt   sql.NullString
		status    string
	)
	if err := sc.Scan(&r.ID, &r.Project, &r.Provider, &startedAt, &endedAt, &status); err != nil {
		return Run{}, err
	}
	r.Status = RunStatus(status)
	var err error
	if r.StartedAt, err = parseTime(startedAt); err != nil {
		return Run{}, err
	}
	if r.EndedAt, err = parseTimePtr(endedAt); err != nil {
		return Run{}, err
	}
	return r, nil
}

func scanFeature(sc scanner) (FeatureRow, error) {
	var (
		f         FeatureRow
		status    string
		deps      string
		updatedAt string
	)
	if err := sc.Scan(&f.ID, &f.RunID, &f.Title, &f.Description, &status,
		&f.Priority, &deps, &updatedAt); err != nil {
		return FeatureRow{}, err
	}
	f.Status = project.Status(status)
	if err := json.Unmarshal([]byte(deps), &f.DependsOn); err != nil {
		return FeatureRow{}, fmt.Errorf("unmarshaling depends_on: %w", err)
	}
	var err error
	if f.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return FeatureRow{}, err
	}
	return f, nil
}

func scanAttempt(sc scanner) (Attempt, error) {
	var (
		a         Attempt
		success   int
		startedAt string
		endedAt   sql.NullString
	)
	if err := sc.Scan(&a.ID, &a.RunID, &a.FeatureID, &a.Provider, &a.Prompt,
		&a.Output, &a.Stderr, &a.ExitCode, &success, &a.SessionID,
		&startedAt, &endedAt, &a.DurationMs); err != nil {
		return Attempt{}, err
	}
	a.Success = success != 0
	var err error
	if a.StartedAt, err = parseTime(startedAt); err != nil {
		return Attempt{}, err
	}
	if a.EndedAt, err = parseTimePtr(endedAt); err != nil {
		return Attempt{}, err
	}
	return a, nil
}

func scanLogLine(sc scanner) (LogLine, error) {
	var (
		l  LogLine
		ts string
	)
	if err := sc.Scan(&l.ID, &l.RunID, &l.AttemptID, &ts, &l.Level, &l.Msg, &l.Attrs); err != nil {
		return LogLine{}, err
	}
	var err error
	if l.TS, err = parseTime(ts); err != nil {
		return LogLine{}, err
	}
	return l, nil
}

// --- value helpers ---

func fmtTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func fmtTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing time %q: %w", s, err)
	}
	return t.UTC(), nil
}

func parseTimePtr(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid || ns.String == "" {
		return nil, nil
	}
	t, err := parseTime(ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
