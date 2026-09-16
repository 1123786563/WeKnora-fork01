import assert from 'node:assert/strict';
import test from 'node:test';
import { createSandboxConfigurationsApi, parseSandboxConfigurationConflict } from './sandbox-configurations.ts';

const config = {
  sandbox_type: 'e2b',
  e2b: {
    api_key: '<redacted>',
    api_url: 'https://api.e2b.app',
    template_id: 'template-1',
  },
};

const record = {
  id: 'sandbox/1',
  name: 'Remote sandboxes',
  description: 'Workspace execution',
  sandbox_type: 'e2b',
  config,
  created_at: '2030-01-01T00:00:00Z',
  updated_at: '2030-01-01T00:00:00Z',
};

test('sandbox configuration API maps exact routes and preserves masked secrets', async () => {
  const requests: unknown[] = [];
  const api = createSandboxConfigurationsApi(async (request) => {
    requests.push(request);
    switch (request.method) {
      case 'GET':
        if (request.path === '/api/v1/sandbox-configs') {
          return { success: true, data: [record], workspace_scripts_disabled: true };
        }
        if (request.path.endsWith('/sandboxes')) {
          return { success: true, data: { sandbox_count: 2, session_ids: ['session-1'], agent_names: ['Research'] } };
        }
        return { success: true, data: record };
      case 'PUT':
        return { success: true, data: record, workspace_scripts_disabled: false };
      case 'POST':
        return { success: true, data: record };
      case 'DELETE':
        return { success: true };
      default:
        throw new Error('unexpected method');
    }
  });

  assert.deepEqual(await api.list(), { items: [{ ...record, config }], workspaceScriptsDisabled: true });
  assert.deepEqual(await api.inventory('sandbox/1'), {
    sandboxCount: 2,
    sessionIds: ['session-1'],
    agentNames: ['Research'],
    unverifiable: false,
  });
  assert.equal((await api.get('sandbox/1')).id, 'sandbox/1');
  assert.equal((await api.create({ name: 'New', config })).id, 'sandbox/1');
  assert.equal((await api.update('sandbox/1', { name: 'Updated', config })).id, 'sandbox/1');
  assert.equal((await api.setWorkspacePolicy(true)).workspaceScriptsDisabled, false);
  await api.remove('sandbox/1');
  await api.remove('sandbox/1', undefined, true);

  assert.deepEqual(requests, [
    { method: 'GET', path: '/api/v1/sandbox-configs' },
    { method: 'GET', path: '/api/v1/sandbox-configs/sandbox%2F1/sandboxes' },
    { method: 'GET', path: '/api/v1/sandbox-configs/sandbox%2F1' },
    { method: 'POST', path: '/api/v1/sandbox-configs', body: { name: 'New', config } },
    { method: 'PUT', path: '/api/v1/sandbox-configs/sandbox%2F1', body: { name: 'Updated', config } },
    { method: 'PUT', path: '/api/v1/sandbox-configs/workspace-policy', body: { scripts_disabled: true } },
    { method: 'DELETE', path: '/api/v1/sandbox-configs/sandbox%2F1' },
    { method: 'DELETE', path: '/api/v1/sandbox-configs/sandbox%2F1?force=true' },
  ]);
});

test('sandbox configuration API fails closed on malformed envelopes and empty ids', async () => {
  const api = createSandboxConfigurationsApi(async () => ({ success: false, data: [] }));
  await assert.rejects(() => api.list(), /successful list envelope/);
  await assert.rejects(() => api.get(''), /must not be empty/);
  await assert.rejects(() => api.inventory(''), /must not be empty/);
});

test('sandbox configuration conflict parser distinguishes live and unverifiable inventory', () => {
  assert.deepEqual(parseSandboxConfigurationConflict({
    error: {
      code: 'sandboxes_still_live',
      message: 'still live',
      data: { sandbox_count: 1, session_ids: ['session-1'], agent_names: ['Agent'] },
    },
  }), {
    code: 'sandboxes_still_live',
    message: 'still live',
    inventory: { sandboxCount: 1, sessionIds: ['session-1'], agentNames: ['Agent'], unverifiable: false },
  });
  assert.deepEqual(parseSandboxConfigurationConflict({ error: { code: 'sandbox_inventory_unverifiable' } }), {
    code: 'sandbox_inventory_unverifiable',
  });
  assert.equal(parseSandboxConfigurationConflict({ error: { code: 'other' } }), null);
});
