import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { TaskOfficeError } from './task-office-errors.ts';

/**
 * Attention Inbox 域（T08，module-seams §5.1「Home、Task list、Attention inbox
 * 的一致读投影……Interaction decision」）：kind×action 冻结矩阵镜像 Go
 * ValidateInteractionAction（internal/modules/workbench/interaction.go:38-50），
 * decision_id 在模块内冻结——同一 item+action 的重复提交与重试重放同一 durable
 * 身份（AC1）；receipt 如实区分 recorded / delivery-unknown / superseded / gone，
 * recorded 只表示决定已记录，绝不解释为外部派发完成（AC2）。
 * mobile-core 不依赖 api-client：INTERACTION_* 是跨包契约码字符串（沿
 * TASK_STREAM_CURSOR_EXPIRED 先例）。
 */
export type InteractionKindValue = 'tool_approval' | 'budget' | 'recovery';
export type InteractionActionValue = 'approve' | 'reject' | 'extend' | 'retry' | 'provide_result' | 'terminate';

export const INTERACTION_ACTIONS: Readonly<Record<InteractionKindValue, readonly InteractionActionValue[]>> = {
  tool_approval: ['approve', 'reject'],
  budget: ['extend'],
  recovery: ['retry', 'provide_result', 'terminate'],
};

export function interactionActionAllowed(kind: InteractionKindValue, action: string): boolean {
  return (INTERACTION_ACTIONS[kind] as readonly string[]).includes(action);
}

export interface InboxItem {
  interactionId: string;
  runId: string;
  kind: InteractionKindValue;
  argsHash: string;
  expectedRevision: number;
  createdAt: string;
}

export interface ResolvedDecisionRecord {
  interactionId: string;
  runId: string;
  kind: InteractionKindValue;
  decisionId: string;
  action: InteractionActionValue;
  argsHash: string;
  expectedRevision: number;
}

export type AttentionDecisionReceipt =
  | { status: 'recorded'; record: ResolvedDecisionRecord }
  | { status: 'delivery-unknown'; interactionId: string; decisionId: string }
  | { status: 'superseded'; interactionId: string }
  | { status: 'gone'; interactionId: string };

export interface InboxView {
  items: InboxItem[];
}

export interface AttentionDecisionInput {
  item: InboxItem;
  action: InteractionActionValue;
}

export interface InteractionBackendPort {
  inbox(input: { limit: number }): Promise<InboxItem[]>;
  decide(input: { item: InboxItem; decisionId: string; action: InteractionActionValue }): Promise<ResolvedDecisionRecord>;
}

/** 跨包契约码（api-client task-office.ts 的 decide 分类写入 error.code）。 */
export const INTERACTION_SUPERSEDED = 'INTERACTION_SUPERSEDED';
export const INTERACTION_DELIVERY_UNKNOWN = 'INTERACTION_DELIVERY_UNKNOWN';
export const INTERACTION_GONE = 'INTERACTION_GONE';

function errorCode(error: unknown): string | undefined {
  if (error instanceof Error && 'code' in error) {
    const code = (error as { code?: unknown }).code;
    return typeof code === 'string' ? code : undefined;
  }
  return undefined;
}

let decisionCounter = 0;

/** decision_id 是幂等身份而非机密：优先 crypto.randomUUID，降级为单调计数+随机后缀。 */
export function newDecisionId(): string {
  const cryptoApi = (globalThis as { crypto?: { randomUUID?: () => string } }).crypto;
  if (cryptoApi?.randomUUID) return cryptoApi.randomUUID();
  decisionCounter += 1;
  return `dec-${Date.now().toString(36)}-${decisionCounter.toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

export interface AttentionDeciderDeps {
  interactions(): InteractionBackendPort | undefined;
  lease(): ScopeLease | undefined;
  /** 决定到达终态（recorded/delivery-unknown/superseded/gone 均已落地或失效）后回调：宿主失效 home/inbox 读。 */
  onDecided(): void;
}

export function createAttentionDecider(deps: AttentionDeciderDeps): {
  inbox(): Promise<InboxView>;
  decide(input: AttentionDecisionInput): Promise<AttentionDecisionReceipt>;
} {
  let inboxEpoch = 0;
  const frozenDecisionIds = new Map<string, string>();
  const requireLease = (): ScopeLease => {
    const lease = deps.lease();
    if (!lease || !leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    return lease;
  };
  const requireInteractions = (): InteractionBackendPort => {
    const backend = deps.interactions();
    if (backend === undefined) throw new TaskOfficeError('TASK_OFFICE_INTERACTIONS_UNAVAILABLE');
    return backend;
  };
  return {
    async inbox(): Promise<InboxView> {
      const backend = requireInteractions();
      const lease = requireLease();
      const epoch = ++inboxEpoch;
      let items: InboxItem[];
      try {
        items = await backend.inbox({ limit: 50 });
      } catch (error) {
        if (error instanceof TaskOfficeError) throw error;
        throw new TaskOfficeError('TASK_OFFICE_BACKEND', { cause: error });
      }
      if (epoch !== inboxEpoch) throw new TaskOfficeError('TASK_OFFICE_SUPERSEDED');
      if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
      return { items };
    },
    async decide(input: AttentionDecisionInput): Promise<AttentionDecisionReceipt> {
      const backend = requireInteractions();
      const lease = requireLease();
      if (!interactionActionAllowed(input.item.kind, input.action)) {
        throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
      }
      const key = `${input.item.interactionId}::${input.action}`;
      const decisionId = frozenDecisionIds.get(key) ?? newDecisionId();
      frozenDecisionIds.set(key, decisionId);
      let record: ResolvedDecisionRecord;
      try {
        record = await backend.decide({ item: input.item, decisionId, action: input.action });
      } catch (error) {
        const code = errorCode(error);
        if (code === INTERACTION_DELIVERY_UNKNOWN) {
          // 决定已落地、外部派发未知：不冒充成功，保留同一 decision_id 供重试重放。
          deps.onDecided();
          return { status: 'delivery-unknown', interactionId: input.item.interactionId, decisionId };
        }
        if (code === INTERACTION_SUPERSEDED) {
          deps.onDecided();
          return { status: 'superseded', interactionId: input.item.interactionId };
        }
        if (code === INTERACTION_GONE) {
          deps.onDecided();
          return { status: 'gone', interactionId: input.item.interactionId };
        }
        throw error instanceof TaskOfficeError ? error : new TaskOfficeError('TASK_OFFICE_BACKEND', { cause: error });
      }
      // 与 mutate() 同一守卫：写落地后 scope 已撤销 → SCOPE_CHANGED（receipt 不跨 scope 泄漏）。
      if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
      deps.onDecided();
      return { status: 'recorded', record };
    },
  };
}
