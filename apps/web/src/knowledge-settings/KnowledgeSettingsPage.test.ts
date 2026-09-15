import assert from 'node:assert/strict';
import test from 'node:test';

import {
  getKnowledgeSettingsSections,
  getKnowledgeBaseActivityPath,
  summarizeKnowledgeSettings,
  type KnowledgeSettingsInput,
} from './KnowledgeSettingsPage.tsx';

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
  assert.deepEqual(getKnowledgeSettingsSections(documentKnowledgeBase).map((section) => section.key), [
    'vectorStore', 'parser', 'storage', 'activity',
  ]);
  assert.deepEqual(getKnowledgeSettingsSections({ id: 'kb-2', name: 'FAQ', type: 'faq' }).map((section) => section.key), [
    'vectorStore', 'activity',
  ]);
});

test('summarizes parser, vector, storage, and activity state from the existing KB response', () => {
  assert.deepEqual(summarizeKnowledgeSettings(documentKnowledgeBase), {
    parser: { kind: 'configured', label: 'MinerU', detail: 'PDF, DOCX' },
    vectorStore: { kind: 'ready', label: 'Search vectors', detail: 'pgvector · tenant' },
    storage: { kind: 'configured', label: 'S3', detail: 'storage-1' },
    activity: { kind: 'available', label: '1 recent event', detail: 'updated · success' },
  });
});

test('represents absent bindings and activity without inventing configuration', () => {
  assert.deepEqual(summarizeKnowledgeSettings({ id: 'kb-3', name: 'Empty', type: 'document' }), {
    parser: { kind: 'empty', label: 'Default parser', detail: 'No file-type overrides' },
    vectorStore: { kind: 'default', label: 'System default', detail: 'No explicit binding' },
    storage: { kind: 'default', label: 'System default', detail: 'No explicit instance' },
    activity: { kind: 'empty', label: 'No activity yet', detail: 'Changes will appear here' },
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
  });
});
