import { EMPTY_DRAFT, evaluateSubmitReadiness, recommendLeadAgent, type AgentOption, type KnowledgeResource, type LeadAgentRecommendation, type NewTaskDraft, type SubmitReadiness, type TaskAttachmentRef } from '@weknora/domain/mobile';
import type { NetworkStatusPort, TaskOffice, TaskStartReceipt } from '@weknora/mobile-core';
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
  /** T10（#40）离线确认门：离线时 submit 在派发前拒绝、草稿保持保存；缺省不拦截。 */
  network?: NetworkStatusPort;
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
  /** T10（#40）：网络离线标记（驱动 New 屏提示行；submit 的离线拒绝与此同源）。 */
  offline: boolean;
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

/** rejected 终态的用户可见文案（B3-F41）：服务端确定性拒绝，修改目标后重试=新请求标识。 */
export const SUBMISSION_REJECTED_COPY = '上次提交已被服务端拒绝；修改目标后重新提交将使用新的请求标识。';

/** T10（#40）离线确认门文案：草稿已加密保存，联网后由用户手动确认提交（绝不自动重放）。 */
export const OFFLINE_SUBMIT_COPY = '当前离线：目标已加密保存为草稿；恢复联网后请手动点击提交确认发送。';

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
    offline: false,
  };
  let intentRequestId: string | undefined;
  let draftDirty = false; // loading 窗口内的用户编辑（B3-F40）：初始化不得覆盖
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
    const agents = patch.agents ?? state.agents; // 以合并后目录求推荐（B3-F39）
    publish({ ...state, ...patch, draft, readiness: evaluateSubmitReadiness(draft), recommendation: recommendLeadAgent(agents) });
  };
  const initialized = (async (): Promise<void> => {
    try {
      const [stored, agents, knowledge] = await Promise.all([
        ports.drafts?.load().catch(() => undefined),
        ports.agents().catch(() => [] as const), // 目录是可降级输入（B3-F59，对照 knowledge 先例）
        ports.knowledge?.().catch(() => [] as const) ?? ([] as const),
      ]);
      let pending: TaskStartReceipt[] = [];
      try { pending = await ports.office.reconcilePending(); } catch { /* 恢复失败不阻塞新建流 */ }
      if (disposed) return;
      const storedDraft = stored ?? emptyDraft();
      // loading 窗口内的用户编辑优先（B3-F40）；agentId 为空时兜底推荐。
      const draft = draftDirty
        ? { ...state.draft, agentId: state.draft.agentId ?? recommendLeadAgent(agents).agent?.id ?? null }
        : (storedDraft.agentId === null ? { ...storedDraft, agentId: recommendLeadAgent(agents).agent?.id ?? null } : storedDraft);
      const offline = ports.network !== undefined ? !(await ports.network.online().catch(() => false)) : false;
      const unresolved = pending.find((receipt) => receipt.phase !== 'bound' && receipt.phase !== 'rejected');
      if (unresolved !== undefined) intentRequestId = unresolved.requestId; // 回填（B3-F37）：重试同 ID
      publish({
        ...state,
        draft,
        agents,
        knowledge,
        loading: false,
        offline,
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
      draftDirty = true;
      project({ draft: { ...state.draft, ...patch } });
      persistDraft();
    },
    setAttachments(attachments) {
      draftDirty = true;
      project({ draft: { ...state.draft, attachments: [...attachments] } });
      persistDraft();
    },
    toggleKnowledge(knowledgeId) {
      draftDirty = true;
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
      // submitting 必须在首个 await（网络探测）之前同步置位（审查修复轮 1）：
      // 检查-置位原子，同帧双击在探测挂起期间即被 submitting 守卫拒绝，绝不双派发计费 Run。
      publish({ ...state, submitting: true, offline: false, error: undefined });
      // T10（#40）离线确认门：派发前拒绝（AC2——离线不能执行 Run），草稿保持加密保存。
      let offline = false;
      if (ports.network !== undefined) {
        offline = !(await ports.network.online().catch(() => false)); // 探测失败 = 离线（fail closed）
      }
      if (offline) {
        persistDraft();
        publish({ ...state, submitting: false, offline: true, error: OFFLINE_SUBMIT_COPY });
        return undefined;
      }
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
          draftDirty = false;
          const cleared: NewTaskDraft = { ...emptyDraft(), agentId: state.draft.agentId, budgetUpper: state.draft.budgetUpper };
          publish({ ...state, draft: cleared, readiness: evaluateSubmitReadiness(cleared), recommendation: state.recommendation, submitting: false, inFlight: undefined });
          await ports.drafts?.save(cleared).catch(() => undefined);
        } else if (receipt.phase === 'rejected') {
          intentRequestId = undefined; // 终态（B3-F41）：新意图=新 request_id，不驱动同 ID 重试
          publish({ ...state, submitting: false, inFlight: undefined, error: SUBMISSION_REJECTED_COPY });
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
