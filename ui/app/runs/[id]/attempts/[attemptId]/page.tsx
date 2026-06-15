import Link from "next/link";
import { notFound } from "next/navigation";
import { getAttempt } from "@/lib/queries";
import { fmtTime, fmtDuration, statusClass, attemptBadgeClass, attemptLabel } from "@/lib/format";
import Markdown from "@/components/Markdown";

export const dynamic = "force-dynamic";

export default async function AttemptPage({
  params,
}: {
  params: Promise<{ id: string; attemptId: string }>;
}) {
  const { id, attemptId } = await params;

  const a = await getAttempt(attemptId);
  if (!a) notFound();

  return (
    <>
      <p className="muted">
        <Link href={`/runs/${id}`}>← run {id.slice(0, 8)}</Link>
      </p>
      <h2>
        Attempt for <span className="mono">{a.feature_id}</span>{" "}
        <span className={attemptBadgeClass(a.success, a.exit_code, a.ended_at)}>
          {attemptLabel(a.success, a.exit_code, a.ended_at)}
        </span>
      </h2>

      <div className="grid">
        <Stat label="Provider" value={a.provider} />
        <Stat label="Started" value={fmtTime(a.started_at)} />
        <Stat label="Ended" value={fmtTime(a.ended_at)} />
        <Stat label="Duration" value={fmtDuration(a.duration_ms)} />
        <Stat label="Session" value={a.session_id || "—"} />
      </div>

      <h2>Prompt</h2>
      <pre>{a.prompt}</pre>

      <h2>Output</h2>
      {a.output
        ? <Markdown>{a.output}</Markdown>
        : <p className="muted">(empty)</p>}

      {a.stderr ? (
        <>
          <h2>Stderr</h2>
          <pre>{a.stderr}</pre>
        </>
      ) : null}
    </>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="stat">
      <div className="num mono" style={{ fontSize: 14 }}>
        {value}
      </div>
      <div className="label">{label}</div>
    </div>
  );
}
