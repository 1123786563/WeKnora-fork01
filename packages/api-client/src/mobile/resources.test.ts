import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileResourceRemote } from './resources.ts';
import type { ClientRequest } from '../client.ts';

const ORIGIN = 'https://weknora.example.test';

interface Recorder {
  seen: ClientRequest[];
  request: (input: ClientRequest) => Promise<unknown>;
}

function recorder(responder: (input: ClientRequest) => unknown): Recorder {
  const seen: ClientRequest[] = [];
  return {
    seen,
    request: async (input: ClientRequest): Promise<unknown> => {
      seen.push(input);
      return responder(input);
    },
  };
}

/** 真实 GET /api/v1/agents 响应形状（internal/handler/custom_agent.go:269-273：
 * success/data/disabled_own_agent_ids；行字段见 internal/types/custom_agent.go:65-104）。 */
const AGENTS_WIRE = {
  success: true,
  data: [
    {
      id: 'builtin-quick-answer', name: '快速问答', description: '检索增强问答', avatar: '🤖',
      is_builtin: true, tenant_id: 7, created_by: '',
      config: { agent_mode: 'quick-answer', system_prompt: 'SECRET-SYSTEM-PROMPT' },
    },
    {
      id: '3f2b8c0e-1', name: '研究助手', description: '深度研究',
      is_builtin: false, created_by: 'member-1', creator_name: '张三',
      config: { agent_mode: 'smart-reasoning', agent_type: 'rag-qa', system_prompt: 'SECRET-2' },
    },
  ],
  disabled_own_agent_ids: ['3f2b8c0e-1'],
};

/** 真实 GET /api/v1/knowledge-bases 行形状（internal/types/knowledgebase.go:59-140 +
 * buildKBListResponse 附加 vector_store_status，取值 available/unavailable，internal/types/vectorstore.go:544-580）。 */
const KNOWLEDGE_WIRE = {
  success: true,
  data: [
    { id: 'kb-1', name: '员工手册', knowledge_count: 3, updated_at: '2026-09-01T00:00:00Z', is_processing: false, vector_store_status: 'available' },
    { id: 'kb-2', name: 'FAQ', knowledge_count: 0, updated_at: '2026-09-02T00:00:00Z', is_processing: true },
    { id: 'kb-3', name: '归档库', knowledge_count: 0, updated_at: '2026-09-03T00:00:00Z', is_processing: false, vector_store_status: 'unavailable' },
    { id: 'kb-4', name: '共享库', knowledge_count: 2, updated_at: '2026-09-04T00:00:00Z', is_processing: false },
  ],
};

/** 真实 GET /api/v1/apps/connections 响应形状（internal/handler/app_connector_connection.go:55-92）。 */
const CONNECTIONS_WIRE = {
  success: true,
  data: [
    { id: 'conn-1', kind: 'personal', state: 'active', owner_id: 'member-1', auth_version: 4 },
    { id: 'conn-2', kind: 'space', state: 'revoked', owner_id: null, auth_version: 1 },
  ],
};

test('availableAgents maps wire rows to presentation-safe options and reports disabled ids', async () => {
  const spy = recorder(() => AGENTS_WIRE);
  const remote = createMobileResourceRemote({ origin: ORIGIN, request: spy.request });

  const result = await remote.availableAgents('resource-token');

  assert.deepEqual(result.rows, [
    { id: 'builtin-quick-answer', name: '快速问答', summary: '检索增强问答', kind: 'general', capability: { state: 'supported', reason: '' } },
    { id: '3f2b8c0e-1', name: '研究助手', summary: '深度研究', kind: 'custom', capability: { state: 'supported', reason: '' } },
  ]);
  assert.deepEqual([...result.disabledOwnAgentIds], ['3f2b8c0e-1']);
  assert.equal(JSON.stringify(result).includes('SECRET'), false, 'agent config must never reach the semantic rows');
  assert.equal(spy.seen.length, 1);
  assert.equal(spy.seen[0]!.method, 'GET');
  assert.equal(spy.seen[0]!.path, '/api/v1/agents');
  assert.equal((spy.seen[0]!.headers as Record<string, string> | undefined)?.authorization, 'Bearer resource-token');
});

test('knowledgeBases derives scan status from processing and store status without inventing indexed', async () => {
  const spy = recorder(() => KNOWLEDGE_WIRE);
  const remote = createMobileResourceRemote({ origin: ORIGIN, request: spy.request });

  const rows = await remote.knowledgeBases('resource-token');

  assert.deepEqual(rows, [
    { id: 'kb-1', title: '员工手册', scan_status: 'indexed', document_count: 3, updated_at: '2026-09-01T00:00:00Z' },
    { id: 'kb-2', title: 'FAQ', scan_status: 'scanning', document_count: 0, updated_at: '2026-09-02T00:00:00Z' },
    { id: 'kb-3', title: '归档库', scan_status: 'failed', document_count: 0, updated_at: '2026-09-03T00:00:00Z' },
    { id: 'kb-4', title: '共享库', scan_status: 'pending', document_count: 2, updated_at: '2026-09-04T00:00:00Z' },
  ]);
  assert.equal(spy.seen[0]!.path, '/api/v1/knowledge-bases');
});

test('connections passes through lifecycle fields and nothing else', async () => {
  const spy = recorder(() => CONNECTIONS_WIRE);
  const remote = createMobileResourceRemote({ origin: ORIGIN, request: spy.request });

  const rows = await remote.connections('resource-token');

  assert.deepEqual(rows, [
    { id: 'conn-1', kind: 'personal', state: 'active' },
    { id: 'conn-2', kind: 'space', state: 'revoked' },
  ]);
  assert.equal(JSON.stringify(rows).includes('owner_id'), false);
  assert.equal(JSON.stringify(rows).includes('auth_version'), false);
  assert.equal(spy.seen[0]!.path, '/api/v1/apps/connections');
});

test('failed envelopes and malformed payloads fail closed', async () => {
  const failing = recorder(() => ({ success: false, error: { code: 'FORBIDDEN', message: 'no' } }));
  const remote = createMobileResourceRemote({ origin: ORIGIN, request: failing.request });
  await assert.rejects(remote.availableAgents('resource-token'), /success must be true/);

  const malformed = recorder(() => ({ success: true }));
  const badRemote = createMobileResourceRemote({ origin: ORIGIN, request: malformed.request });
  await assert.rejects(badRemote.knowledgeBases('resource-token'), /data is required/);

  const notArray = recorder(() => ({ success: true, data: { nope: true } }));
  const weirdRemote = createMobileResourceRemote({ origin: ORIGIN, request: notArray.request });
  await assert.rejects(weirdRemote.connections('resource-token'), /must be an array/);
});

test('origin and access token are validated before any request leaves', async () => {
  const spy = recorder(() => AGENTS_WIRE);
  assert.throws(() => createMobileResourceRemote({ origin: 'http://weknora.example.test', request: spy.request }), /HTTPS/);
  const remote = createMobileResourceRemote({ origin: ORIGIN, request: spy.request });
  await assert.rejects(remote.availableAgents('  '), /access token is required/);
  assert.equal(spy.seen.length, 0, 'invalid input must not reach the wire');
});
