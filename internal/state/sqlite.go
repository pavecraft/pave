package state

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no CGO)
)

// openSQLite opens a SQLite-backed Store. dsn is a file path; the parent
// directory is created if needed. The special value ":memory:" (or a DSN
// containing "mode=memory") opens an in-memory database.
func openSQLite(ctx context.Context, dsn string) (Store, error) {
	if !isMemoryDSN(dsn) {
		if dir := filepath.Dir(dsn); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("creating db directory %q: %w", dir, err)
			}
		}
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}
	// A single connection avoids "database is locked" within a process and
	// keeps an in-memory DB alive across queries.
	db.SetMaxOpenConns(1)

	if !isMemoryDSN(dsn) {
		// WAL mode: readers (pave ui) never block writers (pave run) and vice versa.
		// Persists on the file after first set — future connections inherit it.
		if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
			// Non-fatal: WAL may be unavailable on network filesystems (NFS/CIFS).
			fmt.Fprintf(os.Stderr, "pave: warning: could not enable WAL mode: %v\n", err)
		}
		// busy_timeout: retry on cross-process lock contention for up to 5s
		// instead of returning SQLITE_BUSY immediately. Session-scoped.
		if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
			return nil, fmt.Errorf("setting sqlite busy_timeout: %w", err)
		}
	}

	d := dialect{name: "sqlite", migrateDir: "sqlite"}
	st, err := newSQLStore(ctx, db, d)
	if err != nil {
		db.Close()
		return nil, err
	}
	return st, nil
}

func isMemoryDSN(dsn string) bool {
	return dsn == ":memory:" ||
		filepath.Base(dsn) == ":memory:" ||
		strings.Contains(dsn, "mode=memory")
}
