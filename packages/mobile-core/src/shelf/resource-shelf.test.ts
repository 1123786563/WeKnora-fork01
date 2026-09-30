import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import { createInMemoryResourceRemote, type ResourceRemoteScript } from './in-memory-resource-remote.ts';
import { createResourceShelf } from './resource-shelf.ts';
import type { ResourceRemote } from './ports.ts';

const SCOPE = { deploymentOrigin: 'https://weknora.example.test', userId: 'member-1', tenantId: 'tenant-1' };

function openShelf(script: ResourceRemoteScript) {
  const remote = createInMemoryResourceRemote(script);
  const refreshes: number[] = [];
  const shelf = createResourceShelf({
    remote,
    accessTokenFor: async (_origin: string, options?: { refresh?: boolean }) => {
      if (options?.refresh) {
        refreshes.push(1);
        return 'token-2';
      }
      return 'token-1';
    },
  });
  const lease = new RuntimeScopeLease(SCOPE);
  return { shelf, handle: shelf.open({ lease: lease.asScopeLease() }), remote, refreshes, lease };
}

test('browse projects the three resource classes and never surfaces raw agent config', async () => {
  const { handle } = openShelf({
    agents: [
      { id: 'builtin-quick-answer', name: 'Quick Answer', summary: 'Fast answers', kind: 'general', capability: { state: 'supported', reason: '' }, config: { system_prompt: 'SECRET-PROMPT' } },
      { id: 'agent-2', name: 'Research', summary: 'Deep research', kind: 'custom', capability: { state: 'supported', reason: '' } },
    ],
    disabledOwnAgentIds: ['agent-2'],
    knowledgeBases: [{ id: 'kb-1', title: 'Handbook', scan_status: 'indexed', document_count: 3, updated_at: '2026-09-01T00:00:00Z' }],
    connections: [{ id: 'conn-1', kind: 'personal', state: 'active' }],
  });

  const page = await handle.browse();

  assert.equal(page.tenantId, 'tenant-1');
  assert.deepEqual(page.agents.map((agent) => agent.id), ['builtin-quick-answer'], 'disabled own agents must not appear as available');
  assert.deepEqual(page.knowledge.map((resource) => resource.title), ['Handbook']);
  assert.deepEqual(page.connections, [{ id: 'conn-1', kind: 'personal', state: 'active', connected: true, capability: { state: 'supported', reason: '' } }]);
  assert.deepEqual(page.classVerdicts, {
    agent: { state: 'supported', reason: '' },
    knowledge: { state: 'supported', reason: '' },
    connection: { state: 'supported', reason: '' },
  });
  assert.equal(JSON.stringify(page).includes('SECRET-PROMPT'), false, 'raw agent config must not survive the projection');
});

test('a 403 revocation invalidates the projection immediately and notifies subscribers', async () => {
  const script: ResourceRemoteScript = {
    agents: [{ id: 'builtin-quick-answer', name: 'Quick Answer', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }],
    knowledgeBases: [{ id: 'kb-1', title: 'Handbook', scan_status: 'indexed', document_count: 1, updated_at: '2026-09-01T00:00:00Z' }],
    connections: [],
  };
  const { handle } = openShelf(script);
  const events: unknown[] = [];
  handle.subscribe((event) => events.push(event));
  const first = await handle.browse();
  assert.equal(first.knowledge.length, 1);

  script.status = { knowledgeBases: 403 }; // 撤权：成员对该类资源失去授权
  const second = await handle.browse();

  assert.deepEqual(second.classVerdicts.knowledge, { state: 'forbidden', reason: 'http_403' });
  assert.deepEqual(second.knowledge, [], 'sensitive fields must not be refilled from the stale projection');
  assert.deepEqual(second.classVerdicts.agent, { state: 'supported', reason: '' }, 'one revoked class must not take down the others');
  assert.deepEqual(handle.selection({ knowledgeId: 'kb-1' }), { allowed: false, state: 'forbidden', reason: 'http_403' });
  assert.deepEqual(events, [{ type: 'authorization-revoked', resourceClass: 'knowledge' }]);
});

test('a server failure is unavailable, not an authorization revocation', async () => {
  const { handle } = openShelf({ connections: [{ id: 'conn-1', kind: 'space', state: 'active' }], status: { connections: 503 } });
  const events: unknown[] = [];
  handle.subscribe((event) => events.push(event));

  const page = await handle.browse();

  assert.deepEqual(page.classVerdicts.connection, { state: 'unavailable', reason: 'http_503' });
  assert.deepEqual(page.connections, []);
  assert.deepEqual(events, [], '5xx must not be reported as an authorization revocation');
});

test('a 401 retries exactly once with a refreshed token and never loops', async () => {
  const script: ResourceRemoteScript = {
    agents: [{ id: 'builtin-quick-answer', name: 'Quick Answer', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }],
  };
  script.status = { agents: (token: string) => (token === 'token-1' ? 401 : undefined) };
  const { handle, remote, refreshes } = openShelf(script);

  const page = await handle.browse();
  assert.deepEqual(page.agents.map((agent) => agent.id), ['builtin-quick-answer']);
  assert.deepEqual(remote.calls.filter((call) => call.kind === 'agents').map((call) => call.token), ['token-1', 'token-2']);
  assert.equal(refreshes.length, 1);

  script.status = { agents: () => 401 };
  const looping = await handle.browse();
  assert.deepEqual(looping.classVerdicts.agent, { state: 'unavailable', reason: 'http_401' }, 'a second 401 must fail closed instead of looping');
});

test('only a structured 401 status triggers the refresh retry: 403/5xx/network failures never refresh', async () => {
  // 重试判定必须读结构化 status 而非 reason 字面量（'http_401' 是展示字符串）。
  // 本测试锁定：一切非 401 失败——包括 reason 恰以 'http_' 开头的 403/5xx 与无 status 的
  // 网络错误——都不得触发 accessTokenFor({ refresh: true })。
  const refreshes: string[] = [];
  const failingRemote = (status: number | undefined): ResourceRemote => ({
    availableAgents: async () => {
      const error = new Error('boom');
      if (status !== undefined) (error as { status?: number }).status = status;
      throw error;
    },
    knowledgeBases: async () => [],
    connections: async () => [],
  });
  const openFailing = (status: number | undefined) => {
    const shelf = createResourceShelf({
      remote: failingRemote(status),
      accessTokenFor: async (_origin: string, options?: { refresh?: boolean }) => {
        if (options?.refresh) {
          refreshes.push(`refresh:${status}`);
          return 'token-2';
        }
        return 'token-1';
      },
    });
    const lease = new RuntimeScopeLease(SCOPE);
    return shelf.open({ lease: lease.asScopeLease() });
  };

  const forbidden = await openFailing(403).browse();
  assert.deepEqual(forbidden.classVerdicts.agent, { state: 'forbidden', reason: 'http_403' });

  const unavailable = await openFailing(503).browse();
  assert.deepEqual(unavailable.classVerdicts.agent, { state: 'unavailable', reason: 'http_503' });

  const offline = await openFailing(undefined).browse();
  assert.deepEqual(offline.classVerdicts.agent, { state: 'unavailable', reason: 'fetch_failed' }, 'errors without a structured status are network failures, not auth challenges');

  assert.deepEqual(refreshes, [], 'no failure other than a structured 401 may trigger a token refresh');
});

test('a revoked lease fails every operation closed even without an explicit close', async () => {
  const { handle, lease } = openShelf({ agents: [] });
  lease.revoke();

  await assert.rejects(handle.browse(), /SHELF_SCOPE_CLOSED/);
  assert.throws(() => handle.selection({ agentId: 'builtin-quick-answer' }), /SHELF_SCOPE_CLOSED/);
});

test('a browse that resolves after close is discarded', async () => {
  let resolveAgents!: (value: { rows: Array<Record<string, unknown>>; disabledOwnAgentIds: Set<string> }) => void;
  const remote: ResourceRemote = {
    availableAgents: () => new Promise((resolve) => { resolveAgents = resolve; }),
    knowledgeBases: async () => [],
    connections: async () => [],
  };
  const shelf = createResourceShelf({ remote, accessTokenFor: async () => 'token-1' });
  const lease = new RuntimeScopeLease(SCOPE);
  const handle = shelf.open({ lease: lease.asScopeLease() });

  const browsing = handle.browse();
  // 最小修正（相对简报逐字测试）：accessTokenFor 是 Promise seam，remote 调用发生在
  // 微任务里——同步调用 resolveAgents 时它尚未被赋值（TypeError: resolveAgents is not
  // a function）。让出一个宏任务使 availableAgents 已被调用、resolveAgents 已捕获，
  // 再 close：断言与被测行为（迟到结果被丢弃）保持逐字不变。
  await new Promise<void>((resolve) => setImmediate(resolve));
  handle.close('tenant-switch');
  resolveAgents({ rows: [{ id: 'late-agent', name: 'Late', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }], disabledOwnAgentIds: new Set() });

  await assert.rejects(browsing, /SHELF_SCOPE_CLOSED/);
});

test('close notifies subscribers with the scope reason exactly once', () => {
  const { handle } = openShelf({});
  const events: unknown[] = [];
  handle.subscribe((event) => events.push(event));

  handle.close('sign-out');
  handle.close('sign-out');

  assert.deepEqual(events, [{ type: 'scope-closed', reason: 'sign-out' }]);
});

test('selection explains allowed, unavailable and forbidden without guessing', async () => {
  const { handle } = openShelf({
    agents: [
      { id: 'agent-1', name: 'Research', summary: '', kind: 'custom', capability: { state: 'supported', reason: '' } },
      { id: 'agent-2', name: 'Blocked', summary: '', kind: 'custom', capability: { state: 'forbidden', reason: 'policy' } },
    ],
    knowledgeBases: [{ id: 'kb-1', title: 'Handbook', scan_status: 'indexed', document_count: 0, updated_at: '' }],
  });
  await handle.browse();

  assert.deepEqual(handle.selection({ agentId: 'agent-1' }), { allowed: true, selection: { kind: 'agent', agentId: 'agent-1' } });
  assert.deepEqual(handle.selection({ agentId: 'agent-2' }), { allowed: false, state: 'forbidden', reason: 'policy' });
  assert.deepEqual(handle.selection({ agentId: 'missing' }), { allowed: false, state: 'unavailable', reason: 'agent_not_found' });
  assert.deepEqual(handle.selection({ knowledgeId: 'kb-1' }), { allowed: true, selection: { kind: 'knowledge_ref', knowledgeId: 'kb-1' } });
  assert.deepEqual(handle.selection({ knowledgeId: 'missing' }), { allowed: false, state: 'unavailable', reason: 'knowledge_not_found' });
});

test('browse queries narrow display without changing verdicts', async () => {
  const { handle } = openShelf({
    agents: [
      { id: 'agent-1', name: 'Research Bot', summary: 'deep research', kind: 'custom', capability: { state: 'supported', reason: '' } },
      { id: 'agent-2', name: 'Writer', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } },
    ],
    knowledgeBases: [
      { id: 'kb-1', title: 'Handbook', scan_status: 'indexed', document_count: 0, updated_at: '' },
      { id: 'kb-2', title: 'Policies', scan_status: 'pending', document_count: 0, updated_at: '' },
    ],
  });

  const kindOnly = await handle.browse({ kind: 'custom' });
  assert.deepEqual(kindOnly.agents.map((agent) => agent.id), ['agent-1'], 'kind narrows agent display only');
  assert.deepEqual(kindOnly.knowledge.map((resource) => resource.id), ['kb-1', 'kb-2'], 'an agent-only filter leaves knowledge untouched');
  assert.deepEqual(kindOnly.classVerdicts.agent, { state: 'supported', reason: '' }, 'filtering never changes verdicts');

  const keyword = await handle.browse({ keyword: 'hand' });
  assert.deepEqual(keyword.agents.map((agent) => agent.id), [], 'filterAgents matches name+summary only — no agent contains "hand"');
  assert.deepEqual(keyword.knowledge.map((resource) => resource.id), ['kb-1'], 'keyword narrows knowledge by title');
  assert.deepEqual(keyword.connections, []);
});
