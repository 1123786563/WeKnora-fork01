export interface KnowledgeBaseAccessInput {
  permission?: unknown;
  viaShare?: boolean;
  workspaceRole?: unknown;
}

function normalized(value: unknown): string { return typeof value === 'string' ? value.trim().toLowerCase() : ''; }

export function knowledgeBaseCapabilities(record: Record<string, unknown>): { isFaq: boolean; wikiEnabled: boolean; graphEnabled: boolean } {
  const strategy = record.indexing_strategy && typeof record.indexing_strategy === 'object' && !Array.isArray(record.indexing_strategy)
    ? record.indexing_strategy as Record<string, unknown> : {};
  const isFaq = normalized(record.type) === 'faq';
  return { isFaq, wikiEnabled: !isFaq && strategy.wiki_enabled === true, graphEnabled: !isFaq && strategy.graph_enabled === true };
}

/** Graph is its own Vue feature flag; it must not inherit Wiki availability. */
export function canOpenKnowledgeGraph(record: Record<string, unknown>): boolean {
  return knowledgeBaseCapabilities(record).graphEnabled;
}

export function canEditKnowledgeBase(input: KnowledgeBaseAccessInput): boolean {
  const permission = normalized(input.permission);
  if (input.viaShare) return permission === 'owner' || permission === 'admin' || permission === 'editor';
  return permission === 'owner' || permission === 'admin' || permission === 'editor' || ['owner', 'admin'].includes(normalized(input.workspaceRole));
}

export function canManageKnowledgeBase(input: KnowledgeBaseAccessInput): boolean {
  const permission = normalized(input.permission);
  if (input.viaShare) return permission === 'admin' || permission === 'owner';
  return permission === 'owner' || permission === 'admin' || ['owner', 'admin'].includes(normalized(input.workspaceRole));
}

export function canMutateKnowledge(input: KnowledgeBaseAccessInput): boolean {
  if (!canEditKnowledgeBase(input)) return false;
  if (input.viaShare) return true;
  return ['owner', 'admin', 'contributor'].includes(normalized(input.workspaceRole));
}
