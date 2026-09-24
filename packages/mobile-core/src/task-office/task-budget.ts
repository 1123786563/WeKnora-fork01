import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';

/**
 * Task Budget 深模块（T09 #39，module-seams §5「预算扩展」归 Task Office 所有）：
 * - 四数字读（预计/已用/预占/剩余，根预算行聚合含委派 Run）与授权扩额；
 * - 幂等纪律（对齐 Web TaskBudget.tsx:38-40）：每个「待完成的逻辑扩额」持有一个
 *   idempotency key——失败（含 typed 拒绝前的传输失败）保留同键重试，成功后清除，
 *   下一次用户动作取新键；服务端 exactly-once per key，重试永不加倍；
 * - scope 围栏：每次提交前与完成后检查 lease（切租户/换部署/登出即拒，迟到成功
 *   不越 scope 原样返回）；token 经 authorizedRequest 通道，不入本模块任何返回值；
 * - wire 错误经跨包契约码（api-client remote 写入 error.code，#38 INTERACTION_* 先例）
 *   翻译为 typed TaskBudgetError；扩额是纯预算决定，永不携带外部操作授权。
 */

export interface TaskBudgetFacts {
  taskId: string;
  rootRunId: string;
  limitCredits: number;
  usedCredits: number;
  heldCredits: number;
  remainingCredits: number;
  deadline?: string;
  delegatedRunIds: string[];
  pausedRunIds: string[];
  canExtend: boolean;
}

export interface TaskBudgetExtendInput { taskId: string; additionalCredits: number }
export interface TaskBudgetExtendReceipt { additionalCredits: number; resumedRuns: number }

export interface TaskBudgetBackendPort {
  facts(taskId: string): Promise<TaskBudgetFacts>;
  extend(input: { taskId: string; additionalCredits: number; idempotencyKey: string }): Promise<TaskBudgetExtendReceipt>;
}

export type TaskBudgetErrorCode =
  | 'TASK_BUDGET_SCOPE_CHANGED'
  | 'TASK_BUDGET_INVALID_INPUT'
  | 'TASK_BUDGET_FORBIDDEN'
  | 'TASK_BUDGET_NOT_FOUND'
  | 'TASK_BUDGET_INSUFFICIENT'
  | 'TASK_BUDGET_EXPIRED'
  | 'TASK_BUDGET_BACKEND';

const TASK_BUDGET_WIRE_CODES = new Set([
  'TASK_BUDGET_FORBIDDEN', 'TASK_BUDGET_NOT_FOUND', 'TASK_BUDGET_INSUFFICIENT',
  'TASK_BUDGET_EXPIRED', 'TASK_BUDGET_INVALID_INPUT',
]);

export class TaskBudgetError extends Error {
  constructor(readonly code: TaskBudgetErrorCode, options?: { cause?: unknown }) {
    super(code, options);
    this.name = 'TaskBudgetError';
  }
}

function errorCodeOf(error: unknown): string | undefined {
  if (typeof error !== 'object' || error === null) return undefined;
  const code = (error as { code?: unknown }).code;
  return typeof code === 'string' ? code : undefined;
}

function toBudgetError(error: unknown): TaskBudgetError {
  if (error instanceof TaskBudgetError) return error;
  const code = errorCodeOf(error);
  if (code !== undefined && TASK_BUDGET_WIRE_CODES.has(code)) {
    return new TaskBudgetError(code as TaskBudgetErrorCode, { cause: error });
  }
  return new TaskBudgetError('TASK_BUDGET_BACKEND', { cause: error });
}

export function createTaskBudgetOps(deps: {
  backend: TaskBudgetBackendPort;
  lease(): ScopeLease | undefined;
  newIdempotencyKey?: () => string;
}): { budget(taskId: string): Promise<TaskBudgetFacts>; extendBudget(input: TaskBudgetExtendInput): Promise<TaskBudgetExtendReceipt> } {
  let pendingKey: string | undefined;
  const nextKey = (): string => {
    if (deps.newIdempotencyKey) return deps.newIdempotencyKey();
    if (typeof globalThis.crypto?.randomUUID === 'function') return globalThis.crypto.randomUUID();
    throw new TaskBudgetError('TASK_BUDGET_INVALID_INPUT', { cause: new Error('newIdempotencyKey port is required on platforms without crypto.randomUUID') });
  };
  const requireLease = (): ScopeLease => {
    const lease = deps.lease();
    if (lease === undefined || !leaseActive(lease)) throw new TaskBudgetError('TASK_BUDGET_SCOPE_CHANGED');
    return lease;
  };
  return {
    async budget(taskId) {
      const id = taskId.trim();
      if (id === '') throw new TaskBudgetError('TASK_BUDGET_INVALID_INPUT');
      const lease = requireLease();
      try {
        const facts = await deps.backend.facts(id);
        if (!leaseActive(lease)) throw new TaskBudgetError('TASK_BUDGET_SCOPE_CHANGED');
        return facts;
      } catch (error) {
        if (error instanceof TaskBudgetError) throw error;
        throw toBudgetError(error);
      }
    },
    async extendBudget(input) {
      const id = input.taskId.trim();
      const credits = input.additionalCredits;
      if (id === '') throw new TaskBudgetError('TASK_BUDGET_INVALID_INPUT');
      if (!Number.isSafeInteger(credits) || credits <= 0) throw new TaskBudgetError('TASK_BUDGET_INVALID_INPUT');
      const lease = requireLease();
      if (pendingKey === undefined) pendingKey = nextKey();
      const key = pendingKey;
      try {
        const receipt = await deps.backend.extend({ taskId: id, additionalCredits: credits, idempotencyKey: key });
        if (!leaseActive(lease)) throw new TaskBudgetError('TASK_BUDGET_SCOPE_CHANGED');
        pendingKey = undefined; // 成功才清除：失败重试沿用同键（服务端 exactly-once）
        return receipt;
      } catch (error) {
        if (error instanceof TaskBudgetError) throw error;
        throw toBudgetError(error);
      }
    },
  };
}
