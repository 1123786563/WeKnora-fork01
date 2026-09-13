// PlatformShell organizations sub-filter tests (jsdom + react-dom), following
// the harness in apps/web/src/platform/platform-shell-guide-reopen.test.tsx.
//
// Slice: R017 shell sub-filter closeout. Vue parity baseline —
//   frontend/src/views/organization/OrganizationList.vue:3-4 (ListSpaceSidebar
//   mode="organization" — the shared-space scope rail: 全部 / 我创建的 /
//   我加入的, backed by spaceSelection 'all' | 'created' | 'joined').
//   Vue menu.vue itself has NO organizations submenu (only the
//   pending-join-requests badge on the nav entry), so the shell mirrors the
//   established KB quick-filter block (plat-shell__kb-filters) as the
//   equivalent interaction, and navigates via the shared ?scope= convention.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });

const require = nodeModule.createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/organizations' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
const jsdomNavigator = dom.window.navigator;
try { Object.defineProperty(jsdomNavigator, 'language', { value: 'zh-CN', configurable: true }); } catch { /* keep default locale */ }
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: jsdomNavigator });

const { createRoot } = await import('react-dom/client');
const { PlatformShell } = await import('./PlatformShell.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
  window.history.replaceState({}, '', '/platform/organizations');
});

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});

function fakeClient(): Record<string, unknown> {
  return {
    auth: { me: async () => ({ user: { id: 'u1', username: 'tester', email: 'tester@local.dev', avatar: '' } }) },
    sessions: {
      list: async () => ({ data: [], total: 0, page: 1, page_size: 30 }),
    },
  };
}

async function mountShell(atPath: string): Promise<HTMLElement> {
  window.history.replaceState({}, '', atPath);
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(
      PlatformShell,
      { client: fakeClient() as never, onLogout: () => undefined, children: React.createElement('div', null, 'page') },
    ));
  });
  await settle(20);
  return container;
}

// The organizations block reuses the KB quick-filter classes (same visual
// language); tests disambiguate through the localized aria-label.
const orgFilters = (root: HTMLElement) => root.querySelector('.plat-shell__kb-filters[aria-label="共享空间"]');
const orgFilterLinks = (root: HTMLElement) => [...(orgFilters(root)?.querySelectorAll('a') ?? [])] as HTMLAnchorElement[];

test('organizations route renders the scope sub-filter with the Vue rail entries', async () => {
  const root = await mountShell('/platform/organizations');
  const block = orgFilters(root);
  assert.ok(block, 'expected an organizations sub-filter block under the shell nav');
  const links = orgFilterLinks(root);
  assert.equal(links.length, 3, 'sub-filter mirrors Vue ListSpaceSidebar: all/created/joined');
  const labels = links.map((link) => link.textContent);
  assert.deepEqual(labels, ['全部', '我创建的', '我加入的']);
  const hrefs = links.map((link) => link.getAttribute('href'));
  assert.deepEqual(hrefs, ['/platform/organizations', '/platform/organizations?scope=created', '/platform/organizations?scope=joined']);
});

test('sub-filter active state follows the ?scope= deep link', async () => {
  const allRoot = await mountShell('/platform/organizations');
  const allLinks = orgFilterLinks(allRoot);
  assert.match(allLinks[0]?.className ?? '', /plat-shell__kb-filter--active/, 'default (no param) activates 全部');
  assert.doesNotMatch(allLinks[1]?.className ?? '', /plat-shell__kb-filter--active/);

  const createdRoot = await mountShell('/platform/organizations?scope=created');
  const createdLinks = orgFilterLinks(createdRoot);
  assert.match(createdLinks[1]?.className ?? '', /plat-shell__kb-filter--active/, '?scope=created activates 我创建的');
  assert.doesNotMatch(createdLinks[0]?.className ?? '', /plat-shell__kb-filter--active/);

  const joinedRoot = await mountShell('/platform/organizations?scope=joined');
  const joinedLinks = orgFilterLinks(joinedRoot);
  assert.match(joinedLinks[2]?.className ?? '', /plat-shell__kb-filter--active/, '?scope=joined activates 我加入的');
});

test('other routes keep the shell clean: KB page shows the KB block, not the org block', async () => {
  const kbRoot = await mountShell('/platform/knowledge-bases');
  assert.equal(orgFilters(kbRoot), null, 'org sub-filter must not render outside /platform/organizations');
  const kbBlock = kbRoot.querySelector('.plat-shell__kb-filters');
  assert.ok(kbBlock, 'KB quick-filter block stays on the KB route');
  assert.equal(kbBlock.querySelectorAll('a').length, 2, 'KB block keeps its all/mine entries');

  const chatRoot = await mountShell('/platform/creatChat');
  assert.equal(orgFilters(chatRoot), null);
  assert.equal(chatRoot.querySelector('.plat-shell__kb-filters'), null);
});

test('collapsed sidebar hides the org sub-filter like the other shell blocks', async () => {
  window.localStorage.setItem('weknora_sidebar_collapsed', 'true');
  const root = await mountShell('/platform/organizations');
  assert.equal(orgFilters(root), null, 'collapsed sidebar hides the org sub-filter');
});
