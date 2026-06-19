# pave

[![Go](https://img.shields.io/badge/go-1.22+-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Release](https://img.shields.io/github/v/release/paveforge/pave)](https://github.com/paveforge/pave/releases/latest)

> An autonomous, local-first development orchestrator that reads a project's
> feature spec, tracks implementation state, and drives an AI coding CLI
> (Claude Code or GitHub Copilot) to implement pending features — pausing and
> resuming gracefully around provider rate limits.

## Why pave?

Running `claude` or `copilot` manually works well for one feature at a time — but
it doesn't scale to a full backlog. You have to remember what's done, re-prompt on
every restart, and babysit it through rate limits. `pave` removes all of that: you
write a feature list, run `pave run`, and walk away. It implements features in order,
persists state across sessions, backs off on limits, and picks up exactly where it
left off after a crash or restart.

`pave` is the orchestration layer on top of an AI coding CLI. It manages the CLI as
a subprocess — it does **not** reimplement an agent loop.

## Features

- **Feature tracking** across sessions, persisted in a database.
- **Autonomous loop** that implements pending features in dependency/priority order.
- **Rate-limit aware**: detects usage limits, backs off with jitter, resumes on reset.
- **Resilient retries**: exponential backoff on transient failures (network errors, etc.).
- **Interactive control**: pause, resume, terminate a task, or quit — live, mid-run.
- **Crash-safe & resumable**: a `kill -9` never re-implements completed features.
- **Provider-agnostic**: Claude Code today, GitHub Copilot included, easy to add more.
- **Provider fallback**: switch to a secondary provider when the primary is limited.
- **Pluggable storage**: SQLite (default), PostgreSQL, or Turso (libSQL, cloud).
- **Built-in web UI**: inspect runs, attempts, prompts, and output in a local dashboard.

## Prerequisites

Install at least one AI coding CLI and make sure it is on your `PATH`:

- **[Claude Code](https://claude.com/claude-code)** — `claude` (recommended)
- **[GitHub Copilot CLI](https://githubnext.com/projects/copilot-cli)** — `gh copilot`

## Installation

### Quick install (Linux & macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/paveforge/pave/main/install.sh | bash
```

Downloads the latest release binary to `~/.local/bin/pave`. No Go required.

```sh
# Install a specific version
PAVE_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/paveforge/pave/main/install.sh | bash

# Install to a custom directory
PAVE_INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/paveforge/pave/main/install.sh | bash
```

See [Releases](https://github.com/paveforge/pave/releases) for all available versions.

### From source (Go 1.22+)

```sh
go install github.com/paveforge/pave/cmd/pave@latest
```

Or build a single static binary (no CGO):

```sh
CGO_ENABLED=0 go build -o pave ./cmd/pave
```

## Quick start

```sh
cd your-project
pave init               # scaffold .pave/pave.yaml + FEATURES.md
$EDITOR FEATURES.md     # list the features you want
pave status             # see what's pending
pave run --dry-run      # preview which features will be implemented
pave run -y             # implement them
```

A `FEATURES.md` entry looks like:

```markdown
- [ ] Add user login — email + password auth (priority: 1)
- [ ] Add logout endpoint (depends: add-user-login)
- [x] Project scaffolding
```

- `- [ ]` is pending, `- [x]` is already implemented.
- Text after an em dash (`—`) is the description; passed verbatim to the AI.
- A trailing `(priority: N, depends: id1, id2)` block carries metadata.
- The feature ID is the slug of the title (e.g. `add-user-login`).

## Workflow

### New project

```sh
pave init               # scaffold .pave/pave.yaml + FEATURES.md
$EDITOR FEATURES.md     # describe what you want built
pave run                # implement pending features
```

### Existing project

If your project already has code satisfying some features, you have two options:

**Option A — mark them done manually** in `FEATURES.md`:
```markdown
- [x] Project scaffolding   # already done — pave skips [x] items
- [ ] Add user login
```

**Option B — let pave detect them automatically:**
```sh
pave status --scan      # walks the codebase; marks implemented features
pave run                # implements only what's still pending
```

Run `pave status --scan` before the first `pave run` on an existing project. After
that, state is persisted and subsequent runs pick up exactly where they left off.

### Writing good feature descriptions

Descriptions can be as long as needed — write the full context the AI needs:

```markdown
- [ ] Add user login — email + password auth with bcrypt hashing, JWT session tokens
  stored in httpOnly cookies, rate-limit to 5 attempts/min, and a /login POST endpoint
  (priority: 1)
```

The title (before `—`) becomes the feature ID. The description (after `—`) is
included verbatim in the prompt sent to the AI provider.

## Commands

| Command | Description |
|---|---|
| `pave init` | Scaffold `.pave/pave.yaml` and `FEATURES.md`; creates the database. Never overwrites existing files. |
| `pave status [--scan]` | Show implemented vs. pending. `--scan` auto-detects already-implemented features from the codebase. |
| `pave run [flags]` | Implement pending features. |
| `pave limits` | Report rate-limit status and next reset time. |
| `pave ui` | Launch the local web UI (embedded in the binary). |
| `pave update` | Update pave to the latest released version. |

### `pave run` flags

| Flag | Description |
|---|---|
| `--feature <id>` | Implement only this one feature. |
| `--dry-run` | Print the plan without invoking the provider. |
| `--max-features <N>` | Stop after N features. |
| `-y, --yes` | Skip the confirmation prompt. |

### Global flags

| Flag | Description |
|---|---|
| `--config <path>` | Path to config file (default `.pave/pave.yaml`). |
| `-v, --verbose` | Enable debug logging. |

## Interactive controls (during `pave run`)

While a run is in progress you can steer it from the keyboard:

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
model: ""                        # provider-specific model; empty = provider default
effort: ""                       # effort level (claude only): low | medium | high | xhigh | max
task_timeout: 30m                # per-feature subprocess timeout
auto_commit: false               # require explicit opt-in before committing
max_retries: 3                   # retries per feature on non-limit failure (0 = no retry)

# Backoff between non-limit failure retries (network errors, exit 1, etc.)
# Delays: 30s → 60s → 120s … capped at backoff_max.
retry:
  backoff_initial: 30s
  backoff_max: 10m

database:
  driver: sqlite                 # sqlite | postgres | turso
  dsn: ".pave/state.db"         # file path (sqlite) or connection URL

# Backoff when a provider usage limit is detected.
# Much longer scale than retry — limits typically reset after hours.
limiter:
  window: 5h                     # rolling usage window length
  backoff_initial: 1m            # first backoff interval
  backoff_max: 5h                # cap on backoff

ui:
  port: 4000                     # port for the local web viewer
```

### Claude models

Set `model` to request a specific Claude model. When empty, the `claude` CLI uses
its own default. Available models change over time — see the
[Claude models reference](https://docs.anthropic.com/en/docs/about-claude/models).

Common choices as of mid-2025:

| Model ID | Notes |
|---|---|
| `claude-opus-4-5` | Most capable; slowest / highest cost |
| `claude-sonnet-4-5` | Balanced (CLI default) |
| `claude-haiku-4-5` | Fastest / lowest cost |

Set `effort` to control Claude's extended thinking budget:
`low` · `medium` · `high` · `xhigh` · `max` (empty = provider default)

### Database drivers

| Driver | DSN example | Notes |
|---|---|---|
| `sqlite` | `.pave/state.db` | Default. Pure-Go, no CGO. Directory auto-created. |
| `postgres` | `postgres://user:pass@host:5432/db?sslmode=disable` | Uses `pgx`. |
| `turso` | `libsql://your-db.turso.io` | Set `TURSO_AUTH_TOKEN` in the env. |

Override driver and DSN with the `PAVE_DRIVER` and `PAVE_DSN` environment variables.

## UI

pave ships a built-in web viewer:

```sh
pave ui            # opens http://localhost:4000
pave ui -P 8080    # custom port
```

No Node.js, no download, no `npm install` — the UI is embedded directly in the
`pave` binary. It connects to the same database configured in `pave.yaml` and shows
live run data: features, attempts, prompts, output, and log streams.

![pave UI — live run dashboard](https://github.com/user-attachments/assets/362bf0e4-3b79-4536-b970-89b29c7bbbe5)

### `pave ui` flags

| Flag | Description |
|---|---|
| `-P, --port <N>` | Port to listen on (default `4000`). |

## How it works

```
load config → verify provider → parse FEATURES.md
  → reconcile spec with persisted state (spec defines existence, state keeps status)
  → for each pending feature (deps & priority respected):
       wait for limiter clearance
       run provider as a subprocess (capturing prompt, output, timing)
       on failure: retry with exponential backoff up to max_retries
       record the attempt; mark implemented / failed / pending
  → on rate limit: back off and resume; on signal/quit: persist and exit cleanly
```

Every run, feature, attempt (including the full prompt and output), log line, and
limiter window is recorded in the database — fully inspectable via the local UI.

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
2. Branch from `main`; keep commits small and scoped to one concern.
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

## License

[MIT](LICENSE) © paveforge
