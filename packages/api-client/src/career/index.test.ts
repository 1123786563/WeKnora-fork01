import assert from 'node:assert/strict';
import test from 'node:test';
import { createCareerApi } from './index.ts';
import type { CareerScope } from '@weknora/contracts';
const scope: CareerScope = { deploymentOrigin: 'https://weknora.example', tenantId: 'tenant-1', actorId: 'actor-1' };
const validProfile = { id: 'p1', revision: 1, facts: [] };

test('Career API decodes response and rejects authority fields and mismatched receipts', async () => {
  const api = createCareerApi(async () => ({ success: true, data: validProfile }));
  assert.equal((await api.getProfile(scope)).id, 'p1');
  const spoof = createCareerApi(async () => ({ success: true, data: { ...validProfile, tenant_id: 't2' } }));
  await assert.rejects(spoof.getProfile(scope), /tenant/i);
  const mismatch = createCareerApi(async () => ({ success: true, requestId: 'other', data: validProfile }));
  await assert.rejects(mismatch.updateProfile(scope, { requestId: 'r1', expectedRevision: 1, facts: [] }), /request/i);
});
