import { listLogLines, getRun } from "@/lib/queries";

export const dynamic = "force-dynamic";

// GET streams new log_lines for a run as Server-Sent Events. It polls the
// database every 500ms, flushing rows with id greater than the last seen. The
// stream ends shortly after the run reaches a terminal state.
export async function GET(
  _req: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;
  const encoder = new TextEncoder();

  const stream = new ReadableStream({
    async start(controller) {
      let lastId = 0;
      let closed = false;
      let idleTerminalTicks = 0;

      const send = (data: unknown) => {
        controller.enqueue(encoder.encode(`data: ${JSON.stringify(data)}\n\n`));
      };

      while (!closed) {
        try {
          const rows = await listLogLines(id, lastId);
          if (rows.length > 0) {
            lastId = rows[rows.length - 1].id;
            send(rows);
          } else {
            // Once the run is terminal and no new rows arrive, end the stream.
            const run = await getRun(id);
            if (run && run.status !== "running") {
              idleTerminalTicks++;
              if (idleTerminalTicks > 4) break; // ~2s grace
            }
          }
        } catch (e) {
          send([{ id: -1, ts: new Date().toISOString(), level: "error", msg: String(e), attrs: "" }]);
          break;
        }
        await new Promise((r) => setTimeout(r, 500));
      }

      controller.close();
      void closed;
    },
  });

  return new Response(stream, {
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache, no-transform",
      Connection: "keep-alive",
    },
  });
}
