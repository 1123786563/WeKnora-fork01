// F2 — 超管侧栏 TenantSelector（Vue components/TenantSelector.vue 同构）单测：
//   • 纯核心：tenantSearchPath（纯数字=tenant_id 精确+keyword 模糊）、
//     parseTenantSearchPage 宽松解析、hasMoreTenantPages 分页判定；
//   • 组件：触发卡 → 下拉（标题/搜索/列表/选中 check）→ 选择回调；
//   • 挂载门控：PlatformShell 仅在 canAccessAllTenants 且展开时挂载
//     （Vue menu.vue:53；本账号非超管运行时不可见——源码级验证）。
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.png') || specifier.endsWith('.svg')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });

const require = nodeModule.createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge-bases' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  SVGElement: dom.window.SVGElement,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  MutationObserver: dom.window.MutationObserver,
  IS_REACT_ACT_ENVIRONMENT: true,
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? clearTimeout,
});
try { Object.defineProperty(dom.window.navigator, 'language', { value: 'zh-CN', configurable: true }); } catch { /* keep jsdom default */ }
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
// MessagePlugin（加载失败 toast）命令式 API 的 React 19 adapter（同款注入）。
{
  const { renderAdapter } = require('tdesign-react/lib/_util/react-render.js') as { renderAdapter: (renderer: unknown) => void };
  const { createRoot } = require('react-dom/client') as { createRoot: (container: Element) => unknown };
  renderAdapter?.(createRoot);
}

const { createRoot } = await import('react-dom/client');
const { TenantSelector, tenantSearchPath, parseTenantSearchPage, hasMoreTenantPages } = await import('./TenantSelector.tsx');
const { PlatformShell } = await import('./PlatformShell.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

const settle = (ms = 10) => act(async () => { await new Promise((resolve) => setTimeout(resolve, ms)); });

test('tenantSearchPath mirrors the Vue searchTenants query contract', () => {
  assert.equal(tenantSearchPath('', 1, 20), '/api/v1/tenants/search?page=1&page_size=20');
  assert.equal(tenantSearchPath('acme', 2, 20), '/api/v1/tenants/search?keyword=acme&page=2&page_size=20');
  // 纯数字关键词同时作为 tenant_id（精确）与 keyword（模糊）。
  assert.equal(tenantSearchPath('42', 1, 20), '/api/v1/tenants/search?keyword=42&tenant_id=42&page=1&page_size=20');
  assert.equal(tenantSearchPath('  acme  ', 1, 20), '/api/v1/tenants/search?keyword=acme&page=1&page_size=20');
});

test('parseTenantSearchPage tolerates envelope/nesting variations and hasMoreTenantPages paginates', () => {
  assert.deepEqual(parseTenantSearchPage({ success: true, data: { items: [{ id: 3, name: 'Ops' }, { id: 4 }], total: 9 } }), {
    items: [{ id: 3, name: 'Ops' }, { id: 4, name: '#4' }],
    total: 9,
  });
  assert.deepEqual(parseTenantSearchPage({ items: [] }), { items: [], total: 0 });
  assert.deepEqual(parseTenantSearchPage(null), { items: [], total: 0 });
  assert.equal(hasMoreTenantPages(20, 9), false);
  assert.equal(hasMoreTenantPages(20, 21), true);
});

async function mountSelector(options: {
  request?: (input: { method: 'GET'; path: string }) => Promise<unknown>;
  currentTenantId?: string | null;
  canCreateTenant?: boolean;
}): Promise<{ container: HTMLElement; calls: string[]; selected: Array<{ id: string; name: string }>; created: { count: number } }> {
  const calls: string[] = [];
  const selected: Array<{ id: string; name: string }> = [];
  const created = { count: 0 };
  const container = document.createElement('div');
  document.body.append(container);
  const request = options.request ?? (async (input: { method: 'GET'; path: string }) => {
    calls.push(input.path);
    return { success: true, data: { items: [{ id: 7, name: 'Acme Space' }, { id: 8, name: 'Beta Space' }], total: 2 } };
  });
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(TenantSelector, {
      locale: 'zh-CN',
      request,
      currentTenantName: 'Beta Space',
      currentTenantId: options.currentTenantId ?? '8',
      canCreateTenant: options.canCreateTenant ?? false,
      switchPending: false,
      onSelectTenant: (id, name) => { selected.push({ id, name }); },
      onCreateTenant: () => { created.count += 1; },
    }));
  });
  await settle();
  return { container, calls, selected, created };
}

test('F2: the selector opens a searchable tenant list and reports the selection', async () => {
  const { container, calls, selected } = await mountSelector({});
  try {
    const trigger = container.querySelector('.tenant-trigger') as HTMLElement | null;
    assert.ok(trigger, 'trigger card renders (Vue tenant-trigger)');
    assert.ok(container.textContent?.includes('当前空间'), 'locale copy: current tenant label');
    assert.ok(container.textContent?.includes('Beta Space'), 'current tenant name');
    await act(async () => trigger!.click());
    await settle();
    const dropdown = container.querySelector('.tenant-dropdown');
    assert.ok(dropdown, 'dropdown opens on trigger click');
    assert.ok(dropdown?.textContent?.includes('切换空间'), 'dropdown title copy');
    assert.equal(calls[0], '/api/v1/tenants/search?page=1&page_size=20', 'first page loads on open');
    const items = Array.from(dropdown?.querySelectorAll('.tenant-item') ?? []);
    assert.equal(items.length, 2, 'both tenants listed');
    const selectedRow = items.find((item) => item.className.includes('selected'));
    assert.ok(selectedRow, 'the current tenant row carries the selected state');
    assert.ok(selectedRow?.querySelector('.check-icon'), 'selected row renders the check icon');
    const target = items.find((item) => !item.className.includes('selected')) as HTMLElement;
    await act(async () => target.click());
    await settle();
    assert.deepEqual(selected, [{ id: '7', name: 'Acme Space' }], 'selection is reported to the host with id+name');
    assert.equal(container.querySelector('.tenant-dropdown'), null, 'dropdown closes after selection');
  } finally {
    await act(async () => mountedRoot?.unmount());
    mountedRoot = undefined;
    document.body.replaceChildren();
  }
});

test('F2: numeric search input re-queries with tenant_id after the 300ms debounce', async () => {
  const { container, calls } = await mountSelector({});
  try {
    await act(async () => (container.querySelector('.tenant-trigger') as HTMLElement).click());
    await settle();
    const input = container.querySelector('.search-input') as HTMLInputElement | null;
    assert.ok(input, 'search input renders');
    const setter = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
    await act(async () => {
      setter?.call(input, '42');
      input!.dispatchEvent(new window.Event('input', { bubbles: true }));
    });
    await settle(60);
    assert.equal(calls.length, 1, 'still inside the debounce window');
    await settle(280);
    assert.equal(calls[1], '/api/v1/tenants/search?keyword=42&tenant_id=42&page=1&page_size=20', 'numeric keyword re-queries as tenant_id');
  } finally {
    await act(async () => mountedRoot?.unmount());
    mountedRoot = undefined;
    document.body.replaceChildren();
  }
});

test('F2: the create entry is host-wired and only rendered for canCreateTenant', async () => {
  const withCreate = await mountSelector({ canCreateTenant: true });
  try {
    await act(async () => (withCreate.container.querySelector('.tenant-trigger') as HTMLElement).click());
    await settle();
    const entry = withCreate.container.querySelector('.tenant-create-action');
    assert.ok(entry, 'create entry renders when canCreateTenant');
    assert.ok(entry?.textContent?.includes('创建新空间'));
    await act(async () => (entry as HTMLElement).click());
    await settle();
    assert.equal(withCreate.created.count, 1, 'create entry delegates to the host dialog');
  } finally {
    await act(async () => mountedRoot?.unmount());
    mountedRoot = undefined;
    document.body.replaceChildren();
  }
  const withoutCreate = await mountSelector({ canCreateTenant: false });
  assert.equal(withoutCreate.container.querySelector('.tenant-create-action'), null, 'no create entry without the capability');
  await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

function shellClient(options: { canAccessAllTenants: boolean }) {
  return {
    auth: {
      me: async () => ({
        user: { id: 'u1', username: 'tester', email: 't@local.dev', avatar: '', can_access_all_tenants: options.canAccessAllTenants },
        tenant: { id: 1, name: 'Home' },
        memberships: [{ tenant_id: 1, tenant_name: 'Home', role: 'owner' }],
        capabilities: {},
      }),
    },
    sessions: { list: async () => ({ data: [], total: 0, page: 1, page_size: 30 }) },
    organizations: { list: async () => ({ items: [], total: 0 }) },
    request: async () => ({ success: true, data: { items: [{ id: 7, name: 'Acme Space' }], total: 1 } }),
  };
}

async function mountShell(options: { canAccessAllTenants: boolean }): Promise<void> {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => mountedRoot?.render(React.createElement(PlatformShell, {
    client: shellClient(options) as never,
    onLogout: () => undefined,
    onTenantSwitch: async () => undefined,
    children: React.createElement('div', null, 'page'),
  })));
  await settle(30);
}

test('F2: PlatformShell mounts the sidebar TenantSelector only for canAccessAllTenants (expanded rail)', async () => {
  await mountShell({ canAccessAllTenants: true });
  assert.ok(document.querySelector('[data-testid="tenant-selector"]'), 'superadmin sees the sidebar tenant selector');
  await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();

  await mountShell({ canAccessAllTenants: false });
  assert.equal(document.querySelector('[data-testid="tenant-selector"]'), null, 'ordinary member does not see the sidebar tenant selector');
});
