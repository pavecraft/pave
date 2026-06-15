# CLAUDE.md — Rules for AI agents working in this repo

This file defines the conventions every agent (and human) must follow when
modifying `pave`.

## Project basics

- **Module:** `github.com/pavecraft/pave`
- **Language:** Go 1.22+
- **Build:** `cd ui && npm ci && npm run build && cd .. && CGO_ENABLED=0 go build ./cmd/pave`
  (The React/Vite UI must be built first so `//go:embed all:dist` has files to embed.)
- **Test:** `go test ./... -race -count=1`
- **Lint:** `go vet ./...` and `gofmt -l .` must report nothing.

## Hard rules

1. **No CGO.** The binary must be statically linkable. Use `modernc.org/sqlite`,
   never `mattn/go-sqlite3`. Do not add any dependency that requires CGO.
2. **No `exec.Command` in business logic.** All subprocess work goes through
   `internal/proc`. Providers, planner, and commands call `proc.Start`.
3. **Always pass `context.Context`** to anything that does I/O or spawns a
   subprocess, and honor cancellation.
4. **Wrap errors with context:** `fmt.Errorf("loading config: %w", err)`.
5. **No global mutable state.** Pass dependencies explicitly.
6. **Respect the dependency direction.** No import cycles.
   `planner → {provider, limiter, project, state, interactive}`,
   `provider → {project, proc}`, everything → `config`.

## Testing rules

- **Every exported function must have at least one test.**
- Use **table-driven tests** for all logic (parser, limiter, reconcile, planner, scanner).
- Add `t.Parallel()` to tests that don't share mutable state.
- `proc` tests use real subprocesses (e.g. `sleep`, `echo`, or a built helper).
- `state` tests use a real DB: SQLite in-memory by default; Postgres/Turso are
  skipped unless their env/containers are available.
- Planner tests use a **mock `Provider`** and a **mock interactive channel**.

## Style

- `gofmt` formatting is mandatory.
- Prefer stdlib over third-party deps.
- Keep packages small and single-purpose.
- Exported symbols get doc comments starting with the symbol name.

## Database rules

- The `Store` interface is the only seam. No driver-specific SQL outside
  `internal/state/*`.
- Migrations are **additive-only** — never drop or rename columns/tables.
- Migration SQL lives under `internal/state/migrations/{sqlite,postgres,turso}/`.

## Subprocess rules

- Set the working directory explicitly on every subprocess.
- Never pass secrets as command-line args (visible in process listings).
- Apply the configured per-task timeout (default 30m).
- Support pause/resume/stop via `proc` so the interactive loop can control runs.
