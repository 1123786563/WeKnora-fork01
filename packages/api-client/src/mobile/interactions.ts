import {
  ContractError,
  parseInteraction,
  parseInteractionDecision,
  type InteractionRecord,
} from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

/**
 * 类型化交互 SDK（MX-005）：list/decide 适配 A 类现网路由。
 * 决定 body 与 packages/contracts InteractionRecord 同构（MX-003 冻结）；
 * 错误语义：409=revision/digest 冲突（不自动重发旧决定）、403=无权（不清 token、
 * 由调用方隐藏敏感字段）、unknown 状态不得伪装为 failed 后重试。
 */
export interface InteractionDecisionInput {
  id: string;
  decision_id: string;
  kind: string;
  action: string;
  args_hash: string;
  expected_revision: number;
}

type Request = (input: ClientRequest) => Promise<unknown>;

function pathId(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return encodeURIComponent(value);
}

function unwrap(value: unknown): unknown {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new Error('interaction response must be a success envelope');
  }
  const envelope = value as Record<string, unknown>;
  if (envelope.success !== true) throw new Error('interaction response.success must be true');
  if (!Object.prototype.hasOwnProperty.call(envelope, 'data')) {
    throw new Error('interaction response.data is required');
  }
  return envelope.data;
}

function parseItems(value: unknown, runID?: string): InteractionRecord[] {
  if (!Array.isArray(value)) throw new ContractError('items', 'expected an array');
  return value.map((item) => {
    const record = parseInteraction(item);
    if (runID !== undefined && record.id === '') throw new ContractError('id', 'expected non-empty');
    return record;
  });
}

export function createInteractionsApi(request: Request) {
  return {
    /** GET /workbench/executions/:run_id/interactions —— 当前交互列表（pending 项 decision_id 为空串）。 */
    async list(runID: string): Promise<InteractionRecord[]> {
      const data = unwrap(await request({ method: 'GET', path: `/workbench/executions/${pathId(runID, 'runID')}/interactions` }));
      return parseItems(data);
    },
    /**
     * POST /workbench/executions/interactions/:id/decisions。
     * 请求前本地冻结决定并校验 kind×action 矩阵；服务端 CAS 落地。
     * ACK 仅表示决定已记录，不代表外部操作成功——调用方须按返回值或重读更新交互。
     */
    async decide(input: InteractionDecisionInput): Promise<InteractionRecord> {
      const body = parseInteractionDecision(input);
      const data = unwrap(await request({
        method: 'POST',
        path: `/workbench/executions/interactions/${pathId(input.id, 'id')}/decisions`,
        body,
      }));
      return parseInteraction(data);
    },
  };
}
