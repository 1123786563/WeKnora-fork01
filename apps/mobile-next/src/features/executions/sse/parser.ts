// SSE 字节级 parser（RW-010）。
// 帧格式对齐真实 Go writer（本仓库核验）：
//  - agent_run 流（internal/handler/session/agent_run.go:175-228）：
//      event: <seq数字>\ndata: {seq,attempt_id?,type,payload}\n\n
//      event: run\ndata: {run_id,session_id,status,...,seq,revision}
//      event: keepalive\ndata: {"seq":N}（15s）
//      event: error\ndata: {"code","message","seq"}
//  - workbench 流（internal/handler/session/workbench_read.go:318）：
//      id: <seq>\nevent: <type>\ndata: <payload json>\n\n
//  - 409 cursor_expired 以 HTTP 状态在建立连接时返回，不是 SSE 帧。
// 约束（详细设计 §5.1）：UTF-8 可跨 chunk；CRLF/LF 混合；多行 data 先拼接再 JSON；
// 控制帧不进业务投影、不推进业务 cursor。

export interface SseFrame {
  id: string | null;
  event: string | null;
  data: string; // 多行 data 以 \n 拼接
}

export class SseByteParser {
  private buffer = "";
  private pendingLines: string[] = [];
  private decoder = new TextDecoder("utf-8"); // 流式解码：跨块中文安全

  /** 喂入任意大小字节块，返回完整帧（可能 0..n 个） */
  push(chunk: Uint8Array): SseFrame[] {
    this.buffer += this.decoder.decode(chunk, { stream: true });
    const frames: SseFrame[] = [];
    // 行分割支持 \n、\r\n、\r；帧以空行结束
    let nl: { index: number; len: number } | -1;
    while ((nl = this.findLineBreak()) !== -1) {
      const line = this.buffer.slice(0, nl.index);
      this.buffer = this.buffer.slice(nl.index + nl.len);
      if (line === "") {
        const frame = this.takeFrame();
        if (frame) frames.push(frame);
      } else {
        this.pendingLines.push(line);
      }
    }
    return frames;
  }

  /** 流结束：flush 解码器残余字节并产出最后一个未定界帧 */
  finish(): SseFrame[] {
    this.buffer += this.decoder.decode();
    if (this.buffer.endsWith("\r")) this.buffer = this.buffer.slice(0, -1) + "\n"; // 尾部独立 CR 定界
    const frames: SseFrame[] = [];
    let nl: { index: number; len: number } | -1;
    while ((nl = this.findLineBreak()) !== -1) {
      const line = this.buffer.slice(0, nl.index);
      this.buffer = this.buffer.slice(nl.index + nl.len);
      if (line === "") {
        const frame = this.takeFrame();
        if (frame) frames.push(frame);
      } else {
        this.pendingLines.push(line);
      }
    }
    if (this.pendingLines.length) {
      const frame = this.takeFrame();
      if (frame) frames.push(frame);
    }
    return frames;
  }

  private findLineBreak(): { index: number; len: number } | -1 {
    const n = this.buffer.indexOf("\n");
    const r = this.buffer.indexOf("\r");
    if (r >= 0 && (n < 0 || r < n)) {
      if (r === this.buffer.length - 1) {
        // 块尾 \r：无法判断是 CRLF 前半还是独立 CR，等待下一字节（finish 中单独处理）
        return n >= 0 ? { index: n, len: 1 } : -1;
      }
      return this.buffer[r + 1] === "\n" ? { index: r, len: 2 } : { index: r, len: 1 };
    }
    if (n >= 0) return { index: n, len: 1 };
    return -1;
  }

  private takeFrame(): SseFrame | null {
    if (!this.pendingLines.length) return null;
    let id: string | null = null;
    let event: string | null = null;
    const dataLines: string[] = [];
    for (const line of this.pendingLines) {
      if (line.startsWith(":")) continue; // 注释行/心跳注释
      if (line.startsWith("id:")) id = line.slice(3).trimStart();
      else if (line.startsWith("event:")) event = line.slice(6).trimStart();
      else if (line.startsWith("data:")) dataLines.push(line.slice(5).replace(/^ /, ""));
    }
    this.pendingLines = [];
    if (id === null && event === null && dataLines.length === 0) return null;
    return { id, event, data: dataLines.join("\n") };
  }
}

// ---- Run 事件流装配（agent_run 协议）----

export type RunStreamItem =
  | { kind: "event"; seq: number; attemptId: string | null; type: string; payload: unknown }
  | { kind: "run_snapshot"; run: Record<string, unknown>; seq: number }
  | { kind: "keepalive"; seq: number }
  | { kind: "stream_error"; code: string; message: string; seq: number }
  | { kind: "malformed"; reason: string; raw: string };

export interface AssemblerCallbacks {
  onItem(item: RunStreamItem): void;
  /** 业务 seq 缺口：停止提交投影，等待快照恢复（详细设计 §5.2） */
  onGap(expected: number, received: number): void;
}

export class RunEventAssembler {
  private appliedSeq = 0; // 业务 cursor：只被业务事件推进
  private readonly seen = new Set<number>();

  constructor(private cb: AssemblerCallbacks, private opts: { startSeq?: number; strictGap?: boolean } = {}) {
    this.appliedSeq = opts.startSeq ?? 0;
  }

  get cursor(): number {
    return this.appliedSeq;
  }

  frame(frame: SseFrame): void {
    const dataText = frame.data;
    let data: unknown = null;
    if (dataText !== "") {
      try {
        data = JSON.parse(dataText);
      } catch {
        this.cb.onItem({ kind: "malformed", reason: "data JSON 解析失败", raw: dataText });
        return;
      }
    }
    const ev = frame.event ?? "message";

    if (ev === "keepalive") {
      // 控制帧：不进业务投影、不推进 cursor
      const seq = readNum(data, "seq") ?? this.appliedSeq;
      this.cb.onItem({ kind: "keepalive", seq });
      return;
    }
    if (ev === "run") {
      const run = (data ?? {}) as Record<string, unknown>;
      const seq = readNum(run, "seq") ?? this.appliedSeq;
      this.cb.onItem({ kind: "run_snapshot", run, seq });
      return;
    }
    if (ev === "error") {
      const e = (data ?? {}) as Record<string, unknown>;
      this.cb.onItem({
        kind: "stream_error",
        code: String(e.code ?? "stream_error"),
        message: String(e.message ?? ""),
        seq: readNum(e, "seq") ?? this.appliedSeq,
      });
      return;
    }
    if (/^\d+$/.test(ev)) {
      // 业务事件帧：event 名是 seq；data 是完整 RunEvent
      this.applyEvent(Number(ev), data);
      return;
    }
    // workbench 风格：id: seq + event: type + data: payload
    if (frame.id !== null && /^\d+$/.test(frame.id)) {
      this.applyEvent(Number(frame.id), { seq: Number(frame.id), type: ev, payload: data });
      return;
    }
    this.cb.onItem({ kind: "malformed", reason: `未知 event 名 ${ev}`, raw: dataText });
  }

  private applyEvent(seqFromName: number, data: unknown) {
    const e = (data ?? {}) as Record<string, unknown>;
    const seq = readNum(e, "seq") ?? seqFromName;
    if (!Number.isInteger(seq) || seq < 1) {
      this.cb.onItem({ kind: "malformed", reason: `无效 seq ${seq}`, raw: JSON.stringify(e) });
      return;
    }
    const type = String(e.type ?? "");
    if (!type) {
      // 残缺事件不推进 cursor、不记录 seen（详细设计：缺字段不用空值补成成功）
      this.cb.onItem({ kind: "malformed", reason: "事件缺 type", raw: JSON.stringify(e) });
      return;
    }
    if (this.seen.has(seq) || seq <= this.appliedSeq) {
      return; // 重复 seq 幂等丢弃
    }
    if (this.opts.strictGap !== false && this.appliedSeq > 0 && seq > this.appliedSeq + 1) {
      this.cb.onGap(this.appliedSeq + 1, seq);
      // 缺口时不提交本事件；由恢复控制器取快照后重建
      return;
    }
    this.seen.add(seq);
    this.appliedSeq = seq;
    this.cb.onItem({
      kind: "event",
      seq,
      attemptId: e.attempt_id != null ? String(e.attempt_id) : null,
      type,
      payload: e.payload ?? null,
    });
  }
}

function readNum(v: unknown, key: string): number | null {
  if (v && typeof v === "object" && key in (v as Record<string, unknown>)) {
    const n = (v as Record<string, unknown>)[key];
    if (typeof n === "number" && Number.isFinite(n)) return n;
    if (typeof n === "string" && /^\d+$/.test(n)) return Number(n);
  }
  return null;
}
