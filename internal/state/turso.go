package state

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/tursodatabase/libsql-client-go/libsql" // database/sql driver "libsql"
)

// openTurso opens a Turso/libSQL-backed Store. dsn is a libsql:// or https://
// URL. If the DSN carries no auth token and TURSO_AUTH_TOKEN is set, the token
// is appended as a query parameter.
func openTurso(ctx context.Context, dsn string) (Store, error) {
	dsn = withTursoToken(dsn, os.Getenv("TURSO_AUTH_TOKEN"))
	db, err := sql.Open("libsql", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening turso: %w", err)
	}
	d := dialect{name: "turso", migrateDir: "turso"}
	st, err := newSQLStore(ctx, db, d)
	if err != nil {
		db.Close()
		return nil, err
	}
	return st, nil
}

// withTursoToken appends authToken to dsn unless one is already present or the
// token is empty.
func withTursoToken(dsn, authToken string) string {
	if authToken == "" || strings.Contains(dsn, "authToken=") || strings.Contains(dsn, "jwt=") {
		return dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "authToken=" + authToken
}
