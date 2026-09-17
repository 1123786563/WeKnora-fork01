import {
  evaluateCommand,
  type ExecutionDTO,
  type ExecutionEvent,
  type InteractionRecord,
} from '@weknora/contracts';
import type { ProductScope } from '../platform/product-session';

export interface ExecutionApi {
  start(input: { request_id: string; session_id: string; agent_id: string; target_id: string; workspace_ref: string; text: string; budget_upper: number }): Promise<unknown>;
  lookup(requestID: string): Promise<unknown>;
  command(runID: string, input: { action: 'cancel'; expected_revision: number } | { action: 'steer'; text: string; expected_revision: number }): Promise<unknown>;
}

export interface ConversationScope {
  origin: string;
  userId: string | null;
  tenantId: string | null;
  spaceId: string | null;
}

export type ConversationMessageRole = 'user' | 'assistant' | 'tool' | 'system';

export interface ConversationMessage {
  id: string;
  role: ConversationMessageRole;
  text: string;
  agentID?: string;
  createdAt?: string;
}

export type PendingInteractionStatus = 'pending' | 'approved' | 'rejected' | 'expired';

export interface PendingInteraction {
  id: string;
  kind: 'approval' | 'question' | 'permission' | 'unknown';
  status: PendingInteractionStatus;
  label: string;
  reason?: string;
}

export interface ConversationCapabilities {
  canCancel: boolean;
  canSteer: boolean;
  canAttach: boolean;
  canVoice: boolean;
}

export interface ConversationCommands {
  cancel(runID: string, expectedRevision?: number): Promise<void>;
  steer(runID: string, text: string, expectedRevision?: number): Promise<void>;
  refreshPending?(interactionID: string): Promise<void>;
}

export interface ConversationExecution {
  runID: string;
  requestID: string;
  status: string;
  revision?: number;
  reason?: string;
}

export interface ConversationViewModel {
  scope: ConversationScope;
  messages: ConversationMessage[];
  pendingInteractions: PendingInteraction[];
  capabilities: ConversationCapabilities;
  execution: ConversationExecution | null;
  commands: ConversationCommands;
  send?: SendController;
}


export interface ConversationViewModelInput {
  scope: ConversationScope;
  messages?: ConversationMessage[];
  pendingInteractions?: PendingInteraction[];
  capabilities?: Partial<ConversationCapabilities>;
  execution?: ExecutionDTO | { run_id: string; request_id: string; status: string; revision?: number; reason?: string } | null;
  commands: ConversationCommands;
  send?: SendController;
}

export interface ProductAgentOption { id: string; name: string; }

// ─────────────────────────────────────────────────────────────────────────────
// 产品会话投影（MX-017）：事件驱动、零 Happy 依赖。
// - 同一 (run_id, seq) 事件只应用一次（重复投递/重放不产生重复消息）；
// - 消息稳定 id：text.delta 以 payload.message_id 为准（缺失回退 seq）；
// - 三态分离（run/execution/settlement，G08 消费）；
// - 命令准入消费 evaluateCommand（D-013，G03 消费），不再硬编码布尔；
// - pendingInteractions 来自真实 interactions 列表（kind 语义映射）。
// ─────────────────────────────────────────────────────────────────────────────

export interface ProductConversationState {
  messages: ConversationMessage[];
  runStatus: string;
  executionStatus: string;
  settlementStatus: string;
  revision: number;
  capabilities: ConversationCapabilities;
  pendingInteractions: PendingInteraction[];
}

export interface ProductConversationViewModel {
  readonly runID: string;
  state(): ProductConversationState;
  /** 幂等应用事件批次（同 seq 跳过）；返回应用后的只读快照。 */
  applyEvents(events: readonly ExecutionEvent[]): ProductConversationState;
  /** 服务端执行 DTO 刷新三态/能力/revision（与事件投影同源）。 */
  applyExecution(execution: ExecutionDTO): ProductConversationState;
  /** 真实交互列表 → pending 投影。 */
  applyInteractions(records: readonly InteractionRecord[]): ProductConversationState;
  commands: ConversationCommands;
}

const INTERACTION_KIND_MAP: Record<string, PendingInteraction['kind']> = {
  tool_approval: 'approval',
  budget: 'permission',
  recovery: 'question',
};

export function createProductConversationViewModel(input: {
  scope: ProductScope;
  spaceId: string | null;
  sessionId: string;
  runID: string;
  executions: ExecutionApi;
}): ProductConversationViewModel {
  const identity = input.scope.identity();
  let state: ProductConversationState = {
    messages: [],
    runStatus: 'queued',
    executionStatus: 'unknown',
    settlementStatus: 'unknown',
    revision: 0,
    capabilities: { canCancel: false, canSteer: false, canAttach: false, canVoice: false },
    pendingInteractions: [],
  };
  const appliedSeqs = new Set<number>();
  const messageOrder: string[] = [];
  const messagesById = new Map<string, ConversationMessage>();
  let execution: ExecutionDTO | null = null;

  function capabilitiesFromExecution(): ConversationCapabilities {
    if (!execution) return { canCancel: false, canSteer: false, canAttach: false, canVoice: false };
    return {
      canCancel: evaluateCommand(execution, 'cancel').allowed,
      canSteer: evaluateCommand(execution, 'steer').allowed,
      canAttach: false,
      canVoice: false,
    };
  }

  function upsertMessage(message: ConversationMessage): void {
    const existing = messagesById.get(message.id);
    if (existing) {
      existing.text = message.text;
      return;
    }
    messagesById.set(message.id, message);
    messageOrder.push(message.id);
  }

  return {
    runID: input.runID,
    state: () => ({
      ...state,
      messages: messageOrder.map((id) => ({ ...messagesById.get(id)! })),
    }),
    applyEvents(events) {
      for (const event of [...events].sort((a, b) => a.seq - b.seq)) {
        if (event.run_id !== input.runID) continue;
        if (appliedSeqs.has(event.seq)) continue; // 同 seq 重复投递：跳过（frozen 场景）
        appliedSeqs.add(event.seq);
        if (event.type === 'text.delta') {
          const payload = event.payload as { message_id?: string; text?: string };
          const id = typeof payload.message_id === 'string' && payload.message_id !== '' ? payload.message_id : `seq:${event.seq}`;
          const existing = messagesById.get(id);
          const text = (existing?.text ?? '') + (typeof payload.text === 'string' ? payload.text : '');
          upsertMessage({ id, role: 'assistant', text, createdAt: event.occurred_at });
        }
        // 未知事件类型：保留 seq（已应用）不丢弃语义；渲染层安全降级（MX-003 冻结规则）
      }
      state = { ...state, messages: messageOrder.map((id) => ({ ...messagesById.get(id)! })) };
      return this.state();
    },
    applyExecution(next) {
      execution = next;
      state = {
        ...state,
        runStatus: next.run_status,
        executionStatus: next.execution_status,
        settlementStatus: next.settlement_status,
        revision: next.revision,
        capabilities: capabilitiesFromExecution(),
      };
      return this.state();
    },
    applyInteractions(records) {
      state = {
        ...state,
        pendingInteractions: records.map((record) => ({
          id: record.id,
          kind: INTERACTION_KIND_MAP[record.kind] ?? 'unknown',
          status: record.decision_id === '' ? 'pending' : record.action === 'reject' ? 'rejected' : 'approved',
          label: record.kind,
        })),
      };
      return this.state();
    },
    commands: {
      cancel: async (runID, expectedRevision) => {
        const revision = expectedRevision ?? state.revision;
        if (revision <= 0) throw new Error('CANCEL_REQUIRES_SNAPSHOT_REVISION');
        await input.executions.command(runID, { action: 'cancel', expected_revision: revision });
      },
      steer: async (runID, text, expectedRevision) => {
        const revision = expectedRevision ?? state.revision;
        if (revision <= 0) throw new Error('STEER_REQUIRES_SNAPSHOT_REVISION');
        await input.executions.command(runID, { action: 'steer', text, expected_revision: revision });
      },
      refreshPending: async () => {
        // 交互刷新与提交状态刷新分离（G02 语义）：交互列表由宿主经 interactions API 拉取后 apply
        throw new Error('REFRESH_INTERACTIONS_VIA_INTERACTIONS_API');
      },
    },
  };
}

/**
 * The only boundary where Happy-shaped session data is allowed to become a
 * product conversation model. The view never receives Happy credentials or
 * raw protocol messages, and product scope remains explicit in every model.
 */
export function createConversationViewModel(input: ConversationViewModelInput): ConversationViewModel {
  // ExecutionDTO carries run_status/execution_status instead of the legacy
  // wire fields, so normalize per branch: the DTO path keeps its statuses and
  // an empty request id (the DTO does not carry one).
  const raw = input.execution;
  const execution = raw == null ? null : {
    runID: raw.run_id,
    requestID: 'request_id' in raw ? raw.request_id : '',
    status: 'status' in raw ? raw.status : raw.execution_status,
    ...('revision' in raw && raw.revision !== undefined ? { revision: raw.revision } : {}),
    ...('reason' in raw && raw.reason !== undefined ? { reason: raw.reason } : {}),
  };
  return {
    scope: { ...input.scope },
    messages: [...(input.messages ?? [])],
    pendingInteractions: [...(input.pendingInteractions ?? [])],
    capabilities: {
      canCancel: false,
      canSteer: false,
      canAttach: false,
      canVoice: false,
      ...input.capabilities,
    },
    execution,
    commands: input.commands,
    send: input.send,
  };
}

export interface SendController {
  submit(text: string, requestID: string): Promise<void>;
  draft(): string;
  busy(): boolean;
}

/** A single-flight sender. A failed request deliberately leaves its draft. */
export function createSendController(send: (text: string, requestID: string) => Promise<void>): SendController {
  let text = '';
  let pending = false;
  return {
    draft: () => text,
    busy: () => pending,
    async submit(value: string, requestID: string): Promise<void> {
      if (pending) throw new Error('SEND_IN_PROGRESS');
      if (value.trim() === '') return;
      if (requestID.trim() === '') throw new Error('REQUEST_ID_REQUIRED');
      text = value;
      pending = true;
      try {
        await send(value, requestID);
        text = '';
      } finally {
        pending = false;
      }
    },
  };
}
