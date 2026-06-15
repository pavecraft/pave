// Row types mirror the pave database schema (internal/state). All timestamps
// are RFC3339 strings.

export interface Run {
  id: string;
  project: string;
  provider: string;
  started_at: string;
  ended_at: string | null;
  status: string;
}

export interface Feature {
  id: string;
  run_id: string;
  title: string;
  description: string;
  status: string;
  priority: number;
  depends_on: string; // JSON array
  updated_at: string;
}

export interface Attempt {
  id: string;
  run_id: string;
  feature_id: string;
  provider: string;
  prompt: string;
  output: string;
  stderr: string;
  exit_code: number;
  success: number; // 0 | 1
  session_id: string;
  started_at: string;
  ended_at: string | null;
  duration_ms: number;
}

export interface LogLine {
  id: number;
  run_id: string;
  attempt_id: string;
  ts: string;
  level: string;
  msg: string;
  attrs: string;
}
