export type KnowledgeEditorSection =
  | 'basic' | 'models' | 'vectorStore' | 'faq' | 'parser' | 'chunking'
  | 'multimodal' | 'asr' | 'graph' | 'advanced' | 'storage' | 'datasource'
  | 'share' | 'activity';

export type KnowledgeEditorSectionGroup = {
  key: 'basic' | 'processing' | 'data' | 'integration' | 'management';
  labelKey: string;
  items: KnowledgeEditorSection[];
};

// Source of truth: frontend/src/views/knowledge/KnowledgeBaseEditorModal.vue
// navGroups. Keep this pure so visibility rules can be tested without React or
// a browser and so the React shell cannot silently drift from Vue's topology.
export const KNOWLEDGE_EDITOR_SECTION_GROUPS: readonly KnowledgeEditorSectionGroup[] = [
  { key: 'basic', labelKey: 'knowledgeEditor.navGroups.basic', items: ['basic', 'models', 'vectorStore', 'faq'] },
  // Vue navGroups picks these keys in this exact order from navItems.
  { key: 'processing', labelKey: 'knowledgeEditor.navGroups.processing', items: ['parser', 'chunking', 'multimodal', 'asr', 'graph', 'advanced'] },
  { key: 'data', labelKey: 'knowledgeEditor.navGroups.data', items: ['storage', 'datasource'] },
  { key: 'integration', labelKey: 'knowledgeEditor.navGroups.integration', items: ['share'] },
  { key: 'management', labelKey: 'knowledgeEditor.navGroups.management', items: ['activity'] },
];

export function visibleKnowledgeEditorSections(input: {
  type: 'document' | 'faq';
  editing: boolean;
  canShare?: boolean;
  canViewActivity?: boolean;
  isLiteMode?: boolean;
  canManageDatasource?: boolean;
}): KnowledgeEditorSectionGroup[] {
  const canShare = input.editing && input.canShare !== false && input.isLiteMode !== true;
  // Vue adds activity only after its owner/admin gate resolves. An omitted
  // The current React entry opens this editor only after the same owner/admin
  // mutation gate used by Vue. Preserve that existing call shape when no
  // explicit permission result is available, while allowing callers that have
  // resolved the gate to hide the section with false.
  const canViewActivity = input.editing && input.canViewActivity !== false;
  const allowed = new Set<KnowledgeEditorSection>([
    'basic', 'models', 'vectorStore',
    ...(input.type === 'faq'
      ? ['faq' as const]
      : ['parser', 'multimodal', 'asr', 'graph', 'advanced', 'storage', 'chunking'] as const),
    ...(input.type === 'document' && input.editing && input.canManageDatasource !== false ? ['datasource' as const] : []),
    ...(canShare ? ['share' as const] : []),
    ...(canViewActivity ? ['activity' as const] : []),
  ]);
  return KNOWLEDGE_EDITOR_SECTION_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter((section) => allowed.has(section)),
  })).filter((group) => group.items.length > 0);
}

export function normalizeKnowledgeEditorSection(
  section: KnowledgeEditorSection,
  input: Parameters<typeof visibleKnowledgeEditorSections>[0],
): KnowledgeEditorSection {
  return visibleKnowledgeEditorSections(input).some((group) => group.items.includes(section)) ? section : 'basic';
}
