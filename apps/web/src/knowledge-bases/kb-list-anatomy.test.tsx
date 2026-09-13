import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';
import { createScopeController } from '@weknora/domain';

// R009 kb-list anatomy slice (Vue KnowledgeBaseList.vue authority):
//   (a) compact responsive card grid, hover-revealed star + more menu
//   (b) vertical icon rail (all/favorites/recents/workspace) replaces chips
//   (c) amber uninitialized warning banner
//   (d) list-fetch failure renders the Vue empty state (no raw JSON leak)

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg?raw') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge-bases' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
// The live parity environment runs zh-CN (locale seeded via localStorage); the
// page resolves its locale from navigator.language, so pin it for assertions.
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'zh-CN' });

const { createRoot } = await import('react-dom/client');
const { KnowledgeBasesPage } = await import('../App.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.history.replaceState(null, '', '/platform/knowledge-bases');
  dom.window.localStorage.clear();
});

interface OwnedRow {
  id: string;
  name: string;
  description?: string;
  type?: 'document' | 'faq';
  knowledge_count?: number;
  chunk_count?: number;
  creator_id?: string;
  summary_model_id?: string;
  embedding_model_id?: string;
}

function makeClient(options: {
  owned?: OwnedRow[];
  shared?: unknown[];
  failList?: boolean;
} = {}) {
  const owned: OwnedRow[] = options.owned ?? [
    { id: 'kb-doc', name: 'Parity KB Demo', description: 'parity test data', type: 'document', knowledge_count: 2, creator_id: 'u-1', summary_model_id: 'm-1', embedding_model_id: 'm-2' },
    { id: 'kb-faq', name: 'parity-faq-kb', type: 'faq', chunk_count: 5, knowledge_count: 0, creator_id: 'u-1', summary_model_id: 'm-1' },
    { id: 'kb-uninit', name: '未初始化库', type: 'document', knowledge_count: 0, creator_id: 'u-1' },
  ];
  return {
    auth: {
      me: async () => ({ user: { id: 'u-1', is_system_admin: false }, memberships: [{ tenant_id: 't-1', role: 'contributor' }] }),
    },
    knowledgeBases: {
      list: options.failList
        ? async () => { throw new Error('mock failure'); }
        : async () => owned,
      togglePin: async () => ({ is_pinned: true }),
      duplicate: async () => ({}),
      remove: async () => ({}),
      create: async () => ({ id: 'kb-new' }),
      update: async () => ({}),
    },
    identity: { organizations: { knowledgeBaseShares: { listShared: async () => options.shared ?? [] } } },
  } as unknown as WeKnoraClient;
}

async function mountPage(client: WeKnoraClient, search = '') {
  if (search) dom.window.history.replaceState(null, '', '/platform/knowledge-bases' + search);
  const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'u-1', tenantId: 't-1' });
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<KnowledgeBasesPage client={client} scopeController={scopeController} />);
  });
  await act(async () => {});
  return container;
}

test('(b) renders the vertical icon rail (all/favorites/recents/workspace) and drops the chips toolbar', async () => {
  const container = await mountPage(makeClient());
  const rail = container.querySelector('.kb-list-rail');
  assert.ok(rail, 'expected a .kb-list-rail element (ListSpaceSidebar port)');
  const labels = Array.from(rail.querySelectorAll('.kb-list-rail-label')).map((el) => el.textContent);
  assert.deepEqual(labels, ['全部', '收藏', '最近', '本空间']);
  const active = rail.querySelector('.kb-list-rail-item.kb-list-rail-item-active .kb-list-rail-label');
  assert.equal(active?.textContent, '本空间', 'contributor defaults to the workspace scope like Vue');
  assert.equal(container.querySelector('.wk-toolbar'), null, 'no horizontal search/creator toolbar');
  assert.equal(container.querySelector('.wk-kb-scope'), null, 'no horizontal scope chips');
  assert.equal(container.querySelector('select'), null, 'no native creator select');
});

test('(a) renders compact cards in a responsive grid with hover-revealed star and more control', async () => {
  const container = await mountPage(makeClient());
  const grid = container.querySelector('.kb-list-grid');
  assert.ok(grid, 'expected a .kb-list-grid element');
  const cards = Array.from(container.querySelectorAll('.kb-list-card'));
  assert.equal(cards.length, 3);
  const faq = container.querySelector('.kb-list-card.kb-list-card-faq');
  assert.ok(faq, 'faq card carries the faq type class');
  assert.ok(container.querySelector('.kb-list-card.kb-list-card-document'), 'document card carries the document type class');
  for (const card of cards) {
    assert.ok(card.querySelector('.kb-favorite-star'), 'favorite star present on every card');
    assert.ok(card.querySelector('.kb-list-card-more'), 'more control present on every card');
    assert.ok(card.querySelector('.kb-list-card-desc'), 'description block present');
    assert.ok(card.getAttribute('data-kb-id'), 'data-kb-id preserved for highlight/scroll');
  }
  assert.equal(container.querySelector('.wk-kb-card-actions'), null, 'no always-visible action button row');
  const docCard = container.querySelector('[data-kb-id="kb-doc"]');
  const countText = docCard?.querySelector('.kb-list-badge-count')?.textContent ?? '';
  assert.match(countText, /2/, 'document badge renders the knowledge count');
});

test('(a) section headers carry label/count and collapse on click', async () => {
  const container = await mountPage(makeClient());
  const headers = Array.from(container.querySelectorAll('.kb-list-section-header'));
  assert.ok(headers.length >= 1, 'at least one section header renders');
  const mine = headers.find((el) => (el.textContent ?? '').includes('我创建的'));
  assert.ok(mine, 'mine section header renders');
  assert.match(mine?.querySelector('.kb-list-section-count')?.textContent ?? '', /3/);
  assert.notEqual(mine?.getAttribute('aria-expanded'), null, 'header exposes aria-expanded');
  const before = container.querySelectorAll('.kb-list-card').length;
  await act(async () => {
    mine.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
  });
  await act(async () => {});
  const after = container.querySelectorAll('.kb-list-card').length;
  assert.ok(after < before, 'clicking the section header collapses its cards');
});

test('(c) uninitialized KBs render the amber warning banner with icon + text', async () => {
  const container = await mountPage(makeClient());
  const banner = container.querySelector('.kb-list-warning');
  assert.ok(banner, 'expected .kb-list-warning element');
  assert.ok(banner.querySelector('svg'), 'banner carries an info icon');
  assert.match(banner.textContent ?? '', /部分知识库尚未初始化/);
});

test('(d) list-fetch failure falls back to the Vue empty state and never leaks the raw error', async () => {
  const container = await mountPage(makeClient({ failList: true }));
  const empty = container.querySelector('.kb-list-empty');
  assert.ok(empty, 'error state renders the empty-state fallback');
  assert.ok(container.querySelector('.kb-list-empty-img'), 'empty state carries the illustration');
  assert.match(container.querySelector('.kb-list-empty-title')?.textContent ?? '', /暂无知识库/);
  const cta = container.querySelector('[data-guide="kb-list-create"]');
  assert.ok(cta, 'create CTA present in the empty state');
  assert.equal(document.body.textContent?.includes('mock failure'), false, 'raw backend JSON must not leak into the UI');
  assert.equal(container.querySelector('.wk-pagination'), null, 'Vue renders the full list without pagination');

  // Vue favorites/recents empty states carry a scope hint and never the
  // create CTA (KnowledgeBaseList.vue:645-659); ?scope deep link covered here,
  // the rail-click path by the live screenshot evidence.
  const favContainer = await mountPage(makeClient(), '?scope=favorites');
  assert.match(favContainer.querySelector('.kb-list-empty-title')?.textContent ?? '', /暂无收藏/);
  assert.equal(favContainer.querySelector('.kb-list-empty [data-guide="kb-list-create"]'), null, 'favorites empty must not offer the create CTA');
  assert.equal(favContainer.querySelector('.kb-list-empty-img'), null, 'favorites empty uses an icon, not the illustration');
});

test('(a) the more menu exposes exactly the Vue card actions', async () => {
  const container = await mountPage(makeClient());
  const more = container.querySelector('[data-kb-id="kb-doc"] .kb-list-card-more');
  assert.ok(more, 'more control present');
  await act(async () => {
    more.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
  });
  await act(async () => {});
  const menu = container.querySelector('.kb-list-more-menu');
  assert.ok(menu, 'menu opens');
  const text = menu.textContent ?? '';
  for (const label of ['置顶', '创建副本', '设置', '删除']) {
    assert.ok(text.includes(label), 'menu contains ' + label);
  }
  assert.equal(text.includes('编辑'), false, 'Vue does not expose a separate edit menu item');
  assert.equal(text.includes('分享'), false, 'Vue share dialog is not a card-menu action');
});

test('favorites star still persists to localStorage (existing behavior kept)', async () => {
  const container = await mountPage(makeClient());
  const star = container.querySelector('[data-kb-id="kb-doc"] .kb-favorite-star');
  assert.ok(star);
  await act(async () => {
    star.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
  });
  await act(async () => {});
  const stored = JSON.parse(dom.window.localStorage.getItem('wk-kb-favorites') ?? '[]') as string[];
  assert.ok(stored.includes('kb-doc'), 'favorite persisted');
  const favRailItem = Array.from(container.querySelectorAll('.kb-list-rail-item'))[1];
  assert.match(favRailItem.getAttribute('title') ?? '', /\(1\)/, 'rail favorites tooltip reflects the star count (Vue tooltipText)');
});
