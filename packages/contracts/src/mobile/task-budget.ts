import { ContractError } from '../index.ts';

/**
 * Task Budget wire 契约（T09 #39）。四个数字全部是 millionths-of-a-Credit 的
 * 安全整数（与既有 additional_credits wire 单位一致，commercial.Credits 定义见
 * internal/modules/commercial/amount.go:11）；四数恒非负（显式拒绝自洽负数行，
 * 终审修复 #2），remaining 与三数的算术一致性由解析器强制——序列化缺陷
 * fail-closed，客户端永不展示幻影数字。delegated 与 paused 清单来自根预算行
 * 聚合（子 Run 预占天然计入根行）；can_extend 是服务端对「账单权威 OR 任务
 * Owner」的判定投影，grant 协作者恒 false。
 */
export interface TaskBudgetWireFacts {
  task_id: string;
  root_run_id: string;
  limit_credits: number;
  used_credits: number;
  held_credits: number;
  remaining_credits: number;
  deadline?: string;
  delegated_run_ids: string[];
  paused_run_ids: string[];
  can_extend: boolean;
}

export interface TaskBudgetWireExtension {
  task_id: string;
  additional_credits: number;
  resumed_runs: number;
}

function nonEmptyString(value: unknown, field: string): string {
  if (typeof value !== 'string' || value.trim() === '') {
    throw new ContractError(field, 'expected a non-empty string');
  }
  return value;
}

function microCredits(value: unknown, field: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value)) {
    throw new ContractError(field, 'expected a safe integer (millionths of a Credit)');
  }
  return value;
}

function idList(value: unknown, field: string): string[] {
  if (!Array.isArray(value)) throw new ContractError(field, 'expected an array of run ids');
  return value.map((entry) => nonEmptyString(entry, field));
}

export function parseTaskBudgetFacts(value: unknown): TaskBudgetWireFacts {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('task budget', 'expected an object');
  }
  const row = value as Record<string, unknown>;
  const limit = microCredits(row.limit_credits, 'limit_credits');
  if (limit < 0) {
    throw new ContractError('limit_credits', 'must be non-negative');
  }
  const used = microCredits(row.used_credits, 'used_credits');
  const held = microCredits(row.held_credits, 'held_credits');
  const remaining = microCredits(row.remaining_credits, 'remaining_credits');
  if (remaining !== limit - used - held) {
    throw new ContractError('remaining_credits', `expected ${limit - used - held} (limit - used - held), got ${remaining}`);
  }
  if (used < 0 || held < 0) {
    throw new ContractError('credits', 'used/held must be non-negative');
  }
  if (remaining < 0) {
    throw new ContractError('remaining_credits', 'must be non-negative');
  }
  if (row.deadline !== undefined && typeof row.deadline !== 'string') {
    throw new ContractError('deadline', 'must be a string when present');
  }
  if (typeof row.can_extend !== 'boolean') {
    throw new ContractError('can_extend', 'expected a boolean');
  }
  const base: TaskBudgetWireFacts = {
    task_id: nonEmptyString(row.task_id, 'task_id'),
    root_run_id: nonEmptyString(row.root_run_id, 'root_run_id'),
    limit_credits: limit,
    used_credits: used,
    held_credits: held,
    remaining_credits: remaining,
    delegated_run_ids: idList(row.delegated_run_ids, 'delegated_run_ids'),
    paused_run_ids: idList(row.paused_run_ids, 'paused_run_ids'),
    can_extend: row.can_extend,
  };
  return row.deadline === undefined ? base : { ...base, deadline: row.deadline };
}

export function parseTaskBudgetExtension(value: unknown): TaskBudgetWireExtension {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('task budget extension', 'expected an object');
  }
  const row = value as Record<string, unknown>;
  const additional = microCredits(row.additional_credits, 'additional_credits');
  if (additional <= 0) {
    throw new ContractError('additional_credits', 'must be positive');
  }
  const resumed = microCredits(row.resumed_runs, 'resumed_runs');
  if (resumed < 0) {
    throw new ContractError('resumed_runs', 'must be non-negative');
  }
  return { task_id: nonEmptyString(row.task_id, 'task_id'), additional_credits: additional, resumed_runs: resumed };
}
