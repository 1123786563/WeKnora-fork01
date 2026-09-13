import assert from 'node:assert/strict';
import test from 'node:test';
import { createConfigurationApi } from './configuration.ts';
import type { NativeFileSource } from './ports.ts';

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

test('agents.copy posts to the dedicated copy subresource and returns the new agent', async () => {
  const requests: unknown[] = [];
  const api = createConfigurationApi(async (request) => {
    requests.push(request);
    return { success: true, data: { id: 'agent-2', name: 'Agent (copy)', is_builtin: false } };
  });
  const copied = await api.agents.copy('agent/1');
  assert.equal(copied.id, 'agent-2');
  assert.deepEqual(requests, [{ method: 'POST', path: '/api/v1/agents/agent%2F1/copy' }]);
  await assert.rejects(() => api.agents.copy(''), /must not be empty/);
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
    if ((request.path as string).includes('/models/')) {
      return { success: true, data: { fields: { api_key: { configured: true }, app_secret: { configured: false } } } };
    }
    return { success: true, data: { fields: { api_key: { configured: false }, token: { configured: true } } } };
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

test('maps MCP test and tool inventory responses including empty tools and descriptions', async () => {
  const requests: unknown[] = [];
  let toolsCall = 0;
  const api = createConfigurationApi(async (request) => {
    requests.push(request);
    if (request.path.endsWith('/test')) return { success: true, data: { success: false, message: 'connection refused' } };
    if (request.path.endsWith('/tools')) {
      toolsCall += 1;
      return toolsCall === 1 ? { success: true, data: [{ name: 'search', description: '' }] } : { success: true, data: [] };
    }
    return { success: true, data: [{ name: 'search', description: 'Search sources' }] };
  });

  assert.deepEqual(await api.mcp.test('mcp/1'), { success: false, message: 'connection refused' });
  assert.deepEqual(await api.mcp.tools('mcp/1'), [{ name: 'search', description: '' }]);
  assert.deepEqual(requests, [
    { method: 'POST', path: '/api/v1/mcp-services/mcp%2F1/test' },
    { method: 'GET', path: '/api/v1/mcp-services/mcp%2F1/tools' },
  ]);
  assert.deepEqual(await api.mcp.tools('mcp/1'), []);
});

test('fails closed when credential status omits a required field', async () => {
  const api = createConfigurationApi(async () => ({
    success: true,
    data: { fields: { api_key: { configured: true } } },
  }));

  await assert.rejects(() => api.models.credentials.put('model-1', { apiKey: 'new-key' }), /app_secret/);
});

test('rejects an unexpected response body from credential DELETE', async () => {
  const api = createConfigurationApi(async () => ({ success: true }));

  await assert.rejects(() => api.models.credentials.remove('model-1', 'api_key'), /204|empty|undefined/);
});

test('accepts only an empty 204 response from MCP OAuth revoke', async () => {
  const api = createConfigurationApi(async () => undefined);

  await api.mcp.oauth.revoke('mcp-1');
});

test('sends unsaved model credentials to the connection probe', async () => {
  const requests: unknown[] = [];
  const api = createConfigurationApi(async (request) => {
    requests.push(request);
    return { success: true, data: { available: true, message: 'ok' } };
  });

  await api.models.connection.remote({
    modelName: 'new-model',
    baseUrl: ' https://model.test ',
    apiKey: 'draft-key',
    appSecret: 'draft-secret',
  });

  assert.deepEqual(requests, [{
    method: 'POST',
    path: '/api/v1/initialization/remote/check',
    body: { modelName: 'new-model', baseUrl: 'https://model.test', apiKey: 'draft-key', appSecret: 'draft-secret' },
  }]);
});

test('preserves MCP test tools and resources and rejects malformed nested entries', async () => {
  const api = createConfigurationApi(async () => ({
    success: true,
    data: {
      success: true,
      message: 'connected',
      tools: [{ name: 'search', description: '', inputSchema: { type: 'object' } }],
      resources: [{ uri: 'file:///docs', name: 'Docs', description: '', mimeType: 'text/plain' }],
    },
  }));

  assert.deepEqual(await api.mcp.test('mcp-1'), {
    success: true,
    message: 'connected',
    tools: [{ name: 'search', description: '', inputSchema: { type: 'object' } }],
    resources: [{ uri: 'file:///docs', name: 'Docs', description: '', mimeType: 'text/plain' }],
  });

  const malformed = createConfigurationApi(async () => ({
    success: true,
    data: { success: true, tools: [{ description: 'missing name' }] },
  }));
  await assert.rejects(() => malformed.mcp.test('mcp-1'), /name/);
});

test('maps MCP metadata, usage generation, and tool policy mutations to the shared routes', async () => {
  const requests: unknown[] = [];
  const api = createConfigurationApi(async (request) => {
    requests.push(request);
    if (request.path.endsWith('/metadata')) return { success: true, data: { service_id: 'mcp-1', tools: [{ name: 'search', description: 'Search' }], instructions: 'Use search', server_name: 'Docs', server_version: '1', server_description: 'Docs server', synced_at: '2030-01-01T00:00:00Z', stale: false } };
    if (request.path.endsWith('/usage-instructions/generate')) return { success: true, data: { usage_instructions: 'Use the search tool.' } };
    if (request.method === 'GET') return { success: true, data: [{ id: 'approval-1', service_id: 'mcp-1', tool_name: 'search', require_approval: true, enabled: false }] };
    return { success: true };
  });
  assert.deepEqual(await api.mcp.metadata.get('mcp-1'), { serviceId: 'mcp-1', tools: [{ name: 'search', description: 'Search' }], instructions: 'Use search', serverName: 'Docs', serverVersion: '1', serverDescription: 'Docs server', syncedAt: '2030-01-01T00:00:00Z', stale: false });
  assert.equal(await api.mcp.usageInstructions.generate('mcp-1', 'zh-CN'), 'Use the search tool.');
  assert.deepEqual(await api.mcp.toolApprovals.list('mcp-1'), [{ id: 'approval-1', serviceId: 'mcp-1', toolName: 'search', requireApproval: true, enabled: false }]);
  await api.mcp.toolApprovals.update('mcp-1', 'search/tool', { enabled: true, requireApproval: false });
  assert.deepEqual(requests, [
    { method: 'GET', path: '/api/v1/mcp-services/mcp-1/metadata' },
    { method: 'POST', path: '/api/v1/mcp-services/mcp-1/usage-instructions/generate', body: { language: 'zh-CN' } },
    { method: 'GET', path: '/api/v1/mcp-services/mcp-1/tool-approvals' },
    { method: 'PUT', path: '/api/v1/mcp-services/mcp-1/tool-approvals/search%2Ftool', body: { enabled: true, require_approval: false } },
  ]);
});

test('lists model providers and sends model debug as an injected multipart request', async () => {
  const requests: any[] = [];
  const file: NativeFileSource = { uri: 'file:///tmp/input.png', name: 'input.png', type: 'image/png' };
  const api = createConfigurationApi(async (request) => {
    requests.push(request);
    if (request.path === '/api/v1/models/providers?model_type=chat') {
      return { success: true, data: [{ value: 'openai', label: 'OpenAI', description: 'Chat', defaultUrls: {}, modelTypes: ['chat'] }] };
    }
    return { success: true, data: {
      ok: false, elapsed_ms: 12, request: { input: 'hello' }, raw_response: { status: 429 }, observations: { stream: true }, error: 'rate limited',
    } };
  });

  assert.deepEqual(await api.models.providers.list('chat'), [{ value: 'openai', label: 'OpenAI', description: 'Chat', defaultUrls: {}, modelTypes: ['chat'] }]);
  assert.deepEqual(await api.models.debug('model/1', {
    input: 'hello', documents: ['doc-1'], options: { temperature: 0, thinking: false }, file,
  }), { ok: false, elapsedMs: 12, request: { input: 'hello' }, rawResponse: { status: 429 }, observations: { stream: true }, error: 'rate limited' });
  assert.deepEqual(requests, [
    { method: 'GET', path: '/api/v1/models/providers?model_type=chat' },
    {
      method: 'POST', path: '/api/v1/models/model%2F1/debug', nativeFile: file,
      multipartFields: { input: 'hello', documents: '["doc-1"]', options: '{"temperature":0,"thinking":false}' },
    },
  ]);
});

test('maps model connection checks to the existing initialization routes', async () => {
  const requests: any[] = [];
  const api = createConfigurationApi(async (request) => { requests.push(request); return { success: true, data: { available: true, message: 'ok', dimension: 768 } }; });
  const input = { modelName: 'embed-v1', baseUrl: 'https://model.test/', provider: 'openai', customHeaders: { 'X-Trace': 'yes' }, modelId: 'model/1' };
  assert.deepEqual(await api.models.connection.remote(input), { available: true, message: 'ok', dimension: 768 });
  assert.deepEqual(await api.models.connection.embedding(input), { available: true, message: 'ok', dimension: 768 });
  assert.deepEqual(await api.models.connection.rerank(input), { available: true, message: 'ok', dimension: 768 });
  assert.deepEqual(await api.models.connection.asr(input), { available: true, message: 'ok', dimension: 768 });
  assert.deepEqual(requests.map((request) => request.path), ['/api/v1/initialization/remote/check', '/api/v1/initialization/embedding/test', '/api/v1/initialization/rerank/check', '/api/v1/initialization/asr/check']);
  assert.equal(requests[0].body.baseUrl, 'https://model.test/');
  assert.deepEqual(requests[0].body.customHeaders, { 'X-Trace': 'yes' });
  await assert.rejects(() => api.models.connection.remote({ modelName: ' ' }), /modelName must not be empty/);
});

test('sends browser model debug files in the multipart body', async () => {
  const requests: any[] = [];
  const file = new Blob(['image'], { type: 'image/png' });
  const api = createConfigurationApi(async (request) => {
    requests.push(request);
    return { success: true, data: {
      ok: true, elapsed_ms: 4, request: {}, raw_response: 'described', observations: {},
    } };
  });

  await api.models.debug('model-1', { input: 'describe', file });

  const body = requests[0]?.body;
  assert.ok(body instanceof FormData);
  assert.equal(body.get('input'), 'describe');
  assert.equal(await (body.get('file') as Blob).text(), 'image');
});

test('maps catalog and installed-skill endpoints, including encoded file paths and accepted envelopes', async () => {
  const requests: any[] = [];
  const api = createConfigurationApi(async (request) => {
    requests.push(request);
    const path = request.path as string;
    if (path === '/api/v1/skills/catalog') return request.method === 'GET'
      ? { success: true, data: [{ id: 'cat/1', name: 'pdf', installations: [] }] }
      : { success: true, data: { id: 'cat/1', name: 'pdf' } };
    if (path.endsWith('/install')) return { success: true, data: { installs: {} } };
    if (path.endsWith('/files')) return { success: true, data: [{ path: 'SKILL.md', size: 4 }] };
    if (path.includes('/files/content')) return { success: true, data: { path: 'a/b.md', size: 3, encoding: 'utf-8', content: 'abc' } };
    if (path === '/api/v1/sandbox-configs/cfg%2F1/skills') return { success: true, data: [{ id: 'skill/1', name: 'pdf', enabled: true, status: 'ready' }] };
    if (path.endsWith('/reinstall')) return { success: true, data: { skill_id: 'skill/1' } };
    if (path.endsWith('/stop') || request.method === 'PATCH') return { success: true, data: { id: 'skill/1', name: 'pdf', enabled: false, status: 'ready' } };
    if (request.method === 'DELETE') return { success: true, data: { skill_id: 'skill/1' } };
    return { success: true, data: { id: 'skill/1', name: 'pdf', enabled: true, status: 'ready' } };
  });

  assert.deepEqual((await api.skills.catalog.list())[0]?.name, 'pdf');
  assert.equal((await api.skills.catalog.register({ source: '@owner/pdf' })).id, 'cat/1');
  assert.deepEqual(await api.skills.catalog.install('cat/1', ['cfg/1']), { installs: {} });
  assert.deepEqual(await api.skills.catalog.files('cat/1'), [{ path: 'SKILL.md', size: 4 }]);
  assert.equal((await api.skills.catalog.file('cat/1', 'a/b.md')).content, 'abc');
  await api.skills.catalog.remove('cat/1');

  assert.equal((await api.skills.installed.list('cfg/1'))[0]?.status, 'ready');
  assert.equal((await api.skills.installed.get('cfg/1', 'skill/1')).name, 'pdf');
  await api.skills.installed.files('cfg/1', 'skill/1');
  await api.skills.installed.file('cfg/1', 'skill/1', 'a/b.md');
  assert.equal((await api.skills.installed.reinstall('cfg/1', 'skill/1')).skillId, 'skill/1');
  assert.equal((await api.skills.installed.stop('cfg/1', 'skill/1')).enabled, false);
  const enabled = false;
  await api.skills.installed.update('cfg/1', 'skill/1', { enabled });
  await api.skills.installed.remove('cfg/1', 'skill/1');

  assert.deepEqual(requests.map((request) => `${request.method} ${request.path}`), [
    'GET /api/v1/skills/catalog', 'POST /api/v1/skills/catalog', 'POST /api/v1/skills/catalog/cat%2F1/install',
    'GET /api/v1/skills/catalog/cat%2F1/files', 'GET /api/v1/skills/catalog/cat%2F1/files/content?path=a%2Fb.md', 'DELETE /api/v1/skills/catalog/cat%2F1',
    'GET /api/v1/sandbox-configs/cfg%2F1/skills', 'GET /api/v1/sandbox-configs/cfg%2F1/skills/skill%2F1',
    'GET /api/v1/sandbox-configs/cfg%2F1/skills/skill%2F1/files', 'GET /api/v1/sandbox-configs/cfg%2F1/skills/skill%2F1/files/content?path=a%2Fb.md',
    'POST /api/v1/sandbox-configs/cfg%2F1/skills/skill%2F1/reinstall', 'POST /api/v1/sandbox-configs/cfg%2F1/skills/skill%2F1/stop',
    'PATCH /api/v1/sandbox-configs/cfg%2F1/skills/skill%2F1', 'DELETE /api/v1/sandbox-configs/cfg%2F1/skills/skill%2F1',
  ]);
});

test('rejects malformed skill status and missing accepted data instead of returning empty success', async () => {
  const malformedStatus = createConfigurationApi(async () => ({ success: true, data: [{ id: 's', name: 'skill', enabled: true, status: 'unknown' }] }));
  await assert.rejects(() => malformedStatus.skills.installed.list('cfg'), /status/);

  const missingData = createConfigurationApi(async () => ({ success: true }));
  await assert.rejects(() => missingData.skills.catalog.install('cat', []), /data/);
});

test('returns per-config results from a partially accepted catalog install', async () => {
  const api = createConfigurationApi(async () => ({
    success: false,
    data: {
      installs: { 'cfg-1': 'skill-1' },
      errors: { 'cfg-2': 'sandbox config not found' },
    },
  }));

  assert.deepEqual(await api.skills.catalog.install('cat-1', ['cfg-1', 'cfg-2']), {
    installs: { 'cfg-1': 'skill-1' },
    errors: { 'cfg-2': 'sandbox config not found' },
  });
});

test('accepts an actual 204 empty response only for a 204-compatible action', async () => {
  const api = createConfigurationApi(async () => undefined);
  await api.mcp.oauth.revoke('mcp-1');
  await assert.rejects(() => api.skills.catalog.remove('cat-1'), /success|object|empty/);
});
