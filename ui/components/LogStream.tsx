"use client";

import { useEffect, useRef, useState } from "react";

interface LogLine {
  id: number;
  ts: string;
  level: string;
  msg: string;
  attrs: string;
}

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
        const batch: LogLine[] = JSON.parse(ev.data);
        if (batch.length > 0) {
          setLines((prev) => [...prev, ...batch]);
        }
      } catch {
        /* ignore malformed frame */
      }
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
        {lines.length === 0 ? (
          <div className="muted">Waiting for log output…</div>
        ) : (
          lines.map((l) => (
            <div className="log-line" key={l.id}>
              <span className="log-ts">{new Date(l.ts).toLocaleTimeString()}</span>
              <span className={`log-${l.level}`}>
                {l.msg}
                {l.attrs && l.attrs !== "" ? ` ${l.attrs}` : ""}
              </span>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
