// Database adapter for the pave viewer. It reads the same database pave writes
// to, selected by PAVE_DRIVER / PAVE_DSN (the `pave ui` command injects these).
//
// One libSQL client covers both the `sqlite` driver (via a file: DSN) and the
// `turso` driver (via a libsql:// URL). Postgres uses `pg`. Queries are written
// with `?` placeholders and rebound for Postgres.

import "server-only";

export type Driver = "sqlite" | "postgres" | "turso";

export interface Db {
  all<T = Record<string, unknown>>(sql: string, params?: unknown[]): Promise<T[]>;
  get<T = Record<string, unknown>>(sql: string, params?: unknown[]): Promise<T | null>;
}

function driver(): Driver {
  return (process.env.PAVE_DRIVER as Driver) || "sqlite";
}

function dsn(): string {
  const raw = process.env.PAVE_DSN;
  if (!raw) return "file:.pave/state.db"; // last-resort default; pave ui always injects PAVE_DSN
  return raw;
}

// rebind rewrites `?` placeholders to `$1, $2, ...` for Postgres.
function rebind(sql: string): string {
  let n = 0;
  return sql.replace(/\?/g, () => `$${++n}`);
}

let cached: Db | null = null;

export function db(): Db {
  if (cached) return cached;
  cached = driver() === "postgres" ? new PostgresDb() : new LibsqlDb();
  return cached;
}

// --- libSQL (sqlite file: or turso libsql://) ---

class LibsqlDb implements Db {
  private clientPromise: Promise<import("@libsql/client").Client> | null = null;

  private async client() {
    if (!this.clientPromise) {
      this.clientPromise = (async () => {
        const { createClient } = await import("@libsql/client");
        let url = dsn();
        // @libsql/client requires a file: URL for local SQLite.
        // pave ui injects an absolute path; we just need to prefix it.
        if (driver() === "sqlite" && !url.startsWith("file:") && !url.includes("://")) {
          url = "file:" + (url.startsWith("/") ? url : "/" + url);
        }
        const authToken = process.env.TURSO_AUTH_TOKEN;
        return createClient(authToken ? { url, authToken } : { url });
      })();
    }
    return this.clientPromise;
  }

  async all<T>(sql: string, params: unknown[] = []): Promise<T[]> {
    const c = await this.client();
    const res = await c.execute({ sql, args: params as never[] });
    return res.rows as unknown as T[];
  }

  async get<T>(sql: string, params: unknown[] = []): Promise<T | null> {
    const rows = await this.all<T>(sql, params);
    return rows[0] ?? null;
  }
}

// --- Postgres ---

class PostgresDb implements Db {
  private poolPromise: Promise<import("pg").Pool> | null = null;

  private async pool() {
    if (!this.poolPromise) {
      this.poolPromise = (async () => {
        const { Pool } = await import("pg");
        return new Pool({ connectionString: dsn() });
      })();
    }
    return this.poolPromise;
  }

  async all<T>(sql: string, params: unknown[] = []): Promise<T[]> {
    const p = await this.pool();
    const res = await p.query(rebind(sql), params as unknown[]);
    return res.rows as T[];
  }

  async get<T>(sql: string, params: unknown[] = []): Promise<T | null> {
    const rows = await this.all<T>(sql, params);
    return rows[0] ?? null;
  }
}
