// 聊天发送与流式消费（M07 接线，RW-015 补全）。
// 协议（api-facts §3/§6 + 源码核验）：
//   POST /agent-chat/:session_id，body 见 ChatSendInput（internal/handler/session/types.go:47-66）
//   响应为 SSE：每帧 event: message，data = StreamResponse{id,response_type,content,done,...}
//   断线续流：GET /sessions/continue-stream/:session_id?message_id=
// 投影规则：answer 增量拼接同一条 assistant 消息；tool_call/tool_result 独立卡片；
// error 显示为持久错误；complete/stop 终止；未知 response_type 安全展示不崩溃。
import { SseByteParser } from "@/features/executions/sse/parser";
import { decodeStreamChunk, type StreamChunkWire } from "@/contracts/workbench";
import { ApiError } from "@/api/http";

export interface ChatSendInput {
  query: string;
  agent_id?: string;
  agent_enabled?: boolean;
  knowledge_base_ids?: string[];
  attachment_ids?: string[];
  web_search_enabled?: boolean;
  channel?: string;
}

export type ChatMessagePart =
  | { kind: "text"; text: string }
  | { kind: "tool_call"; label: string }
  | { kind: "tool_result"; label: string }
  | { kind: "thinking"; text: string }
  | { kind: "references"; text: string }
  | { kind: "error"; text: string };

export interface ChatMessage {
  stableId: string;
  role: "user" | "assistant" | "system";
  parts: ChatMessagePart[];
  streaming: boolean;
  createdAt: string;
}

export type ChatStreamEvent =
  | { kind: "chunk"; chunk: StreamChunkWire }
  | { kind: "message_update" }
  | { kind: "done" }
  | { kind: "stream_error"; message: string }
  | { kind: "reconnect_required"; messageId: string | null };

/** 纯投影：StreamChunk 流 → 消息列表（不依赖网络，可测） */
export class ChatProjection {
  private messages: ChatMessage[] = [];
  private assistantSeq = 0;

  get list(): ChatMessage[] {
    return this.messages;
  }

  /** 用户发送先入列（发送失败时保留草稿由上层处理，此处不回滚） */
  appendUser(text: string): void {
    this.messages.push({
      stableId: `u_${Date.now().toString(36)}_${this.assistantSeq++}`,
      role: "user",
      parts: [{ kind: "text", text }],
      streaming: false,
      createdAt: new Date().toISOString(),
    });
  }

  applyChunk(chunk: StreamChunkWire): void {
    switch (chunk.response_type) {
      case "answer": {
        const last = this.lastStreamingAssistant();
        if (last) {
          const part = last.parts.find((p) => p.kind === "text") as { kind: "text"; text: string } | undefined;
          if (part) part.text += chunk.content;
          else last.parts.push({ kind: "text", text: chunk.content });
        } else {
          this.messages.push({
            stableId: `a_${Date.now().toString(36)}_${this.assistantSeq++}`,
            role: "assistant",
            parts: [{ kind: "text", text: chunk.content }],
            streaming: true,
            createdAt: new Date().toISOString(),
          });
        }
        break;
      }
      case "thinking":
        this.pushPart({ kind: "thinking", text: chunk.content });
        break;
      case "tool_call":
        this.pushPart({ kind: "tool_call", label: chunk.content || "工具调用" });
        break;
      case "tool_result":
        this.pushPart({ kind: "tool_result", label: chunk.content || "工具结果" });
        break;
      case "references":
        this.pushPart({ kind: "references", text: chunk.content || "引用" });
        break;
      case "error": {
        // 错误必须持久展示，不得吞掉（不用全局 catch 变成功）
        const last = this.lastStreamingAssistant();
        const msg = chunk.content || "服务返回错误";
        if (last) last.parts.push({ kind: "error", text: msg });
        else this.messages.push({
          stableId: `a_${Date.now().toString(36)}_${this.assistantSeq++}`,
          role: "assistant",
          parts: [{ kind: "error", text: msg }],
          streaming: false,
          createdAt: new Date().toISOString(),
        });
        this.settleStreaming();
        break;
      }
      case "complete":
      case "stop":
        this.settleStreaming();
        break;
      default:
        // 未知 response_type：安全展示原文，不崩溃不伪装
        if (chunk.content) this.pushPart({ kind: "text", text: chunk.content });
        break;
    }
    if (chunk.done) this.settleStreaming();
  }

  /** 流结束（EOF/中断）：停止 streaming 标记；中断不是失败 */
  settleStreaming(): void {
    for (const m of this.messages) if (m.role === "assistant") m.streaming = false;
  }

  lastAssistantMessageId(): string | null {
    const last = [...this.messages].reverse().find((m) => m.role === "assistant");
    return last?.stableId ?? null;
  }

  private lastStreamingAssistant(): ChatMessage | null {
    for (let i = this.messages.length - 1; i >= 0; i--) {
      const m = this.messages[i]!;
      if (m.role === "assistant") return m.streaming ? m : null;
    }
    return null;
  }

  private pushPart(part: ChatMessagePart): void {
    const last = this.lastStreamingAssistant() ?? this.newAssistant();
    last.parts.push(part);
  }

  private newAssistant(): ChatMessage {
    const m: ChatMessage = {
      stableId: `a_${Date.now().toString(36)}_${this.assistantSeq++}`,
      role: "assistant",
      parts: [],
      streaming: true,
      createdAt: new Date().toISOString(),
    };
    this.messages.push(m);
    return m;
  }
}

export interface ChatStreamHandlers {
  onUpdate(projection: ChatProjection): void;
  onDone(projection: ChatProjection): void;
  onError(e: Error, projection: ChatProjection): void;
  /** 断流：携带最后 assistant 消息 id，可走 continue-stream 恢复 */
  onDisconnected(projection: ChatProjection, lastMessageId: string | null): void;
}

export interface ChatStreamDeps {
  /** POST /agent-chat/:session_id 返回 SSE Response */
  fetchImpl?: typeof fetch;
  buildUrl(sessionId: string): string;
  headers(): Record<string, string>;
}

/** 消费 POST 响应的 SSE 流（每帧 event: message） */
export async function consumeChatStream(
  res: Response,
  projection: ChatProjection,
  handlers: ChatStreamHandlers,
): Promise<"done" | "disconnected"> {
  const parser = new SseByteParser();
  let sawDone = false;
  const body = (res.body as unknown as ReadableStream<Uint8Array> | null) ?? null;
  if (!body) {
    handlers.onDisconnected(projection, projection.lastAssistantMessageId());
    return "disconnected";
  }
  const reader = body.getReader();
  const applyFrame = (dataText: string) => {
    try {
      const chunk = decodeStreamChunk(JSON.parse(dataText));
      projection.applyChunk(chunk);
      handlers.onUpdate(projection);
      if (chunk.done || chunk.response_type === "complete" || chunk.response_type === "stop") sawDone = true;
    } catch {
      // 残缺帧不伪装成功；也不终止整条流（后续帧可能有效）
    }
  };
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      for (const frame of parser.push(value)) {
        if (frame.data !== "" && (frame.event ?? "message") === "message") applyFrame(frame.data);
      }
    }
    for (const frame of parser.finish()) {
      if (frame.data !== "" && (frame.event ?? "message") === "message") applyFrame(frame.data);
    }
  } catch {
    handlers.onDisconnected(projection, projection.lastAssistantMessageId());
    return "disconnected";
  }
  if (sawDone) {
    projection.settleStreaming();
    handlers.onDone(projection);
    return "done";
  }
  // EOF 无终态帧：交给恢复（continue-stream），不是失败
  handlers.onDisconnected(projection, projection.lastAssistantMessageId());
  return "disconnected";
}

export class ChatService {
  constructor(private deps: ChatStreamDeps) {}

  async send(sessionId: string, input: ChatSendInput, projection: ChatProjection, handlers: ChatStreamHandlers, signal?: AbortSignal): Promise<void> {
    projection.appendUser(input.query);
    handlers.onUpdate(projection);
    const fetchImpl = this.deps.fetchImpl ?? (globalThis.fetch as typeof fetch);
    let res: Response;
    try {
      res = await fetchImpl(this.deps.buildUrl(sessionId), {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "text/event-stream", ...this.deps.headers() },
        body: JSON.stringify({
          query: input.query,
          agent_id: input.agent_id,
          agent_enabled: input.agent_id ? true : undefined,
          knowledge_base_ids: input.knowledge_base_ids?.length ? input.knowledge_base_ids : undefined,
          attachment_ids: input.attachment_ids?.length ? input.attachment_ids : undefined,
          web_search_enabled: input.web_search_enabled,
          channel: input.channel ?? "mobile",
        }),
        signal,
      });
    } catch (e) {
      if ((e as Error).name === "AbortError") return;
      handlers.onError(new ApiError("network", `发送失败：${(e as Error).message}`), projection);
      return;
    }
    if (!res.ok) {
      let message = `发送失败（HTTP ${res.status}）`;
      try {
        const text = await res.text();
        const body = JSON.parse(text) as Record<string, unknown>;
        message = String(body.message ?? body.error ?? message);
      } catch {
        // 保留默认信息
      }
      handlers.onError(new ApiError(res.status === 401 ? "unauthorized" : res.status === 403 ? "forbidden" : "server", message), projection);
      return;
    }
    await consumeChatStream(res, projection, handlers);
  }
}
