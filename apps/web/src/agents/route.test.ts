import assert from 'node:assert/strict';
import test from 'node:test';
import { buildAgentPath, parseAgentRoute, sectionForField } from './route.ts';

test('parses an edit deep-link and normalizes section aliases', () => {
  assert.deepEqual(parseAgentRoute('/platform/agents?edit=a-1&section=models&highlight=rerank_model'), {
    editId: 'a-1', section: 'model', highlight: 'rerank_model', sourceTenantId: null,
  });
  assert.deepEqual(parseAgentRoute('/platform/agents?edit=a-1&section=integration-embed'), {
    editId: 'a-1', section: 'integration-embed', highlight: null, sourceTenantId: null,
  });
});

test('ignores unrelated paths and safely builds links', () => {
  assert.equal(parseAgentRoute('/platform/knowledge-bases?edit=a-1'), null);
  assert.equal(buildAgentPath('a/1', 'prompts', 'tenant 2'), '/platform/agents?edit=a%2F1&section=prompts&sourceTenantId=tenant+2');
});

test('maps readiness fields to the owning editor section', () => {
  assert.equal(sectionForField('summary_model'), 'model');
  assert.equal(sectionForField('rerank_model'), 'model');
  assert.equal(sectionForField('allowed_tools'), 'tools');
  assert.equal(sectionForField('unknown'), 'basic');
});
