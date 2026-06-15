import type { Run, Feature, Attempt, LogLine, FeatureHistoryRow } from "./types";

const BASE = "/api";

async function get<T>(path: string): Promise<T> {
  const res = await fetch(BASE + path);
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json() as Promise<T>;
}

export const listRuns = (limit = 50) => get<Run[]>(`/runs?limit=${limit}`);
export const getRun = (id: string) => get<Run>(`/runs/${id}`);
export const listFeatures = (runId: string) => get<Feature[]>(`/runs/${runId}/features`);
export const listAttempts = (runId: string) => get<Attempt[]>(`/runs/${runId}/attempts`);
export const getAttempt = (id: string) => get<Attempt>(`/attempts/${id}`);
export const featureHistory = () => get<FeatureHistoryRow[]>(`/features/history`);

export type { Run, Feature, Attempt, LogLine, FeatureHistoryRow };
