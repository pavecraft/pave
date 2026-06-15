import { featureHistory } from "@/lib/queries";

export const dynamic = "force-dynamic";

export default async function FeaturesPage() {
  let rows;
  try {
    rows = await featureHistory();
  } catch (e) {
    return (
      <div className="card">
        <h2>Cannot read the pave database</h2>
        <pre>{String(e)}</pre>
      </div>
    );
  }

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
                const successes = Number(r.successes) || 0;
                const attempts = Number(r.attempts) || 0;
                const rate = attempts ? Math.round((successes / attempts) * 100) : 0;
                return (
                  <tr key={r.feature_id}>
                    <td className="mono">{r.feature_id}</td>
                    <td>{attempts}</td>
                    <td>{successes}</td>
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
