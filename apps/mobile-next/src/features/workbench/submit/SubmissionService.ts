// 提交状态机（RW-013，详细设计 §4.2）：
// draft → prepared（request_id + canonical hash 先落盘，落盘失败不发送）
//      → sending → accepted / rejected / uncertain
// uncertain → lookup 原 request_id 对账，不自动新建 ID 重执行。
// 离线仅保存草稿；网络恢复不自动补发危险指令。
import { ApiError } from "@/api/http";
import type { WeKnoraApi, StartExecutionInput } from "@/api/weknora";
import type { MobileStore } from "@/platform/store";

export type SubmissionState = "draft" | "prepared" | "sending" | "uncertain" | "accepted" | "rejected";

export interface SubmissionDraft {
  text: string;
  agentId: string;
  targetId: string;
  workspaceRef: string;
  budgetUpper: number;
  knowledgeBaseIds: string[];
  sessionId: string | null;
}

export type SubmitOutcome =
  | { kind: "accepted"; requestId: string; runId: string; status: string }
  | { kind: "rejected"; requestId: string; reason: string; kindDetail: string }
  | { kind: "uncertain"; requestId: string; note: string };

/** 规范输入摘要：版本化、有序、确定性（数组不任意排序——按用户选择顺序） */
export function canonicalInput(scopeKey: string, draft: SubmissionDraft): string {
  return JSON.stringify({
    v: 1,
    scope: scopeKey,
    session_id: draft.sessionId,
    agent_id: draft.agentId,
    target_id: draft.targetId,
    workspace_ref: draft.workspaceRef,
    text: draft.text,
    budget_upper: draft.budgetUpper,
    knowledge_base_ids: draft.knowledgeBaseIds.slice(),
  });
}

/** 简单 FNV-1a 摘要（足够检测输入漂移；非安全哈希） */
export function fnv1a(s: string): string {
  let h = 0x811c9dc5;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 0x01000193) >>> 0;
  }
  return h.toString(16).padStart(8, "0") + "-" + s.length.toString(16);
}

export interface SubmissionDeps {
  api: WeKnoraApi;
  store: MobileStore;
  scopeKey(): string;
  canWrite(): boolean;
  newRequestId(): string;
  /** ACK 不明时对账原请求 */
  lookup(requestId: string): Promise<{ runId: string | null; status: string | null }>;
}

export class SubmissionService {
  constructor(private d: SubmissionDeps) {}

  /** 校验草稿可提交（描述非空、预算正整数、Agent/目标已选） */
  validate(draft: SubmissionDraft): string | null {
    if (!draft.text.trim()) return "请描述你的任务目标";
    if (!draft.agentId) return "请选择 Agent";
    if (!draft.targetId) return "请选择执行目标";
    if (!Number.isInteger(draft.budgetUpper) || draft.budgetUpper <= 0) return "预算需为正整数";
    return null;
  }

  /**
   * 提交一次意图。强制顺序：prepared 落盘成功 → 才发网络。
   * 返回 outcome；uncertain 时调用方提供"核实原请求"，不重发。
   */
  async submit(draft: SubmissionDraft): Promise<SubmitOutcome> {
    const invalid = this.validate(draft);
    if (invalid) return { kind: "rejected", requestId: "", reason: invalid, kindDetail: "validation" };
    if (!this.d.canWrite()) {
      return { kind: "rejected", requestId: "", reason: "当前空间切换中或只读，草稿已保留", kindDetail: "scope_readonly" };
    }
    const scopeKey = this.d.scopeKey();
    const requestId = this.d.newRequestId();
    const input = canonicalInput(scopeKey, draft);
    const inputHash = fnv1a(input);

    // prepared：先持久化；存储失败不得发送
    try {
      await this.d.store.savePending({
        scopeKey,
        requestId,
        inputHash,
        input,
        state: "prepared",
        runId: null,
        createdAt: new Date().toISOString(),
      });
    } catch (e) {
      return { kind: "rejected", requestId, reason: `本地保存失败，未发送：${(e as Error).message}`, kindDetail: "storage" };
    }

    // sending：一次 HTTP
    await this.d.store.savePending({
      scopeKey,
      requestId,
      inputHash,
      input,
      state: "sending",
      runId: null,
      createdAt: new Date().toISOString(),
    });

    const body: StartExecutionInput = {
      request_id: requestId,
      session_id: draft.sessionId ?? "",
      agent_id: draft.agentId,
      target_id: draft.targetId,
      workspace_ref: draft.workspaceRef,
      text: draft.text,
      budget_upper: draft.budgetUpper,
    };

    try {
      const res = await this.d.api.startExecution(body);
      await this.d.store.savePending({
        scopeKey,
        requestId,
        inputHash,
        input,
        state: "accepted",
        runId: res.run_id,
        createdAt: new Date().toISOString(),
      });
      return { kind: "accepted", requestId, runId: res.run_id, status: res.status };
    } catch (e) {
      if (e instanceof ApiError && (e.kind === "network" || e.kind === "aborted")) {
        // ACK 丢失：可能已被受理 → uncertain，对账原请求
        await this.d.store.savePending({
          scopeKey,
          requestId,
          inputHash,
          input,
          state: "uncertain",
          runId: null,
          createdAt: new Date().toISOString(),
        });
        return { kind: "uncertain", requestId, note: "正在核实原请求，请勿重复提交" };
      }
      await this.d.store.savePending({
        scopeKey,
        requestId,
        inputHash,
        input,
        state: "rejected",
        runId: null,
        createdAt: new Date().toISOString(),
      });
      const err = e as ApiError;
      return {
        kind: "rejected",
        requestId,
        reason: err.message,
        kindDetail: err.kind,
      };
    }
  }

  /** uncertain 对账：lookup 原 request_id；unknown 不是失败 */
  async reconcile(requestId: string): Promise<SubmitOutcome> {
    const scopeKey = this.d.scopeKey();
    const pending = await this.d.store.readPending(scopeKey, requestId);
    if (!pending) return { kind: "rejected", requestId, reason: "找不到原请求记录", kindDetail: "missing" };
    const found = await this.d.lookup(requestId);
    if (found.runId) {
      await this.d.store.savePending({ ...pending, state: "accepted", runId: found.runId });
      return { kind: "accepted", requestId, runId: found.runId, status: found.status ?? "unknown" };
    }
    // 仍未受理：保持 uncertain，允许用户稍后再查或明确放弃
    return { kind: "uncertain", requestId, note: "原请求暂未找到受理记录" };
  }

  /** 重启恢复：列出未决提交（uncertain/sending）待对账 */
  async pendingReconcile(): Promise<string[]> {
    const list = await this.d.store.listPending(this.d.scopeKey(), ["uncertain", "sending", "prepared"]);
    return list.map((p) => p.requestId);
  }

  /** 相同 request_id 不同输入必须冲突 */
  async assertInputConsistent(requestId: string, draft: SubmissionDraft): Promise<boolean> {
    const scopeKey = this.d.scopeKey();
    const pending = await this.d.store.readPending(scopeKey, requestId);
    if (!pending) return true;
    return pending.inputHash === fnv1a(canonicalInput(scopeKey, draft));
  }
}

/** request_id 生成：非安全敏感，时间戳+随机足够唯一；native 装配可注入 expo-crypto 版本 */
export const newRequestId = (): string =>
  `req_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 10)}`;
