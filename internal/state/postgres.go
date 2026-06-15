package state

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"
)

// openPostgres opens a PostgreSQL-backed Store. dsn is a standard connection
// URL, e.g. "postgres://user:pass@host:5432/dbname?sslmode=disable".
func openPostgres(ctx context.Context, dsn string) (Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening postgres: %w", err)
	}
	d := dialect{name: "postgres", migrateDir: "postgres", postgres: true}
	st, err := newSQLStore(ctx, db, d)
	if err != nil {
		db.Close()
		return nil, err
	}
	return st, nil
}
