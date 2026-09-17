import assert from 'node:assert/strict';
import test from 'node:test';
import * as nodeModule from 'node:module';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const {
  getKnowledgeSettingsSections,
  getKnowledgeBaseDataSourcesPath,
  getKnowledgeBaseActivityPath,
  getKnowledgeBaseSharesPath,
  summarizeKnowledgeSettings,
  knowledgeSettingsCanEdit,
} = await import('./KnowledgeSettingsPage.tsx');
type KnowledgeSettingsInput = import('./KnowledgeSettingsPage.tsx').KnowledgeSettingsInput;

const documentKnowledgeBase: KnowledgeSettingsInput = {
  id: 'kb-1',
  name: 'Product docs',
  type: 'document',
  chunking_config: {
    parser_engine_rules: [
      { file_types: ['pdf', 'docx'], engine: 'mineru' },
    ],
  },
  vector_store_source: 'tenant',
  vector_store_name: 'Search vectors',
  vector_store_engine_type: 'pgvector',
  vector_store_status: 'ready',
  storage_backend_id: 'storage-1',
  storage_provider_config: { provider: 's3' },
  activity: [{ id: 'activity-1', action: 'updated', outcome: 'success' }],
};

test('exposes Vue-aligned sections and gates document-only sections for FAQ bases', () => {
  // R441: the basic section joins the ported sections (Vue basic group).
  assert.deepEqual(getKnowledgeSettingsSections(documentKnowledgeBase).map((section) => section.key), [
    'basic', 'models', 'multimodal', 'asr', 'vectorStore', 'parser', 'chunking', 'advanced', 'storage', 'datasource', 'share', 'activity', 'graph',
  ]);
  // Vue keeps basic and models in the basic group for FAQ bases (plus the faq
  // section); multimodal/asr and chunking/advanced stay document-only.
  assert.deepEqual(getKnowledgeSettingsSections({ id: 'kb-2', name: 'FAQ', type: 'faq' }).map((section) => section.key), [
    'basic', 'models', 'faq', 'vectorStore', 'datasource', 'share', 'activity',
  ]);
});

test('exposes every Vue knowledge-base detail section in the editor order', () => {
  assert.deepEqual(getKnowledgeSettingsSections(documentKnowledgeBase).map((section) => section.key), [
    'basic', 'models', 'multimodal', 'asr', 'vectorStore', 'parser', 'chunking', 'advanced', 'storage', 'datasource', 'share', 'activity', 'graph',
  ]);
});

test('builds the Vue detail endpoints for datasource and share sections', () => {
  assert.equal(getKnowledgeBaseDataSourcesPath('kb/a'), '/api/v1/datasource?kb_id=kb%2Fa');
  assert.equal(getKnowledgeBaseSharesPath('kb/a'), '/api/v1/knowledge-bases/kb%2Fa/shares');
});

test('summarizes parser, vector, storage, and activity state from the existing KB response', () => {
  assert.deepEqual(summarizeKnowledgeSettings(documentKnowledgeBase), {
    parser: { kind: 'configured', label: 'MinerU', detail: 'PDF, DOCX' },
    vectorStore: { kind: 'ready', label: 'Search vectors', detail: 'pgvector · tenant' },
    storage: { kind: 'configured', label: 'S3', detail: 'storage-1' },
    activity: { kind: 'available', label: '1 recent event', detail: 'updated · success', labelKey: 'kbSettings.summary.activity.countOne', labelParams: { count: 1 } },
    datasource: { kind: 'empty', label: 'No data sources', detail: 'Add an external connector', labelKey: 'kbSettings.summary.datasource.emptyLabel', detailKey: 'kbSettings.summary.datasource.emptyDetail' },
    share: { kind: 'empty', label: 'Not shared', detail: 'No spaces have access', labelKey: 'kbSettings.summary.share.emptyLabel', detailKey: 'kbSettings.summary.share.emptyDetail' },
    graph: { kind: 'default', label: 'Knowledge graph disabled', detail: 'Configure extraction when graph storage is enabled', labelKey: 'kbSettings.summary.graph.disabledLabel', detailKey: 'kbSettings.summary.graph.disabledDetail' },
  });
});

test('represents absent bindings and activity without inventing configuration', () => {
  assert.deepEqual(summarizeKnowledgeSettings({ id: 'kb-3', name: 'Empty', type: 'document' }), {
    // R456: the fallback branches of the parser/vectorStore/storage tiles
    // carry summary keys; data-driven literals stay key-free.
    parser: { kind: 'empty', label: 'Default parser', detail: 'No file-type overrides', labelKey: 'kbSettings.summary.parser.defaultLabel', detailKey: 'kbSettings.summary.parser.noOverridesDetail' },
    vectorStore: { kind: 'default', label: 'System default', detail: 'No explicit binding', labelKey: 'kbSettings.summary.vectorStore.defaultLabel', detailKey: 'kbSettings.summary.vectorStore.noBindingDetail' },
    storage: { kind: 'default', label: 'System default', detail: 'No explicit instance', labelKey: 'kbSettings.summary.storage.defaultLabel', detailKey: 'kbSettings.summary.storage.noInstanceDetail' },
    activity: { kind: 'empty', label: 'No activity yet', detail: 'Changes will appear here', labelKey: 'kbSettings.summary.activity.emptyLabel', detailKey: 'kbSettings.summary.activity.emptyDetail' },
    datasource: { kind: 'empty', label: 'No data sources', detail: 'Add an external connector', labelKey: 'kbSettings.summary.datasource.emptyLabel', detailKey: 'kbSettings.summary.datasource.emptyDetail' },
    share: { kind: 'empty', label: 'Not shared', detail: 'No spaces have access', labelKey: 'kbSettings.summary.share.emptyLabel', detailKey: 'kbSettings.summary.share.emptyDetail' },
    graph: { kind: 'default', label: 'Knowledge graph disabled', detail: 'Configure extraction when graph storage is enabled', labelKey: 'kbSettings.summary.graph.disabledLabel', detailKey: 'kbSettings.summary.graph.disabledDetail' },
  });
});

test('surfaces unavailable vector bindings and uses the existing activity endpoint contract', () => {
  assert.equal(getKnowledgeBaseActivityPath('kb/a'), '/api/v1/knowledge-bases/kb%2Fa/activity?limit=30');
  assert.deepEqual(summarizeKnowledgeSettings({
    id: 'kb-4',
    name: 'Unavailable',
    vector_store_name: 'Search vectors',
    vector_store_status: 'unavailable',
  }).vectorStore, {
    kind: 'unavailable',
    label: 'Search vectors',
    detail: 'Check the global vector-store settings',
    detailKey: 'kbSettings.summary.vectorStore.unavailableDetail',
  });
});

test('knowledge settings editing follows Vue owner/admin capability', () => {
  assert.equal(knowledgeSettingsCanEdit('owner'), true);
  assert.equal(knowledgeSettingsCanEdit('admin'), true);
  assert.equal(knowledgeSettingsCanEdit('viewer'), false);
  assert.equal(knowledgeSettingsCanEdit(undefined), false);
});
