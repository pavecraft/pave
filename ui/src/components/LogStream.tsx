import { useEffect, useRef, useState } from "react";
import type { LogLine } from "../lib/types";

export default function LogStream({ runId }: { runId: string }) {
  const [lines, setLines] = useState<LogLine[]>([]);
  const [connected, setConnected] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const es = new EventSource(`/api/runs/${runId}/stream`);
    es.onopen = () => setConnected(true);
    es.onerror = () => setConnected(false);
    es.onmessage = (ev) => {
      try {
        const batch: LogLine[] = JSON.parse(ev.data as string);
        if (batch.length > 0) setLines((prev) => [...prev, ...batch]);
      } catch { /* ignore malformed frame */ }
    };
    return () => es.close();
  }, [runId]);

  useEffect(() => {
    const box = boxRef.current;
    if (box) box.scrollTop = box.scrollHeight;
  }, [lines]);

  return (
    <div className="card">
      <h2>
        Live log{" "}
        <span className={connected ? "badge badge-run" : "badge"}>
          {connected ? "streaming" : "disconnected"}
        </span>
      </h2>
      <div className="log" ref={boxRef}>
        {lines.length === 0
          ? <div className="muted">Waiting for log output…</div>
          : lines.map((l) => (
            <div className="log-line" key={l.ID}>
              <span className="log-ts">{new Date(l.TS).toLocaleTimeString()}</span>
              <span className={`log-${l.Level}`}>
                {l.Msg}{l.Attrs && l.Attrs !== "" ? ` ${l.Attrs}` : ""}
              </span>
            </div>
          ))}
      </div>
    </div>
  );
}
