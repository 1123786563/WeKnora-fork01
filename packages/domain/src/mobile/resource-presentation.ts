/**
 * 资源展示领域模型（MX-022）。
 * 冻结规则：
 * - 撤权（403）后敏感字段立即不可见——展示模型从服务端事实重建，不从缓存回填；
 * - 「用此知识提问」仅以引用方式（knowledge ref）进入新任务，不复制文档本体跨空间；
 * - 收藏归属 scope（origin/user/tenant），与资源权限互不混淆；
 * - 扫描/索引状态文字+颜色双通道展示。
 */

import type { AgentCapabilityState } from './agent-options.ts';

export type KnowledgeScanStatus = 'pending' | 'scanning' | 'indexed' | 'failed';

export interface KnowledgeResource {
  id: string;
  title: string;
  scanStatus: KnowledgeScanStatus;
  documentCount: number;
  updatedAt: string;
}

/** Connection 生命周期（internal/modules/appconnector/model.go:15-19：active/revoked/pending_reauthorization）。 */
export type ConnectionLifecycleState = 'active' | 'revoked' | 'pending_reauthorization' | 'unknown';

export interface ConnectionResource {
  id: string;
  kind: 'personal' | 'space';
  state: ConnectionLifecycleState;
  connected: boolean;
  capability: { state: AgentCapabilityState; reason: string };
}

export interface ResourceFavorites {
  /** 归一化 scope 键（origin/user/tenant，MX-011 normalize） */
  scopeKey: string;
  knowledgeIds: string[];
}

/** 连接能力三态裁决：无状态记录=unavailable（没有能力事实不得放行，不猜测）。 */
export function connectionCapability(state: ConnectionLifecycleState): ConnectionResource['capability'] {
  if (state === 'active') return { state: 'supported', reason: '' };
  if (state === 'revoked') return { state: 'unavailable', reason: 'connection_revoked' };
  if (state === 'pending_reauthorization') return { state: 'unavailable', reason: 'reauthorization_required' };
  return { state: 'unavailable', reason: 'connection_state_not_reported' };
}

/** 服务端连接行 → 展示模型（只提升 id/kind/state 三个字段——wire 视图本无凭据字段，仍做结构性过滤双保险）。 */
export function toConnectionResource(row: Record<string, unknown>): ConnectionResource {
  const state: ConnectionLifecycleState =
    row.state === 'active' || row.state === 'revoked' || row.state === 'pending_reauthorization' ? row.state : 'unknown';
  return {
    id: String(row.id ?? ''),
    kind: row.kind === 'space' ? 'space' : 'personal',
    state,
    connected: state === 'active',
    capability: connectionCapability(state),
  };
}

/** 服务端知识行 → 展示模型（未知扫描状态按 pending 展示——不臆造 indexed）。 */
export function toKnowledgeResource(row: Record<string, unknown>): KnowledgeResource {
  const scan = row.scan_status ?? row.scanStatus;
  const status: KnowledgeScanStatus =
    scan === 'scanning' || scan === 'indexed' || scan === 'failed' ? scan : 'pending';
  return {
    id: String(row.id ?? ''),
    title: String(row.title ?? row.name ?? ''),
    scanStatus: status,
    documentCount: Number(row.document_count ?? row.documentCount ?? 0) || 0,
    updatedAt: String(row.updated_at ?? row.updatedAt ?? ''),
  };
}

export interface KnowledgeAccessError extends Error {
  code: 'forbidden';
}

/** 403 撤权后的展示投影：敏感字段（标题等）全部不可见，仅保留不可用事实。 */
export function revokedKnowledgeProjection(): { visibleSensitiveFields: string[]; canAskWithKnowledge: boolean } {
  return { visibleSensitiveFields: [], canAskWithKnowledge: false };
}

/**
 * 「用此知识提问」输入构造：仅引用（ref=kb id），不携带文档内容；
 * 撤权/未授权时返回 null（按钮禁用原因由调用方展示）。
 */
export function knowledgeRefForPrompt(resource: KnowledgeResource, access: { revoked: boolean }): { kind: 'knowledge_ref'; knowledgeId: string } | null {
  if (access.revoked) return null;
  return { kind: 'knowledge_ref', knowledgeId: resource.id };
}

/** 收藏读写按 scope 键隔离（不同空间互不可见）。 */
export function createScopedFavorites(scopeKey: string): ResourceFavorites & {
  isFavorite(id: string): boolean;
  toggle(id: string): void;
} {
  const ids = new Set<string>();
  return {
    scopeKey,
    get knowledgeIds() {
      return [...ids];
    },
    isFavorite: (id) => ids.has(id),
    toggle(id) {
      if (ids.has(id)) ids.delete(id);
      else ids.add(id);
    },
  };
}

/** 扫描状态展示（文字+tone 双通道）。 */
export function scanStatusPresentation(status: KnowledgeScanStatus): { label: string; tone: 'neutral' | 'brand' | 'warning' | 'danger' } {
  switch (status) {
    case 'indexed': return { label: '已索引', tone: 'brand' };
    case 'scanning': return { label: '扫描中', tone: 'warning' };
    case 'failed': return { label: '扫描失败', tone: 'danger' };
    default: return { label: '待扫描', tone: 'neutral' };
  }
}
