// Row types mirror the pave database schema (internal/state). All timestamps
// are RFC3339 strings.

export interface Run {
  ID: string;
  Project: string;
  Provider: string;
  StartedAt: string;
  EndedAt: string | null;
  Status: string;
}

export interface Feature {
  ID: string;
  RunID: string;
  Title: string;
  Description: string;
  Status: string;
  Priority: number;
  DependsOn: string[];
  UpdatedAt: string;
}

export interface Attempt {
  ID: string;
  RunID: string;
  FeatureID: string;
  Provider: string;
  Prompt: string;
  Output: string;
  Stderr: string;
  ExitCode: number;
  Success: boolean;
  SessionID: string;
  StartedAt: string;
  EndedAt: string | null;
  DurationMs: number;
}

export interface LogLine {
  ID: number;
  RunID: string;
  AttemptID: string;
  TS: string;
  Level: string;
  Msg: string;
  Attrs: string;
}

export interface FeatureHistoryRow {
  FeatureID: string;
  Attempts: number;
  Successes: number;
}
