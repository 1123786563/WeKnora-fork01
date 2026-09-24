import { EMPTY_DRAFT, evaluateSubmitReadiness, recommendLeadAgent, type AgentOption, type KnowledgeResource, type LeadAgentRecommendation, type NewTaskDraft, type SubmitReadiness, type TaskAttachmentRef } from '@weknora/domain/mobile';
import type { TaskOffice, TaskStartReceipt } from '@weknora/mobile-core';
import type { NewTaskDraftPersistence } from './new-task-drafts.ts';
// 简报测试文件经 './new-task-view.ts' 导入该类型（brief 内部契约）；再导出以保持两个文件逐字兼容。
export type { NewTaskDraftPersistence };

export interface NewTaskControllerPorts {
  office: Pick<TaskOffice, 'start' | 'reconcilePending'>;
  agents(): Promise<readonly AgentOption[]>;
  /** 可附加知识（Resource Shelf browse 投影）；缺省空列表（附加资源是可选输入）。 */
  knowledge?(): Promise<readonly KnowledgeResource[]>;
  /** 加密离线草稿（Scoped Vault）；缺省时草稿仅存活于本控制器（组合根显式决定）。 */
  drafts?: NewTaskDraftPersistence;
  newRequestId(): string;
}

export interface NewTaskViewState {
  draft: NewTaskDraft;
  agents: readonly AgentOption[];
  knowledge: readonly KnowledgeResource[];
  recommendation: LeadAgentRecommendation;
  readiness: SubmitReadiness;
  loading: boolean;
  submitting: boolean;
  /** 上一次未决提交（本会话失败或重启恢复），驱动「同一意图同 request_id」重入。 */
  inFlight?: TaskStartReceipt;
  error?: string;
}

export interface NewTaskController {
  state(): NewTaskViewState;
  subscribe(listener: (state: NewTaskViewState) => void): () => void;
  update(patch: Partial<Omit<NewTaskDraft, 'attachments' | 'knowledgeIds'>>): void;
  setAttachments(attachments: TaskAttachmentRef[]): void;
  toggleKnowledge(knowledgeId: string): void;
  refreshAgents(): Promise<void>;
  submit(): Promise<TaskStartReceipt | undefined>;
  cancelKeepingDraft(): Promise<void>;
  whenInitialized(): Promise<void>;
  dispose(): void;
}

const emptyDraft = (): NewTaskDraft => ({ ...EMPTY_DRAFT, attachments: [], knowledgeIds: [] });

/**
 * New 屏控制器（module-seams §5.4：task-form 纯策略的宿主编排，Screen 只见状态与意图）。
 * AC2 的保留语义全部落在这里：未就绪/离线/冲突一律不清草稿；只有 bound 才清空可编辑
 * 字段；未决意图记住 requestId，用户显式重试时以同一 id 重入（D5：绝不换 ID 重建）。
 */
export function createNewTaskController(ports: NewTaskControllerPorts): NewTaskController {
  let state: NewTaskViewState = {
    draft: emptyDraft(),
    agents: [],
    knowledge: [],
    recommendation: { agent: undefined, reason: 'no_supported_agent' },
    readiness: evaluateSubmitReadiness(emptyDraft()),
    loading: true,
    submitting: false,
  };
  let intentRequestId: string | undefined;
  let disposed = false;
  const listeners = new Set<(state: NewTaskViewState) => void>();
  const publish = (next: NewTaskViewState): void => {
    state = next;
    if (!disposed) for (const listener of [...listeners]) listener(state);
  };
  const persistDraft = (): void => {
    void ports.drafts?.save(state.draft).catch(() => undefined);
  };
  const project = (patch: Partial<NewTaskViewState> & { draft?: NewTaskDraft }): void => {
    const draft = patch.draft ?? state.draft;
    publish({ ...state, ...patch, draft, readiness: evaluateSubmitReadiness(draft), recommendation: recommendLeadAgent(state.agents) });
  };
  const initialized = (async (): Promise<void> => {
    try {
      const [stored, agents, knowledge] = await Promise.all([
        ports.drafts?.load().catch(() => undefined),
        ports.agents(),
        ports.knowledge?.().catch(() => [] as const) ?? ([] as const),
      ]);
      let pending: TaskStartReceipt[] = [];
      try { pending = await ports.office.reconcilePending(); } catch { /* 恢复失败不阻塞新建流 */ }
      if (disposed) return;
      const draft = stored ?? emptyDraft();
      if (draft.agentId === null) draft.agentId = recommendLeadAgent(agents).agent?.id ?? null;
      const unresolved = pending.find((receipt) => receipt.phase !== 'bound' && receipt.phase !== 'rejected');
      publish({
        ...state,
        draft,
        agents,
        knowledge,
        loading: false,
        readiness: evaluateSubmitReadiness(draft),
        recommendation: recommendLeadAgent(agents),
        ...(unresolved === undefined ? {} : { inFlight: unresolved }),
      });
    } catch (cause) {
      if (disposed) return;
      publish({ ...state, loading: false, error: cause instanceof Error ? cause.message : String(cause) });
    }
  })();
  return {
    state: () => state,
    subscribe(listener) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    update(patch) {
      project({ draft: { ...state.draft, ...patch } });
      persistDraft();
    },
    setAttachments(attachments) {
      project({ draft: { ...state.draft, attachments: [...attachments] } });
      persistDraft();
    },
    toggleKnowledge(knowledgeId) {
      const attached = state.draft.knowledgeIds.includes(knowledgeId);
      project({ draft: { ...state.draft, knowledgeIds: attached ? state.draft.knowledgeIds.filter((id) => id !== knowledgeId) : [...state.draft.knowledgeIds, knowledgeId] } });
      persistDraft();
    },
    async refreshAgents() {
      try {
        const agents = await ports.agents();
        if (!disposed) project({ agents });
      } catch {
        /* 目录刷新失败保持现状（recommendation 不回退） */
      }
    },
    async submit() {
      const readiness = evaluateSubmitReadiness(state.draft);
      if (!readiness.ready || state.submitting) {
        project({ readiness });
        persistDraft();
        return undefined;
      }
      publish({ ...state, submitting: true, error: undefined });
      try {
        const receipt = await ports.office.start(
          {
            text: state.draft.text,
            agentId: state.draft.agentId!,
            budgetUpper: state.draft.budgetUpper,
            ...(state.draft.knowledgeIds.length === 0 ? {} : { knowledgeIds: [...state.draft.knowledgeIds] }),
            ...(state.draft.attachments.length === 0 ? {} : { attachments: [...state.draft.attachments] }),
          },
          intentRequestId === undefined ? {} : { requestId: intentRequestId },
        );
        if (receipt.phase === 'bound' && receipt.runId !== undefined) {
          intentRequestId = undefined;
          const cleared: NewTaskDraft = { ...emptyDraft(), agentId: state.draft.agentId, budgetUpper: state.draft.budgetUpper };
          publish({ ...state, draft: cleared, readiness: evaluateSubmitReadiness(cleared), recommendation: state.recommendation, submitting: false, inFlight: undefined });
          await ports.drafts?.save(cleared).catch(() => undefined);
        } else {
          intentRequestId = receipt.requestId;
          publish({ ...state, submitting: false, inFlight: receipt });
        }
        return receipt;
      } catch (cause) {
        publish({ ...state, submitting: false, error: cause instanceof Error ? cause.message : String(cause) });
        return undefined;
      }
    },
    async cancelKeepingDraft() {
      persistDraft();
    },
    whenInitialized: () => initialized,
    dispose() {
      disposed = true;
      listeners.clear();
    },
  };
}
