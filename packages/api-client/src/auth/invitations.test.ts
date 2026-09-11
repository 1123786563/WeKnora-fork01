import test from 'node:test';
import assert from 'node:assert/strict';
import { createInvitationsApi } from './invitations.ts';

test('invitation lookup keeps token in POST body', async () => {
  let call: any;
  const api = createInvitationsApi(async (input) => { call = input; return { success: true, data: { tenant_id: 7, role: 'viewer', expires_at: 'x' } }; });
  const result = await api.lookup('secret-token');
  assert.equal(result.data?.tenant_id, 7);
  assert.equal(call.method, 'POST');
  assert.equal(call.path, '/api/v1/auth/invitations/lookup');
  assert.deepEqual(call.body, { token: 'secret-token' });
  assert.equal(call.path.includes('secret-token'), false);
});
