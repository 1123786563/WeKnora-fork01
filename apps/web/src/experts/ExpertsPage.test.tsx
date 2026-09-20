// jsdom harness (same pattern as agent-editor.test.tsx): node:test + createRoot
// + act, real timers (the instantiate flow defers the editor deep link by
// NAVIGATE_DELAY_MS, asserted against window.history), CSS imports stubbed.
// The M4 tab surfaces (remote skillsets, tenant experts) drive the same
// client.market.* surface the MarketPage tests mock.
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
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
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

const SKILLSET_ANALYST = {
  slug: 'data-analyst', name: '数据分析专家', description: '表格清洗与图表分析',
  skill_slugs: ['csv-clean', 'chart-gen'], installed: false,
};
const SKILLSET_RESUME = {
  slug: 'resume-coach', name: '简历教练', description: '',
  skill_slugs: [], installed: true,
};
const SKILLSET_DETAIL_ANALYST = {
  slug: 'data-analyst', name: '数据分析专家', description: '表格清洗与图表分析',
  name_en: 'Data Analyst', description_en: 'Clean tables and chart results',
  skill_slugs: ['csv-clean', 'chart-gen'], installed: false, stale: false,
};

const TENANT_EXPERT_1 = {
  id: 'tex-1', name: '投放优化师', description: '广告投放策略复盘',
  publisher_name: 'Alice', installed: false, created_at: '2026-09-01T10:00:00Z',
};
const TENANT_EXPERT_2 = {
  id: 'tex-2', name: '客服质检员', description: '',
  publisher_name: 'Bob', installed: true, created_at: '2026-09-02T10:00:00Z',
};

// The agents list loadAgents (agents/api.ts) reads — u-1 owns two, u-2 one.
const AGENTS_ROWS = [
  { id: 'ag-mine', name: '我的分析师', description: '自己的 agent', is_builtin: false, created_by: 'u-1' },
  { id: 'ag-mine-2', name: '我的写手', description: '', is_builtin: false, created_by: 'u-1' },
  { id: 'ag-other', name: '同事的 agent', description: '', is_builtin: false, created_by: 'u-2' },
  { id: 'ag-builtin', name: '快答', description: '', is_builtin: true, created_by: '' },
];

interface InstantiateCall { id: string; input?: { agentName?: string } | undefined }
interface SkillsetInstallCall { slug: string; input?: { agentName?: string } | undefined }
interface TenantInstallCall { id: string; input?: { agentName?: string } | undefined }
interface PublishCall { agentId: string; input: { name?: string; description?: string } }

interface ClientOptions {
  listReject?: Error;
  instantiateReject?: Error;
  pendingSkills?: string[];
  viewerRole?: 'admin' | 'contributor' | 'viewer';
  viewerUserId?: string;
  skillsetsReject?: Error;
  skillsetsStale?: boolean;
  skillsetReject?: Error;
  skillsetInstallReject?: Error;
  tenantReject?: Error;
  tenantInstallReject?: Error;
  publishReject?: Error;
  unpublishReject?: Error;
}

function makeClient(options: ClientOptions = {}) {
  const instantiateCalls: InstantiateCall[] = [];
  const skillsetInstallCalls: SkillsetInstallCall[] = [];
  const tenantInstallCalls: TenantInstallCall[] = [];
  const publishCalls: PublishCall[] = [];
  const unpublishCalls: string[] = [];
  // Mutable per-client tenant rows so publish/unpublish reloads are observable.
  let tenantRows = [TENANT_EXPERT_1, TENANT_EXPERT_2];
  const client = {
    request: async (input: { method: string; path: string }) => {
      if (input.method === 'GET' && input.path === '/api/v1/agents') return { data: AGENTS_ROWS };
      throw new Error('unexpected request ' + input.method + ' ' + input.path);
    },
    auth: {
      me: async () => ({
        user: { id: options.viewerUserId ?? 'u-1', is_system_admin: false },
        tenant: { id: 1 },
        memberships: [{ tenant_id: 1, role: options.viewerRole ?? 'admin' }],
      }),
    },
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
    market: {
      skillsets: async () => {
        if (options.skillsetsReject) throw options.skillsetsReject;
        return { skillsets: [SKILLSET_ANALYST, SKILLSET_RESUME], stale: options.skillsetsStale ?? false };
      },
      skillset: async (slug: string) => {
        if (options.skillsetReject) throw options.skillsetReject;
        if (slug !== SKILLSET_ANALYST.slug) throw new Error('unknown skillset ' + slug);
        return SKILLSET_DETAIL_ANALYST;
      },
      installSkillset: async (slug: string, input?: { agentName?: string }) => {
        skillsetInstallCalls.push({ slug, input });
        if (options.skillsetInstallReject) throw options.skillsetInstallReject;
        return {
          agent: { id: 'ag-market', name: input?.agentName ?? SKILLSET_ANALYST.name, description: '', avatar: '', config: { expert_source: { expert_id: 'exp-m', source: 'market', slug } } },
          pending_skills: options.pendingSkills ?? [],
          skill_install_ids: (options.pendingSkills ?? []).map((_, index) => `m-install-${index}`),
          expert_id: 'exp-m',
          snapshot_sha256: 'sha-256',
        };
      },
      tenantExperts: async () => {
        if (options.tenantReject) throw options.tenantReject;
        return { experts: tenantRows };
      },
      publishAgentAsExpert: async (agentId: string, input: { name?: string; description?: string }) => {
        publishCalls.push({ agentId, input });
        if (options.publishReject) throw options.publishReject;
        const source = AGENTS_ROWS.find((row) => row.id === agentId);
        tenantRows = [...tenantRows, { id: 'tex-new', name: input?.name || source?.name || '', description: input?.description ?? '', publisher_name: 'Alice', installed: false, created_at: '2026-09-20T10:00:00Z' }];
        return { id: 'tex-new', agent_id: agentId, name: input?.name || source?.name || '', description: input?.description, snapshot_sha256: 'sha-256', published_by: 'u-1', published_at: '2026-09-20T10:00:00Z', updated_at: '2026-09-20T10:00:00Z' };
      },
      unpublishExpert: async (expertId: string) => {
        unpublishCalls.push(expertId);
        if (options.unpublishReject) throw options.unpublishReject;
        tenantRows = tenantRows.filter((row) => row.id !== expertId);
      },
      installTenantExpert: async (id: string, input?: { agentName?: string }) => {
        tenantInstallCalls.push({ id, input });
        if (options.tenantInstallReject) throw options.tenantInstallReject;
        return {
          agent: { id: 'ag-tenant', name: input?.agentName ?? TENANT_EXPERT_1.name, description: '', avatar: '', config: { expert_source: { expert_id: id, source: 'tenant', slug: '' } } },
          pending_skills: options.pendingSkills ?? [],
          skill_install_ids: (options.pendingSkills ?? []).map((_, index) => `t-install-${index}`),
        };
      },
    },
  };
  return { client: client as unknown as WeKnoraClient, instantiateCalls, skillsetInstallCalls, tenantInstallCalls, publishCalls, unpublishCalls };
}

let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  // deep-link navigations and tab state from the previous test must not leak
  window.history.replaceState({}, '', '/platform/experts');
});

const $ = (root: ParentNode, selector: string): Element | null => root.querySelector(selector);
const $$ = (root: ParentNode, selector: string): Element[] => Array.from(root.querySelectorAll(selector));

async function mountPage(client: WeKnoraClient, url = '/platform/experts'): Promise<HTMLElement> {
  window.history.replaceState({}, '', url);
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(ExpertsPage, { client }));
  });
  // let the tab's list promises (and the viewer hydrate) resolve and commit
  await act(async () => { await Promise.resolve(); });
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

async function setInputValue(input: HTMLInputElement, value: string): Promise<void> {
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
    setter?.call(input, value);
    input.dispatchEvent(new window.Event('input', { bubbles: true }));
  });
}

async function setName(root: ParentNode, value: string): Promise<void> {
  const input = $(root, 'input') as HTMLInputElement | null;
  assert.ok(input, 'agent-name input missing');
  await setInputValue(input, value);
}

function locationPath(): string {
  return window.location.pathname + window.location.search;
}

// --- tabs ------------------------------------------------------------------------------

test('default tab is builtin; the tab bar carries three tabs and builtin cards render', async () => {
  const { client } = makeClient();
  const root = await mountPage(client);
  const tabs = $$ (root, '[data-experts-tab]');
  assert.deepEqual(tabs.map((tab) => tab.getAttribute('data-experts-tab')), ['builtin', 'remote', 'tenant']);
  assert.equal(tabs[0]?.getAttribute('aria-selected'), 'true');
  assert.equal(tabs[1]?.getAttribute('aria-selected'), 'false');
  assert.equal($$(root, '[data-expert-id]').length, 2, 'builtin cards render on the default tab');
  assert.equal($$(root, '[data-skillset-slug]').length, 0, 'remote grid must not render while builtin is active');
  assert.equal($$(root, '[data-tenant-expert]').length, 0, 'tenant grid must not render while builtin is active');
});

test('tab clicks switch surfaces and persist ?tab= in the URL; builtin removes it', async () => {
  const { client } = makeClient();
  const root = await mountPage(client);
  // a drawer opened on one tab must not survive a tab switch (no stacked dialogs)
  await click(root, '[data-expert-id="stock-assistant"]');
  assert.ok($(document.body, '[data-expert-detail="stock-assistant"]'));
  await click(root, '[data-experts-tab="remote"]');
  assert.equal(locationPath(), '/platform/experts?tab=remote');
  assert.equal($(document.body, '[data-expert-detail="stock-assistant"]'), null, 'the builtin drawer closes on tab switch');
  assert.equal($$(root, '[data-skillset-slug]').length, 2);
  assert.equal($$(root, '[data-expert-id]').length, 0);

  await click(root, '[data-experts-tab="tenant"]');
  assert.equal(locationPath(), '/platform/experts?tab=tenant');
  assert.equal($$(root, '[data-tenant-expert]').length, 2);

  await click(root, '[data-experts-tab="builtin"]');
  assert.equal(locationPath(), '/platform/experts');
  assert.equal($$(root, '[data-expert-id]').length, 2);
});

test('?tab= deep links survive mount: remote renders directly, invalid values fall back to builtin', async () => {
  const remote = makeClient();
  const remoteRoot = await mountPage(remote.client, '/platform/experts?tab=remote');
  assert.equal($$(remoteRoot, '[data-skillset-slug]').length, 2);

  const tenant = makeClient();
  const tenantRoot = await mountPage(tenant.client, '/platform/experts?tab=tenant');
  assert.equal($$(tenantRoot, '[data-tenant-expert]').length, 2);

  const bogus = makeClient();
  const bogusRoot = await mountPage(bogus.client, '/platform/experts?tab=nonsense');
  assert.equal($$(bogusRoot, '[data-expert-id]').length, 2, 'invalid tab falls back to builtin');
});

// --- list ------------------------------------------------------------------------------

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

  const emptyClient = {
    auth: { me: async () => ({ user: { id: 'u-1' }, tenant: { id: 1 }, memberships: [{ tenant_id: 1, role: 'admin' }] }) },
    request: async () => ({ data: AGENTS_ROWS }),
    experts: { list: async () => [], get: async () => { throw new Error('unused'); }, instantiate: async () => { throw new Error('unused'); } },
  };
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

// --- remote-market tab ------------------------------------------------------------------

test('remote tab renders the skillset grid with skill-count and installed chips plus the stale banner when stale', async () => {
  const stale = makeClient({ skillsetsStale: true });
  const staleRoot = await mountPage(stale.client, '/platform/experts?tab=remote');
  assert.ok($(staleRoot, '[data-testid="experts-stale-banner"]'), 'stale banner missing');
  const analyst = $(staleRoot, '[data-skillset-slug="data-analyst"]');
  assert.ok(analyst, 'skillset card missing');
  assert.equal(analyst?.textContent?.includes('数据分析专家'), true);
  assert.equal(analyst?.textContent?.includes('2 技能'), true);
  assert.equal(analyst?.textContent?.includes('已安装'), false, 'not-installed skillset carries no installed chip');
  const resume = $(staleRoot, '[data-skillset-slug="resume-coach"]');
  assert.equal(resume?.textContent?.includes('已安装'), true, 'installed skillset shows the chip');

  const fresh = makeClient();
  const freshRoot = await mountPage(fresh.client, '/platform/experts?tab=remote');
  assert.equal($(freshRoot, '[data-testid="experts-stale-banner"]'), null);
});

test('remote tab failure renders the error with retry', async () => {
  const failing = makeClient({ skillsetsReject: new Error('registry unreachable') });
  const root = await mountPage(failing.client, '/platform/experts?tab=remote');
  assert.equal($(root, '[role="alert"]')?.textContent?.includes('registry unreachable'), true);
  assert.equal($$(root, '[data-skillset-slug]').length, 0);
});

test('remote skillset drawer lists the skill slugs and the install reuses the create-agent flow', async () => {
  const { client, skillsetInstallCalls } = makeClient({ pendingSkills: ['csv-clean'] });
  const root = await mountPage(client, '/platform/experts?tab=remote');
  await click(root, '[data-skillset-slug="data-analyst"]');
  const drawer = $(document.body, '[data-skillset-detail="data-analyst"]');
  assert.ok(drawer, 'skillset drawer missing');
  assert.equal(drawer.getAttribute('role'), 'dialog');
  assert.equal(drawer.textContent?.includes('csv-clean'), true);
  assert.equal(drawer.textContent?.includes('chart-gen'), true);

  await click(drawer, '[data-skillset-install="data-analyst"]');
  assert.deepEqual(skillsetInstallCalls, [{ slug: 'data-analyst', input: { agentName: '数据分析专家' } }]);
  assert.equal($(document.body, '[role="status"]')?.textContent, '已创建，1 个技能待安装');

  await new Promise((resolve) => setTimeout(resolve, 800));
  assert.equal(locationPath(), '/platform/agents?edit=ag-market');
});

test('remote skillset install failure keeps the page and toasts the error', async () => {
  const { client, skillsetInstallCalls } = makeClient({ skillsetInstallReject: new Error('install exploded') });
  const root = await mountPage(client, '/platform/experts?tab=remote');
  await click(root, '[data-skillset-slug="data-analyst"]');
  await click(document.body, '[data-skillset-install="data-analyst"]');
  assert.equal(skillsetInstallCalls.length, 1);
  assert.equal($(document.body, '[role="status"]')?.textContent, '创建失败：install exploded');
  await new Promise((resolve) => setTimeout(resolve, 800));
  assert.equal(locationPath(), '/platform/experts?tab=remote', 'a failed install must not navigate');
});

// --- tenant tab -------------------------------------------------------------------------

test('tenant tab lists published experts with publisher and installed badge; the publish entry follows ownership/admin', async () => {
  const { client } = makeClient({ viewerRole: 'admin' });
  const root = await mountPage(client, '/platform/experts?tab=tenant');
  const card = $(root, '[data-tenant-expert="tex-1"]');
  assert.ok(card, 'tenant expert card missing');
  assert.equal(card?.textContent?.includes('投放优化师'), true);
  assert.equal(card?.textContent?.includes('发布者：Alice'), true);
  assert.equal($(root, '[data-tenant-expert="tex-2"]')?.textContent?.includes('已安装'), true);
  assert.ok($(root, '[data-expert-publish-open]'), 'admin sees the publish entry');
  assert.ok($(root, '[data-tenant-unpublish="tex-1"]'), 'admin sees the unpublish action');
});

test('a member with no own agents sees neither the publish entry nor the admin unpublish action', async () => {
  const { client } = makeClient({ viewerRole: 'viewer', viewerUserId: 'u-3' });
  const root = await mountPage(client, '/platform/experts?tab=tenant');
  assert.equal($(root, '[data-expert-publish-open]'), null, 'no publish entry without owned agents or admin');
  assert.equal($(root, '[data-tenant-unpublish="tex-1"]'), null, 'unpublish is admin-only');
  assert.equal($$(root, '[data-tenant-expert]').length, 2, 'the listing itself is Viewer+');
});

test('tenant list failure renders the error with retry, and the empty workspace renders the empty state', async () => {
  const failing = makeClient({ tenantReject: new Error('tenant market down') });
  const errorRoot = await mountPage(failing.client, '/platform/experts?tab=tenant');
  assert.equal($(errorRoot, '[role="alert"]')?.textContent?.includes('tenant market down'), true);

  const empty = makeClient({ viewerRole: 'viewer', viewerUserId: 'u-3' });
  // reuse the harness with an empty tenant list
  const emptyClient = {
    ...empty.client,
    market: { ...empty.client.market, tenantExperts: async () => ({ experts: [] }) },
  } as unknown as WeKnoraClient;
  const emptyRoot = await mountPage(emptyClient, '/platform/experts?tab=tenant');
  assert.equal($(emptyRoot, 'main')?.textContent?.includes('空间还没有发布的专家'), true);
});

test('publish picker lists only the viewer agents, rides name overrides onto publishAgentAsExpert and reloads the list', async () => {
  const { client, publishCalls } = makeClient({ viewerRole: 'contributor' });
  const root = await mountPage(client, '/platform/experts?tab=tenant');
  await click(root, '[data-expert-publish-open]');
  const dialog = $(document.body, '[data-expert-publish-dialog]');
  assert.ok(dialog, 'publish dialog missing');
  assert.equal(dialog.getAttribute('role'), 'dialog');
  // only the viewer's own agents — not the colleague's, not builtin
  assert.ok($(dialog, '[data-publish-agent="ag-mine"]'));
  assert.ok($(dialog, '[data-publish-agent="ag-mine-2"]'));
  assert.equal($(dialog, '[data-publish-agent="ag-other"]'), null);
  assert.equal($(dialog, '[data-publish-agent="ag-builtin"]'), null);

  // pick the second own agent and type a name override
  await click(dialog, '[data-publish-agent="ag-mine-2"]');
  const nameInput = $(dialog, 'input') as HTMLInputElement | null;
  assert.ok(nameInput, 'publish name input missing');
  await setInputValue(nameInput, '空间首席写手');
  await click(dialog, '[data-expert-publish-confirm]');

  assert.deepEqual(publishCalls, [{ agentId: 'ag-mine-2', input: { name: '空间首席写手' } }]);
  assert.equal($(document.body, '[data-expert-publish-dialog]'), null, 'dialog closes on success');
  assert.equal($(document.body, '[role="status"]')?.textContent, '已发布');
  assert.ok($(root, '[data-tenant-expert="tex-new"]'), 'the published expert appears after the reload');
});

test('publish failure keeps the dialog and toasts the message', async () => {
  const { client, publishCalls } = makeClient({ viewerRole: 'contributor', publishReject: new Error('not allowed') });
  const root = await mountPage(client, '/platform/experts?tab=tenant');
  await click(root, '[data-expert-publish-open]');
  const dialog = $(document.body, '[data-expert-publish-dialog]');
  await click(dialog as ParentNode, '[data-expert-publish-confirm]');
  assert.equal(publishCalls.length, 1);
  assert.ok($(document.body, '[data-expert-publish-dialog]'), 'dialog stays open on failure');
  assert.equal($(document.body, '[role="status"]')?.textContent, '发布失败：not allowed');
});

test('tenant install rides installTenantExpert into the shared create-agent flow', async () => {
  const { client, tenantInstallCalls } = makeClient({ pendingSkills: ['shared-skill'] });
  const root = await mountPage(client, '/platform/experts?tab=tenant');
  await click(root, '[data-tenant-install="tex-1"]');
  assert.deepEqual(tenantInstallCalls, [{ id: 'tex-1', input: undefined }]);
  assert.equal($(document.body, '[role="status"]')?.textContent, '已创建，1 个技能待安装');
  await new Promise((resolve) => setTimeout(resolve, 800));
  assert.equal(locationPath(), '/platform/agents?edit=ag-tenant');
});

test('tenant install failure keeps the page and toasts the error', async () => {
  const { client } = makeClient({ tenantInstallReject: new Error('snapshot stale') });
  const root = await mountPage(client, '/platform/experts?tab=tenant');
  await click(root, '[data-tenant-install="tex-1"]');
  assert.equal($(document.body, '[role="status"]')?.textContent, '创建失败：snapshot stale');
  await new Promise((resolve) => setTimeout(resolve, 800));
  assert.equal(locationPath(), '/platform/experts?tab=tenant');
});

test('unpublish confirms through the alertdialog then removes the card', async () => {
  const { client, unpublishCalls } = makeClient({ viewerRole: 'admin' });
  const root = await mountPage(client, '/platform/experts?tab=tenant');
  await click(root, '[data-tenant-unpublish="tex-2"]');
  const dialog = $(document.body, '[data-expert-unpublish-dialog]');
  assert.ok(dialog, 'unpublish confirm missing');
  assert.equal(dialog.getAttribute('role'), 'alertdialog');
  assert.equal(dialog.textContent?.includes('客服质检员'), true, 'confirm names the expert');
  assert.equal(unpublishCalls.length, 0, 'nothing fires before confirmation');

  await click(dialog, '[data-expert-unpublish-confirm]');
  assert.deepEqual(unpublishCalls, ['tex-2']);
  assert.equal($(document.body, '[data-expert-unpublish-dialog]'), null);
  assert.equal($(document.body, '[role="status"]')?.textContent, '已取消发布');
  assert.equal($(root, '[data-tenant-expert="tex-2"]'), null, 'the card disappears after the reload');
});
