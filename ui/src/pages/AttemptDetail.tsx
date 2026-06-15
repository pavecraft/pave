import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { getAttempt, type Attempt } from "../lib/api";
import { fmtTime, fmtDuration, attemptBadgeClass, attemptLabel } from "../lib/format";
import Markdown from "../components/Markdown";

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="stat">
      <div className="num mono" style={{ fontSize: 14 }}>{value}</div>
      <div className="label">{label}</div>
    </div>
  );
}

export default function AttemptDetail() {
  const { id, attemptId } = useParams<{ id: string; attemptId: string }>();
  const [attempt, setAttempt] = useState<Attempt | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!attemptId) return;
    getAttempt(attemptId)
      .then(setAttempt)
      .catch((e: unknown) => setError(String(e)));
  }, [attemptId]);

  if (error) return <div className="card"><h2>Error</h2><pre>{error}</pre></div>;
  if (!attempt) return <div className="muted">Loading…</div>;

  const a = attempt;
  return (
    <>
      <p className="muted"><Link to={`/runs/${id}`}>← run {id?.slice(0, 8)}</Link></p>
      <h2>
        Attempt for <span className="mono">{a.FeatureID}</span>{" "}
        <span className={attemptBadgeClass(a.Success, a.ExitCode, a.EndedAt)}>
          {attemptLabel(a.Success, a.ExitCode, a.EndedAt)}
        </span>
      </h2>

      <div className="grid">
        <Stat label="Provider" value={a.Provider} />
        <Stat label="Started" value={fmtTime(a.StartedAt)} />
        <Stat label="Ended" value={fmtTime(a.EndedAt)} />
        <Stat label="Duration" value={fmtDuration(a.DurationMs)} />
        <Stat label="Session" value={a.SessionID || "—"} />
      </div>

      <h2>Prompt</h2>
      <pre>{a.Prompt}</pre>

      <h2>Output</h2>
      {a.Output ? <Markdown>{a.Output}</Markdown> : <p className="muted">(empty)</p>}

      {a.Stderr && (
        <>
          <h2>Stderr</h2>
          <pre>{a.Stderr}</pre>
        </>
      )}
    </>
  );
}
