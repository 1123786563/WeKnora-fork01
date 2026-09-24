import type { AttentionState } from './task-office-errors.ts';

/**
 * T14（Issue #44）Legacy Task 域合同：从未有过 Run 的旧 Session 以同一身份
 * （taskId = sessionId，ADR-0004）呈现。卡片只携带历史能证明的事实；Run 级
 * 新安全语义（Run 指令/审批/预算/Agent Version）由显式门禁标记为需新建 Run，
 * 门禁在模块内派生（纯函数），不来自 wire——服务端 legacy 行本来就不携带
 * 这些字段（Go 侧键集锁定）。
 */

export type LegacyTaskIntent = 'follow-up' | 'run-command' | 'decision' | 'budget' | 'agent-version';

export interface LegacyTaskCapability {
  state: 'supported' | 'unavailable';
  reason: string;
}

export type LegacyTaskGates = Record<LegacyTaskIntent, LegacyTaskCapability>;

export const LEGACY_TASK_NEW_RUN_REASON = 'legacy task has no Run; a new Run admission is required';

/** 显式升级门禁（AC2）：普通追问 supported；四类新安全语义意图一律 unavailable。 */
export function legacyTaskGates(): LegacyTaskGates {
  return {
    'follow-up': { state: 'supported', reason: 'ordinary follow-up keeps the existing chat semantics' },
    'run-command': { state: 'unavailable', reason: LEGACY_TASK_NEW_RUN_REASON },
    'decision': { state: 'unavailable', reason: LEGACY_TASK_NEW_RUN_REASON },
    'budget': { state: 'unavailable', reason: LEGACY_TASK_NEW_RUN_REASON },
    'agent-version': { state: 'unavailable', reason: LEGACY_TASK_NEW_RUN_REASON },
  };
}

/** 后端行（wire 语义行，attention 恒 'none'——无 Run 即无可证明关注来源）。 */
export interface LegacyBackendTask {
  taskId: string;
  title: string;
  attention: 'none';
  archivedAt?: string;
  updatedAt: string;
}

export interface LegacyTaskBackendPage {
  items: LegacyBackendTask[];
  nextCursor?: string;
}

export interface LegacyMessage {
  messageId: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  createdAt?: string;
}

export interface LegacyFollowUpInput {
  taskId: string;
  question: string;
  signal?: AbortSignal;
}

export interface LegacyTaskBackendPort {
  list(input: { search?: string; archived?: boolean; cursor?: string; limit?: number }): Promise<LegacyTaskBackendPage>;
  history(taskId: string): Promise<LegacyMessage[]>;
  followUp(input: LegacyFollowUpInput): Promise<void>;
}

/** Scriptable scenario Adapter（module-seams §12：remote-owned 依赖的 in-memory 场景）。 */
export interface ScenarioLegacyTaskBackendHandlers {
  list?: (input: { search?: string; archived?: boolean; cursor?: string; limit?: number }) => Promise<LegacyTaskBackendPage>;
  history?: (taskId: string) => Promise<LegacyMessage[]>;
  followUp?: (input: LegacyFollowUpInput) => Promise<void>;
}

export interface ScenarioLegacyTaskBackend extends LegacyTaskBackendPort {
  calls: Array<
    | { kind: 'list'; input: { search?: string; archived?: boolean; cursor?: string; limit?: number } }
    | { kind: 'history'; taskId: string }
    | { kind: 'followUp'; input: LegacyFollowUpInput }
  >;
}

export function createScenarioLegacyTaskBackend(handlers: ScenarioLegacyTaskBackendHandlers = {}): ScenarioLegacyTaskBackend {
  const calls: ScenarioLegacyTaskBackend['calls'] = [];
  const scenario: ScenarioLegacyTaskBackend = {
    calls,
    async list(input) {
      calls.push({ kind: 'list', input });
      return handlers.list ? handlers.list(input) : { items: [] };
    },
    async history(taskId) {
      calls.push({ kind: 'history', taskId });
      return handlers.history ? handlers.history(taskId) : [];
    },
    async followUp(input) {
      calls.push({ kind: 'followUp', input });
      await handlers.followUp?.(input);
    },
  };
  return scenario;
}

/** 展示卡片：后端行 + 模块内门禁（gates 不来自 wire）。 */
export interface LegacyTaskCard extends LegacyBackendTask {
  kind: 'legacy';
  gates: LegacyTaskGates;
}

export interface LegacyTaskListPage {
  items: LegacyTaskCard[];
  nextCursor?: string;
  duplicateTaskIds: string[];
}

export type { AttentionState };
