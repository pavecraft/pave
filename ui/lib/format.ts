export function fmtTime(s: string | null): string {
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

// attemptBadgeClass returns the right CSS class for an attempt result.
// success=true → green; exit_code != 0 → red; exit_code = 0 but success=false → yellow (interrupted).
// attemptBadgeClass returns the right CSS class for an attempt result.
// If ended_at is null the attempt is still running.
export function attemptBadgeClass(
  success: number | boolean,
  exitCode: number,
  endedAt?: string | null,
): string {
  if (!endedAt) return "badge badge-run";   // still running
  if (success) return "badge badge-ok";
  if (exitCode !== 0) return "badge badge-err";
  return "badge badge-warn"; // exit 0 but not success: interrupted
}

// attemptLabel returns the human-readable result label for an attempt.
export function attemptLabel(
  success: number | boolean,
  exitCode: number,
  endedAt?: string | null,
): string {
  if (!endedAt) return "running";
  if (success) return "success";
  return `exit ${exitCode}`;
}

export function parseDeps(json: string): string[] {
  try {
    const v = JSON.parse(json || "[]");
    return Array.isArray(v) ? v : [];
  } catch {
    return [];
  }
}
