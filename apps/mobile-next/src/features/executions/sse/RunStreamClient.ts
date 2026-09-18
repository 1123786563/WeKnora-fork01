// Run 事件流客户端（RW-010 运行时接线）：HTTP SSE → 字节 → SseByteParser → RunEventAssembler。
// 传输用 expo/fetch（原生流式响应体）；409 cursor_expired → 快照重取回调。
// 页面/M08/M07 通过 startRunStream 订阅；切空间由 AbortSignal（ScopeCoordinator）打断，
// 断流/EOF 不是任务失败，交给恢复回调（详细设计 §5.1）。
import { SseByteParser, RunEventAssembler, type RunStreamItem } from "./parser";

export interface RunStreamHandlers {
  onItem(item: RunStreamItem): void;
  onGap(expected: number, received: number): void;
  /** cursor 过期/断流：应取 snapshot 后从 watermark 续流 */
  onRecover(reason: "cursor_expired" | "disconnected" | "http_error", detail?: string): void;
  /** 流正常结束（终态 run 帧） */
  onClosed(): void;
}

export interface RunStreamOptions {
  url: string; // 完整 events URL（含 Last-Event-ID 由本层追加）
  headers: Record<string, string>;
  signal: AbortSignal;
  startSeq: number; // 快照 watermark；0 从头
  handlers: RunStreamHandlers;
  /** 注入式传输（测试替身）；默认 expo/fetch */
  fetchImpl?: typeof fetch;
}

export class RunStreamClient {
  private assembler: RunEventAssembler;
  private parser = new SseByteParser();
  private closed = false;

  constructor(private opts: RunStreamOptions) {
    this.assembler = new RunEventAssembler(
      {
        onItem: opts.handlers.onItem,
        onGap: (e, r) => opts.handlers.onGap(e, r),
      },
      { startSeq: opts.startSeq },
    );
  }

  get cursor(): number {
    return this.assembler.cursor;
  }

  /** 建立连接并消费到流结束/中断；返回是否正常关闭 */
  async run(): Promise<"closed" | "recover" | "aborted"> {
    const url = `${this.opts.url}${this.opts.url.includes("?") ? "&" : "?"}after=${this.assembler.cursor}`;
    const fetchImpl = this.opts.fetchImpl ?? (globalThis.fetch as typeof fetch);
    let res: Response;
    try {
      res = await fetchImpl(url, { headers: { ...this.opts.headers, "Last-Event-ID": String(this.assembler.cursor) }, signal: this.opts.signal });
    } catch (e) {
      if ((e as Error).name === "AbortError" || this.opts.signal.aborted) return "aborted";
      this.opts.handlers.onRecover("disconnected", (e as Error).message);
      return "recover";
    }
    if (res.status === 409) {
      this.opts.handlers.onRecover("cursor_expired");
      return "recover";
    }
    if (!res.ok) {
      this.opts.handlers.onRecover("http_error", `HTTP ${res.status}`);
      return "recover";
    }
    if (res.status === 204) {
      // once=1 空页或已终止
      this.closed = true;
      this.opts.handlers.onClosed();
      return "closed";
    }
    try {
      const reader = (res.body as unknown as ReadableStream<Uint8Array>).getReader();
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        for (const frame of this.parser.push(value)) this.assembler.frame(frame);
      }
      for (const frame of this.parser.finish()) this.assembler.frame(frame);
    } catch (e) {
      if (this.opts.signal.aborted) return "aborted";
      this.opts.handlers.onRecover("disconnected", (e as Error).message);
      return "recover";
    }
    // EOF：交给恢复控制器（不是任务失败）
    if (!this.closed) this.opts.handlers.onRecover("disconnected", "eof");
    return this.closed ? "closed" : "recover";
  }
}
