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
  subscribe(listener: (state: LegacyTasksViewState) => void): () => void;
  reload(): Promise<void>;
  loadMore(): Promise<void>;
  openHistory(taskId: string): Promise<void>;
  submitFollowUp(taskId: string, question: string): Promise<void>;
  whenSettled(): Promise<void>;
  dispose(): void;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

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
      const messages = await office.legacyHistory(taskId);
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
      const page = await office.legacyTasks({});
      return { followUpState: 'sent' as const, followUpError: undefined, items: page.items, hasMore: page.nextCursor !== undefined };
    }),
    whenSettled: () => pending,
    dispose: () => { listeners.clear(); },
  };
  void controller.reload();
  return controller;
}
