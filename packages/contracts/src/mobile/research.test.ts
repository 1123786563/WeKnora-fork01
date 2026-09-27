import assert from 'node:assert/strict';
import { test } from 'node:test';
import { parseAnnotationListResponse, parseResearchListResponse } from './research.ts';

const delegation = {
  delegation_id: 'd1', run_id: 'r1', session_id: 's1',
  objective: 'survey retrieval baselines', sources: ['kb-1'],
  status: 'assigned', created_at: '2026-09-26T00:00:00Z',
};

test('parseResearchListResponse accepts a real envelope', () => {
  const parsed = parseResearchListResponse({ success: true, data: { items: [delegation, { ...delegation, delegation_id: 'd2', status: 'completed', summary: '3 findings' }] } });
  assert.equal(parsed.items.length, 2);
  assert.equal(parsed.items[1].status, 'completed');
  assert.equal(parsed.items[1].summary, '3 findings');
});

test('parseResearchListResponse rejects malformed envelopes wholesale', () => {
  for (const bad of [
    undefined, null, 42, 'x', [],
    {}, { success: false, data: { items: [] } },
    { success: true }, { success: true, data: null },
    { success: true, data: {} },
    { success: true, data: { items: 'nope' } },
    { success: true, data: { items: [{ ...delegation, delegation_id: 7 }] } },
    { success: true, data: { items: [{ ...delegation, sources: 'kb-1' }] } },
    { success: true, data: { items: [{ ...delegation, status: 'running' }] } },
    { success: true, data: { items: [{ ...delegation, created_at: undefined }] } },
    { success: true, data: { items: [{ ...delegation, objective: null }] } },
  ]) {
    assert.throws(() => parseResearchListResponse(bad), `must reject: ${JSON.stringify(bad)}`);
  }
});

test('parseAnnotationListResponse accepts a real envelope and rejects malformed rows', () => {
  const annotation = {
    annotation_id: 'an1', run_id: 'r1', material_id: 'm1:0',
    base_version: '9a2f1c3d4e5f6a7b', body: '结论第三段缺引用',
    author_id: 'u3', created_at: '2026-09-26T00:00:00Z',
  };
  const parsed = parseAnnotationListResponse({ success: true, data: { items: [annotation] } });
  assert.equal(parsed.items[0].base_version, '9a2f1c3d4e5f6a7b');
  assert.equal(parsed.items[0].body, '结论第三段缺引用');

  for (const bad of [
    { success: true, data: { items: [{ ...annotation, material_id: '' }] } },
    { success: true, data: { items: [{ ...annotation, base_version: 12 }] } },
    { success: true, data: { items: [{ ...annotation, body: undefined }] } },
    { success: true, data: { items: [annotation, null] } },
  ]) {
    assert.throws(() => parseAnnotationListResponse(bad), `must reject: ${JSON.stringify(bad)}`);
  }
});
