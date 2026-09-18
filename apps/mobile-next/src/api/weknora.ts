// WeKnora 端点封装（RW-006）：真实路径与 wire 解码（api-facts.md）。
import { HttpClient, type RequestInit2 } from "./http";
import {
  decodeArtifactList,
  decodeConnectionList,
  decodeExecutionTargetList,
  decodeInteraction,
  decodeInteractionList,
  decodeKnowledgeBaseList,
  decodeRunView,
  decodeSnapshot,
  decodeStartExecutionResponse,
  decodeUsageSummary,
  type ArtifactWire,
  type ConnectionWire,
  type ExecutionTargetWire,
  type InteractionWire,
  type KnowledgeBaseWire,
  type RunViewWire,
  type SnapshotWire,
  type StartExecutionResponseWire,
  type UsageSummaryWire,
} from "@/contracts/workbench";
import {
  decodeAgentList,
  decodeLoginResponse,
  decodeSessionList,
  decodeTenantList,
  decodeUserLoose,
  type AgentWire,
  type LoginResponseWire,
  type SessionListWire,
  type TenantWire,
  type UserWire,
} from "@/contracts/auth";
import { decodeTemporaryDocument, type TemporaryDocumentWire } from "@/contracts/attachments";

export interface StartExecutionInput {
  request_id: string;
  session_id: string;
  agent_id: string;
  target_id: string;
  workspace_ref: string;
  text: string;
  budget_upper: number;
}

export interface DecisionInput {
  pending_id: string;
  expected_revision: number;
  action: string; // approve/reject/extend/retry/provide_result/terminate
  args_hash?: string;
  resource_ref?: string;
  reason?: string;
  result?: unknown;
}

export class WeKnoraApi {
  constructor(private http: HttpClient) {}

  // ---- auth ----
  login(email: string, password: string, init?: RequestInit2): Promise<LoginResponseWire> {
    return this.http.request("/auth/login", { method: "POST", body: { email, password }, ...init }, decodeLoginResponse);
  }
  me(init?: RequestInit2): Promise<UserWire> {
    // 真实 /auth/me → {data:{user,memberships,tenant,capabilities}}（live-backend.mts 核验）
    return this.http.request("/auth/me", { ...init }, (v) => {
      const r = (v ?? {}) as Record<string, unknown>;
      return decodeUserLoose(r.user ?? v);
    });
  }
  logout(init?: RequestInit2): Promise<void> {
    return this.http.request("/auth/logout", { method: "POST", ...init });
  }
  tenants(init?: RequestInit2): Promise<TenantWire[]> {
    return this.http.request("/tenants", { ...init }, decodeTenantList);
  }
  switchTenant(tenantId: string, init?: RequestInit2): Promise<void> {
    return this.http.request("/auth/switch-tenant", { method: "POST", body: { tenant_id: tenantId }, ...init });
  }

  // ---- sessions ----
  sessions(params?: { page?: number; page_size?: number; keyword?: string }, init?: RequestInit2): Promise<SessionListWire> {
    const q = new URLSearchParams();
    if (params?.page) q.set("page", String(params.page));
    if (params?.page_size) q.set("page_size", String(params.page_size));
    if (params?.keyword) q.set("keyword", params.keyword);
    const qs = q.toString();
    return this.http.request(`/sessions${qs ? `?${qs}` : ""}`, { ...init }, decodeSessionList);
  }
  createSession(title: string, init?: RequestInit2): Promise<{ id: string }> {
    return this.http.request("/sessions", { method: "POST", body: { title }, ...init }, (v) => {
      const r = (v ?? {}) as Record<string, unknown>;
      return { id: String((r.id as string) ?? (r.session_id as string) ?? "") };
    });
  }

  // ---- 附件（临时文档；RW-027。真实 bytes multipart，字段名 file）----
  uploadAttachment(
    sessionId: string,
    file: { uri: string; name: string; mimeType: string },
    agentId?: string,
    signal?: AbortSignal,
  ): Promise<TemporaryDocumentWire> {
    const fd = new FormData();
    fd.append("file", { uri: file.uri, name: file.name, type: file.mimeType } as unknown as Blob);
    if (agentId) fd.append("agent_id", agentId);
    return this.http.uploadMultipart(
      `/sessions/${encodeURIComponent(sessionId)}/attachments`,
      fd,
      { signal },
      decodeTemporaryDocument,
    );
  }
  getAttachment(sessionId: string, attachmentId: string, signal?: AbortSignal): Promise<TemporaryDocumentWire> {
    return this.http.request(
      `/sessions/${encodeURIComponent(sessionId)}/attachments/${encodeURIComponent(attachmentId)}`,
      { signal },
      decodeTemporaryDocument,
    );
  }

  // ---- agents ----
  agents(init?: RequestInit2): Promise<AgentWire[]> {
    return this.http.request("/agents", { ...init }, decodeAgentList);
  }

  // ---- workbench executions ----
  startExecution(input: StartExecutionInput, init?: RequestInit2): Promise<StartExecutionResponseWire> {
    return this.http.request("/workbench/executions", { method: "POST", body: input, ...init }, decodeStartExecutionResponse);
  }
  getExecution(runId: string, init?: RequestInit2): Promise<RunViewWire> {
    return this.http.request(`/workbench/executions/${encodeURIComponent(runId)}`, { ...init }, decodeRunView);
  }
  lookupRequest(requestId: string, init?: RequestInit2): Promise<RunViewWire | null> {
    return this.http.request(
      `/workbench/executions/requests/${encodeURIComponent(requestId)}`,
      { ...init },
      (v) => (v == null ? null : decodeRunView(v)),
    );
  }
  snapshot(runId: string, init?: RequestInit2): Promise<SnapshotWire> {
    return this.http.request(`/workbench/executions/${encodeURIComponent(runId)}/snapshot`, { ...init }, decodeSnapshot);
  }
  interactions(runId: string, init?: RequestInit2): Promise<InteractionWire[]> {
    return this.http.request(`/workbench/executions/${encodeURIComponent(runId)}/interactions`, { ...init }, decodeInteractionList);
  }
  decide(interactionId: string, input: DecisionInput, init?: RequestInit2): Promise<InteractionWire> {
    return this.http.request(
      `/workbench/executions/interactions/${encodeURIComponent(interactionId)}/decisions`,
      { method: "POST", body: input, ...init },
      decodeInteraction,
    );
  }
  command(runId: string, action: "cancel" | "steer", expectedRevision: number, text?: string, init?: RequestInit2): Promise<RunViewWire> {
    const body =
      action === "cancel"
        ? { action, expected_revision: expectedRevision }
        : { action, text: text ?? "", expected_revision: expectedRevision };
    return this.http.request(
      `/workbench/executions/${encodeURIComponent(runId)}/commands`,
      { method: "POST", body, ...init },
      decodeRunView,
    );
  }

  // ---- targets ----
  executionTargets(init?: RequestInit2): Promise<ExecutionTargetWire[]> {
    return this.http.request("/execution-targets", { ...init }, decodeExecutionTargetList);
  }

  // ---- resources ----
  knowledgeBases(init?: RequestInit2): Promise<KnowledgeBaseWire[]> {
    return this.http.request("/knowledge-bases", { ...init }, decodeKnowledgeBaseList);
  }
  connections(init?: RequestInit2): Promise<ConnectionWire[]> {
    return this.http.request("/apps/connections", { ...init }, decodeConnectionList);
  }
  artifacts(runId: string, init?: RequestInit2): Promise<ArtifactWire[]> {
    return this.http.request(`/workbench/executions/${encodeURIComponent(runId)}/artifacts`, { ...init }, decodeArtifactList);
  }

  // ---- usage ----
  usageSummary(init?: RequestInit2): Promise<UsageSummaryWire> {
    return this.http.request("/commercial/usage", { ...init }, decodeUsageSummary);
  }
  commercialSummary(init?: RequestInit2): Promise<UsageSummaryWire> {
    return this.http.request("/commercial/summary", { ...init }, decodeUsageSummary);
  }
}
