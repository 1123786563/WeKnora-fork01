import assert from 'node:assert/strict';
import { test } from 'node:test';
import { ApiError } from '../errors.ts';
import type { ClientRequest } from '../client.ts';
import { createMobileResearchRemote } from './research.ts';

const delegationRow = {
  delegation_id: 'd1', run_id: 'r1', session_id: 's1',
  objective: 'survey', sources: ['kb-1'], status: 'assigned',
  created_at: '2026-09-26T00:00:00Z',
};

const annotationRow = {
  annotation_id: 'an1', run_id: 'r1', material_id: 'm1:0',
  base_version: '9a2f1c3d4e5f6a7b', body: 'b', author_id: 'u3',
  created_at: '2026-09-26T00:00:00Z',
};

test('delegate POSTs to the research endpoint and projects a semantic row', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileResearchRemote({
    origin: 'https://weknora.example.com',
    request: async (input) => {
      requests.push(input);
      return { success: true, data: { delegation: delegationRow } };
    },
  });
  const row = await remote.delegate({ runId: 'r1', objective: 'survey', sources: ['kb-1'] });
  assert.equal(requests[0]!.method, 'POST');
  assert.equal(requests[0]!.path, '/api/v1/workbench/executions/r1/research');
  assert.deepEqual(requests[0]!.body, { objective: 'survey', sources: ['kb-1'] });
  assert.equal(row.delegationId, 'd1');
  assert.equal(row.status, 'assigned');
  assert.equal(row.sessionId, 's1');
});

test('complete posts the summary to the delegation summary endpoint', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileResearchRemote({
    origin: 'https://weknora.example.com',
    request: async (input) => {
      requests.push(input);
      return { success: true, data: { delegation: { ...delegationRow, status: 'completed', summary: 'done' } } };
    },
  });
  const row = await remote.complete({ runId: 'r1', delegationId: 'd1', summary: 'done' });
  assert.equal(requests[0]!.method, 'POST');
  assert.equal(requests[0]!.path, '/api/v1/workbench/executions/r1/research/d1/summary');
  assert.deepEqual(requests[0]!.body, { summary: 'done' });
  assert.equal(row.status, 'completed');
  assert.equal(row.summary, 'done');
});

test('list/annotations GET the run-scoped endpoints and map rows', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileResearchRemote({
    origin: 'https://weknora.example.com',
    request: async (input) => {
      requests.push(input);
      if (input.path.endsWith('/research')) return { success: true, data: { items: [delegationRow] } };
      return { success: true, data: { items: [annotationRow] } };
    },
  });
  const delegations = await remote.list('r1');
  assert.equal(delegations.runId, 'r1');
  assert.equal(delegations.delegations[0]!.delegationId, 'd1');
  const annotations = await remote.annotations('r1');
  assert.equal(annotations.annotations[0]!.baseVersion, '9a2f1c3d4e5f6a7b');
  assert.equal(annotations.annotations[0]!.materialId, 'm1:0');
  assert.deepEqual(requests.map((request) => request.path), [
    '/api/v1/workbench/executions/r1/research',
    '/api/v1/workbench/executions/r1/annotations',
  ]);
});

test('annotate maps a 409 stale base version to RESEARCH_BASE_VERSION_CONFLICT', async () => {
  const remote = createMobileResearchRemote({
    origin: 'https://weknora.example.com',
    request: async () => { throw new ApiError({ status: 409, code: 'annotation_base_version_conflict', message: 'annotation_base_version_conflict' }); },
  });
  await assert.rejects(
    remote.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'stale', body: 'x' }),
    (error: unknown) => (error as { code?: string }).code === 'RESEARCH_BASE_VERSION_CONFLICT',
  );
});

test('generic failures and malformed envelopes map to RESEARCH_BACKEND with the code attached', async () => {
  const remote = createMobileResearchRemote({
    origin: 'https://weknora.example.com',
    request: async () => { throw new ApiError({ status: 500, code: 'research_backend', message: 'research_backend' }); },
  });
  const failure = await remote.list('r1').then(() => undefined, (error: unknown) => error as { code?: string });
  assert.equal(failure?.code, 'RESEARCH_BACKEND');

  const bad = createMobileResearchRemote({ origin: 'https://weknora.example.com', request: async () => ({ success: true }) });
  await assert.rejects(bad.list('r1'), /RESEARCH_BACKEND/);
});
