import type { LegacyMessage, LegacyTaskCard, TaskOffice } from '@weknora/mobile-core';

/** Legacy Task 视图状态：列表 + 历史 + 追问回执；cursor/epoch 归 Task Office，本控制器只持有渲染态。 */
export interface LegacyTasksViewState {
  loading: boolean;
  error?: string;
  items: LegacyTaskCard[];
  hasMore: boolean;
  duplicateTaskIds: string[];
  history?: { taskId: string; messages: LegacyMessage[]; error?: string };
  followUpState: 'idle' | 'sending' | 'sent';
  followUpError?: string;
}

export interface LegacyTasksController {
  state(): LegacyTasksViewState;
  subscribe(listener: (next: LegacyTasksViewState) => void): () => void;
  reload(): Promise<void>;
  loadMore(): Promise<void>;
  openHistory(taskId: string): Promise<void>;
  submitFollowUp(taskId: string, question: string): Promise<void>;
  whenSettled(): Promise<void>;
  dispose(): void;
}

/** TaskOfficeError 的 message 即裸错误码——先查文案表，未命中兜底原样（B3-F14）。 */
const LEGACY_ERROR_COPY: Record<string, string> = {
  TASK_OFFICE_SUPERSEDED: '列表已被其他操作更新，请刷新。',
  TASK_OFFICE_SCOPE_CHANGED: '登录状态或活动空间已变化，请重新进入。',
};

function errorMessage(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  return LEGACY_ERROR_COPY[message] ?? message;
}

/** 历史页大小：请求 100，服务端按自身上限自然 clamp（B3-F24）。 */
const HISTORY_PAGE_LIMIT = 100;

export function createLegacyTasksController(
  office: Pick<TaskOffice, 'legacyTasks' | 'moreLegacyTasks' | 'legacyHistory' | 'followUp'>,
): LegacyTasksController {
  let state: LegacyTasksViewState = { loading: true, items: [], hasMore: false, duplicateTaskIds: [], followUpState: 'idle' };
  const listeners = new Set<(next: LegacyTasksViewState) => void>();
  let pending: Promise<void> = Promise.resolve();
  const publish = (patch: Partial<LegacyTasksViewState>): void => {
    state = { ...state, ...patch };
    for (const listener of listeners) listener(state);
  };

  const run = (action: () => Promise<Partial<LegacyTasksViewState>>): Promise<void> => {
    pending = pending.then(async () => {
      try {
        publish(await action());
      } catch (error) {
        publish({ loading: false, error: errorMessage(error) });
      }
    });
    return pending;
  };

  const controller: LegacyTasksController = {
    state: () => state,
    subscribe(listener) {
      listeners.add(listener);
      return () => { listeners.delete(listener); };
    },
    reload: () => run(async () => {
      const page = await office.legacyTasks({});
      return { loading: false, error: undefined, items: page.items, hasMore: page.nextCursor !== undefined, duplicateTaskIds: page.duplicateTaskIds };
    }),
    loadMore: () => run(async () => {
      const page = await office.moreLegacyTasks();
      return { loading: false, items: [...state.items, ...page.items], hasMore: page.nextCursor !== undefined, duplicateTaskIds: page.duplicateTaskIds };
    }),
    openHistory: (taskId: string) => run(async () => {
      const messages = await office.legacyHistory(taskId, { limit: HISTORY_PAGE_LIMIT });
      return { loading: false, history: { taskId, messages } };
    }),
    submitFollowUp: (taskId: string, question: string) => run(async () => {
      const trimmed = question.trim();
      if (trimmed === '') return { followUpState: 'idle' as const, followUpError: undefined };
      publish({ followUpState: 'sending', followUpError: undefined });
      try {
        await office.followUp({ taskId, question: trimmed });
      } catch (error) {
        return { followUpState: 'idle' as const, followUpError: errorMessage(error) };
      }
      // 追问成功后的列表刷新也纳入收口（B3-F23）：刷新失败不得让 followUpState
      // 永久卡死 'sending'（发送按钮持续禁用且 reload 也不复位）。
      try {
        const page = await office.legacyTasks({});
        return { followUpState: 'sent' as const, followUpError: undefined, items: page.items, hasMore: page.nextCursor !== undefined };
      } catch (error) {
        return { followUpState: 'idle' as const, followUpError: `已发送，但刷新列表失败：${errorMessage(error)}` };
      }
    }),
    whenSettled: () => pending,
    dispose: () => { listeners.clear(); },
  };
  void controller.reload();
  return controller;
}
