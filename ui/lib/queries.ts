import "server-only";
import { db } from "./db";
import type { Run, Feature, Attempt, LogLine } from "./types";

export async function listRuns(limit = 50): Promise<Run[]> {
  return db().all<Run>(
    `SELECT id, project, provider, started_at, ended_at, status
     FROM runs ORDER BY started_at DESC LIMIT ?`,
    [limit],
  );
}

export async function getRun(id: string): Promise<Run | null> {
  return db().get<Run>(
    `SELECT id, project, provider, started_at, ended_at, status
     FROM runs WHERE id = ?`,
    [id],
  );
}

export async function listFeatures(runId: string): Promise<Feature[]> {
  return db().all<Feature>(
    `SELECT id, run_id, title, description, status, priority, depends_on, updated_at
     FROM features WHERE run_id = ? ORDER BY priority ASC, id ASC`,
    [runId],
  );
}

export async function listAttempts(runId: string): Promise<Attempt[]> {
  return db().all<Attempt>(
    `SELECT id, run_id, feature_id, provider, prompt, output, stderr, exit_code,
            success, session_id, started_at, ended_at, duration_ms
     FROM attempts WHERE run_id = ? ORDER BY started_at ASC`,
    [runId],
  );
}

export async function getAttempt(id: string): Promise<Attempt | null> {
  return db().get<Attempt>(
    `SELECT id, run_id, feature_id, provider, prompt, output, stderr, exit_code,
            success, session_id, started_at, ended_at, duration_ms
     FROM attempts WHERE id = ?`,
    [id],
  );
}

export async function listLogLines(runId: string, afterId = 0): Promise<LogLine[]> {
  return db().all<LogLine>(
    `SELECT id, run_id, attempt_id, ts, level, msg, attrs
     FROM log_lines WHERE run_id = ? AND id > ? ORDER BY id ASC`,
    [runId, afterId],
  );
}

// featureHistory aggregates attempts per feature across all runs.
export interface FeatureHistoryRow {
  feature_id: string;
  attempts: number;
  successes: number;
}

export async function featureHistory(): Promise<FeatureHistoryRow[]> {
  return db().all<FeatureHistoryRow>(
    `SELECT feature_id,
            COUNT(*) AS attempts,
            SUM(success) AS successes
     FROM attempts
     GROUP BY feature_id
     ORDER BY attempts DESC`,
  );
}
