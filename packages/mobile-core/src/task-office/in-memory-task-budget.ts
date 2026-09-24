import type { TaskBudgetBackendPort, TaskBudgetExtendReceipt, TaskBudgetFacts } from './task-budget.ts';

/** 场景 Adapter（测试/演示）：可脚本化 facts、extend 应答与拒绝码。 */
export interface ScenarioTaskBudgetScript {
  facts: TaskBudgetFacts;
  extend?: (input: { taskId: string; additionalCredits: number; idempotencyKey: string }) =>
    Promise<TaskBudgetExtendReceipt> | TaskBudgetExtendReceipt;
}

export function createScenarioTaskBudgetBackend(script: ScenarioTaskBudgetScript): TaskBudgetBackendPort {
  const appliedKeys = new Set<string>();
  const limitByTask = new Map<string, number>([[script.facts.taskId, script.facts.limitCredits]]);
  return {
    async facts(taskId) {
      const base = script.facts;
      if (taskId !== base.taskId) {
        const error = new Error('TASK_BUDGET_NOT_FOUND');
        (error as unknown as { code?: string }).code = 'TASK_BUDGET_NOT_FOUND';
        throw error;
      }
      return { ...base, limitCredits: limitByTask.get(taskId) ?? base.limitCredits };
    },
    async extend(input) {
      if (appliedKeys.has(input.idempotencyKey)) {
        return { additionalCredits: input.additionalCredits, resumedRuns: 0 }; // exactly-once per key
      }
      appliedKeys.add(input.idempotencyKey);
      if (script.extend) return await script.extend(input);
      limitByTask.set(input.taskId, (limitByTask.get(input.taskId) ?? 0) + input.additionalCredits);
      return { additionalCredits: input.additionalCredits, resumedRuns: script.facts.pausedRunIds.length };
    },
  };
}
