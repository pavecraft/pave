import Link from "next/link";
import { notFound } from "next/navigation";
import { getRun, listFeatures, listAttempts } from "@/lib/queries";
import { fmtTime, fmtDuration, statusClass, parseDeps, attemptBadgeClass, attemptLabel } from "@/lib/format";
import LogStream from "@/components/LogStream";
import type { Attempt } from "@/lib/types";

export const dynamic = "force-dynamic";

export default async function RunPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  const run = await getRun(id);
  if (!run) notFound();

  const [features, attempts] = await Promise.all([listFeatures(id), listAttempts(id)]);

  const counts = features.reduce<Record<string, number>>((acc, f) => {
    acc[f.status] = (acc[f.status] || 0) + 1;
    return acc;
  }, {});

  // Group attempts by feature for inline display
  const attemptsByFeature = attempts.reduce<Record<string, Attempt[]>>((acc, a) => {
    (acc[a.feature_id] ??= []).push(a);
    return acc;
  }, {});

  return (
    <>
      <p className="muted">
        <Link href="/">← all runs</Link>
      </p>
      <h2>
        Run <span className="mono">{run.id.slice(0, 8)}</span>{" "}
        <span className={statusClass(run.status)}>{run.status}</span>
      </h2>

      <div className="grid" style={{ marginBottom: 20 }}>
        <Stat label="Provider" value={run.provider} />
        <Stat label="Started" value={fmtTime(run.started_at)} />
        <Stat label="Ended" value={fmtTime(run.ended_at)} />
        <Stat label="Implemented" value={String(counts["implemented"] || 0)} />
        <Stat label="Failed" value={String(counts["failed"] || 0)} />
        <Stat label="Pending" value={String(counts["pending"] || 0)} />
      </div>

      {run.status === "running" && <LogStream runId={run.id} />}

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
              const featureAttempts = attemptsByFeature[f.id] ?? [];
              const latest = featureAttempts[featureAttempts.length - 1];
              return (
                <tr key={f.id}>
                  <td style={{ paddingLeft: 16 }}>
                    <span className={statusClass(f.status)}>{f.status}</span>
                  </td>
                  <td>
                    <div>{f.title}</div>
                    <div className="muted mono" style={{ fontSize: 11 }}>{f.id}</div>
                    {parseDeps(f.depends_on).length > 0 && (
                      <div className="muted" style={{ fontSize: 11 }}>
                        needs: {parseDeps(f.depends_on).join(", ")}
                      </div>
                    )}
                  </td>
                  <td className="muted">{featureAttempts.length || "—"}</td>
                  <td>
                    {latest ? (
                      <span className={attemptBadgeClass(latest.success, latest.exit_code, latest.ended_at)}>
                        {attemptLabel(latest.success, latest.exit_code, latest.ended_at)}
                      </span>
                    ) : <span className="muted">—</span>}
                  </td>
                  <td className="muted">{latest ? fmtDuration(latest.duration_ms) : "—"}</td>
                  <td>
                    {featureAttempts.map((a) => (
                      <Link key={a.id} href={`/runs/${run.id}/attempts/${a.id}`} style={{ marginRight: 8 }}>
                        {featureAttempts.length > 1 ? `#${featureAttempts.indexOf(a) + 1}` : "details"}
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

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="stat">
      <div className="num">{value}</div>
      <div className="label">{label}</div>
    </div>
  );
}
