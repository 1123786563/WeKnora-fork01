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
  { key: 'processing', labelKey: 'knowledgeEditor.navGroups.processing', items: ['parser', 'chunking', 'multimodal', 'asr', 'graph', 'advanced'] },
  { key: 'data', labelKey: 'knowledgeEditor.navGroups.data', items: ['storage', 'datasource'] },
  { key: 'integration', labelKey: 'knowledgeEditor.navGroups.integration', items: ['share'] },
  { key: 'management', labelKey: 'knowledgeEditor.navGroups.management', items: ['activity'] },
];

export function visibleKnowledgeEditorSections(input: {
  type: 'document' | 'faq';
  editing: boolean;
}): KnowledgeEditorSectionGroup[] {
  return KNOWLEDGE_EDITOR_SECTION_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter((section) => {
      if (input.type === 'faq' && !['basic', 'models', 'faq'].includes(section)) return false;
      if (!input.editing && ['datasource', 'share', 'activity'].includes(section)) return false;
      return true;
    }),
  })).filter((group) => group.items.length > 0);
}

export function normalizeKnowledgeEditorSection(
  section: KnowledgeEditorSection,
  input: { type: 'document' | 'faq'; editing: boolean },
): KnowledgeEditorSection {
  return visibleKnowledgeEditorSections(input).some((group) => group.items.includes(section)) ? section : 'basic';
}
