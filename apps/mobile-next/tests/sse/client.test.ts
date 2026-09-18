import { RunStreamClient } from "@/features/executions/sse/RunStreamClient";
import type { RunStreamItem } from "@/features/executions/sse/parser";

// 内存流桩：Response body 为 ReadableStream（与 expo/fetch 形状一致）
const sseResponse = (frames: string[], init?: { status?: number }) =>
  new Response(new ReadableStream<Uint8Array>({
    start(controller) {
      for (const f of frames) controller.enqueue(new TextEncoder().encode(f));
      controller.close();
    },
  }), { status: init?.status ?? 200, headers: { "Content-Type": "text/event-stream" } });

const enc = new TextEncoder();

describe("RW-010 RunStreamClient", () => {
  const mkHandlers = () => {
    const items: RunStreamItem[] = [];
    const gaps: Array<[number, number]> = [];
    const recovers: string[] = [];
    let closed = 0;
    return {
      items,
      gaps,
      recovers,
      closedCount: () => closed,
      handlers: {
        onItem: (i: RunStreamItem) => items.push(i),
        onGap: (e: number, r: number) => gaps.push([e, r]),
        onRecover: (reason: string) => recovers.push(reason),
        onClosed: () => {
          closed++;
        },
      },
    };
  };

  it("消费完整流：事件 + run 终态帧 → closed", async () => {
    const h = mkHandlers();
    const client = new RunStreamClient({
      url: "https://w.example/api/v1/workbench/executions/r1/events",
      headers: {},
      signal: new AbortController().signal,
      startSeq: 0,
      handlers: h.handlers,
      fetchImpl: (async () =>
        sseResponse([
          'event: 1\ndata: {"seq":1,"type":"run_started","payload":{}}\n\n',
          'event: 2\ndata: {"seq":2,"type":"log","payload":{"text":"ok"}}\n\n',
          'event: run\ndata: {"run_id":"r1","status":"succeeded","seq":2}\n\n',
        ])) as unknown as typeof fetch,
    });
    const result = await client.run();
    expect(result).toBe("recover"); // EOF 后无 closed 标记（终态由 run 帧判断，控制器决定恢复）
    expect(h.items.filter((i) => i.kind === "event").map((i) => (i as { seq: number }).seq)).toEqual([1, 2]);
    expect(h.items.some((i) => i.kind === "run_snapshot")).toBe(true);
    expect(h.recovers).toEqual(["disconnected"]);
    expect(client.cursor).toBe(2);
  });

  it("409 → cursor_expired 恢复信号", async () => {
    const h = mkHandlers();
    const client = new RunStreamClient({
      url: "https://w.example/api/v1/x/events",
      headers: {},
      signal: new AbortController().signal,
      startSeq: 5,
      handlers: h.handlers,
      fetchImpl: (async () => new Response("{}", { status: 409 })) as unknown as typeof fetch,
    });
    expect(await client.run()).toBe("recover");
    expect(h.recovers).toEqual(["cursor_expired"]);
  });

  it("连接失败 → disconnected（不是任务失败）", async () => {
    const h = mkHandlers();
    const client = new RunStreamClient({
      url: "https://w.example/api/v1/x/events",
      headers: {},
      signal: new AbortController().signal,
      startSeq: 0,
      handlers: h.handlers,
      fetchImpl: (async () => {
        throw new TypeError("network down");
      }) as unknown as typeof fetch,
    });
    expect(await client.run()).toBe("recover");
    expect(h.recovers).toEqual(["disconnected"]);
  });

  it("abort → aborted，不触发恢复", async () => {
    const h = mkHandlers();
    const ac = new AbortController();
    ac.abort();
    const client = new RunStreamClient({
      url: "https://w.example/api/v1/x/events",
      headers: {},
      signal: ac.signal,
      startSeq: 0,
      handlers: h.handlers,
      fetchImpl: (async () => {
        throw Object.assign(new Error("Aborted"), { name: "AbortError" });
      }) as unknown as typeof fetch,
    });
    expect(await client.run()).toBe("aborted");
    expect(h.recovers).toHaveLength(0);
  });

  it("startSeq 传续游标：Last-Event-ID 与 after 参数", async () => {
    let seenUrl = "";
    let seenHeaders: Record<string, string> = {};
    const h = mkHandlers();
    const client = new RunStreamClient({
      url: "https://w.example/api/v1/x/events?once=1",
      headers: { Authorization: "Bearer t" },
      signal: new AbortController().signal,
      startSeq: 7,
      handlers: h.handlers,
      fetchImpl: (async (url: any, init: any) => {
        seenUrl = String(url);
        seenHeaders = init.headers;
        return sseResponse([]);
      }) as unknown as typeof fetch,
    });
    await client.run();
    expect(seenUrl).toContain("after=7");
    expect(seenHeaders["Last-Event-ID"]).toBe("7");
    expect(seenHeaders.Authorization).toBe("Bearer t");
    void enc;
  });
});
