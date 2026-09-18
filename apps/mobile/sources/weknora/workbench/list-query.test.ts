import test from 'node:test';
import assert from 'node:assert/strict';
import { buildExecutionListQuery } from './list-query.ts';

test('filter encodes agent ID without introducing another parameter', () => {
  const q = buildExecutionListQuery({ agentID: 'a&tenant_id=other', status: 'running' });
  const p = new URLSearchParams(q);
  assert.equal(p.get('agent_id'), 'a&tenant_id=other');
  assert.equal(p.has('tenant_id'), false);
});

test('empty filter produces an empty query without reserved keys', () => {
  const p = new URLSearchParams(buildExecutionListQuery({}));
  assert.equal(p.toString(), '');
  assert.equal(p.has('tenant_id'), false);
  assert.equal(p.has('owner_id'), false);
});

test('cursor and limit round-trip as their own parameters', () => {
  const p = new URLSearchParams(buildExecutionListQuery({ cursor: 'eyJ2IjoxfQ==', limit: 100 }));
  assert.equal(p.get('cursor'), 'eyJ2IjoxfQ==');
  assert.equal(p.get('limit'), '100');
  assert.equal(p.getAll('status').length, 0);
});

test('empty optional values are omitted instead of sent blank', () => {
  const p = new URLSearchParams(buildExecutionListQuery({ status: '', agentID: undefined, cursor: '  ' }));
  assert.equal(p.toString(), '');
});
