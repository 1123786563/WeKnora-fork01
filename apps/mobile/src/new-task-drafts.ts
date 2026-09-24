import type { NewTaskDraft } from '@weknora/domain/mobile';

export const NEW_TASK_DRAFT_ID = 'new-task';

/** Scoped Vault drafts 的结构子集（put/get 与 ScopedStore.drafts 签名一致，测试可注入内存替身）。 */
export interface ScopedDraftLike {
  get(id: string): Promise<{ body: string } | undefined>;
  put(input: { id: string; body: string }): Promise<void>;
}

export interface NewTaskDraftPersistence {
  load(): Promise<NewTaskDraft | undefined>;
  save(draft: NewTaskDraft): Promise<void>;
}

/** 加密离线草稿通道（#32 Scoped Vault 之上的 New 屏适配；跨进程持久化证据属 #40）。 */
export function createScopedNewTaskDrafts(store: ScopedDraftLike): NewTaskDraftPersistence {
  return {
    async load() {
      const entry = await store.get(NEW_TASK_DRAFT_ID).catch(() => undefined);
      if (!entry) return undefined;
      let value: unknown;
      try {
        value = JSON.parse(entry.body);
      } catch {
        return undefined;
      }
      if (typeof value !== 'object' || value === null) return undefined;
      // 字段级防御（B3-F42）：脏持久化体逐项降级为空值——evaluateSubmitReadiness
      // 直接 draft.text.trim()/attachments.filter()，脏类型会让 New 屏整屏崩溃。
      const row = value as Partial<NewTaskDraft> & Record<string, unknown>;
      const cleanArray = <T>(candidate: unknown): T[] => (Array.isArray(candidate) ? candidate as T[] : []);
      const draft: NewTaskDraft = {
        text: typeof row.text === 'string' ? row.text : '',
        agentId: typeof row.agentId === 'string' ? row.agentId : null,
        budgetUpper: typeof row.budgetUpper === 'number' && Number.isSafeInteger(row.budgetUpper) && row.budgetUpper >= 0 ? row.budgetUpper : 0,
        attachments: cleanArray(row.attachments),
        knowledgeIds: cleanArray(row.knowledgeIds),
      };
      return draft;
    },
    async save(draft) {
      await store.put({ id: NEW_TASK_DRAFT_ID, body: JSON.stringify(draft) });
    },
  };
}
