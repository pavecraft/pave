package state

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"
)

//go:embed migrations
var migrationsFS embed.FS

// runMigrations applies every .sql file for the dialect, in filename order.
// Statements use IF NOT EXISTS so applying them repeatedly is safe.
func runMigrations(ctx context.Context, db *sql.DB, d dialect) error {
	dir := path.Join("migrations", d.migrateDir)
	entries, err := migrationsFS.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading migrations for %s: %w", d.name, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		body, err := migrationsFS.ReadFile(path.Join(dir, name))
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}
		for _, stmt := range splitStatements(string(body)) {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("applying %s: %w", name, err)
			}
		}
	}
	return nil
}

// splitStatements splits a SQL file into individual executable statements,
// dropping ones that are empty or comment-only.
func splitStatements(sqlText string) []string {
	var out []string
	for _, chunk := range strings.Split(sqlText, ";") {
		if isExecutable(chunk) {
			out = append(out, chunk)
		}
	}
	return out
}

// isExecutable reports whether chunk contains anything other than whitespace
// and SQL line comments.
func isExecutable(chunk string) bool {
	for _, line := range strings.Split(chunk, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		return true
	}
	return false
}
