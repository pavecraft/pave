import { useEffect, useState, useRef } from "react";
import { Link } from "react-router-dom";
import { listRuns, type Run } from "../lib/api";
import { fmtTime, statusClass } from "../lib/format";

export default function Dashboard() {
  const [runs, setRuns] = useState<Run[]>([]);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const load = () =>
    listRuns(50)
      .then(setRuns)
      .catch((e: unknown) => setError(String(e)));

  useEffect(() => {
    load();
    intervalRef.current = setInterval(() => {
      load();
    }, 5000);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, []);

  if (error)
    return (
      <div className="card">
        <h2>Cannot read the pave database</h2>
        <pre>{error}</pre>
      </div>
    );

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
                <tr key={r.ID}>
                  <td><span className={statusClass(r.Status)}>{r.Status}</span></td>
                  <td>{fmtTime(r.StartedAt)}</td>
                  <td>{r.Provider}</td>
                  <td className="mono muted">{r.Project}</td>
                  <td>
                    <Link to={`/runs/${r.ID}`} className="mono">
                      {r.ID.slice(0, 8)}
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
