import { useEffect, useState } from "react";
import { featureHistory, type FeatureHistoryRow } from "../lib/api";

export default function Features() {
  const [rows, setRows] = useState<FeatureHistoryRow[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    featureHistory()
      .then(setRows)
      .catch((e: unknown) => setError(String(e)));
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
      <h2>Feature history</h2>
      <p className="muted">Attempts aggregated across all runs.</p>
      {rows.length === 0 ? (
        <div className="card empty">No attempts recorded yet.</div>
      ) : (
        <div className="card">
          <table>
            <thead>
              <tr>
                <th>Feature</th>
                <th>Attempts</th>
                <th>Successes</th>
                <th>Success rate</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => {
                const rate = r.Attempts ? Math.round((r.Successes / r.Attempts) * 100) : 0;
                return (
                  <tr key={r.FeatureID}>
                    <td className="mono">{r.FeatureID}</td>
                    <td>{r.Attempts}</td>
                    <td>{r.Successes}</td>
                    <td>{rate}%</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
