# pave UI

A local Next.js viewer for `pave` run data. It reads the **same** database that
`pave` writes to (no separate store) and streams live run output.

## Requirements

- Node.js ≥ 20

## Setup

```sh
cd ui
npm install
```

## Running

The easiest way is from the repo root, which injects the configured database:

```sh
pave ui                 # runs `next dev` in ui/ with PAVE_DRIVER/PAVE_DSN set
```

Or run it directly, pointing at a database yourself:

```sh
PAVE_DRIVER=sqlite PAVE_DSN=/abs/path/to/.pave/state.db npm run dev
# Postgres:
PAVE_DRIVER=postgres PAVE_DSN='postgres://user:pass@localhost/pave' npm run dev
# Turso:
PAVE_DRIVER=turso PAVE_DSN='libsql://your-db.turso.io' TURSO_AUTH_TOKEN=... npm run dev
```

Then open http://localhost:3000.

## Pages

- `/` — dashboard: recent runs.
- `/runs/[id]` — run detail: feature table, attempts, and a live log stream
  (Server-Sent Events) while the run is in progress.
- `/runs/[id]/attempts/[attemptId]` — full prompt, output, and stderr.
- `/features` — per-feature attempt history across all runs.

## How data flows

One `@libsql/client` handles both the `sqlite` (via `file:` DSN) and `turso`
(`libsql://`) drivers; `pg` handles Postgres. See `lib/db.ts`. The live log uses
`/api/runs/[id]/stream`, which polls `log_lines` every 500ms and flushes new
rows over SSE.
