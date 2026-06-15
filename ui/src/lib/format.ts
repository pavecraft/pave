export function fmtTime(s: string | null | undefined): string {
  if (!s) return "—";
  const d = new Date(s);
  if (isNaN(d.getTime())) return s;
  return d.toLocaleString();
}

export function fmtDuration(ms: number): string {
  if (!ms) return "—";
  if (ms < 1000) return `${ms}ms`;
  const sec = Math.round(ms / 1000);
  if (sec < 60) return `${sec}s`;
  const min = Math.floor(sec / 60);
  return `${min}m ${sec % 60}s`;
}

export function statusClass(status: string): string {
  switch (status) {
    case "implemented":
    case "completed":
      return "badge badge-ok";
    case "failed":
      return "badge badge-err";
    case "in_progress":
    case "running":
      return "badge badge-run";
    case "interrupted":
      return "badge badge-warn";
    default:
      return "badge";
  }
}

export function attemptBadgeClass(success: boolean, exitCode: number, endedAt?: string | null): string {
  if (!endedAt) return "badge badge-run";
  if (success) return "badge badge-ok";
  if (exitCode !== 0) return "badge badge-err";
  return "badge badge-warn";
}

export function attemptLabel(success: boolean, exitCode: number, endedAt?: string | null): string {
  if (!endedAt) return "running";
  if (success) return "success";
  return `exit ${exitCode}`;
}

export function parseDeps(deps: string[]): string[] {
  return Array.isArray(deps) ? deps : [];
}
