import assert from 'node:assert/strict';
import test from 'node:test';
import { knowledgeBaseSettingsDraft, validateKnowledgeBaseSettings } from './settings.ts';

test('native knowledge-base settings draft preserves Vue name, description and indexing flags', () => {
  assert.deepEqual(knowledgeBaseSettingsDraft({ name: ' Docs ', description: 'Desc', indexing_strategy: { wiki_enabled: true, graph_enabled: false } }), { name: ' Docs ', description: 'Desc', wikiEnabled: true, graphEnabled: false });
});

test('native knowledge-base settings rejects a blank name before PUT', () => {
  assert.equal(validateKnowledgeBaseSettings({ name: ' ' }), 'knowledgeEditor.messages.nameRequired');
  assert.equal(validateKnowledgeBaseSettings({ name: 'Docs' }), null);
});
