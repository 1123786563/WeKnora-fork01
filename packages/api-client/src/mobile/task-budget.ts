import type { ClientRequest } from '../client.ts';
import { ApiError, isNamedError } from '../errors.ts';
import { parseTaskBudgetExtension, parseTaskBudgetFacts } from '@weknora/contracts';
import { requireDeploymentOrigin } from './deployment-origin.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface TaskBudgetRemoteOptions {
  /** 部署 Origin：构造即强校验（绝对 HTTPS、无 path/query/fragment、无内嵌凭据）。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输、不持有 token。 */
  request: Request;
}

/** 语义行（api-client 不依赖 mobile-core——依赖方向沿 materials 先例；与 mobile-core
 * TaskBudgetBackendPort 的 TaskBudgetFacts/TaskBudgetExtendReceipt 结构逐字一致，
 * 结构可赋值由 composition 装配处的 apps/mobile typecheck 证明）。 */
export interface MobileTaskBudgetFacts {
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

export interface MobileTaskBudgetExtendReceipt {
  additionalCredits: number;
  resumedRuns: number;
}

/** 与 mobile-core TaskBudgetBackendPort 结构逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface MobileTaskBudgetRemote {
  facts(taskId: string): Promise<MobileTaskBudgetFacts>;
  extend(input: { taskId: string; additionalCredits: number; idempotencyKey: string }): Promise<MobileTaskBudgetExtendReceipt>;
}

function unwrap(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new Error('task budget response must be a success envelope');
  }
  const envelope = value as { success?: unknown; data?: unknown };
  if (envelope.success !== true || !Object.prototype.hasOwnProperty.call(envelope, 'data')) {
    throw new Error('task budget response.success must be true with data');
  }
  const data = envelope.data;
  if (typeof data !== 'object' || data === null || Array.isArray(data)) {
    throw new Error('task budget response data must be an object');
  }
  return data as Record<string, unknown>;
}

/** 跨包契约码（沿 #38 INTERACTION_* 先例）：mobile-core 按 error.code 分类，不得改名。 */
function coded(cause: unknown, code: string): Error {
  const translated = new Error(code, { cause });
  (translated as unknown as { code?: string }).code = code;
  return translated;
}

function translate(error: unknown): unknown {
  // ApiError 以鸭子判定识别（isNamedError）：构造器设 name='ApiError'，与 wire 畸形替身
  // （plain Error + name 属性）同构，避免 instanceof 对跨副本实例/替身失效。
  if (!isNamedError(error, 'ApiError')) return error;
  const apiError = error as ApiError;
  switch (apiError.status) {
    case 400: return coded(apiError, 'TASK_BUDGET_INVALID_INPUT');
    case 403: return coded(apiError, 'TASK_BUDGET_FORBIDDEN');
    case 404: return coded(apiError, 'TASK_BUDGET_NOT_FOUND');
    case 409: return apiError.code === 'TASK_BUDGET_EXPIRED'
      ? coded(apiError, 'TASK_BUDGET_EXPIRED')
      : coded(apiError, 'TASK_BUDGET_INSUFFICIENT');
    default: return error;
  }
}

export function createMobileTaskBudgetRemote(options: TaskBudgetRemoteOptions): MobileTaskBudgetRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  return {
    async facts(taskId) {
      const path = `/api/v1/commercial/tasks/${encodeURIComponent(taskId)}/budget`;
      try {
        const row = parseTaskBudgetFacts(unwrap(await request({ method: 'GET', path })));
        return {
          taskId: row.task_id,
          rootRunId: row.root_run_id,
          limitCredits: row.limit_credits,
          usedCredits: row.used_credits,
          heldCredits: row.held_credits,
          remainingCredits: row.remaining_credits,
          ...(row.deadline === undefined ? {} : { deadline: row.deadline }),
          delegatedRunIds: row.delegated_run_ids,
          pausedRunIds: row.paused_run_ids,
          canExtend: row.can_extend,
        };
      } catch (error) {
        throw translate(error);
      }
    },
    async extend(input) {
      const path = `/api/v1/commercial/tasks/${encodeURIComponent(input.taskId)}/budget/extend`;
      try {
        const row = parseTaskBudgetExtension(unwrap(await request({
          method: 'POST', path,
          body: { additional_credits: input.additionalCredits, idempotency_key: input.idempotencyKey },
        })));
        return { additionalCredits: row.additional_credits, resumedRuns: row.resumed_runs };
      } catch (error) {
        throw translate(error);
      }
    },
  };
}
