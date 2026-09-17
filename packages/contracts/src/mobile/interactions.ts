import { ContractError } from '../index.ts';

// 冻结的交互域与动作矩阵 —— 镜像 internal/workbench/interaction.go ValidateInteractionAction。
// 域不相交：接受工具不授予预算，扩展预算不解决恢复提示（Go 侧注释语义原样保留）。
export type InteractionKind = 'tool_approval' | 'budget' | 'recovery';
export type InteractionAction = 'approve' | 'reject' | 'extend' | 'retry' | 'provide_result' | 'terminate';

export const INTERACTION_ACTIONS_BY_KIND: Readonly<Record<InteractionKind, readonly InteractionAction[]>> = {
  tool_approval: ['approve', 'reject'],
  budget: ['extend'],
  recovery: ['retry', 'provide_result', 'terminate'],
};

// A 类现网 wire：GET interactions 列表项与 POST decisions body 同为
// workbench.InteractionDecision（id/decision_id/kind/action/args_hash/expected_revision）。
// B 类提案中的 content_digest / budget authorized_upper / question answer / connection
// authorize 字段未经服务端注册，不进入本结构（决策 D-012）。
export interface InteractionRecord {
  id: string;
  decision_id: string;
  kind: InteractionKind;
  action: InteractionAction;
  args_hash: string;
  expected_revision: number;
}

function nonEmpty(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new ContractError(path, 'expected a non-empty string');
  return value;
}

function kind(value: unknown, path: string): InteractionKind {
  const raw = nonEmpty(value, path);
  if (raw !== 'tool_approval' && raw !== 'budget' && raw !== 'recovery') {
    throw new ContractError(path, 'unknown interaction kind');
  }
  return raw;
}

function action(value: unknown, path: string): InteractionAction {
  const raw = nonEmpty(value, path);
  const allowed: readonly InteractionAction[] = ['approve', 'reject', 'extend', 'retry', 'provide_result', 'terminate'];
  if (!allowed.includes(raw)) throw new ContractError(path, 'unknown interaction action');
  return raw;
}

/** kind 与 action 不允许互换（预算域不能发 approve 等）。 */
export function interactionActionAllowed(value: InteractionKind, act: InteractionAction): boolean {
  return INTERACTION_ACTIONS_BY_KIND[value].includes(act);
}

/** 列表项解析：结构完整 + kind/action 矩阵一致；decision_id/args_hash 允许为空串（pending 项）。 */
export function parseInteraction(value: unknown): InteractionRecord {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('', 'expected an object');
  }
  const row = value as Record<string, unknown>;
  const parsedKind = kind(row.kind, 'kind');
  const parsedAction = action(row.action, 'action');
  if (!interactionActionAllowed(parsedKind, parsedAction)) {
    throw new ContractError('action', `action ${parsedAction} is not valid for kind ${parsedKind}`);
  }
  const revision = row.expected_revision;
  if (typeof revision !== 'number' || !Number.isSafeInteger(revision) || revision < 0) {
    throw new ContractError('expected_revision', 'expected a non-negative safe integer');
  }
  return {
    id: nonEmpty(row.id, 'id'),
    decision_id: typeof row.decision_id === 'string' ? row.decision_id : (() => { throw new ContractError('decision_id', 'expected a string'); })(),
    kind: parsedKind,
    action: parsedAction,
    args_hash: typeof row.args_hash === 'string' ? row.args_hash : (() => { throw new ContractError('args_hash', 'expected a string'); })(),
    expected_revision: revision,
  };
}

/** 决定 body 解析：decision_id 与 args_hash 必填非空（镜像 workbench_commands.go handler 校验）。 */
export function parseInteractionDecision(value: unknown): InteractionRecord {
  const record = parseInteraction(value);
  if (record.decision_id.trim() === '') throw new ContractError('decision_id', 'decision_id is required');
  if (record.args_hash.trim() === '') throw new ContractError('args_hash', 'args_hash is required');
  return record;
}
