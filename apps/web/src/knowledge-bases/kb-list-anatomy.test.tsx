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
  is_pinned?: boolean;
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

test('(a) q deep-link filters the Vue section rows as well as the cards', async () => {
  const container = await mountPage(makeClient(), '?q=faq');
  const cards = Array.from(container.querySelectorAll('.kb-list-card'));
  assert.equal(cards.length, 1, 'query should only render matching cards');
  assert.equal(cards[0]?.getAttribute('data-kb-id'), 'kb-faq');
  const mine = container.querySelector('.kb-list-section-header');
  assert.ok(mine, 'filtered results keep the section header');
  assert.equal(mine?.querySelector('.kb-list-section-count')?.textContent, '1', 'section count follows filtered results');
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

test('(a) pinned cards use the filled pin icon in the more menu', async () => {
  const container = await mountPage(makeClient({
    owned: [{ id: 'kb-pinned', name: 'Pinned KB', type: 'document', is_pinned: true, creator_id: 'u-1' }],
  }));
  const more = container.querySelector('[data-kb-id="kb-pinned"] .kb-list-card-more');
  assert.ok(more);
  await act(async () => {
    more.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
  });
  await act(async () => {});
  assert.ok(container.querySelector('.kb-list-more-menu svg[data-kb-icon="pin-filled"]'), 'pinned action uses pin-filled');
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

// R009 rail drag-expand slice (Vue ListSpaceSidebar.vue:146-235, 296-343): the
// collapsed icon strip and the 208px expanded nav panel are two states of the
// same rail; the right-edge resize handle drags the width and snaps
// (>= 120px -> expand, else collapse), persisting via
// localStorage['sidebar-collapsed-list-expanded'].

test('(b) rail drag past the 120px snap threshold expands into the panel with full labels + counts and drags back collapsed', async () => {
  const container = await mountPage(makeClient());
  const rail = container.querySelector('.kb-list-rail') as HTMLElement;
  assert.ok(rail);
  assert.equal(rail.classList.contains('kb-list-rail-expanded'), false, 'starts collapsed (Vue default)');
  assert.ok(rail.querySelector('.kb-list-rail-strip'), 'collapsed strip DOM');
  assert.equal(rail.querySelector('.kb-list-rail-panel'), null, 'no expanded panel while collapsed');
  const handle = rail.querySelector('.kb-list-rail-handle');
  assert.ok(handle, 'right-edge resize handle present (Vue .resize-handle)');

  // Drag right by +90px from 56 -> 146 (< 120 delta from collapsed start? no:
  // Vue compares the absolute width, 146 >= 120 -> expands on release).
  await act(async () => {
    handle.dispatchEvent(new dom.window.MouseEvent('mousedown', { bubbles: true, cancelable: true, clientX: 100 }));
  });
  assert.equal(rail.classList.contains('kb-list-rail-dragging'), true, 'dragging class disables the width transition');
  assert.equal((rail as HTMLElement).style.width, '56px', 'inline width tracks from the collapsed start (ListSpaceSidebar.vue:3)');
  assert.equal(document.body.style.cursor, 'col-resize', 'body cursor matches Vue onDragStart');
  await act(async () => {
    document.dispatchEvent(new dom.window.MouseEvent('mousemove', { bubbles: true, clientX: 190 }));
  });
  assert.equal((rail as HTMLElement).style.width, '146px', 'drag width follows the pointer (clamped to [56, 228])');
  await act(async () => {
    document.dispatchEvent(new dom.window.MouseEvent('mouseup', { bubbles: true }));
  });
  assert.equal(rail.classList.contains('kb-list-rail-expanded'), true, 'width 146 >= snap threshold 120 expands');
  assert.equal((rail as HTMLElement).style.width, '', 'expanded width comes from CSS (208px), not inline');
  assert.equal(document.body.style.cursor, '', 'body cursor restored on drag end');
  const panel = rail.querySelector('.kb-list-rail-panel');
  assert.ok(panel, 'expanded nav panel DOM (Vue .expanded-panel, ListSpaceSidebar.vue:77)');
  const labels = Array.from(panel.querySelectorAll('.kb-list-rail-panel-label')).map((el) => el.textContent);
  assert.deepEqual(labels, ['全部', '收藏', '最近', '本空间'], 'expanded panel renders the full text labels');
  const counts = Array.from(panel.querySelectorAll('.kb-list-rail-panel-count')).map((el) => el.textContent);
  assert.deepEqual(counts, ['3', '3'], 'count badges: all + mine always, favorites/recents hidden at 0 (Vue :83/:93/:101/:109)');
  assert.equal(panel.querySelectorAll('.kb-list-rail-divider').length, 1, 'divider between recents and workspace (Vue :103)');
  const activeLabel = panel.querySelector('.kb-list-rail-panel-item-active .kb-list-rail-panel-label');
  assert.equal(activeLabel?.textContent, '本空间', 'active state carried into the expanded panel');
  assert.equal(dom.window.localStorage.getItem('sidebar-collapsed-list-expanded'), 'true', 'expanded state persisted (Vue storageKey :198/:232)');

  // Drag back: 208 - 110 = 98 < 120 -> collapse to the strip.
  await act(async () => {
    handle.dispatchEvent(new dom.window.MouseEvent('mousedown', { bubbles: true, cancelable: true, clientX: 300 }));
  });
  await act(async () => {
    document.dispatchEvent(new dom.window.MouseEvent('mousemove', { bubbles: true, clientX: 190 }));
  });
  assert.equal((rail as HTMLElement).style.width, '98px');
  await act(async () => {
    document.dispatchEvent(new dom.window.MouseEvent('mouseup', { bubbles: true }));
  });
  assert.equal(rail.classList.contains('kb-list-rail-expanded'), false, 'drag under the threshold collapses back');
  assert.ok(rail.querySelector('.kb-list-rail-strip'), 'collapsed strip DOM restored');
  assert.equal(rail.querySelector('.kb-list-rail-panel'), null, 'panel unmounted after collapse');
  assert.equal(dom.window.localStorage.getItem('sidebar-collapsed-list-expanded'), 'false', 'collapsed state persisted');
});

test('(b) rail expanded state round-trips through localStorage (Vue storageKey sidebar-collapsed-list-expanded)', async () => {
  dom.window.localStorage.setItem('sidebar-collapsed-list-expanded', 'true');
  const container = await mountPage(makeClient());
  const rail = container.querySelector('.kb-list-rail') as HTMLElement;
  assert.ok(rail);
  assert.equal(rail.classList.contains('kb-list-rail-expanded'), true, 'seeds expanded from localStorage like Vue (:200)');
  assert.ok(rail.querySelector('.kb-list-rail-panel'), 'panel mounted directly');
  assert.equal(rail.querySelector('.kb-list-rail-strip'), null, 'strip not rendered while expanded (Vue v-if/v-else)');
});

test('(b) expanded panel keeps the ?scope deep-link semantics (setSpace path unchanged)', async () => {
  dom.window.localStorage.setItem('sidebar-collapsed-list-expanded', 'true');
  const container = await mountPage(makeClient());
  const fav = Array.from(container.querySelectorAll('.kb-list-rail-panel-item'))[1] as HTMLButtonElement;
  assert.equal(fav?.textContent, '收藏', 'favorites panel item present');
  await act(async () => {
    fav.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
  });
  await act(async () => {});
  assert.equal(dom.window.location.search, '?scope=favorites', 'panel click still writes ?scope to the URL');
  assert.equal(fav.classList.contains('kb-list-rail-panel-item-active'), true, 'panel item reflects the active scope');
});
// R009 org rail slice (Vue ListSpaceSidebar.vue:111-125 / :35-49 +
// KnowledgeBaseList.vue:897-985): orgs with a positive share count render a
// per-org entry below the workspace bucket ("共享给我" section); clicking one
// switches to the per-space view — ?scope=<orgId> filters merged cards to
// that org's shared KBs.

interface SharedRow {
  knowledge_base: Record<string, unknown> | null;
  permission: string;
  shared_at: string;
  share_id: string;
  organization_id: string;
  org_name: string;
}

function sharedRow(overrides: Partial<SharedRow> & { share_id: string; knowledge_base: Record<string, unknown> | null }): SharedRow {
  return {
    permission: 'editor',
    shared_at: '2026-01-01T00:00:00Z',
    organization_id: 'org-alpha',
    org_name: '设计组',
    ...overrides,
  } as SharedRow;
}

const orgSharedRows = (): SharedRow[] => [
  sharedRow({ share_id: 'share-a1', knowledge_base: { id: 'kb-sh-a1', name: '共享库A1', type: 'document', knowledge_count: 4, creator_id: 'u-2' } }),
  sharedRow({ share_id: 'share-a2', knowledge_base: { id: 'kb-sh-a2', name: '共享库A2', type: 'faq', chunk_count: 9, knowledge_count: 0, creator_id: 'u-2' }, permission: 'viewer' }),
  sharedRow({ share_id: 'share-b1', organization_id: 'org-beta', org_name: '后端组', knowledge_base: { id: 'kb-sh-b1', name: '共享库B1', type: 'document', knowledge_count: 1, creator_id: 'u-3' } }),
];

test('(b) rail appends per-org entries for orgs with shared KBs (countByOrg > 0 only)', async () => {
  const container = await mountPage(makeClient({ shared: orgSharedRows() }));
  const strip = container.querySelector('.kb-list-rail-strip');
  assert.ok(strip, 'collapsed strip renders');
  const labels = Array.from(strip.querySelectorAll('.kb-list-rail-label')).map((el) => el.textContent);
  assert.deepEqual(labels, ['全部', '收藏', '最近', '本空间', '设计组', '后端组'],
    'org entries render below the workspace bucket (ListSpaceSidebar.vue:113-125)');
  const orgItem = strip.querySelectorAll('.kb-list-rail-item')[4] as HTMLElement;
  assert.match(orgItem.getAttribute('title') ?? '', /设计组 \(2\)/, 'org tooltip is tooltipText(name, count)');
  assert.ok(strip.querySelector('.kb-list-rail-divider'), 'divider separates workspace from the org group (Vue :41)');
  assert.ok(container.querySelector('.kb-list-rail-panel') === null, 'stays collapsed by default');
  // Org with zero shares must not render an entry (organizationsWithCount
  // filters count > 0, ListSpaceSidebar.vue:271-274).
  const bare = await mountPage(makeClient());
  assert.equal((bare.querySelector('.kb-list-rail-strip')?.querySelectorAll('.kb-list-rail-item').length ?? 0), 4, 'no org entries without shares');
});

test('(b) clicking an org rail entry writes ?scope=<orgId> and filters merged cards to that org', async () => {
  const container = await mountPage(makeClient({ shared: orgSharedRows() }));
  const strip = container.querySelector('.kb-list-rail-strip') as HTMLElement;
  const orgItem = strip.querySelectorAll('.kb-list-rail-item')[4] as HTMLElement;
  await act(async () => {
    orgItem.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
  });
  await act(async () => {});
  assert.equal(dom.window.location.search, '?scope=org-alpha', 'per-space view lives in ?scope (KnowledgeBaseList.vue:821)');
  const active = container.querySelector('.kb-list-rail-item.kb-list-rail-item-active .kb-list-rail-label');
  assert.equal(active?.textContent, '设计组', 'active state follows the org scope');
  const cards = Array.from(container.querySelectorAll('.kb-list-card')).map((el) => el.getAttribute('data-kb-id'));
  assert.deepEqual(cards.sort(), ['kb-sh-a1', 'kb-sh-a2'], 'only the org shared KBs render');
  assert.equal(container.querySelectorAll('.kb-list-section-header').length, 0, 'per-space view renders a flat grid (Vue sortedSpaceKbsList)');
});

test('(b) ?scope=<orgId> deep link activates the org entry and filters cards', async () => {
  const container = await mountPage(makeClient({ shared: orgSharedRows() }), '?scope=org-alpha');
  const active = container.querySelector('.kb-list-rail-item.kb-list-rail-item-active .kb-list-rail-label');
  assert.equal(active?.textContent, '设计组', 'deep-linked org entry is active');
  const cards = Array.from(container.querySelectorAll('.kb-list-card')).map((el) => el.getAttribute('data-kb-id'));
  assert.deepEqual(cards.sort(), ['kb-sh-a1', 'kb-sh-a2'], 'cards filtered by organization_id');
  const readonlyCard = container.querySelector('[data-kb-id="kb-sh-a2"]');
  assert.ok(readonlyCard, 'viewer-permission shared KB still renders in its org view');
});

test('(b) unknown org scope keeps the Vue shared-empty state without the create CTA', async () => {
  const container = await mountPage(makeClient(), '?scope=org-gone');
  const empty = container.querySelector('.kb-list-empty');
  assert.ok(empty, 'per-space empty state renders');
  assert.match(container.querySelector('.kb-list-empty-title')?.textContent ?? '', /暂无共享知识库/);
  assert.match(container.querySelector('.kb-list-empty-desc')?.textContent ?? '', /加入共享空间/);
  assert.ok(container.querySelector('.kb-list-empty-img'), 'per-space empty keeps the illustration (KnowledgeBaseList.vue:674-678)');
  assert.equal(container.querySelector('.kb-list-empty [data-guide="kb-list-create"]'), null, 'no create CTA in the per-space empty state');
});

test('(b) stale ?scope=shared deep link resets to the all view', async () => {
  const container = await mountPage(makeClient({ shared: orgSharedRows() }), '?scope=shared');
  const active = container.querySelector('.kb-list-rail-item.kb-list-rail-item-active .kb-list-rail-label');
  assert.equal(active?.textContent, '全部', 'legacy aggregate scope falls back to all (KnowledgeBaseList.vue:1246-1251)');
  const staleSearch = dom.window.location.search;
  assert.ok(staleSearch === '' || staleSearch === '?scope=all', 'stale scope cleaned from the URL, got ' + staleSearch);
  assert.equal(container.querySelectorAll('.kb-list-card').length, 6, 'all view shows owned + shared cards again');
});

test('(b) expanded panel renders the shared-spaces section title and full org labels', async () => {
  dom.window.localStorage.setItem('sidebar-collapsed-list-expanded', 'true');
  const container = await mountPage(makeClient({ shared: orgSharedRows() }));
  const panel = container.querySelector('.kb-list-rail-panel');
  assert.ok(panel, 'expanded panel mounted');
  const title = panel.querySelector('.kb-list-rail-section-title');
  assert.equal(title?.textContent, '共享给我', 'section title (Vue .sidebar-section, listSpaceSidebar.spaces)');
  const labels = Array.from(panel.querySelectorAll('.kb-list-rail-panel-label')).map((el) => el.textContent);
  assert.deepEqual(labels, ['全部', '收藏', '最近', '本空间', '设计组', '后端组'], 'full org labels in the panel');
  const counts = Array.from(panel.querySelectorAll('.kb-list-rail-panel-count')).map((el) => el.textContent);
  assert.deepEqual(counts, ['6', '3', '2', '1'], 'all + mine always, favorites/recents at 0 hidden, org counts from shares');
  const orgPanelItem = panel.querySelectorAll('.kb-list-rail-panel-item')[4] as HTMLElement;
  await act(async () => {
    orgPanelItem.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
  });
  await act(async () => {});
  assert.equal(dom.window.location.search, '?scope=org-alpha', 'panel org click writes the per-space scope');
  const active = panel.querySelector('.kb-list-rail-panel-item-active .kb-list-rail-panel-label');
  assert.equal(active?.textContent, '设计组', 'panel active state follows the org scope');
});
