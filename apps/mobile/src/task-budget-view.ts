import type { TaskBudgetErrorCode, TaskBudgetFacts, TaskOffice } from '@weknora/mobile-core';
import { TaskBudgetError } from '@weknora/mobile-core';

/** T09（#39）预算屏状态机：四数字 + 委派/暂停投影 + 独立扩额操作（对齐 Web
 * TaskBudget.tsx 的「预算决定与外部操作审批相互独立」纪律——文案不承诺任何外部授权）。 */
export interface TaskBudgetViewState {
  loading: boolean;
  facts?: TaskBudgetFacts;
  error?: string;
  message?: string;
  extending: boolean;
}

export const TASK_BUDGET_COPY: Record<TaskBudgetErrorCode, string> = {
  TASK_BUDGET_SCOPE_CHANGED: '登录空间已切换，请重新打开任务预算。',
  TASK_BUDGET_INVALID_INPUT: '追加额度必须为正整数。',
  TASK_BUDGET_FORBIDDEN: '只有任务所有者或获授权的账单管理员可以增加上限。',
  TASK_BUDGET_NOT_FOUND: '未找到该任务的预算。',
  TASK_BUDGET_INSUFFICIENT: '空间余额不足以追加该额度。',
  TASK_BUDGET_EXPIRED: '预算有效期已过，追加不延长截止时间。',
  TASK_BUDGET_BACKEND: '预算服务暂不可用，请稍后重试。',
};

export interface TaskBudgetController {
  state(): TaskBudgetViewState;
  subscribe(listener: (state: TaskBudgetViewState) => void): () => void;
  refresh(): Promise<void>;
  extend(additionalCredits: number): Promise<void>;
  dispose(): void;
}

export function createTaskBudgetController(
  office: Pick<TaskOffice, 'budget' | 'extendBudget'>,
  input: { taskId: string },
): TaskBudgetController {
  let state: TaskBudgetViewState = { loading: true, extending: false };
  const listeners = new Set<(next: TaskBudgetViewState) => void>();
  const emit = (): void => {
    const snapshot = { ...state };
    for (const listener of listeners) listener(snapshot);
  };
  const failWith = (error: unknown): void => {
    state = { ...state, loading: false, extending: false, error: error instanceof TaskBudgetError ? TASK_BUDGET_COPY[error.code] : '预算读取失败，请稍后重试。' };
    emit();
  };
  return {
    state: () => ({ ...state }),
    subscribe(listener) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    async refresh() {
      state = { ...state, loading: true, error: undefined };
      emit();
      try {
        const facts = await office.budget(input.taskId);
        state = { ...state, loading: false, facts };
      } catch (error) {
        failWith(error);
      }
    },
    async extend(additionalCredits: number) {
      if (!Number.isSafeInteger(additionalCredits) || additionalCredits <= 0) {
        state = { ...state, error: TASK_BUDGET_COPY.TASK_BUDGET_INVALID_INPUT };
        emit();
        return;
      }
      state = { ...state, extending: true, error: undefined, message: undefined };
      emit();
      try {
        const receipt = await office.extendBudget({ taskId: input.taskId, additionalCredits });
        let facts: TaskBudgetFacts | undefined;
        try {
          facts = await office.budget(input.taskId); // 权威重取，绝不本地加数
        } catch {
          // 修复轮 1（T09 #39 review）：提交成功 ≠ 重取成功。扩额已在服务端生效
          // （幂等键已清除），此刻重试将以新键二次扣费——必须明示「已提交、待核实」
          // 并引导刷新，不得折叠为「读取失败」引导重试。
          state = {
            ...state, extending: false, error: undefined,
            message: `扩额已提交（追加 ${receipt.additionalCredits} 额度，恢复 ${receipt.resumedRuns} 个运行），但读取最新预算失败；请先刷新核实结果，不要直接重复提交扩额。`,
          };
          emit();
          return;
        }
        state = {
          ...state, extending: false, facts,
          message: `已追加 ${receipt.additionalCredits} 额度，恢复 ${receipt.resumedRuns} 个运行；追加预算不等于外部操作批准。`,
        };
        emit();
      } catch (error) {
        failWith(error);
      }
    },
    dispose() { listeners.clear(); },
  };
}
