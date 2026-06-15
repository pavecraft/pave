import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { getRun, listFeatures, listAttempts, type Run, type Feature, type Attempt } from "../lib/api";
import { fmtTime, fmtDuration, statusClass, parseDeps, attemptBadgeClass, attemptLabel } from "../lib/format";
import LogStream from "../components/LogStream";

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="stat">
      <div className="num">{value}</div>
      <div className="label">{label}</div>
    </div>
  );
}

export default function RunDetail() {
  const { id } = useParams<{ id: string }>();
  const [run, setRun] = useState<Run | null>(null);
  const [features, setFeatures] = useState<Feature[]>([]);
  const [attempts, setAttempts] = useState<Attempt[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    Promise.all([getRun(id), listFeatures(id), listAttempts(id)])
      .then(([r, f, a]) => { setRun(r); setFeatures(f); setAttempts(a); })
      .catch((e: unknown) => setError(String(e)));
  }, [id]);

  if (error) return <div className="card"><h2>Error</h2><pre>{error}</pre></div>;
  if (!run) return <div className="muted">Loading…</div>;

  const counts = features.reduce<Record<string, number>>((acc, f) => {
    acc[f.Status] = (acc[f.Status] || 0) + 1;
    return acc;
  }, {});

  const attemptsByFeature = attempts.reduce<Record<string, Attempt[]>>((acc, a) => {
    (acc[a.FeatureID] ??= []).push(a);
    return acc;
  }, {});

  return (
    <>
      <p className="muted"><Link to="/">← all runs</Link></p>
      <h2>
        Run <span className="mono">{run.ID.slice(0, 8)}</span>{" "}
        <span className={statusClass(run.Status)}>{run.Status}</span>
      </h2>

      <div className="grid" style={{ marginBottom: 20 }}>
        <Stat label="Provider" value={run.Provider} />
        <Stat label="Started" value={fmtTime(run.StartedAt)} />
        <Stat label="Ended" value={fmtTime(run.EndedAt)} />
        <Stat label="Implemented" value={String(counts["implemented"] || 0)} />
        <Stat label="Failed" value={String(counts["failed"] || 0)} />
        <Stat label="Pending" value={String(counts["pending"] || 0)} />
      </div>

      {run.Status === "running" && <LogStream runId={run.ID} />}

      <h2>Features</h2>
      <div className="card" style={{ padding: 0 }}>
        <table>
          <thead>
            <tr>
              <th style={{ paddingLeft: 16 }}>Status</th>
              <th>Title</th>
              <th>Attempts</th>
              <th>Last result</th>
              <th>Duration</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {features.map((f) => {
              const fa = attemptsByFeature[f.ID] ?? [];
              const latest = fa[fa.length - 1];
              return (
                <tr key={f.ID}>
                  <td style={{ paddingLeft: 16 }}>
                    <span className={statusClass(f.Status)}>{f.Status}</span>
                  </td>
                  <td>
                    <div>{f.Title}</div>
                    <div className="muted mono" style={{ fontSize: 11 }}>{f.ID}</div>
                    {parseDeps(f.DependsOn).length > 0 && (
                      <div className="muted" style={{ fontSize: 11 }}>
                        needs: {parseDeps(f.DependsOn).join(", ")}
                      </div>
                    )}
                  </td>
                  <td className="muted">{fa.length || "—"}</td>
                  <td>
                    {latest
                      ? <span className={attemptBadgeClass(latest.Success, latest.ExitCode, latest.EndedAt)}>{attemptLabel(latest.Success, latest.ExitCode, latest.EndedAt)}</span>
                      : <span className="muted">—</span>}
                  </td>
                  <td className="muted">{latest ? fmtDuration(latest.DurationMs) : "—"}</td>
                  <td>
                    {fa.map((a, i) => (
                      <Link key={a.ID} to={`/runs/${run.ID}/attempts/${a.ID}`} style={{ marginRight: 8 }}>
                        {fa.length > 1 ? `#${i + 1}` : "details"}
                      </Link>
                    ))}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </>
  );
}
