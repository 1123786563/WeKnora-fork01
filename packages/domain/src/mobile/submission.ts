/** 结构化 7 字段（与 api-client StartExecutionInput / 服务端 Start body 结构兼容；domain 不依赖 api-client）。 */
export interface MobileStartInput {
  request_id: string;
  session_id: string;
  agent_id: string;
  target_id: string;
  workspace_ref: string;
  text: string;
  budget_upper: number;
}

/** 结构化对账结果（与 api-client RequestLookup 兼容）。 */
export interface MobileRequestLookup {
  state: 'pending' | 'dispatching' | 'admitted' | 'rejected' | 'unknown';
  run_id?: string;
  reason?: string;
}

/**
 * 持久提交身份（MX-006，关闭 G04 客户端面）。
 * 规则（D-018）：
 * - 一次用户意图一个 request_id；网络发送前必须先落盘（request_id+输入摘要+scope）；
 * - ACK 丢失/杀进程后重启：同一 store 中恢复 entry，用 lookup 对账，绝不换 ID 重建任务；
 * - 相同 request_id 不同输入 → 本地冲突（零网络请求）；
 * - lookup unknown/pending 不自动重新 POST、不新建预算预占；
 * - rejected-before-dispatch 才允许受控重试，且必须新 request_id + 用户确认。
 */

export type SubmissionPhase =
  | 'awaiting_ack'        // 已落盘、已发出（或即将发出），等待 ACK
  | 'awaiting_reconciliation' // ACK 丢失/超时：对账中
  | 'bound'               // 已绑定 run_id
  | 'rejected';           // 服务端明确拒绝（可受控重试=新 ID）

export interface SubmissionScope {
  origin: string;
  tenantID: string;
  userID: string;
}

export interface SubmissionEntry {
  request_id: string;
  input_digest: string;
  scope: SubmissionScope;
  phase: SubmissionPhase;
  run_id?: string;
  /** 输入摘要来源的原文引用由调用方持有；store 只保存摘要（最小化敏感落盘）。 */
  updated_at: string;
}

/** 提交存储端口：实现必须先于网络调用完成写入（磁盘满/失败 → submit 抛错且不发送）。 */
export interface SubmissionStore {
  load(requestId: string): SubmissionEntry | undefined;
  save(entry: SubmissionEntry): void;
  listScope(scope: SubmissionScope): SubmissionEntry[];
}

export class SubmissionConflictError extends Error {
  constructor(public readonly requestId: string, public readonly storedDigest: string, public readonly incomingDigest: string) {
    super(`request_id ${requestId} already persisted with different input`);
  }
}

/** 规范化输入摘要：稳定键序 + 七字段全集（与服务端 requestHash 语义对齐：规范化输入变更即冲突）。 */
export function inputDigest(input: MobileStartInput): string {
  const canonical = {
    agent_id: input.agent_id,
    budget_upper: input.budget_upper,
    request_id: input.request_id,
    session_id: input.session_id,
    target_id: input.target_id,
    text: input.text,
    workspace_ref: input.workspace_ref,
  };
  const json = JSON.stringify(canonical);
  let h1 = 0x811c9dc5;
  let h2 = 0x01000193;
  for (let i = 0; i < json.length; i++) {
    h1 = (h1 ^ json.charCodeAt(i)) * 0x01000193 >>> 0;
    h2 = (h2 + json.charCodeAt(i) * (i + 1)) >>> 0;
  }
  return `fnv1a:${h1.toString(16)}${h2.toString(16)}:${json.length}`;
}

export interface SubmissionTransport {
  /** 发送 Start；transport 层负责超时/网络错误（调用方绝不重发同一 request_id）。 */
  start(input: MobileStartInput): Promise<{ run_id: string; request_id: string; status: string }>;
  /** request 对账：pending/dispatching/admitted/rejected/unknown。 */
  lookup(requestId: string): Promise<MobileRequestLookup>;
}

export interface SubmitOutcome {
  entry: SubmissionEntry;
  /** 网络发送是否发生（重放/恢复场景为 false——不产生第二次 POST）。 */
  dispatched: boolean;
}

function sameScope(a: SubmissionScope, b: SubmissionScope): boolean {
  return a.origin === b.origin && a.tenantID === b.tenantID && a.userID === b.userID;
}

export function createSubmissionCoordinator(store: SubmissionStore, transport: SubmissionTransport) {
  return {
    /**
     * 提交一次用户意图。幂等入口：
     * - 未见过该 request_id：落盘 awaiting_ack → 发送 → 绑定 run（ack.request_id 必须回传一致，否则视为协议错误置对账态）。
     * - 已见过且摘要一致：不重发；按当前 phase 恢复（awaiting_* 走 reconcile；bound 直接返回）。
     * - 已见过且摘要不同：SubmissionConflictError，零网络。
     */
    async submit(input: MobileStartInput, scope: SubmissionScope): Promise<SubmitOutcome> {
      const digest = inputDigest(input);
      const existing = store.load(input.request_id);
      if (existing) {
        if (!sameScope(existing.scope, scope)) {
          throw new SubmissionConflictError(input.request_id, existing.input_digest, `${digest} (scope mismatch)`);
        }
        if (existing.input_digest !== digest) {
          throw new SubmissionConflictError(input.request_id, existing.input_digest, digest);
        }
        if (existing.phase === 'bound' && existing.run_id) {
          return { entry: existing, dispatched: false };
        }
        const reconciled = await this.reconcile(input.request_id, scope);
        return { entry: reconciled, dispatched: false };
      }
      const now = new Date().toISOString();
      // 网络前持久化：save 失败（含底层磁盘错误）必须在上抛后且不发送
      store.save({ request_id: input.request_id, input_digest: digest, scope, phase: 'awaiting_ack', updated_at: now });
      try {
        const ack = await transport.start(input);
        if (ack.request_id !== input.request_id) {
          const entry: SubmissionEntry = { request_id: input.request_id, input_digest: digest, scope, phase: 'awaiting_reconciliation', updated_at: new Date().toISOString() };
          store.save(entry);
          return { entry, dispatched: true };
        }
        const entry: SubmissionEntry = { request_id: input.request_id, input_digest: digest, scope, phase: 'bound', run_id: ack.run_id, updated_at: new Date().toISOString() };
        store.save(entry);
        return { entry, dispatched: true };
      } catch {
        // ACK 丢失/网络失败：保持同一 request_id 进入对账态；绝不换 ID、绝不静默重试
        const entry: SubmissionEntry = { request_id: input.request_id, input_digest: digest, scope, phase: 'awaiting_reconciliation', updated_at: new Date().toISOString() };
        store.save(entry);
        return { entry, dispatched: true };
      }
    },

    /** 未知启动对账：lookup 原请求。unknown/pending/dispatching 维持对账态（不重发）。 */
    async reconcile(requestId: string, scope: SubmissionScope): Promise<SubmissionEntry> {
      const existing = store.load(requestId);
      if (!existing || !sameScope(existing.scope, scope)) {
        throw new Error(`no persisted submission ${requestId} for this scope`);
      }
      const lookup = await transport.lookup(requestId);
      let next: SubmissionEntry;
      if (lookup.state === 'admitted' && lookup.run_id) {
        next = { ...existing, phase: 'bound', run_id: lookup.run_id, updated_at: new Date().toISOString() };
      } else if (lookup.state === 'rejected') {
        next = { ...existing, phase: 'rejected', updated_at: new Date().toISOString() };
      } else {
        next = { ...existing, phase: 'awaiting_reconciliation', updated_at: new Date().toISOString() };
      }
      store.save(next);
      return next;
    },

    /** 受控重试：仅 rejected 允许；新意图=新 request_id（由调用方生成并经用户确认）。 */
    retryEntry(requestId: string): SubmissionEntry {
      const existing = store.load(requestId);
      if (!existing || existing.phase !== 'rejected') {
        throw new Error('controlled retry requires an explicitly rejected submission');
      }
      return existing;
    },
  };
}

/** 内存实现（测试与开发用）；产品路径使用原生持久化适配器（MMKV/SQLite）。 */
export function createInMemorySubmissionStore(): SubmissionStore {
  const map = new Map<string, SubmissionEntry>();
  return {
    load: (id) => map.get(id),
    save: (entry) => { map.set(entry.request_id, entry); },
    listScope: (scope) => [...map.values()].filter((entry) => sameScope(entry.scope, scope)),
  };
}
