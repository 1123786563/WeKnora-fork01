// KBW-4 单测：Wiki 索引状态轮询的 Vue WikiBrowser loadStats 口径——
//   挂载拉一次 GET /wiki/stats；仅索引中（is_active || pending_tasks>0）起
//   5s setInterval；回到空闲停表并触发一次 onIndexingSettled；空闲态不再发请求。
// 请求走 client.request 原始通道（保留 pending_tasks/is_active，api-client 的
// stats() 会丢字段）。定时器用 node:test mock.timers 打桩（模块裸调
// setInterval/clearInterval，浏览器语义等价）。
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { mock } from 'node:test';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
hooks.registerHooks?.({ resolve: (specifier, context, nextResolve) => (specifier.endsWith('.css') || specifier.endsWith('.svg')) ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

import * as React from 'react';
import { JSDOM } from 'jsdom';
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'http://localhost/' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  Event: dom.window.Event,
  MutationObserver: dom.window.MutationObserver,
  IS_REACT_ACT_ENVIRONMENT: true,
});

const { createRoot } = await import('react-dom/client');
const { useWikiIndexStatus, WIKI_INDEX_STATUS_POLL_INTERVAL_MS, parseWikiIndexStats, wikiStatsPath } = await import('./wiki-index-status.ts');

type ClientRequestInput = { method: string; path: string };

/** 按序回放响应的 request mock；越界调用记为错误响应（Vue 静默口径不抛出）。 */
function statsClient(responses: unknown[]) {
  const calls: ClientRequestInput[] = [];
  const client = {
    calls,
    request: async (input: ClientRequestInput): Promise<unknown> => {
      calls.push(input);
      const next = responses[calls.length - 1];
      if (next === undefined) return new Error('unexpected request') as never;
      if (next instanceof Error) throw next;
      return next;
    },
  };
  return client;
}

interface ObservedState { stats: ReturnType<typeof parseWikiIndexStats> | null; indexing: boolean }

function mountProbe(client: ReturnType<typeof statsClient>) {
  const states: ObservedState[] = [];
  const settled: number[] = [];
  function Probe() {
    const state = useWikiIndexStatus(client as never, 'kb-1', { onIndexingSettled: () => { settled.push(Date.now()); } });
    states.push({ stats: state.stats, indexing: state.indexing });
    return null;
  }
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  return { states, settled, container, root, Probe };
}

async function flush() {
  await React.act(async () => { await new Promise((resolve) => setImmediate(resolve)); });
}

const idle = { total_pages: 185, pages_by_type: { entity: 100, summary: 29 }, pending_tasks: 0, is_active: false, pending_issues: 0 };
const active = { total_pages: 10, pages_by_type: { entity: 1 }, pending_tasks: 3, is_active: true, pending_issues: 0 };

test('idle stats: one fetch on mount, no polling interval, counts parsed', async () => {
  mock.timers.enable({ apis: ['setInterval'] });
  try {
    const client = statsClient([idle]);
    const probe = mountProbe(client);
    await React.act(async () => { probe.root.render(React.createElement(probe.Probe)); });
    await flush();
    assert.equal(client.calls.length, 1, 'idle stats fetch exactly once on mount');
    assert.deepEqual(client.calls[0], { method: 'GET', path: '/api/v1/knowledgebase/kb-1/wiki/stats' });
    const state = probe.states[probe.states.length - 1];
    assert.equal(state.indexing, false, 'idle response does not light the indexing state');
    assert.equal(state.stats?.pendingTasks, 0);
    assert.equal(state.stats?.isActive, false);
    assert.equal(state.stats?.totalPages, 185);
    assert.deepEqual(state.stats?.pagesByType, { entity: 100, summary: 29 });
    // 空闲不轮询：推 20s 也无第二个请求。
    mock.timers.tick(4 * WIKI_INDEX_STATUS_POLL_INTERVAL_MS);
    await flush();
    assert.equal(client.calls.length, 1, 'no interval request while idle (Vue: 空闲停表)');
    assert.equal(probe.settled.length, 0, 'no settle callback without a prior indexing phase');
    await React.act(async () => { probe.root.unmount(); });
    probe.container.remove();
  } finally {
    mock.timers.reset();
  }
});

test('active stats: 5s polling starts, and the settle transition stops it + fires once', async () => {
  mock.timers.enable({ apis: ['setInterval'] });
  try {
    const client = statsClient([active, active, active, idle, idle]);
    const probe = mountProbe(client);
    await React.act(async () => { probe.root.render(React.createElement(probe.Probe)); });
    await flush();
    assert.equal(client.calls.length, 1);
    assert.equal(probe.states[probe.states.length - 1].indexing, true, 'is_active lights the indexing state');
    // 两个 5s 周期 → 各补一发请求（挂载 1 + 周期 2 = 3）。
    mock.timers.tick(WIKI_INDEX_STATUS_POLL_INTERVAL_MS);
    await flush();
    assert.equal(client.calls.length, 2, 'first 5s tick re-polls while indexing');
    mock.timers.tick(WIKI_INDEX_STATUS_POLL_INTERVAL_MS);
    await flush();
    assert.equal(client.calls.length, 3, 'second 5s tick keeps polling');
    assert.equal(probe.settled.length, 0, 'still indexing, no settle yet');
    // 第三个周期返回 idle：完成沿——停表 + 一次 onIndexingSettled。
    mock.timers.tick(WIKI_INDEX_STATUS_POLL_INTERVAL_MS);
    await flush();
    assert.equal(client.calls.length, 4);
    assert.equal(probe.settled.length, 1, 'indexing→idle transition fires onIndexingSettled once');
    assert.equal(probe.states[probe.states.length - 1].indexing, false);
    // 定时器已清：再推 20s 无新请求。
    mock.timers.tick(4 * WIKI_INDEX_STATUS_POLL_INTERVAL_MS);
    await flush();
    assert.equal(client.calls.length, 4, 'interval cleared after settle (Vue 完成分支)');
    await React.act(async () => { probe.root.unmount(); });
    probe.container.remove();
  } finally {
    mock.timers.reset();
  }
});

test('pending_tasks alone also drives polling; request errors stay silent', async () => {
  mock.timers.enable({ apis: ['setInterval'] });
  try {
    const queuedOnly = { total_pages: 0, pages_by_type: {}, pending_tasks: 2, is_active: false, pending_issues: 1 };
    const client = statsClient([queuedOnly, new Error('boom'), idle]);
    const probe = mountProbe(client);
    await React.act(async () => { probe.root.render(React.createElement(probe.Probe)); });
    await flush();
    assert.equal(probe.states[probe.states.length - 1].indexing, true, 'pendingTasks>0 lights indexing without is_active');
    assert.equal(probe.states[probe.states.length - 1].stats?.pendingIssues, 1);
    // 周期 1 抛错（Vue catch 静默）——轮询继续存活。
    mock.timers.tick(WIKI_INDEX_STATUS_POLL_INTERVAL_MS);
    await flush();
    assert.equal(client.calls.length, 2);
    assert.equal(probe.states[probe.states.length - 1].indexing, true, 'a failed poll keeps the last known state');
    // 周期 2 idle → settle。
    mock.timers.tick(WIKI_INDEX_STATUS_POLL_INTERVAL_MS);
    await flush();
    assert.equal(client.calls.length, 3);
    assert.equal(probe.settled.length, 1);
    await React.act(async () => { probe.root.unmount(); });
    probe.container.remove();
  } finally {
    mock.timers.reset();
  }
});

test('parseWikiIndexStats defaults and wikiStatsPath encoding', () => {
  assert.deepEqual(parseWikiIndexStats(null), { pendingTasks: 0, isActive: false, pendingIssues: 0, totalPages: 0, pagesByType: {} });
  assert.deepEqual(parseWikiIndexStats('junk'), { pendingTasks: 0, isActive: false, pendingIssues: 0, totalPages: 0, pagesByType: {} });
  assert.deepEqual(parseWikiIndexStats({ pending_tasks: 'x', is_active: 1, pages_by_type: { entity: 'y' } }), { pendingTasks: 0, isActive: false, pendingIssues: 0, totalPages: 0, pagesByType: { entity: 0 } });
  assert.equal(wikiStatsPath('kb/1'), '/api/v1/knowledgebase/kb%2F1/wiki/stats');
});
