import { ContractError, parseInteraction } from '../index.ts';
import type { InteractionRecord } from './interactions.ts';

/**
 * Attention Inbox 行契约（T08）：在 MX-003 冻结的 InteractionRecord 之上
 * 要求非空 run_id——收件箱必须能导航回任务并提供 args_hash/expected_revision
 * 决定上下文；created_at 为可选输出字段（Go list 投影在 T08 起携带）。
 * 不修改冻结的 InteractionRecord 本身。
 */
export type InboxInteractionRecord = InteractionRecord & { run_id: string; created_at?: string };

export function parseInteractionWithRun(value: unknown): InboxInteractionRecord {
  const record = parseInteraction(value);
  const row = (typeof value === 'object' && value !== null ? value : {}) as Record<string, unknown>;
  if (typeof row.run_id !== 'string' || row.run_id.trim() === '') {
    throw new ContractError('run_id', 'expected a non-empty string');
  }
  const created_at = row.created_at;
  if (created_at !== undefined && typeof created_at !== 'string') {
    throw new ContractError('created_at', 'must be a string when present');
  }
  return created_at === undefined ? { ...record, run_id: row.run_id } : { ...record, run_id: row.run_id, created_at };
}
