import assert from 'node:assert/strict';
import test from 'node:test';

import { createSettingsApi } from './index.ts';

test('reads and writes tenant KV settings without exposing secret fields', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createSettingsApi(async (request) => {
    requests.push(request);
    if (request.method === 'GET') {
      return { success: true, data: { endpoint: 'https://parser.example', mineru_api_key: 'should-not-leak' } };
    }
    return { success: true, data: { endpoint: 'https://parser.example', mineru_api_key: 'still-not-returned' } };
  });

  assert.deepEqual(await api.parser.config.get(), { endpoint: 'https://parser.example' });
  assert.deepEqual(await api.parser.config.update({ endpoint: 'https://next.example', mineru_api_key: 'secret' }), {
    endpoint: 'https://parser.example',
  });
  assert.deepEqual(requests, [
    { method: 'GET', path: '/api/v1/tenants/kv/parser-engine-config' },
    { method: 'PUT', path: '/api/v1/tenants/kv/parser-engine-config', body: { endpoint: 'https://next.example', mineru_api_key: 'secret' } },
  ]);
});

test('redaction keeps type-metadata flags like requires_api_key while dropping credential values (R486 verify DIFF-B)', async () => {
  const api = createSettingsApi(async () => ({
    success: true,
    data: [
      { name: 'brave', requires_api_key: true, supports_optional_api_key: false, supports_proxy: true, tavily_api_key: 'sk-secret' },
    ],
  }));
  // Any settings read that unwraps .data applies redact(); web-search types
  // is the live consumer whose requires_api_key flag drove the missing
  // credential card.
  const rows = (await (api as unknown as { webSearch: { providers: { types: () => Promise<unknown[]> } } }).webSearch.providers.types()) as Array<Record<string, unknown>>;
  assert.equal(rows[0]!.requires_api_key, true, 'requires_api_key metadata flag survives redaction');
  assert.equal(rows[0]!.supports_optional_api_key, false, 'supports_optional_api_key survives');
  assert.equal(rows[0]!.supports_proxy, true, 'non-secret metadata untouched');
  assert.equal('tavily_api_key' in rows[0]!, false, 'credential-shaped value fields stay redacted');
});

test('preserves raw code envelopes for parser and system probes', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createSettingsApi(async (request) => {
    requests.push(request);
    if (request.path === '/api/v1/system/parser-engines') return { code: 0, data: [{ Name: 'builtin' }], connected: false };
    if (request.path === '/api/v1/system/storage-engine-status') return { code: 0, data: { engines: [], minio_env_available: false } };
    return { code: 0, data: { ok: true, message: 'connected' } };
  });

  assert.deepEqual(await api.parser.engines(), { items: [{ Name: 'builtin' }], connected: false });
  assert.deepEqual(await api.storage.legacy.status(), { engines: [], minio_env_available: false });
  assert.deepEqual(await api.storage.legacy.test({ provider: 's3' }), { ok: true, message: 'connected' });
  assert.deepEqual(requests.map(({ method, path }) => [method, path]), [
    ['GET', '/api/v1/system/parser-engines'],
    ['GET', '/api/v1/system/storage-engine-status'],
    ['POST', '/api/v1/system/storage-engine-check'],
  ]);
});

test('keeps personal memory and env-var mutations encoded and observable', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createSettingsApi(async (request) => {
    requests.push(request);
    if (request.method === 'GET') return { success: true, data: [] };
    return { success: true };
  });

  await api.memory.personal.items.list({ status: 'pending', limit: 10, offset: 2 });
  await api.envVars.list();
  await api.memory.personal.items.remove('memory/id');
  await api.envVars.skill.set('skill/id', 'API_TOKEN', 'secret');
  await api.envVars.sandbox.remove('sandbox/id', 'API_TOKEN');

  assert.deepEqual(requests, [
    { method: 'GET', path: '/api/v1/memory/items?status=pending&limit=10&offset=2' },
    { method: 'GET', path: '/api/v1/me/env-vars' },
    { method: 'DELETE', path: '/api/v1/memory/items/memory%2Fid' },
    { method: 'PUT', path: '/api/v1/me/env-vars/skill', body: { skill_id: 'skill/id', name: 'API_TOKEN', value: 'secret' } },
    { method: 'DELETE', path: '/api/v1/me/env-vars/sandbox', body: { sandbox_config_id: 'sandbox/id', name: 'API_TOKEN' } },
  ]);
});

test('exposes the complete personal-memory action surface with encoded ids', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createSettingsApi(async (request) => {
    requests.push(request);
    if (request.method === 'GET') return { success: true, data: [], total: 7 };
    if (request.method === 'DELETE') return { success: true, data: { removed: 2 } };
    return { success: true, data: { merged: 1 } };
  });

  await api.memory.personal.clear();
  await api.memory.personal.export();
  await api.memory.personal.consolidate();
  await api.memory.personal.items.confirm('memory/id');
  await api.memory.personal.items.reject('memory/id');
  const topics = await api.memory.personal.topics.list({ limit: 20, offset: 40 });
  await api.memory.personal.topics.promote('topic/id');
  await api.memory.personal.topics.remove('topic/id');
  const documents = await api.memory.personal.documents.list({ limit: 20, offset: 40 });
  await api.memory.personal.documents.remove('doc/id');

  // Vue paginates and counts off the envelope total (frontend/src/api/memory.ts).
  assert.deepEqual(topics, { rows: [], total: 7 });
  assert.deepEqual(documents, { rows: [], total: 7 });

  assert.deepEqual(requests.map(({ method, path }) => [method, path]), [
    ['DELETE', '/api/v1/memory/items'],
    ['GET', '/api/v1/memory/export'],
    ['POST', '/api/v1/memory/consolidate'],
    ['POST', '/api/v1/memory/items/memory%2Fid/confirm'],
    ['POST', '/api/v1/memory/items/memory%2Fid/reject'],
    ['GET', '/api/v1/memory/topics?limit=20&offset=40'],
    ['POST', '/api/v1/memory/topics/topic%2Fid/promote'],
    ['DELETE', '/api/v1/memory/topics/topic%2Fid'],
    ['GET', '/api/v1/memory/documents?limit=20&offset=40'],
    ['DELETE', '/api/v1/memory/documents/doc%2Fid'],
  ]);
});

test('provides CRUD and connection-test seams for tenant resource settings', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createSettingsApi(async (request) => {
    requests.push(request);
    if (request.method === 'GET' && request.path.endsWith('/types')) return { success: true, data: [{ type: 's3' }] };
    if (request.method === 'GET') return { success: true, data: [] };
    if (request.method === 'DELETE') return { success: true };
    return { success: true, data: { id: 'id-1', name: 'one' } };
  });

  await api.vectorStores.types();
  await api.vectorStores.create({ name: 'one' });
  await api.vectorStores.update('id/1', { name: 'two' });
  await api.vectorStores.testById('id/1');
  await api.webSearch.providers.remove('provider/1');
  await api.storage.backends.setDefault('backend/1');

  assert.deepEqual(requests.map(({ method, path }) => [method, path]), [
    ['GET', '/api/v1/vector-stores/types'],
    ['POST', '/api/v1/vector-stores'],
    ['PUT', '/api/v1/vector-stores/id%2F1'],
    ['POST', '/api/v1/vector-stores/id%2F1/test'],
    ['DELETE', '/api/v1/web-search-providers/provider%2F1'],
    ['PUT', '/api/v1/storage-backends/backend%2F1/default'],
  ]);
});

test('does not turn unavailable or forbidden responses into empty settings', async () => {
  const api = createSettingsApi(async () => {
    throw new Error('forbidden');
  });
  await assert.rejects(() => api.system.info(), /forbidden/);
});

test('reads the active tenant from the authenticated user envelope', async () => {
  const requests: Array<{ method: string; path: string }> = [];
  const api = createSettingsApi(async (request) => {
    requests.push({ method: request.method, path: request.path });
    return { success: true, data: { user: { id: 'user-1' }, tenant: { id: 7, name: 'React tenant', description: 'Live' } } };
  });

  assert.deepEqual(await api.tenant.get(), { id: 7, name: 'React tenant', description: 'Live' });
  assert.deepEqual(requests, [{ method: 'GET', path: '/api/v1/auth/me' }]);
});

// --- R491: tenant KV prompt templates (Vue getPromptTemplates parity) -----------------

test('promptTemplates reads GET /tenants/kv/prompt-templates like the other KV settings', async () => {
  const requests: Array<{ method: string; path: string }> = [];
  const api = createSettingsApi(async (request) => {
    requests.push(request as { method: string; path: string });
    return {
      success: true,
      data: {
        agent_system_prompt: [
          { id: 'progressive_rag_agent', name: '渐进式 RAG 智能体', description: '带知识库的渐进式检索增强生成智能体系统提示词', content: 'You are WeKnora…', default: true, mode: 'rag' },
        ],
        system_prompt: [{ id: 'default_kb', name: '知识库问答助手', content: '…', default: true }],
      },
    };
  });

  const config = await api.promptTemplates.get();
  assert.deepEqual(requests, [{ method: 'GET', path: '/api/v1/tenants/kv/prompt-templates' }]);
  const agentPrompts = (config as { agent_system_prompt?: Array<{ id: string; default?: boolean }> }).agent_system_prompt;
  assert.equal(agentPrompts?.[0]?.id, 'progressive_rag_agent');
  assert.equal(agentPrompts?.[0]?.default, true);
});
