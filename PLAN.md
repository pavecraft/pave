# Plan: Go HTTP API + React/Vite Static UI (embedded)

## Problem

`pave ui` downloads a Next.js standalone bundle containing `@libsql/client`, a
native Node.js package compiled for Linux. Running on macOS ARM64 fails with
`Cannot find module '@libsql/darwin-arm64'` because the native addon for the
current platform is missing.

## Solution

Move all database access into the Go binary (which already has all three drivers).
Replace the Next.js app with a plain React + Vite SPA. Embed the compiled static
files (`dist/`) directly into the Go binary using `//go:embed`. `pave ui` starts
a Go HTTP server — no Node.js at runtime, no download step, no native binaries
in the UI bundle.

## Architecture After

```
pave ui
  └── Go HTTP server on :4000
        ├── /api/*  → Go handlers reading state.Store directly
        └── /*      → embedded static files (HTML/JS/CSS) + SPA fallback
```

Binary size increase: ~2–3 MB (Vite bundle). No new Go dependencies (stdlib
net/http only).

---

## Step 1 — Extend state.Store

**Files:** `internal/state/store.go`, `internal/state/sql_store.go`,
`internal/state/store_test.go`

Add to the `Store` interface:
```go
GetAttempt(ctx context.Context, id string) (Attempt, error)
FeatureHistory(ctx context.Context) ([]FeatureHistoryRow, error)
```

Add new type to `store.go`:
```go
type FeatureHistoryRow struct {
    FeatureID string
    Attempts  int64
    Successes int64
}
```

Implement in `sql_store.go`:
- `GetAttempt`: `SELECT … FROM attempts WHERE id = ?` using `scanAttempt`
- `FeatureHistory`: `SELECT feature_id, COUNT(*), SUM(success) FROM attempts GROUP BY feature_id`

Add table-driven tests in `store_test.go` for both methods.

---

## Step 2 — Embed package

**New file:** `internal/uistatic/embed.go`

```go
package uistatic

import "embed"

//go:embed all:dist
var Files embed.FS
```

**New file:** `internal/uistatic/dist/.gitkeep` (committed placeholder so the
package compiles without a prior Vite build).

Add to `.gitignore`:
```
internal/uistatic/dist/*
!internal/uistatic/dist/.gitkeep
```

Vite build target: `ui/vite.config.ts` sets `build.outDir: '../internal/uistatic/dist'`.

---

## Step 3 — Go HTTP API

**New package:** `internal/api/`

```
internal/api/
  server.go    — NewServer(store state.Store, files fs.FS) http.Handler
  handlers.go  — one handler per endpoint, JSON via encoding/json
  sse.go       — SSE log-streaming handler
  server_test.go — tests using a mock Store
```

Routes (Go 1.22 ServeMux with `{id}` patterns):

| Path | Store call |
|---|---|
| `GET /api/runs` | `ListRuns(ctx, 50)` |
| `GET /api/runs/{id}` | `GetRun(ctx, id)` |
| `GET /api/runs/{id}/features` | `ListFeatures(ctx, id)` |
| `GET /api/runs/{id}/attempts` | `ListAttempts(ctx, id)` |
| `GET /api/runs/{id}/stream` | SSE: poll `ListLogLines` every 500ms |
| `GET /api/attempts/{id}` | `GetAttempt(ctx, id)` |
| `GET /api/features/history` | `FeatureHistory(ctx)` |
| `GET /*` | static SPA file server with index.html fallback |

JSON uses thin DTO structs with snake_case tags matching `ui/lib/types.ts`.
`ErrNotFound` → 404 JSON response. All other errors → 500.

SSE handler mirrors `ui/app/api/runs/[id]/stream/route.ts`: poll every 500ms,
close 2s after run reaches terminal state.

SPA fallback: serve exact file if it exists in the embedded FS; otherwise serve
`index.html` so React Router handles the path client-side.

**Tests in `server_test.go`:** mock Store (implements `state.Store` interface),
table-driven tests for each endpoint checking status code and JSON shape. SSE
test verifies at least one `data:` frame is emitted.

---

## Step 4 — Update `cmd/pave/ui.go`

Remove: `downloadUI`, `installedUIVersion`, `uiEnv`, `versionTag`, `versionBare`,
`uiReleaseURL`, `proc.StartIO`, all env-var plumbing, `strings` import.

Add: open `state.Store`, create `api.NewServer`, run `http.ListenAndServe`.

```go
import (
    "context"
    "embed" // indirect via uistatic
    "errors"
    "fmt"
    "io/fs"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/spf13/cobra"
    "github.com/pavecraft/pave/internal/api"
    "github.com/pavecraft/pave/internal/config"
    "github.com/pavecraft/pave/internal/proc"
    "github.com/pavecraft/pave/internal/state"
    "github.com/pavecraft/pave/internal/uistatic"
)

func runUI(cmd *cobra.Command, configPath string, portFlag int) error {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    cfg, err := config.Load(configPath)
    if err != nil { return err }

    port := cfg.UI.Port
    if portFlag != 0 { port = portFlag }

    store, err := state.New(ctx, cfg.Database)
    if err != nil { return fmt.Errorf("opening database: %w", err) }
    defer store.Close()

    files, _ := fs.Sub(uistatic.Files, "dist")
    handler := api.NewServer(store, files)
    srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: handler}
    go func() { <-ctx.Done(); srv.Shutdown(context.Background()) }()

    fmt.Fprintf(cmd.OutOrStdout(), "Starting pave UI on http://localhost:%d\n", port)
    go openBrowser(ctx, fmt.Sprintf("http://localhost:%d", port))

    if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
        return err
    }
    return nil
}
```

---

## Step 5 — Update `cmd/pave/version.go`

Remove `installedUIVersion` (no longer needed; UI is always embedded).
`pave version` shows:
```
pave:     1.0.4
pave UI:  1.0.4
```
UI version = `cfg.UI.Version` if set, otherwise same as pave version.

---

## Step 6 — Replace Next.js UI with React + Vite

**Delete:** all existing files under `ui/` (Next.js project).

**New `ui/package.json`:**
- Keep: `react`, `react-dom`, `react-markdown`
- Add: `react-router-dom`, `vite`, `@vitejs/plugin-react`
- Remove: `next`, `@libsql/client`, `pg`, `server-only`

**New file structure:**
```
ui/
  index.html
  vite.config.ts        outDir: '../internal/uistatic/dist'
  tsconfig.json
  package.json
  src/
    main.tsx            ReactDOM.createRoot + BrowserRouter
    App.tsx             Route definitions
    index.css           = existing globals.css (unchanged)
    lib/
      api.ts            fetch wrappers for all 7 endpoints
      types.ts          unchanged
      format.ts         unchanged
    pages/
      Dashboard.tsx     lists runs, polls every 5s while any run is active
      RunDetail.tsx     run + features + attempts table
      AttemptDetail.tsx attempt detail with Markdown output
      Features.tsx      feature history table
    components/
      LogStream.tsx     unchanged (already pure React + EventSource)
      Markdown.tsx      unchanged
```

Pages use `useState` + `useEffect` for data loading. Loading and error states
are shown inline. The JSX is identical to the existing pages — only the data
acquisition changes (DB query → fetch).

**`lib/api.ts` shape:**
```ts
const BASE = '/api';
export const listRuns = () => fetch(`${BASE}/runs`).then(r => r.json());
export const getRun = (id: string) => fetch(`${BASE}/runs/${id}`).then(r => r.json());
// ... one function per endpoint
```

---

## Step 7 — Update Release Workflow

**File:** `.github/workflows/release.yml`

- Remove the `ui-release` job entirely (no separate asset needed).
- Add a `build-ui` step inside the `release` job, before goreleaser:

```yaml
- name: Build UI
  run: cd ui && npm ci && npm run build

- uses: goreleaser/goreleaser-action@v6
  # goreleaser runs go build which embeds internal/uistatic/dist/
```

`test` → `release` (which now builds UI first, then Go binary).

Update `cmd/pave/ui.go` to remove `uiReleaseURL` and download logic — no longer
needed in any form.

---

## Step 8 — Update scaffold

Remove `ui.path` and `ui.version` from `DefaultConfigYAML` in
`internal/scaffold/scaffold.go` (no longer relevant; keep `ui.port`).

Remove `UI.Path` and `UI.Version` from `config.applyDefaults()`. Keep `UI.Port`.

---

## Developer Workflow

```bash
# Build UI first (required before go build)
cd ui && npm install && npm run build

# Then build/test Go
cd ..
go build ./cmd/pave
go test ./...
```

---

## Verification

1. `cd ui && npm run build` → produces `internal/uistatic/dist/`
2. `CGO_ENABLED=0 go build ./cmd/pave` → builds cleanly
3. `go test ./... -race -count=1` → all tests pass
4. `pave init && pave ui` → server starts, browser opens
5. All 4 pages load with correct data
6. Live log streaming works during an active `pave run`
7. Test on macOS ARM64 — no `@libsql` errors
