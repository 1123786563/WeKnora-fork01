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
      try {
        const value: unknown = JSON.parse(entry.body);
        if (typeof value !== 'object' || value === null) return undefined;
        return value as NewTaskDraft; // 字段级校验由 evaluateSubmitReadiness 在投影时兜底
      } catch {
        return undefined;
      }
    },
    async save(draft) {
      await store.put({ id: NEW_TASK_DRAFT_ID, body: JSON.stringify(draft) });
    },
  };
}
