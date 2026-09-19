// jsdom harness (same pattern as agent-editor.test.tsx): node:test + createRoot
// + act, real timers (the instantiate flow defers the editor deep link by
// NAVIGATE_DELAY_MS, asserted against window.history), CSS imports stubbed.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/experts' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { ExpertsPage } = await import('./ExpertsPage.tsx');
import type { WeKnoraClient } from '@weknora/api-client';

// --- fixtures ------------------------------------------------------------------------

const SUMMARY_STOCK = {
  id: 'stock-assistant', label: '股票助手', description: '每日盘面复盘与个股分析',
  icon_name: 'trending-up', color: '#e37318', persona_mbti: 'INTJ', quick_prompt_count: 1,
  skills: ['stock-quote', 'technical-analysis', 'news-digest'],
};
const SUMMARY_WRITER = {
  id: 'writer', label: '写作教练', description: '',
  icon_name: 'pen-line', color: '', persona_mbti: '', quick_prompt_count: 0,
  skills: [],
};
const DETAIL_STOCK = {
  ...SUMMARY_STOCK,
  persona_markdown: '### 定位\n\n你是一名严谨的**股票分析师**。\n\n- 关注基本面\n- 输出结论',
  quick_prompts: [
    { title: '今日复盘', description: '总结今日盘面', prompt: '复盘', color: '#e37318', icon_name: 'calendar' },
  ],
};

interface InstantiateCall { id: string; input?: { agentName?: string } | undefined }

function makeClient(options: { listReject?: Error; instantiateReject?: Error; pendingSkills?: string[] } = {}) {
  const instantiateCalls: InstantiateCall[] = [];
  const client = {
    experts: {
      list: async () => {
        if (options.listReject) throw options.listReject;
        return [SUMMARY_STOCK, SUMMARY_WRITER];
      },
      get: async (id: string) => {
        if (id !== SUMMARY_STOCK.id) throw new Error('unknown expert ' + id);
        return DETAIL_STOCK;
      },
      instantiate: async (id: string, input?: { agentName?: string }) => {
        instantiateCalls.push({ id, input });
        if (options.instantiateReject) throw options.instantiateReject;
        return {
          agent: { id: 'ag-1', name: input?.agentName ?? SUMMARY_STOCK.label, description: '', avatar: '', config: { expert_source: { expert_id: id, source: 'builtin', slug: '' } } },
          pending_skills: options.pendingSkills ?? [],
          skill_install_ids: (options.pendingSkills ?? []).map((_, index) => `install-${index}`),
        };
      },
    },
  };
  return { client: client as unknown as WeKnoraClient, instantiateCalls };
}

let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  // deep-link navigations from the previous test must not leak into the next
  window.history.replaceState({}, '', '/platform/experts');
});

const $ = (root: ParentNode, selector: string): Element | null => root.querySelector(selector);
const $$ = (root: ParentNode, selector: string): Element[] => Array.from(root.querySelectorAll(selector));

async function mountPage(client: WeKnoraClient): Promise<HTMLElement> {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(ExpertsPage, { client }));
  });
  // let the list() promise resolve and the cards commit
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });
  return container;
}

async function click(root: ParentNode, selector: string): Promise<void> {
  const el = $(root, selector);
  assert.ok(el, 'element missing for selector ' + selector);
  await act(async () => {
    el.dispatchEvent(new window.Event('click', { bubbles: true, cancelable: true }));
  });
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });
}

async function setName(root: ParentNode, value: string): Promise<void> {
  const input = $(root, 'input') as HTMLInputElement | null;
  assert.ok(input, 'agent-name input missing');
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
    setter?.call(input, value);
    input.dispatchEvent(new window.Event('input', { bubbles: true }));
  });
}

function locationPath(): string {
  return window.location.pathname + window.location.search;
}

// --- list ----------------------------------------------------------------------------

test('list renders one card per expert with skill-count chips and mbti when present', async () => {
  const { client } = makeClient();
  const root = await mountPage(client);
  const cards = $$ (root, '[data-expert-id]');
  assert.equal(cards.length, 2);
  assert.equal($(root, '[data-expert-id="stock-assistant"]')?.textContent?.includes('股票助手'), true);
  assert.equal($(root, '[data-expert-id="stock-assistant"]')?.textContent?.includes('3 技能'), true);
  assert.equal($(root, '[data-expert-id="writer"]')?.textContent?.includes('0 技能'), true);
  // mbti chip only for the expert that carries one
  assert.equal($$(root, '[data-expert-id]') .filter((card) => card.textContent?.includes('INTJ')).length, 1);
});

test('list failure renders the error with retry, and the empty catalog renders the empty state', async () => {
  const failing = makeClient({ listReject: new Error('experts catalog unavailable') });
  const errorRoot = await mountPage(failing.client);
  assert.equal($(errorRoot, '[role="alert"]')?.textContent?.includes('experts catalog unavailable'), true);
  assert.equal($$(errorRoot, '[data-expert-id]').length, 0);

  const emptyClient = { experts: { list: async () => [], get: async () => { throw new Error('unused'); }, instantiate: async () => { throw new Error('unused'); } } };
  const emptyRoot = await mountPage(emptyClient as unknown as WeKnoraClient);
  assert.equal($(emptyRoot, 'main')?.textContent?.includes('暂无专家模板'), true);
});

// --- detail ---------------------------------------------------------------------------

test('detail drawer renders persona markdown through the chat markdown boundary and lists quick prompts', async () => {
  const { client } = makeClient();
  const root = await mountPage(client);
  await click(root, '[data-expert-id="stock-assistant"]');
  const drawer = $(document.body, '[data-expert-detail="stock-assistant"]');
  assert.ok(drawer, 'detail drawer missing');
  assert.equal(drawer.getAttribute('role'), 'dialog');

  const persona = $(drawer, '[data-expert-persona]');
  assert.ok(persona, 'persona section missing');
  assert.equal($(persona, 'h3')?.textContent, '定位');
  assert.equal($(persona, 'strong')?.textContent, '股票分析师');
  assert.equal($$(persona, 'li').map((li) => li.textContent).join('|'), '关注基本面|输出结论');

  const prompts = $$ (drawer, '[data-expert-prompt]');
  assert.equal(prompts.length, 1);
  assert.equal(prompts[0]?.textContent?.includes('今日复盘'), true);
  assert.equal(prompts[0]?.textContent?.includes('总结今日盘面'), true);
});

// --- instantiate ----------------------------------------------------------------------

test('instantiate uses the expert label by default, toasts the pending-skill count and deep-links the editor', async () => {
  const { client, instantiateCalls } = makeClient({ pendingSkills: ['pending-skill-a', 'pending-skill-b'] });
  const root = await mountPage(client);
  await click(root, '[data-expert-id="stock-assistant"]');
  await click(root, '[data-expert-instantiate="stock-assistant"]');

  assert.deepEqual(instantiateCalls, [{ id: 'stock-assistant', input: { agentName: '股票助手' } }]);
  assert.equal($(document.body, '[role="status"]')?.textContent, '已创建，2 个技能待安装');

  // the deep link is deferred past the toast paint (NAVIGATE_DELAY_MS)
  await new Promise((resolve) => setTimeout(resolve, 800));
  assert.equal(locationPath(), '/platform/agents?edit=ag-1');
});

test('a custom agent name rides the instantiate call and the no-pending branch uses the plain toast', async () => {
  const { client, instantiateCalls } = makeClient();
  const root = await mountPage(client);
  await click(root, '[data-expert-id="stock-assistant"]');
  await setName(root, '我的投顾');
  await click(root, '[data-expert-instantiate="stock-assistant"]');
  assert.deepEqual(instantiateCalls, [{ id: 'stock-assistant', input: { agentName: '我的投顾' } }]);
  assert.equal($(document.body, '[role="status"]')?.textContent, '已创建 Agent');
  await new Promise((resolve) => setTimeout(resolve, 800));
  assert.equal(locationPath(), '/platform/agents?edit=ag-1');
});

test('instantiate failure keeps the page and toasts the error message', async () => {
  const { client, instantiateCalls } = makeClient({ instantiateReject: new Error('boom') });
  const root = await mountPage(client);
  await click(root, '[data-expert-id="stock-assistant"]');
  await click(root, '[data-expert-instantiate="stock-assistant"]');
  assert.equal(instantiateCalls.length, 1);
  assert.equal($(document.body, '[role="status"]')?.textContent, '创建失败：boom');
  await new Promise((resolve) => setTimeout(resolve, 800));
  assert.equal(locationPath(), '/platform/experts', 'a failed instantiate must not navigate');
});
