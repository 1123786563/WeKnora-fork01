import assert from 'node:assert/strict';
import test from 'node:test';
import { createConfigurationApi } from './configuration.ts';

const row = { id: 'model/1', name: 'Model', parameters: { api_key: 'must-not-return', base_url: 'https://model.test' } };

test('uses typed configuration routes and strips secret fields from returned records', async () => {
  const requests: unknown[] = [];
  const api = createConfigurationApi(async (request) => {
    requests.push(request);
    return { success: true, data: [row], total: 1 };
  });
  const models = await api.models.list();
  assert.equal(models[0]?.id, 'model/1');
  assert.equal((models[0]?.parameters as Record<string, unknown>)?.api_key, undefined);
  assert.deepEqual(requests, [{ method: 'GET', path: '/api/v1/models' }]);
});

test('rejects malformed lists and never turns them into empty configuration', async () => {
  const api = createConfigurationApi(async () => ({ success: false, data: [] }));
  await assert.rejects(() => api.agents.list(), /successful list envelope/);
});

test('encodes ids and preserves explicit write failures', async () => {
  const requests: unknown[] = [];
  const api = createConfigurationApi(async (request) => {
    requests.push(request);
    return request.method === 'DELETE' ? { success: true } : { success: true, data: { ...row, id: 'model/1', name: 'Updated' } };
  });
  assert.equal((await api.models.update('model/1', { name: 'Updated' })).name, 'Updated');
  await api.models.remove('model/1');
  assert.equal((requests[0] as { path: string }).path, '/api/v1/models/model%2F1');
  await assert.rejects(() => api.models.get(''), /must not be empty/);
});
