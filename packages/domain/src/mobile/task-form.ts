import type { MobileStartInput } from './submission.ts';

/**
 * 新建任务表单领域模型（MX-015 / M05）。
 * 冻结规则：
 * - 草稿优先：取消输入/未就绪提交/离线一律保留草稿文本（不丢字）；
 * - 附件未就绪（scanning/pending/failed）禁止提交——startCount=0、草稿保留，
 *   并给出原因；附件字段绝不擅加进 StartInput（7 字段冻结，附件经会话准备接口解析）；
 * - 校验：文本非空、预算非负安全整数；
 * - 提交走持久提交协调器（MX-006 submit 语义：先落盘 request_id 再发网络；unknown 导向
 *   原请求核实），本模型只产出待提交输入与就绪裁决，不直接发网络。
 */

export type AttachmentReadiness = 'pending' | 'scanning' | 'ready' | 'failed';

export interface TaskAttachmentRef {
  id: string;
  name: string;
  readiness: AttachmentReadiness;
}

export interface NewTaskDraft {
  text: string;
  agentId: string | null;
  budgetUpper: number;
  attachments: TaskAttachmentRef[];
  knowledgeIds: string[];
}

export const EMPTY_DRAFT: NewTaskDraft = { text: '', agentId: null, budgetUpper: 0, attachments: [], knowledgeIds: [] };

export interface SubmitReadiness {
  ready: boolean;
  reason?: 'text_required' | 'agent_required' | 'budget_invalid' | 'attachments_not_ready';
  blockingAttachments: TaskAttachmentRef[];
}

/** 就绪裁决（纯函数）：任何未就绪附件阻塞提交并保留草稿。 */
export function evaluateSubmitReadiness(draft: NewTaskDraft): SubmitReadiness {
  const text = draft.text.trim();
  if (text === '') return { ready: false, reason: 'text_required', blockingAttachments: [] };
  if (!draft.agentId) return { ready: false, reason: 'agent_required', blockingAttachments: [] };
  if (!Number.isSafeInteger(draft.budgetUpper) || draft.budgetUpper < 0) {
    return { ready: false, reason: 'budget_invalid', blockingAttachments: [] };
  }
  const blocking = draft.attachments.filter((item) => item.readiness !== 'ready');
  if (blocking.length > 0) return { ready: false, reason: 'attachments_not_ready', blockingAttachments: blocking };
  return { ready: true, blockingAttachments: [] };
}

/** 产出 Start 输入（严格 7 字段；附件/知识绝不进入 body——由会话准备另行解析）。 */
export function toStartInput(draft: NewTaskDraft, context: { requestID: string; sessionId: string; targetId: string; workspaceRef: string }): MobileStartInput {
  if (!draft.agentId) throw new Error('AGENT_REQUIRED');
  const readiness = evaluateSubmitReadiness(draft);
  if (!readiness.ready) throw new Error(`SUBMIT_BLOCKED:${readiness.reason}`);
  return {
    request_id: context.requestID,
    session_id: context.sessionId,
    agent_id: draft.agentId,
    target_id: context.targetId,
    workspace_ref: context.workspaceRef,
    text: draft.text.trim(),
    budget_upper: draft.budgetUpper,
  };
}

export interface TaskFormSubmitPorts {
  /** 持久提交（MX-006 协调器语义：先落盘再发；返回是否派发）。 */
  submit(input: MobileStartInput): Promise<{ dispatched: boolean }>;
  /** 草稿持久化（取消/未就绪/离线均保留）。 */
  saveDraft(draft: NewTaskDraft): Promise<void>;
  /** 会话准备：把附件/知识引用解析进已有会话配置（版本化扩展通道；实现由宿主接真实 API）。 */
  prepareSessionRefs?(draft: NewTaskDraft): Promise<void>;
}

export function createTaskForm(initial: NewTaskDraft, ports: TaskFormSubmitPorts) {
  let draft: NewTaskDraft = { ...initial, attachments: [...initial.attachments], knowledgeIds: [...initial.knowledgeIds] };

  return {
    get draft(): NewTaskDraft {
      return { ...draft, attachments: [...draft.attachments], knowledgeIds: [...draft.knowledgeIds] };
    },
    update(patch: Partial<Omit<NewTaskDraft, 'attachments' | 'knowledgeIds'>>): NewTaskDraft {
      draft = { ...draft, ...patch };
      void ports.saveDraft(this.draft);
      return this.draft;
    },
    /** 未就绪提交：零网络、草稿保留、返回原因（UI 展示，不 Toast 假成功）。 */
    async attemptSubmit(context: { requestID: string; sessionId: string; targetId: string; workspaceRef: string }): Promise<{ submitted: boolean; readiness: SubmitReadiness }> {
      const readiness = evaluateSubmitReadiness(draft);
      if (!readiness.ready) {
        await ports.saveDraft(this.draft);
        return { submitted: false, readiness };
      }
      await ports.prepareSessionRefs?.(this.draft);
      const input = toStartInput(draft, context);
      const result = await ports.submit(input);
      // 提交后草稿清空只在成功派发时发生；失败/unknown 由协调器语义保留原请求
      if (result.dispatched) {
        draft = { ...EMPTY_DRAFT, agentId: draft.agentId, budgetUpper: draft.budgetUpper };
        await ports.saveDraft(this.draft);
      }
      return { submitted: result.dispatched, readiness };
    },
    /** 取消输入：显式保留可恢复草稿（离线不自动发送）。 */
    async cancelKeepingDraft(): Promise<NewTaskDraft> {
      await ports.saveDraft(this.draft);
      return this.draft;
    },
  };
}

export type TaskFormController = ReturnType<typeof createTaskForm>;
