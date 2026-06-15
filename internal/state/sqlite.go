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
	// A single connection avoids "database is locked" with the file backend and
	// keeps an in-memory DB alive across queries.
	db.SetMaxOpenConns(1)

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
