/**
 * 资源展示领域模型（MX-022）。
 * 冻结规则：
 * - 撤权（403）后敏感字段立即不可见——展示模型从服务端事实重建，不从缓存回填；
 * - 「用此知识提问」仅以引用方式（knowledge ref）进入新任务，不复制文档本体跨空间；
 * - 收藏归属 scope（origin/user/tenant），与资源权限互不混淆；
 * - 扫描/索引状态文字+颜色双通道展示。
 */

export type KnowledgeScanStatus = 'pending' | 'scanning' | 'indexed' | 'failed';

export interface KnowledgeResource {
  id: string;
  title: string;
  scanStatus: KnowledgeScanStatus;
  documentCount: number;
  updatedAt: string;
}

export interface ConnectionResource {
  id: string;
  name: string;
  connected: boolean;
}

export interface ResourceFavorites {
  /** 归一化 scope 键（origin/user/tenant，MX-011 normalize） */
  scopeKey: string;
  knowledgeIds: string[];
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
