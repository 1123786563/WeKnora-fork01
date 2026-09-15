import assert from 'node:assert/strict';
import test from 'node:test';
import { canEditKnowledgeBase, canManageKnowledgeBase, canMutateKnowledge, knowledgeBaseCapabilities } from './access.ts';

test('knowledge base capabilities follow Vue feature flags and type', () => {
  assert.deepEqual(knowledgeBaseCapabilities({ type: 'faq', indexing_strategy: { wiki_enabled: true, graph_enabled: true } }), {
    isFaq: true, wikiEnabled: false, graphEnabled: false,
  });
  assert.deepEqual(knowledgeBaseCapabilities({ type: 'document', indexing_strategy: { wiki_enabled: true, graph_enabled: true } }), {
    isFaq: false, wikiEnabled: true, graphEnabled: true,
  });
});

test('shared knowledge base permissions do not inherit the local workspace role', () => {
  assert.equal(canEditKnowledgeBase({ permission: 'editor', viaShare: true, workspaceRole: 'viewer' }), true);
  assert.equal(canEditKnowledgeBase({ permission: 'viewer', viaShare: true, workspaceRole: 'admin' }), false);
  assert.equal(canManageKnowledgeBase({ permission: 'editor', viaShare: true, workspaceRole: 'admin' }), false);
  assert.equal(canManageKnowledgeBase({ permission: 'admin', viaShare: true, workspaceRole: 'viewer' }), true);
});

test('document mutations require both KB edit access and contributor-level workspace access', () => {
  assert.equal(canMutateKnowledge({ permission: 'editor', viaShare: true, workspaceRole: 'viewer' }), true);
  assert.equal(canMutateKnowledge({ permission: 'editor', viaShare: false, workspaceRole: 'viewer' }), false);
  assert.equal(canMutateKnowledge({ permission: 'editor', viaShare: false, workspaceRole: 'contributor' }), true);
});
