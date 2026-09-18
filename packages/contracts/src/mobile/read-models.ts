import { ContractError } from '../index.ts';
import type { Capability } from './execution.ts';

// B 类聚合读模型提案（docs/design/mobile-v2/docs/04-api-contracts.md §2）。
// 冻结形状供 MX-013/014/021 实现；服务端路由/DI 接通前不得在产品入口发布。
// 所有 as_of 为服务端时间；cursor 由服务端签名携带排序边界与空间绑定，
// 客户端不可构造、不可跨空间复用。

export interface WorkbenchBootstrap {
  actor: string;
  memberships: WorkspaceMembership[];
  selected_tenant_id: string;
  capabilities: Record<string, Capability>;
  limits: Record<string, number>;
  protocol: number;
  as_of: string;
}

export interface WorkspaceMembership {
  tenant_id: string;
  tenant_name: string;
  role: string;
}

export interface WorkbenchOverview {
  counts: { active_runs: number; pending_interactions: number; unread_notifications: number };
  in_progress: ExecutionSummary[];
  pending_interactions: InteractionSummary[];
  recent_artifacts: ArtifactSummary[];
  as_of: string;
}

export interface ExecutionSummary {
  run_id: string;
  session_id: string;
  title: string;
  run_status: string;
  execution_status: string;
  settlement_status: string;
  updated_at: string;
}

export interface InteractionSummary {
  id: string;
  kind: string;
  /** 标题可缺省（服务端当前仅提供 kind+created_at；展示层以 kind 语义兜底）。 */
  title?: string;
  created_at: string;
}

export interface ArtifactSummary {
  artifact_id: string;
  title: string;
  kind: string;
  produced_at: string;
}

export interface ExecutionListPage {
  items: ExecutionSummary[];
  next_cursor: string | null;
  as_of: string;
}

export interface InboxPage {
  items: InboxItem[];
  unread_count: number;
  next_cursor: string | null;
  as_of: string;
}

export interface InboxItem {
  notification_id: string;
  kind: string;
  title: string;
  body: string;
  created_at: string;
  read: boolean;
  deep_link: string | null;
}

function object(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError(path, 'expected an object');
  }
  return value as Record<string, unknown>;
}

function nonEmpty(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new ContractError(path, 'expected a non-empty string');
  return value;
}

function stringOrNull(value: unknown, path: string): string | null {
  if (value === null) return null;
  return nonEmpty(value, path);
}

function count(value: unknown, path: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) {
    throw new ContractError(path, 'expected a non-negative safe integer');
  }
  return value;
}

function executionSummary(value: unknown, path: string): ExecutionSummary {
  const row = object(value, path);
  return {
    run_id: nonEmpty(row.run_id, `${path}.run_id`),
    session_id: nonEmpty(row.session_id, `${path}.session_id`),
    title: typeof row.title === 'string' ? row.title : '',
    run_status: nonEmpty(row.run_status, `${path}.run_status`),
    execution_status: nonEmpty(row.execution_status, `${path}.execution_status`),
    settlement_status: nonEmpty(row.settlement_status, `${path}.settlement_status`),
    updated_at: nonEmpty(row.updated_at, `${path}.updated_at`),
  };
}

export function parseWorkbenchOverview(value: unknown): WorkbenchOverview {
  const row = object(value, '');
  const counts = object(row.counts, 'counts');
  return {
    counts: {
      active_runs: count(counts.active_runs, 'counts.active_runs'),
      pending_interactions: count(counts.pending_interactions, 'counts.pending_interactions'),
      unread_notifications: count(counts.unread_notifications, 'counts.unread_notifications'),
    },
    in_progress: Array.isArray(row.in_progress) ? row.in_progress.map((item, i) => executionSummary(item, `in_progress[${i}]`)) : (() => { throw new ContractError('in_progress', 'expected an array'); })(),
    pending_interactions: Array.isArray(row.pending_interactions) ? row.pending_interactions.map((item, i) => {
      const r = object(item, `pending_interactions[${i}]`);
      return {
        id: nonEmpty(r.id, `pending_interactions[${i}].id`),
        kind: nonEmpty(r.kind, `pending_interactions[${i}].kind`),
        title: typeof r.title === 'string' ? r.title : '',
        created_at: nonEmpty(r.created_at, `pending_interactions[${i}].created_at`),
      };
    }) : (() => { throw new ContractError('pending_interactions', 'expected an array'); })(),
    recent_artifacts: Array.isArray(row.recent_artifacts) ? row.recent_artifacts.map((item, i) => {
      const r = object(item, `recent_artifacts[${i}]`);
      return {
        artifact_id: nonEmpty(r.artifact_id, `recent_artifacts[${i}].artifact_id`),
        title: nonEmpty(r.title, `recent_artifacts[${i}].title`),
        kind: nonEmpty(r.kind, `recent_artifacts[${i}].kind`),
        produced_at: nonEmpty(r.produced_at, `recent_artifacts[${i}].produced_at`),
      };
    }) : (() => { throw new ContractError('recent_artifacts', 'expected an array'); })(),
    as_of: nonEmpty(row.as_of, 'as_of'),
  };
}

export function parseExecutionListPage(value: unknown): ExecutionListPage {
  const row = object(value, '');
  if (!Array.isArray(row.items)) throw new ContractError('items', 'expected an array');
  return {
    items: row.items.map((item, i) => executionSummary(item, `items[${i}]`)),
    next_cursor: stringOrNull(row.next_cursor, 'next_cursor'),
    as_of: nonEmpty(row.as_of, 'as_of'),
  };
}

export function parseInboxPage(value: unknown): InboxPage {
  const row = object(value, '');
  if (!Array.isArray(row.items)) throw new ContractError('items', 'expected an array');
  return {
    items: row.items.map((item, i) => {
      const r = object(item, `items[${i}]`);
      if (typeof r.read !== 'boolean') throw new ContractError(`items[${i}].read`, 'expected a boolean');
      return {
        notification_id: nonEmpty(r.notification_id, `items[${i}].notification_id`),
        kind: nonEmpty(r.kind, `items[${i}].kind`),
        title: nonEmpty(r.title, `items[${i}].title`),
        body: typeof r.body === 'string' ? r.body : '',
        created_at: nonEmpty(r.created_at, `items[${i}].created_at`),
        read: r.read,
        deep_link: stringOrNull(r.deep_link, `items[${i}].deep_link`),
      };
    }),
    unread_count: count(row.unread_count, 'unread_count'),
    next_cursor: stringOrNull(row.next_cursor, 'next_cursor'),
    as_of: nonEmpty(row.as_of, 'as_of'),
  };
}
