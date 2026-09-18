import { arr, bool, enumOr, num, obj, optNum, optStr, str, type Decoder } from "./wire";

// ---- Workbench 执行（api-facts §5）----
// POST /workbench/executions → 202 {data:{run_id,request_id,status}}

export type RunStatusWire =
  | "queued"
  | "running"
  | "waiting_user"
  | "completed"
  | "failed"
  | "cancelled"
  | "unknown";
export const RUN_STATUSES = ["queued", "running", "waiting_user", "completed", "failed", "cancelled"] as const;
export const decodeRunStatus = enumOr(RUN_STATUSES, "unknown");

export interface StartExecutionResponseWire {
  run_id: string;
  request_id: string;
  status: string;
}

export const decodeStartExecutionResponse: Decoder<StartExecutionResponseWire> = (v) => {
  const r = (v ?? {}) as Record<string, unknown>;
  return {
    run_id: String(r.run_id ?? ""),
    request_id: String(r.request_id ?? ""),
    status: String(r.status ?? ""),
  };
};

export interface RunViewWire {
  run_id: string;
  session_id: string;
  status: string; // 原始枚举保留（未知不冒充）
  revision: number | null;
  budget_upper: number | null;
  spent: number | null;
  as_of: string | null;
}

export const decodeRunView: Decoder<RunViewWire> = (v) => {
  const r = (v ?? {}) as Record<string, unknown>;
  return {
    run_id: String(r.run_id ?? r.id ?? ""),
    session_id: String(r.session_id ?? ""),
    status: String(r.status ?? "unknown"),
    revision: optNum()(r.revision ?? null),
    budget_upper: optNum()(r.budget_upper ?? r.budget ?? null),
    spent: optNum()(r.spent ?? r.used ?? null),
    as_of: optStr()(r.as_of ?? null),
  };
};

export interface SnapshotWire {
  run: RunViewWire;
  watermark: number;
  projection_hint: string | null;
}

export const decodeSnapshot: Decoder<SnapshotWire> = (v) => {
  const r = (v ?? {}) as Record<string, unknown>;
  return {
    run: decodeRunView(r.run ?? r),
    watermark: num()(r.watermark ?? r.seq ?? 0),
    projection_hint: optStr()(r.projection_hint ?? null),
  };
};

// ---- 交互（tool_approval / budget / recovery；question/connection 走专端点，decisions D-05）----
export type InteractionKindWire = "tool_approval" | "budget" | "recovery" | "unknown";
export const decodeInteractionKind = enumOr(["tool_approval", "budget", "recovery"] as const, "unknown");

export interface InteractionWire {
  id: string;
  run_id: string;
  kind: InteractionKindWire;
  status: "pending" | "resolved" | "expired" | "unknown";
  revision: number | null;
  title: string;
  summary: string;
  payload: Record<string, unknown> | null;
  created_at: string | null;
}

export const decodeInteraction: Decoder<InteractionWire> = (v) => {
  const r = (v ?? {}) as Record<string, unknown>;
  return {
    id: String(r.id ?? r.interaction_id ?? r.pending_id ?? ""),
    run_id: String(r.run_id ?? ""),
    kind: decodeInteractionKind(r.kind ?? r.type),
    status: enumOr(["pending", "resolved", "expired"] as const, "unknown")(r.status ?? r.state),
    revision: optNum()(r.revision ?? null),
    title: String(r.title ?? r.action ?? ""),
    summary: String(r.summary ?? r.description ?? ""),
    payload: (r.payload ?? null) as Record<string, unknown> | null,
    created_at: optStr()(r.created_at ?? null),
  };
};

export const decodeInteractionList: Decoder<InteractionWire[]> = (v) => {
  const list = Array.isArray(v)
    ? v
    : ((v as Record<string, unknown>)?.interactions as unknown[]) ??
      ((v as Record<string, unknown>)?.pending as unknown[]) ??
      [];
  return arr(decodeInteraction)(list);
};

// ---- Run SSE 事件（api-facts §6）----
// Run 事件流：event: <seq数字> 的业务事件、event: run 快照帧、event: keepalive、event: error
export interface RunEventWire {
  seq: number;
  type: string; // 事件 type 原始字符串保留
  occurred_at: string | null;
  payload: Record<string, unknown>;
}

export const decodeRunEvent: Decoder<RunEventWire> = (v) => {
  const r = (v ?? {}) as Record<string, unknown>;
  return {
    seq: num()(r.seq ?? r.Seq ?? -1),
    type: String(r.type ?? r.event_type ?? ""),
    occurred_at: optStr()(r.occurred_at ?? null),
    payload: (r.payload ?? {}) as Record<string, unknown>,
  };
};

export interface RunControlFrameWire {
  kind: "run" | "keepalive" | "error";
  run?: RunViewWire;
  seq: number | null;
  code?: string;
  message?: string;
}

// ---- 聊天流 StreamResponse（api-facts §6）----
export type StreamResponseType =
  | "answer"
  | "references"
  | "thinking"
  | "tool_call"
  | "tool_result"
  | "error"
  | "reflection"
  | "session_title"
  | "agent_query"
  | "complete"
  | "stop"
  | "artifacts_pending"
  | "tool_approval_required"
  | "tool_approval_required_resolved"
  | "mcp_oauth_required"
  | "mcp_oauth_required_resolved"
  | "memory_recalled"
  | "steer"
  | "user_message_injected"
  | "context_compacted"
  | "install_prompt"
  | "unknown";

export const STREAM_RESPONSE_TYPES = [
  "answer", "references", "thinking", "tool_call", "tool_result", "error", "reflection",
  "session_title", "agent_query", "complete", "stop", "artifacts_pending",
  "tool_approval_required", "tool_approval_required_resolved", "mcp_oauth_required",
  "mcp_oauth_required_resolved", "memory_recalled", "steer", "user_message_injected",
  "context_compacted", "install_prompt",
] as const;
export const decodeStreamResponseType = enumOr(STREAM_RESPONSE_TYPES, "unknown");

export interface StreamChunkWire {
  id: string | null;
  response_type: StreamResponseType;
  content: string;
  done: boolean;
  session_id: string | null;
  assistant_message_id: string | null;
  tool_calls: unknown[] | null;
  data: Record<string, unknown> | null;
  finish_reason: string | null;
}

export const decodeStreamChunk: Decoder<StreamChunkWire> = (v) => {
  const r = (v ?? {}) as Record<string, unknown>;
  return {
    id: optStr()(r.id ?? null),
    response_type: decodeStreamResponseType(r.response_type ?? r.type),
    content: String(r.content ?? r.answer ?? ""),
    done: bool(false)(r.done),
    session_id: optStr()(r.session_id ?? null),
    assistant_message_id: optStr()(r.assistant_message_id ?? null),
    tool_calls: Array.isArray(r.tool_calls) ? r.tool_calls : null,
    data: (r.data ?? null) as Record<string, unknown> | null,
    finish_reason: optStr()(r.finish_reason ?? null),
  };
};

// ---- 用量（api-facts §9：/commercial/usage|summary）----
export interface UsageSummaryWire {
  as_of: string | null;
  period: string | null;
  available: number | null;
  held: number | null;
  settled: number | null;
  pending: number | null;
  unit: string;
}

export const decodeUsageSummary: Decoder<UsageSummaryWire> = (v) => {
  const r = (v ?? {}) as Record<string, unknown>;
  return {
    as_of: optStr()(r.as_of ?? null),
    period: optStr()(r.period ?? r.period_label ?? null),
    available: optNum()(r.available ?? r.remaining ?? null),
    held: optNum()(r.held ?? r.reserved ?? null),
    settled: optNum()(r.settled ?? r.used ?? null),
    pending: optNum()(r.pending ?? null),
    unit: String(r.unit ?? "credits"),
  };
};

// ---- 执行目标（api-facts §5）----
export interface ExecutionTargetWire {
  id: string;
  name: string;
  status: string;
  capabilities: string[];
  unavailable_reason: string | null;
}

export const decodeExecutionTarget: Decoder<ExecutionTargetWire> = (v) => {
  const r = (v ?? {}) as Record<string, unknown>;
  return {
    id: String(r.id ?? ""),
    name: String(r.name ?? r.label ?? ""),
    status: String(r.status ?? "unknown"),
    capabilities: Array.isArray(r.capabilities) ? r.capabilities.map(String) : [],
    unavailable_reason: optStr()(r.unavailable_reason ?? r.reason ?? null),
  };
};

export const decodeExecutionTargetList: Decoder<ExecutionTargetWire[]> = (v) => {
  const list = Array.isArray(v)
    ? v
    : ((v as Record<string, unknown>)?.targets as unknown[]) ?? ((v as Record<string, unknown>)?.items as unknown[]) ?? [];
  return arr(decodeExecutionTarget)(list);
};

// ---- 知识库 / 连接（M11-M13）----
export interface KnowledgeBaseWire {
  id: string;
  name: string;
  document_count: number | null;
  status: string;
  updated_at: string | null;
}

export const decodeKnowledgeBase: Decoder<KnowledgeBaseWire> = (v) => {
  const r = (v ?? {}) as Record<string, unknown>;
  return {
    id: String(r.id ?? ""),
    name: String(r.name ?? ""),
    document_count: optNum()(r.document_count ?? r.doc_count ?? null),
    status: String(r.status ?? "unknown"),
    updated_at: optStr()(r.updated_at ?? null),
  };
};

export const decodeKnowledgeBaseList: Decoder<KnowledgeBaseWire[]> = (v) => {
  const list = Array.isArray(v)
    ? v
    : ((v as Record<string, unknown>)?.knowledge_bases as unknown[]) ?? ((v as Record<string, unknown>)?.items as unknown[]) ?? [];
  return arr(decodeKnowledgeBase)(list);
};

export interface ConnectionWire {
  id: string;
  provider: string;
  owner_scope: "personal" | "tenant" | "unknown";
  status: string;
  scopes: string[];
  account_label: string | null;
}

export const decodeConnection: Decoder<ConnectionWire> = (v) => {
  const r = (v ?? {}) as Record<string, unknown>;
  return {
    id: String(r.id ?? ""),
    provider: String(r.provider ?? r.app_id ?? ""),
    owner_scope: enumOr(["personal", "tenant"] as const, "unknown")(r.owner_scope ?? r.scope),
    status: String(r.status ?? "unknown"),
    scopes: Array.isArray(r.scopes) ? r.scopes.map(String) : [],
    account_label: optStr()(r.account_label ?? r.account ?? null),
  };
};

export const decodeConnectionList: Decoder<ConnectionWire[]> = (v) => {
  const list = Array.isArray(v)
    ? v
    : ((v as Record<string, unknown>)?.connections as unknown[]) ?? ((v as Record<string, unknown>)?.installations as unknown[]) ?? [];
  return arr(decodeConnection)(list);
};

// ---- 成果 artifact ----
export interface ArtifactWire {
  id: string;
  name: string;
  mime: string;
  version: string;
  size: number | null;
  source_run: string | null;
  created_at: string | null;
  /** 会话级序号：签名下载端点的寻址键（后端 /workbench artifacts 列表下发） */
  index: number | null;
}

export const decodeArtifact: Decoder<ArtifactWire> = (v) => {
  const r = (v ?? {}) as Record<string, unknown>;
  return {
    id: String(r.id ?? r.artifact_id ?? ""),
    name: String(r.name ?? r.filename ?? ""),
    mime: String(r.mime ?? r.mime_type ?? "application/octet-stream"),
    version: String(r.version ?? "1"),
    size: optNum()(r.size ?? null),
    source_run: optStr()(r.source_run ?? r.run_id ?? null),
    created_at: optStr()(r.created_at ?? null),
    index: optNum()(r.index ?? null),
  };
};

export const decodeArtifactList: Decoder<ArtifactWire[]> = (v) => {
  const list = Array.isArray(v)
    ? v
    : ((v as Record<string, unknown>)?.artifacts as unknown[]) ?? ((v as Record<string, unknown>)?.items as unknown[]) ?? [];
  return arr(decodeArtifact)(list);
};

// ---- 成果签名下载链接（后端最小补充：HMAC 短时效 grant）----
export interface ArtifactSignedUrlWire {
  url: string;
  expires_at: string | null;
}

export const decodeArtifactSignedUrl: Decoder<ArtifactSignedUrlWire> = (v) => {
  // HttpClient 已解 data 包裹；此处直接读字段并容忍缺失（缺失 url 视为能力不可用，不冒充）
  const r = (v ?? {}) as Record<string, unknown>;
  return {
    url: String(r.url ?? ""),
    expires_at: optStr()(r.expires_at ?? null),
  };
};
