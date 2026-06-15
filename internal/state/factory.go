package state

import (
	"context"
	"fmt"

	"github.com/pavecraft/pave/internal/config"
)

// New opens a Store for the configured database driver, running migrations as
// needed.
func New(ctx context.Context, cfg config.Database) (Store, error) {
	switch cfg.Driver {
	case config.DriverSQLite:
		return openSQLite(ctx, cfg.DSN)
	case config.DriverPostgres:
		return openPostgres(ctx, cfg.DSN)
	case config.DriverTurso:
		return openTurso(ctx, cfg.DSN)
	default:
		return nil, fmt.Errorf("unknown database driver %q", cfg.Driver)
	}
}
