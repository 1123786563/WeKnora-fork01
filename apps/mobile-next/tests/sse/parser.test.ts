import { readFileSync } from "node:fs";
import { join } from "node:path";
import { SseByteParser, RunEventAssembler, type RunStreamItem } from "@/features/executions/sse/parser";

const enc = new TextEncoder();
const bytes = (s: string) => enc.encode(s);

// fixture 由 tests/sse/gen/main.go 生成（复刻后端 Go SSE writer 字节输出）
const goFixture = readFileSync(join(__dirname, "fixtures", "go-real-bytes.txt"));

describe("RW-010 SseByteParser 字节级解析", () => {
  it("真实 Go 输出字节：业务事件 + 控制帧全部解析", () => {
    const p = new SseByteParser();
    const frames = p.push(goFixture);
    const events = frames.map((f) => f.event);
    expect(events).toContain("1");
    expect(events).toContain("run");
    expect(events).toContain("keepalive");
    expect(events).toContain("error");
    expect(events).toContain("state"); // workbench 风格
    // workbench 帧的 id 是 seq 字符串
    const wb = frames.find((f) => f.event === "tool_call");
    expect(wb?.id).toBe("2");
    expect(JSON.parse(wb!.data)).toEqual({ tool: "search", args: { q: "客户反馈" } });
  });

  it("中文 UTF-8 多字节跨 chunk 边界", () => {
    const raw = 'event: 7\ndata: {"delta":"会议纪要整理中"}\n\n';
    const mid = raw.indexOf("会");
    const chunks = [bytes(raw.slice(0, mid + 1)), bytes(raw.slice(mid + 1))];
    const p = new SseByteParser();
    const f1 = p.push(chunks[0]!);
    const f2 = p.push(chunks[1]!);
    expect(f1).toHaveLength(0);
    const frame = f2[0]!;
    expect(frame.event).toBe("7");
    expect(JSON.parse(frame.data).delta).toBe("会议纪要整理中");
  });

  it("CRLF 跨 chunk（\\r 在块尾 \\n 在块头）与混合换行", () => {
    const p = new SseByteParser();
    const f1 = p.push(bytes("event: 1\r"));
    expect(f1).toHaveLength(0);
    const f2 = p.push(bytes("\ndata: {\"a\":1}\r\n\r\n"));
    expect(f2).toHaveLength(1);
    expect(f2[0]!.event).toBe("1");
    expect(f2[0]!.data).toBe('{"a":1}');
  });

  it("多行 data 以 \\n 拼接后再 JSON 解析", () => {
    const p = new SseByteParser();
    const frames = p.push(bytes('event: message\ndata: {"a":\ndata: 1}\n\n'));
    expect(JSON.parse(frames[0]!.data)).toEqual({ a: 1 });
  });

  it("finish 产出无尾定界的残余帧", () => {
    const p = new SseByteParser();
    p.push(bytes('event: keepalive\ndata: {"seq":2}'));
    const frames = p.finish();
    expect(frames).toHaveLength(1);
    expect(frames[0]!.event).toBe("keepalive");
  });
});

describe("RW-010 RunEventAssembler 投影与恢复规则", () => {
  const collect = () => {
    const items: RunStreamItem[] = [];
    const gaps: Array<[number, number]> = [];
    return { items, gaps };
  };

  it("真实 Go 字节：agent_run 事件推进 cursor；控制帧不推进；workbench 段 seq 重叠被幂等丢弃", () => {
    const c = collect();
    const asm = new RunEventAssembler({
      onItem: (i) => c.items.push(i),
      onGap: (e, r) => c.gaps.push([e, r]),
    });
    const p = new SseByteParser();
    for (const f of p.push(goFixture)) asm.frame(f);
    const events = c.items.filter((i) => i.kind === "event");
    // agent_run 帧 seq 1..4 连续提交；随后 workbench 帧 seq 1..3 重叠（混合喂入场景）→ 幂等丢弃
    expect(events.map((e) => (e as { seq: number }).seq)).toEqual([1, 2, 3, 4]);
    expect(asm.cursor).toBe(4);
    expect(c.items.some((i) => i.kind === "run_snapshot")).toBe(true);
    expect(c.items.some((i) => i.kind === "keepalive")).toBe(true);
    expect(c.items.some((i) => i.kind === "stream_error")).toBe(true);
  });

  it("真实 Go 字节：workbench 流单独消费（id:seq + event:type + data:payload）", () => {
    const c = collect();
    const asm = new RunEventAssembler({ onItem: (i) => c.items.push(i), onGap: () => {} });
    const p = new SseByteParser();
    const all = p.push(goFixture);
    const wbFrames = all.filter((f) => ["state", "tool_call", "artifact"].includes(f.event ?? ""));
    for (const f of wbFrames) asm.frame(f);
    const events = c.items.filter((i) => i.kind === "event");
    expect(events.map((e) => (e as { seq: number }).seq)).toEqual([1, 2, 3]);
    expect(asm.cursor).toBe(3);
    const tc = events.find((e) => (e as { type: string }).type === "tool_call") as { payload: { tool: string } };
    expect(tc.payload.tool).toBe("search");
  });

  it("重复 seq 幂等丢弃", () => {
    const c = collect();
    const asm = new RunEventAssembler({ onItem: (i) => c.items.push(i), onGap: () => {} });
    asm.frame({ id: null, event: "1", data: '{"seq":1,"type":"log","payload":{}}' });
    asm.frame({ id: null, event: "1", data: '{"seq":1,"type":"log","payload":{}}' });
    expect(c.items.filter((i) => i.kind === "event")).toHaveLength(1);
    expect(asm.cursor).toBe(1);
  });

  it("seq 缺口：不提交并触发 onGap（等待快照恢复）", () => {
    const c = collect();
    const asm = new RunEventAssembler({ onItem: (i) => c.items.push(i), onGap: (e, r) => c.gaps.push([e, r]) });
    asm.frame({ id: null, event: "1", data: '{"seq":1,"type":"a","payload":{}}' });
    asm.frame({ id: null, event: "5", data: '{"seq":5,"type":"b","payload":{}}' });
    expect(c.items.filter((i) => i.kind === "event")).toHaveLength(1);
    expect(c.gaps).toEqual([[2, 5]]);
    expect(asm.cursor).toBe(1);
  });

  it("startSeq（快照恢复后）从 watermark 续流，旧 seq 不重复提交", () => {
    const c = collect();
    const asm = new RunEventAssembler({ onItem: (i) => c.items.push(i), onGap: () => {} }, { startSeq: 4 });
    asm.frame({ id: null, event: "4", data: '{"seq":4,"type":"a","payload":{}}' }); // 重复（已应用）
    asm.frame({ id: null, event: "5", data: '{"seq":5,"type":"b","payload":{}}' });
    expect(c.items.filter((i) => i.kind === "event")).toHaveLength(1);
    expect(asm.cursor).toBe(5);
  });

  it("残缺事件（缺 type / seq 无效）报 malformed，不用空值补成功", () => {
    const c = collect();
    const asm = new RunEventAssembler({ onItem: (i) => c.items.push(i), onGap: () => {} });
    asm.frame({ id: null, event: "1", data: '{"seq":1,"payload":{}}' });
    asm.frame({ id: null, event: "2", data: 'not json' });
    expect(c.items.filter((i) => i.kind === "malformed")).toHaveLength(2);
    expect(asm.cursor).toBe(0);
  });

  it("keepalive 不推进业务 cursor", () => {
    const c = collect();
    const asm = new RunEventAssembler({ onItem: (i) => c.items.push(i), onGap: () => {} });
    asm.frame({ id: null, event: "1", data: '{"seq":1,"type":"a","payload":{}}' });
    asm.frame({ id: null, event: "keepalive", data: '{"seq":99}' });
    expect(asm.cursor).toBe(1);
  });
});
