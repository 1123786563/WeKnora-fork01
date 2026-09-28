import assert from 'node:assert/strict';
import test from 'node:test';
import { createCareerApi } from './index.ts';

test('Career API decodes response and rejects authority fields and mismatched receipts', async () => {
  const api = createCareerApi(async () => ({ requestId: 'r1', data: { id: 'p1', revision: 1, facts: [] } }));
  assert.equal((await api.getProfile()).id, 'p1');
  const spoof = createCareerApi(async () => ({ requestId: 'r1', data: { id: 'p1', revision: 1, facts: [], tenantId: 't2' } }));
  await assert.rejects(spoof.getProfile(), /tenant/i);
  const mismatch = createCareerApi(async () => ({ requestId: 'other', data: { id: 'p1', revision: 1, facts: [] } }));
  await assert.rejects(mismatch.getProfile('r1'), /request/i);
});
