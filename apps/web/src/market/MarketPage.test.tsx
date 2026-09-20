// jsdom harness (same pattern as ExpertsPage.test.tsx): node:test + createRoot
// + act, real timers (the 300ms search debounce and the deferred assertions
// wait real milliseconds, like global-command-palette-live-search.test.tsx),
// CSS imports stubbed. The api-client is faked wholesale: the market surface
// (search/rankings/install/tenant/publish), the sandbox-configs loader the
// SkillSettingsPanel uses, the catalog list the status reconciliation polls,
// and the sandbox install-events SSE subscription whose frames the test
// drives by hand.
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
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/market' });
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
const { MarketPage } = await import('./MarketPage.tsx');
import type { MarketRole } from './MarketPage.tsx';
import type { WeKnoraClient } from '@weknora/api-client';

// --- fixtures ------------------------------------------------------------------------

const SKILL_STOCK = { slug: 'stock-quote', name: '股票行情', description: '实时行情与K线', version: '1.2.0' };
const SKILL_WRITER = { slug: 'writer-coach', name: '写作教练', description: '润色与改写', version: '0.9.0' };

const TENANT_PUBLISHED = { catalog_id: 'cat-pub-1', name: '内部行情技能', description: '空间发布', version: '2.0.0', publisher_name: '管理员', installed: true };

const CATALOG_INSTALLED = {
  id: 'cat-1',
  name: '股票行情',
  version: '1.2.0',
  installations: [{ skillId: 'skill-0', sandboxConfigId: 'cfg-1', status: 'installing', enabled: true }],
};
const CATALOG_UNPUBLISHED = { id: 'cat-9', name: '未发布技能', version: '3.1.0', description: '' };

const CONFIG_E2B = {
  id: 'cfg-1', name: 'E2B 主沙箱', sandbox_type: 'e2b',
  config: { e2b: { api_url: 'https://e2b.example.com' } }, created_at: '', updated_at: '',
};
const CONFIG_DOCKER = {
  id: 'cfg-2', name: 'Docker 沙箱', sandbox_type: 'docker',
  config: { docker: { image: 'ubuntu:24.04' } }, created_at: '', updated_at: '',
};

/** One install-events SSE frame as the api-client parser hands it to the callback. */
interface SseFrame { event: { percent: number; stage: string; log?: string; status?: string; done: boolean }; terminal: boolean }
type SseCallback = (frame: SseFrame) => void;

interface ClientCalls {
  search: { query: string }[];
  rankings: string[];
  installSkill: { slug: string; sandboxConfigIds: string[] }[];
  tenantSkills: number;
  publish: string[];
  unpublish: string[];
  catalogList: number;
  sandboxConfigs: number;
  follow: { configId: string; skillId: string }[];
}

function makeClient(options: {
  searchResults?: typeof SKILL_STOCK[];
  searchStale?: boolean;
  searchReject?: Error;
  hotResults?: typeof SKILL_STOCK[];
  newestResults?: typeof SKILL_STOCK[];
  installResult?: { catalog_id: string; install_ids: string[]; errors?: Record<string, string> };
  tenantSkills?: typeof TENANT_PUBLISHED[];
  catalog?: unknown[];
  sandboxConfigs?: unknown[];
} = {}) {
  const calls: ClientCalls = { search: [], rankings: [], installSkill: [], tenantSkills: 0, publish: [], unpublish: [], catalogList: 0, sandboxConfigs: 0, follow: [] };
  const subscribers: SseCallback[] = [];
  const client = {
    market: {
      searchSkills: async (query: string) => {
        calls.search.push({ query });
        if (options.searchReject) throw options.searchReject;
        return { results: options.searchResults ?? [SKILL_STOCK], stale: options.searchStale ?? false };
      },
      rankings: async (kind: string) => {
        calls.rankings.push(kind);
        if (kind === 'newest') return { results: options.newestResults ?? [SKILL_WRITER], stale: false };
        return { results: options.hotResults ?? [SKILL_STOCK], stale: false };
      },
      installSkill: async (slug: string, sandboxConfigIds: string[]) => {
        calls.installSkill.push({ slug, sandboxConfigIds: [...sandboxConfigIds] });
        return options.installResult ?? { catalog_id: 'cat-1', install_ids: sandboxConfigIds.map((_id, index) => `skill-${index}`) };
      },
      tenantSkills: async () => {
        calls.tenantSkills += 1;
        return { skills: options.tenantSkills ?? [TENANT_PUBLISHED] };
      },
      publishSkill: async (catalogId: string) => {
        calls.publish.push(catalogId);
        return { catalog_id: catalogId, published_by: 'admin', published_at: '2026-09-20T00:00:00Z', updated_at: '2026-09-20T00:00:00Z' };
      },
      unpublishSkill: async (catalogId: string) => {
        calls.unpublish.push(catalogId);
        return;
      },
      installTenantSkill: async () => ({ installs: {} }),
    },
    configuration: {
      skills: {
        catalog: {
          list: async () => {
            calls.catalogList += 1;
            return options.catalog ?? [CATALOG_INSTALLED, CATALOG_UNPUBLISHED];
          },
        },
      },
    },
    sandboxConfigurations: {
      list: async () => {
        calls.sandboxConfigs += 1;
        return { items: options.sandboxConfigs ?? [CONFIG_E2B, CONFIG_DOCKER], workspaceScriptsDisabled: false };
      },
    },
    sandbox: {
      skills: {
        followInstallEvents: async (configId: string, skillId: string, onEvent: SseCallback) => {
          calls.follow.push({ configId, skillId });
          subscribers.push(onEvent);
          // The stream stays open until aborted; the test drives the frames.
          return new Promise<void>(() => {});
        },
      },
    },
  };
  return { client: client as unknown as WeKnoraClient, calls, subscribers };
}

let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

const $ = (root: ParentNode, selector: string): Element | null => root.querySelector(selector);
const $$ = (root: ParentNode, selector: string): Element[] => Array.from(root.querySelectorAll(selector));

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});
const flush = async () => {
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });
};

async function mountPage(client: WeKnoraClient, role: MarketRole = 'admin'): Promise<HTMLElement> {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(MarketPage, { client, role }));
  });
  await flush();
  return container;
}

async function click(root: ParentNode, selector: string): Promise<void> {
  const el = $(root, selector) ?? $(document.body, selector);
  assert.ok(el, 'element missing for selector ' + selector);
  await act(async () => {
    el.dispatchEvent(new window.Event('click', { bubbles: true, cancelable: true }));
  });
  await flush();
}

async function typeQuery(root: ParentNode, value: string): Promise<void> {
  const input = $(root, '[data-market-search]') as HTMLInputElement | null;
  assert.ok(input, 'search input missing');
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
    setter?.call(input, value);
    input.dispatchEvent(new window.Event('input', { bubbles: true }));
  });
}

async function check(root: ParentNode, selector: string): Promise<void> {
  const box = $(root, selector) as HTMLInputElement | null;
  assert.ok(box, 'checkbox missing for selector ' + selector);
  await act(async () => {
    box.click();
  });
  await flush();
}

// --- market tab ----------------------------------------------------------------------

test('rankings render on mount and switching a ranking kind refetches that ranking', async () => {
  const { client, calls } = makeClient();
  const root = await mountPage(client);
  assert.deepEqual(calls.rankings, ['hot'], 'the hot ranking loads on mount');
  assert.equal($$(root, '[data-market-card]').length, 1);
  assert.equal($(root, '[data-market-card="stock-quote"]')?.textContent?.includes('股票行情'), true);
  assert.equal($(root, '[data-market-card="stock-quote"]')?.textContent?.includes('1.2.0'), true);

  await click(root, '[data-market-ranking="newest"]');
  assert.deepEqual(calls.rankings, ['hot', 'newest']);
  assert.equal($$(root, '[data-market-card]').length, 1);
  assert.equal($(root, '[data-market-card="writer-coach"]')?.textContent?.includes('写作教练'), true);
});

test('debounced search renders results and the stale banner when the answer is stale', async () => {
  const { client, calls } = makeClient({ searchResults: [SKILL_STOCK, SKILL_WRITER], searchStale: true });
  const root = await mountPage(client);
  await typeQuery(root, '行情');
  await settle(30);
  assert.equal(calls.search.length, 0, 'no request before the debounce elapses');
  await settle(350);
  assert.deepEqual(calls.search, [{ query: '行情' }], 'exactly one search after the debounce');
  assert.equal($(root, '[data-testid="market-stale-banner"]') !== null, true, 'stale banner shows when stale=true');
  assert.equal($$(root, '[data-market-card]').length, 2);
});

test('search failure renders the error state with retry and the empty search renders the no-results state', async () => {
  const failing = makeClient({ searchReject: new Error('market unreachable') });
  const root = await mountPage(failing.client);
  await typeQuery(root, 'nothing');
  await settle(350);
  assert.equal($(root, '[role="alert"]')?.textContent?.includes('market unreachable'), true);
  assert.equal($$(root, '[data-market-card]').length, 0);

  const empty = makeClient({ searchResults: [] });
  const emptyRoot = await mountPage(empty.client);
  await typeQuery(emptyRoot, 'nothing');
  await settle(350);
  assert.equal($(emptyRoot, 'main')?.textContent?.includes('没有匹配的技能'), true);
});

test('install flow: pick configs, 202, SSE progress frames, completion state', async () => {
  const { client, calls, subscribers } = makeClient();
  const root = await mountPage(client);
  await click(root, '[data-market-install="stock-quote"]');

  const drawer = $(document.body, '[data-market-drawer]');
  assert.ok(drawer, 'install drawer missing');
  assert.equal(drawer.getAttribute('role'), 'dialog');
  // two eligible configs -> nothing preselected (the panel default-selects only a single config)
  const confirm = $(drawer, '[data-market-install-confirm]') as HTMLButtonElement | null;
  assert.ok(confirm?.disabled, 'confirm is disabled with no target selected');

  await check(drawer, 'input[data-market-config="cfg-1"]');
  await click(drawer, '[data-market-install-confirm]');

  assert.deepEqual(calls.installSkill, [{ slug: 'stock-quote', sandboxConfigIds: ['cfg-1'] }]);
  // the 202's install_ids pair positionally with the started configs -> the SSE subscription
  assert.deepEqual(calls.follow, [{ configId: 'cfg-1', skillId: 'skill-0' }]);

  const progressDrawer = $(document.body, '[data-market-drawer]');
  assert.ok(progressDrawer, 'progress drawer missing');
  assert.ok($(progressDrawer, '[data-market-install-row="cfg-1"]'), 'per-config progress row missing');
  await act(async () => {
    subscribers[0]!({ event: { percent: 40, stage: 'downloading', log: '拉取技能包', done: false }, terminal: false });
  });
  const row = $(document.body, '[data-market-install-row="cfg-1"]');
  assert.equal(row?.textContent?.includes('40%'), true, 'the live frame percent renders');
  assert.equal(row?.textContent?.includes('拉取技能包'), true, 'the live frame log renders');

  await act(async () => {
    subscribers[0]!({ event: { percent: 100, stage: 'done', status: 'ready', done: true }, terminal: true });
  });
  await flush();
  const done = $(document.body, '[data-testid="market-install-done"]');
  assert.ok(done, 'completion state missing');
  assert.equal(done?.textContent?.includes('1 个成功'), true);
  assert.equal($(document.body, '[data-market-install-row="cfg-1"]')?.textContent?.includes('已就绪'), true);
});

test('partial install failures surface per config and still consume progress for the started ones', async () => {
  const { client, calls, subscribers } = makeClient({
    installResult: { catalog_id: 'cat-1', install_ids: ['skill-0'], errors: { 'cfg-2': 'sandbox unreachable' } },
  });
  const root = await mountPage(client);
  await click(root, '[data-market-install="stock-quote"]');
  const drawer = $(document.body, '[data-market-drawer]')!;
  await check(drawer, 'input[data-market-config="cfg-1"]');
  await check(drawer, 'input[data-market-config="cfg-2"]');
  await click(drawer, '[data-market-install-confirm]');

  assert.equal($(document.body, '[role="status"]')?.textContent?.includes('1 个沙箱未能开始安装'), true, 'partial-failure toast');
  const failedRow = $(document.body, '[data-market-install-row="cfg-2"]');
  assert.equal(failedRow?.textContent?.includes('未能开始'), true);
  assert.equal(failedRow?.textContent?.includes('sandbox unreachable'), true);
  // only the started config subscribes to progress
  assert.deepEqual(calls.follow, [{ configId: 'cfg-1', skillId: 'skill-0' }]);

  await act(async () => {
    subscribers[0]!({ event: { percent: 100, stage: 'failed', status: 'failed', log: 'installer crashed', done: true }, terminal: true });
  });
  await flush();
  const done = $(document.body, '[data-testid="market-install-done"]');
  assert.ok(done);
  assert.equal(done?.textContent?.includes('0 个成功'), true);
  // both failures count: the failed install AND the config that never started
  assert.equal(done?.textContent?.includes('2 个失败'), true);
  assert.equal($(document.body, '[data-market-install-row="cfg-1"]')?.textContent?.includes('installer crashed'), true);
});

// --- tenant tab ----------------------------------------------------------------------

test('tenant tab renders published skills and the admin publish call fires and reloads', async () => {
  const { client, calls } = makeClient();
  const root = await mountPage(client);
  await click(root, '[data-market-tab="tenant"]');
  await flush();

  const row = $(root, '[data-tenant-skill="cat-pub-1"]');
  assert.ok(row, 'published skill row missing');
  assert.equal(row?.textContent?.includes('内部行情技能'), true);
  assert.equal(row?.textContent?.includes('已安装'), true);
  assert.equal(row?.textContent?.includes('管理员'), true, 'publisher chip renders');
  assert.ok($(root, '[data-tenant-unpublish="cat-pub-1"]'), 'admin sees the unpublish affordance');

  // unpublished workspace catalog entries carry the publish affordance
  const publishable = $(root, '[data-tenant-publishable="cat-9"]');
  assert.ok(publishable, 'unpublished catalog entry missing from the publish section');
  const publishBefore = calls.publish.length;
  const catalogBefore = calls.catalogList;
  await click(root, '[data-tenant-publish="cat-9"]');
  assert.deepEqual(calls.publish.slice(publishBefore), ['cat-9'], 'publishSkill fires with the catalog id');
  assert.equal($(document.body, '[role="status"]')?.textContent, '已发布');
  assert.ok(calls.catalogList > catalogBefore, 'the tenant listing reloads after publish');
  assert.ok(calls.tenantSkills >= 2, 'tenantSkills reloaded after publish');
});

test('non-admin roles browse both tabs without any write affordances', async () => {
  const { client } = makeClient();
  const root = await mountPage(client, 'viewer');
  assert.equal($(root, '[data-market-install="stock-quote"]'), null, 'viewers get no market install button');

  await click(root, '[data-market-tab="tenant"]');
  await flush();
  assert.ok($(root, '[data-tenant-skill="cat-pub-1"]'), 'the published list still renders for members');
  assert.equal($(root, '[data-tenant-unpublish="cat-pub-1"]'), null, 'no unpublish for viewers');
  assert.equal($(root, '[data-tenant-install="cat-pub-1"]'), null, 'no install-to-configs for viewers (Admin+ route guard)');
  assert.equal($(root, '[data-tenant-publishable]'), null, 'the admin publish section is hidden');
});
