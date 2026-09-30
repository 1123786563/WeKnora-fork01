import { ContractError } from '../index.ts';

/** T22 (#52)：代码交付生命周期（Go codedelivery.DeliveryState 逐字镜像）。 */
export type CodeDeliveryState = 'prepared' | 'dispatched' | 'pushed' | 'delivered' | 'failed' | 'unknown';

/** 交付追溯记录：审批锚点（action/digest/approver）+ 远端回执（commit/PR/实际远端身份）。 */
export interface CodeDeliveryRecord {
  id: string;
  taskId: string;
  runId: string;
  state: CodeDeliveryState;
  repo: string;
  baselineSha: string;
  branch: string;
  commitSha?: string;
  prNumber?: number;
  prUrl?: string;
  remoteLogin?: string;
  actionId: string;
  actionState: string;
  digest: string;
  approver?: string;
  failure?: string;
  files: number;
  createdAt: string;
  updatedAt: string;
}

const STATES: ReadonlySet<string> = new Set(['prepared', 'dispatched', 'pushed', 'delivered', 'failed', 'unknown']);

function str(row: Record<string, unknown>, key: string): string {
  const value = row[key];
  if (typeof value !== 'string') throw new ContractError(`code_delivery.${key}`, 'expected a string');
  return value;
}

function optional(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined;
}

/** 解析 GET /workbench/executions/:run_id/delivery 的 delivery 行；未知状态/缺字段 fail closed。 */
export function parseCodeDeliveryRecord(value: unknown): CodeDeliveryRecord {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('code_delivery.record', 'must be an object');
  }
  const row = value as Record<string, unknown>;
  const stateRaw = str(row, 'state');
  if (!STATES.has(stateRaw)) throw new ContractError('code_delivery.state', `unknown code delivery state: ${stateRaw}`);
  const state = stateRaw as CodeDeliveryState;
  const id = str(row, 'id');
  const actionId = str(row, 'action_id');
  const digest = str(row, 'digest');
  if (id === '' || actionId === '' || digest === '') {
    throw new ContractError('code_delivery', 'id, action_id and digest are required');
  }
  const files = row.files;
  if (typeof files !== 'number') throw new ContractError('code_delivery.files', 'expected a number');
  const prNumber = row.pr_number;
  if (prNumber !== undefined && typeof prNumber !== 'number') {
    throw new ContractError('code_delivery.pr_number', 'expected a number when present');
  }
  return {
    id, taskId: str(row, 'task_id'), runId: str(row, 'run_id'), state,
    repo: str(row, 'repo'), baselineSha: str(row, 'baseline_sha'), branch: str(row, 'branch'),
    ...(optional(row.commit_sha) === undefined ? {} : { commitSha: optional(row.commit_sha) }),
    ...(typeof prNumber === 'number' && prNumber > 0 ? { prNumber } : {}),
    ...(optional(row.pr_url) === undefined ? {} : { prUrl: optional(row.pr_url) }),
    ...(optional(row.remote_login) === undefined ? {} : { remoteLogin: optional(row.remote_login) }),
    actionId, actionState: str(row, 'action_state'), digest,
    ...(optional(row.approver) === undefined ? {} : { approver: optional(row.approver) }),
    ...(optional(row.failure) === undefined ? {} : { failure: optional(row.failure) }),
    files, createdAt: str(row, 'created_at'), updatedAt: str(row, 'updated_at'),
  };
}
