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

test('recursively removes MCP secrets but preserves credential status', async () => {
  const api = createConfigurationApi(async () => ({
    success: true,
    data: [{
      id: 'mcp-1',
      name: 'MCP',
      auth_config: {
        type: 'api_key',
        api_key: 'secret-a',
        token: 'secret-b',
        nested: { client_secret: 'secret-c', label: 'safe' },
      },
      credentials: { api_key: { configured: true } },
    }],
  }));

  const [service] = await api.mcp.list();
  assert.deepEqual(service?.auth_config, { type: 'api_key', nested: { label: 'safe' } });
  assert.deepEqual(service?.credentials, { api_key: { configured: true } });
});

test('preserves agent disabled state and skill availability from list envelopes', async () => {
  const requests: unknown[] = [];
  const api = createConfigurationApi(async (request) => {
    requests.push(request);
    if ((request.path as string).startsWith('/api/v1/agents')) {
      return {
        success: true,
        data: [{ id: 'agent-1', name: 'Agent', is_builtin: false }],
        disabled_own_agent_ids: ['agent-1'],
      };
    }
    return {
      success: true,
      data: [{ name: 'Research', description: 'Search sources' }],
      skills_available: true,
    };
  });

  assert.deepEqual(await api.agents.listWithState({ creator: 'mine' }), {
    items: [{ id: 'agent-1', name: 'Agent', is_builtin: false }],
    disabledOwnAgentIds: ['agent-1'],
  });
  assert.deepEqual(await api.skills.listWithAvailability('sandbox/1'), {
    items: [{ id: 'Research', name: 'Research', description: 'Search sources' }],
    skillsAvailable: true,
  });
  assert.deepEqual(requests.map((request) => (request as { path: string }).path), [
    '/api/v1/agents?creator=mine',
    '/api/v1/skills?sandbox_config_id=sandbox%2F1',
  ]);
});

test('maps MCP OAuth authorization URL and status through the shared client', async () => {
  const requests: unknown[] = [];
  const api = createConfigurationApi(async (request) => {
    requests.push(request);
    if (request.method === 'POST') return { success: true, data: { authorization_url: 'https://idp.test/authorize', authorization_attempt: 'attempt-1' } };
    return { success: true, data: { authorized: true, state: 'authorized', refresh_available: true, expires_at: '2030-01-01T00:00:00Z' } };
  });
  assert.deepEqual(await api.mcp.oauth.authorizeUrl('service/1', { redirectURI: 'https://api.test/api/v1/mcp-oauth/callback', frontendRedirect: 'weknora://mcp-oauth' }), {
    authorizationUrl: 'https://idp.test/authorize', authorizationAttempt: 'attempt-1',
  });
  assert.deepEqual(await api.mcp.oauth.status('service/1', 'attempt/1'), {
    authorized: true, state: 'authorized', refreshAvailable: true, expiresAt: '2030-01-01T00:00:00Z',
  });
  assert.deepEqual(requests, [
    { method: 'POST', path: '/api/v1/mcp-services/service%2F1/oauth/authorize-url', body: { redirect_uri: 'https://api.test/api/v1/mcp-oauth/callback', frontend_redirect: 'weknora://mcp-oauth' } },
    { method: 'GET', path: '/api/v1/mcp-services/service%2F1/oauth/status?authorization_attempt=attempt%2F1' },
  ]);
});

test('keeps model and MCP credentials on dedicated secret subresources', async () => {
  const requests: unknown[] = [];
  const api = createConfigurationApi(async (request) => {
    requests.push(request);
    if (request.method === 'DELETE') return undefined;
    return { success: true, data: { fields: { api_key: { configured: true }, token: { configured: true } } } };
  });

  assert.deepEqual(await api.models.credentials.put('model/1', { apiKey: 'secret' }), {
    apiKey: true,
    appSecret: false,
  });
  assert.deepEqual(await api.mcp.credentials.put('mcp/1', { token: 'secret' }), {
    apiKey: false,
    token: true,
  });
  await api.models.credentials.remove('model/1', 'api_key');
  assert.deepEqual(requests, [
    { method: 'PUT', path: '/api/v1/models/model%2F1/credentials', body: { api_key: 'secret' } },
    { method: 'PUT', path: '/api/v1/mcp-services/mcp%2F1/credentials', body: { token: 'secret' } },
    { method: 'DELETE', path: '/api/v1/models/model%2F1/credentials/api_key' },
  ]);
});

test('maps MCP test and tool inventory responses without treating an empty response as success', async () => {
  const requests: unknown[] = [];
  let toolsCall = 0;
  const api = createConfigurationApi(async (request) => {
    requests.push(request);
    if (request.path.endsWith('/test')) return { success: true, data: { success: false, message: 'connection refused' } };
    if (request.path.endsWith('/tools')) {
      toolsCall += 1;
      return toolsCall === 1 ? { success: true, data: [{ name: 'search', description: 'Search sources' }] } : { success: true, data: [] };
    }
    return { success: true, data: [{ name: 'search', description: 'Search sources' }] };
  });

  assert.deepEqual(await api.mcp.test('mcp/1'), { success: false, message: 'connection refused' });
  assert.deepEqual(await api.mcp.tools('mcp/1'), [{ name: 'search', description: 'Search sources' }]);
  assert.deepEqual(requests, [
    { method: 'POST', path: '/api/v1/mcp-services/mcp%2F1/test' },
    { method: 'GET', path: '/api/v1/mcp-services/mcp%2F1/tools' },
  ]);
  await assert.rejects(() => api.mcp.tools('mcp/1'), /successful array/);
});
