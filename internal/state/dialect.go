package state

import (
	"strconv"
	"strings"
)

// dialect captures the small SQL differences between backends. All shared SQL
// is written with "?" placeholders and rebound per dialect.
type dialect struct {
	name       string // logical name: sqlite, postgres, turso
	migrateDir string // subdirectory under migrations/
	postgres   bool   // true if placeholders must be $1, $2, ...
}

// rebind rewrites "?" placeholders into the dialect's native form. For Postgres
// it produces $1, $2, ...; for SQLite/Turso it returns the query unchanged.
func (d dialect) rebind(query string) string {
	if !d.postgres {
		return query
	}
	var b strings.Builder
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
			continue
		}
		b.WriteByte(query[i])
	}
	return b.String()
}
