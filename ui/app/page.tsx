import Link from "next/link";
import { listRuns } from "@/lib/queries";
import { fmtTime, statusClass } from "@/lib/format";

export const dynamic = "force-dynamic";

export default async function DashboardPage() {
  let runs;
  try {
    runs = await listRuns(50);
  } catch (e) {
    return <DbError error={e} />;
  }

  return (
    <>
      <h2>Runs</h2>
      {runs.length === 0 ? (
        <div className="card empty">
          No runs yet. Start one with <span className="mono">pave run</span>.
        </div>
      ) : (
        <div className="card">
          <table>
            <thead>
              <tr>
                <th>Status</th>
                <th>Started</th>
                <th>Provider</th>
                <th>Project</th>
                <th>Run</th>
              </tr>
            </thead>
            <tbody>
              {runs.map((r) => (
                <tr key={r.id}>
                  <td>
                    <span className={statusClass(r.status)}>{r.status}</span>
                  </td>
                  <td>{fmtTime(r.started_at)}</td>
                  <td>{r.provider}</td>
                  <td className="mono muted">{r.project}</td>
                  <td>
                    <Link href={`/runs/${r.id}`} className="mono">
                      {r.id.slice(0, 8)}
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}

function DbError({ error }: { error: unknown }) {
  return (
    <div className="card">
      <h2>Cannot read the pave database</h2>
      <p className="muted">
        Set <span className="mono">PAVE_DRIVER</span> and{" "}
        <span className="mono">PAVE_DSN</span>, or launch via{" "}
        <span className="mono">pave ui</span>.
      </p>
      <pre>{String(error)}</pre>
    </div>
  );
}
