# pave

> An autonomous, local-first development orchestrator that reads a project's
> feature spec, tracks implementation state, and drives an AI coding CLI
> (Claude Code or GitHub Copilot) to implement pending features — pausing and
> resuming gracefully around provider rate limits.

`pave` is the orchestration layer on top of an AI coding CLI. The CLIs are great
at implementing a feature when you ask, but they don't track *which* planned
features are done, they don't pause when you hit a usage limit, and they can't
fall back to another provider. `pave` fills that gap. It manages the coding CLI
as a subprocess — it does **not** reimplement an agent loop.

See [CLAUDE.md](CLAUDE.md) for the rules every contributor (human or agent) follows.

## Features

- **Feature tracking** across sessions, persisted in a database.
- **Autonomous loop** that implements pending features in dependency/priority order.
- **Rate-limit aware**: detects usage limits, backs off with jitter, resumes on reset.
- **Interactive control**: pause, resume, terminate a task, or quit — live, mid-run.
- **Crash-safe & resumable**: a `kill -9` never re-implements completed features.
- **Provider-agnostic**: Claude today, Copilot included, easy to add more.
- **Provider fallback**: switch to a secondary provider when the primary is limited.
- **Pluggable storage**: SQLite (default), PostgreSQL, or Turso (libSQL, cloud).

## Installation

```sh
go install github.com/xoai/pave/cmd/pave@latest
```

Or build from source (produces a single static binary, no CGO):

```sh
CGO_ENABLED=0 go build -o pave ./cmd/pave
```

`pave` drives an external coding CLI. Install at least one and make sure it is on
your `PATH`:

- [Claude Code](https://claude.com/claude-code) — `claude`
- GitHub Copilot CLI — `copilot`

## Quick start

```sh
cd your-project
pave init               # scaffold .pave/pave.yaml + FEATURES.md
$EDITOR FEATURES.md     # list the features you want
pave status             # see what's pending
pave run                # implement pending features (asks for confirmation)
pave run -y             # skip confirmation
```

A `FEATURES.md` entry looks like:

```markdown
- [ ] Add user login — email + password auth (priority: 1)
- [ ] Add logout endpoint (depends: add-user-login)
- [x] Project scaffolding
```

- `- [ ]` is pending, `- [x]` is already implemented.
- Text after an em dash (`—`) or colon is the description.
- A trailing `(priority: N, depends: id1, id2)` block carries metadata.
- The feature ID is the slug of the title (e.g. `add-user-login`).

## Commands

| Command | Description |
|---|---|
| `pave init` | Scaffold `.pave/pave.yaml` and `FEATURES.md`; creates the database. Never overwrites existing files. |
| `pave status [--scan]` | Show implemented vs. pending. `--scan` refines from the codebase. |
| `pave run [flags]` | Implement pending features (interactive). |
| `pave limits` | Report rate-limit status and next reset. |
| `pave ui` | Launch the local Next.js viewer (see `ui/`). |

### `pave run` flags

| Flag | Description |
|---|---|
| `--feature <id>` | Implement only this feature. |
| `--dry-run` | Print the plan without invoking the provider. |
| `--max-features <N>` | Stop after N features. |
| `-y, --yes` | Skip the confirmation prompt. |

### Global flags

| Flag | Description |
|---|---|
| `--config <path>` | Path to config file (default `.pave/pave.yaml`). |
| `-v, --verbose` | Enable debug logging. |

## Interactive controls (during `pave run`)

While a run is in progress you can steer it from the keyboard without killing
the process:

| Key | Action |
|---|---|
| `P` | Pause the current task (suspends the subprocess). |
| `R` | Resume a paused task. |
| `T` | Terminate the current task and move to the next (feature stays pending). |
| `Q` | Quit cleanly — state is saved and the run is resumable. |

`Ctrl-C` (SIGINT) and SIGTERM also trigger a clean, resumable shutdown.

## Configuration (`.pave/pave.yaml`)

```yaml
project_path: .                  # target project root
features_file: ./FEATURES.md     # feature spec location
provider: claude                 # claude | copilot
fallback_provider: ""            # optional secondary provider; empty = none
model: ""                        # provider-specific model; empty = default
task_timeout: 30m                # per-feature subprocess timeout
auto_commit: false               # require explicit opt-in before committing
max_retries: 1                   # retries per feature on failure (0 = no retry)

database:
  driver: sqlite                 # sqlite | postgres | turso
  dsn: ".pave/state.db"          # file path (sqlite) or connection URL

limiter:
  window: 5h                     # rolling usage window length
  backoff_initial: 1m            # first backoff interval
  backoff_max: 5h                # cap on backoff
```

### Database drivers

| Driver | DSN example | Notes |
|---|---|---|
| `sqlite` | `.pave/state.db` | Default. Pure-Go, no CGO. Directory auto-created. |
| `postgres` | `postgres://user:pass@host:5432/db?sslmode=disable` | Uses `pgx`. |
| `turso` | `libsql://your-db.turso.io` | Set `TURSO_AUTH_TOKEN` in the env. |

The driver and DSN can be overridden with the `PAVE_DRIVER` and `PAVE_DSN`
environment variables, which is how the `ui/` viewer points at the same data.

## How it works

```
load config → verify provider → parse FEATURES.md
  → reconcile spec with persisted state (spec defines existence, state keeps status)
  → for each pending feature (deps & priority respected):
       wait for limiter clearance
       run provider as a subprocess (capturing prompt, output, timing)
       record the attempt; mark implemented / failed / pending
  → on limit: back off and resume; on signal/quit: persist and exit cleanly
```

Every run, feature, attempt (including the full prompt and output), log line,
and limiter window is recorded in the database, so the state is fully
inspectable — and replayable by the local viewer.

## Building & testing

```sh
go test ./... -race -count=1
go vet ./...
gofmt -l .                       # should print nothing
CGO_ENABLED=0 go build ./cmd/pave
```

## Contributing

1. Read [CLAUDE.md](CLAUDE.md) — it defines the hard rules (no CGO, subprocess
   discipline, error wrapping, dependency direction).
2. Branch from `main`; keep commits small and scoped to one feature ID.
3. **Every exported function needs a test.** Logic packages use table-driven
   tests; concurrency-safe tests use `t.Parallel()`.
4. Run the full check above before opening a PR.

### Adding a provider

Implement the `provider.Provider` interface (`Name`, `Run`, `CheckLimit`,
`Available`) in `internal/provider`, shelling out through `internal/proc` (never
`os/exec` directly), then register it in `provider.ByName`.

### Adding a database driver

Implement the `state.Store` interface in `internal/state`, add migrations under
`internal/state/migrations/<driver>/`, and wire it into `state.New`. Migrations
are additive-only.
