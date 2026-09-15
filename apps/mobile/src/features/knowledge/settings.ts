export interface KnowledgeBaseSettingsDraft { name: string; description: string; wikiEnabled: boolean; graphEnabled: boolean; }

export function knowledgeBaseSettingsDraft(record: Record<string, unknown>): KnowledgeBaseSettingsDraft {
  const strategy = record.indexing_strategy && typeof record.indexing_strategy === 'object' && !Array.isArray(record.indexing_strategy) ? record.indexing_strategy as Record<string, unknown> : {};
  return { name: typeof record.name === 'string' ? record.name : '', description: typeof record.description === 'string' ? record.description : '', wikiEnabled: strategy.wiki_enabled === true, graphEnabled: strategy.graph_enabled === true };
}

export function validateKnowledgeBaseSettings(draft: Pick<KnowledgeBaseSettingsDraft, 'name'>): string | null { return draft.name.trim() ? null : 'knowledgeEditor.messages.nameRequired'; }
