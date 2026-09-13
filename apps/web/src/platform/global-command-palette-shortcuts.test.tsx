// Command-palette scoped ⌘1-9 slice tests (jsdom + react-dom), following the
// harness in shell-sessions-header.test.tsx.
//
// Slice: N003 deferred item — palette-scoped ⌘1-9 quick jumps + per-item kbd
// badges. Vue reference (all checked against the working tree):
//   GlobalCommandPalette.vue:4     keydown is bound on the dialog's .cmdk div,
//     so digits only reach the handler while the palette is open; there is NO
//     window-level digit binding (onGlobalKey:560-574 knows only ⌘K and "/").
//   GlobalCommandPalette.vue:508-520  (metaKey||ctrlKey) + e.key '1'..'9' ->
//     flatItems[n-1].run({cmd:false}); out of range -> no-op WITHOUT
//     preventDefault; digits precede the Enter branch (a digit can't be Enter).
//   GlobalCommandPalette.vue:496-499  shortcutFor(): digit 1-9 for flat
//     indices 0-8, undefined past the 9th slot.
//   GlobalCommandPalette/ResultItem.vue:32-34  badge markup
//     <kbd>⌘</kbd><kbd>{{n}}</kbd> whenever shortcut is defined (recents and
//     commands alike).
//   Flat order (Vue:279-299): empty query -> [recent..., commands...]; with a
//     query -> filtered commands only in this port (live search groups are a
//     separately tracked gap).
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

const require = nodeModule.createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge-bases' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
  // GlobalCommandPalette focuses its input via requestAnimationFrame on open;
  // plain jsdom (no pretendToBeVisual) does not define it, so shim one.
  requestAnimationFrame: (cb: (time: number) => void) => setTimeout(() => cb(16), 0) as unknown as number,
  cancelAnimationFrame: (id: number) => clearTimeout(id),
});

const { createRoot } = await import('react-dom/client');
const { GlobalCommandPalette } = await import('./GlobalCommandPalette.tsx');
import type { GlobalCommandPaletteProps } from './GlobalCommandPalette.tsx';

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

const settle = (ms: number) => act(async () => {
  await new Promise((resolve) => setTimeout(resolve, ms));
});

interface MountOptions {
  open?: boolean;
  recentQueries?: string[];
  initialQuery?: string;
}

async function mountPalette(options: MountOptions = {}): Promise<Record<string, { calls: unknown[] }>> {
  const spies = {
    onClose: { calls: [] as unknown[] },
    onNavigate: { calls: [] as unknown[] },
    onSearch: { calls: [] as unknown[] },
    onClearRecent: { calls: [] as unknown[] },
  };
  const props: GlobalCommandPaletteProps = {
    open: options.open ?? true,
    initialQuery: options.initialQuery ?? '',
    recentQueries: options.recentQueries ?? [],
    locale: 'zh-CN',
    onClose: () => { spies.onClose.calls.push(true); },
    onNavigate: (path) => { spies.onNavigate.calls.push(path); },
    onSearch: (query) => { spies.onSearch.calls.push(query); },
    onClearRecent: () => { spies.onClearRecent.calls.push(true); },
  };
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(React.createElement(GlobalCommandPalette, props));
  });
  await settle(5);
  return spies;
}

const dialog = () => document.querySelector('.cmdk');
const input = () => document.querySelector<HTMLInputElement>('.cmdk__input');

// Press a key on the palette input, like a user with focus in the search box.
const pressKey = (init: { key: string; metaKey?: boolean; ctrlKey?: boolean }) =>
  act(async () => {
    input()?.dispatchEvent(new window.KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init }));
  });

// Press a key straight on window (the rejected app-wide binding surface).
const windowKey = (init: { key: string; metaKey?: boolean; ctrlKey?: boolean }) =>
  act(() => {
    window.dispatchEvent(new window.KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init }));
  });

test('(a) ⌘1 with no recents runs the first quick action: 新建对话 → /platform/creatChat', async () => {
  const spies = await mountPalette();
  assert.ok(dialog(), 'expected the open palette dialog');
  assert.ok(document.body.textContent?.includes('新建对话'));
  await pressKey({ key: '1', metaKey: true });
  assert.deepEqual(spies.onNavigate.calls, ['/platform/creatChat']);
  assert.equal(spies.onClose.calls.length, 1, 'running a quick action closes the palette (Vue runCommand parity)');
  assert.deepEqual(spies.onSearch.calls, [], 'empty query must not record a recent search');
});

test('(b) ⌘N crosses the recents→commands boundary: ⌘3 with two recents runs 新建对话', async () => {
  const spies = await mountPalette({ recentQueries: ['alpha', 'beta'] });
  // Flat order: [alpha(⌘1), beta(⌘2), 新建对话(⌘3), …] — Vue:279-299 group order.
  await pressKey({ key: '3', metaKey: true });
  assert.deepEqual(spies.onNavigate.calls, ['/platform/creatChat'], 'flat index 2 = first command after two recents');
  assert.equal(spies.onClose.calls.length, 1);
});

test('(c) ⌘1 on a recent query fills the input in place; palette stays open (Vue recent run)', async () => {
  const spies = await mountPalette({ recentQueries: ['alpha'] });
  await pressKey({ key: '1', metaKey: true });
  assert.equal(input()?.value, 'alpha', 'recent run mirrors Vue query = q');
  assert.deepEqual(spies.onNavigate.calls, [], 'picking a recent never navigates');
  assert.deepEqual(spies.onClose.calls, [], 'picking a recent keeps the palette open');
});

test('(d) out-of-range ⌘9 with fewer than nine items is a no-op left to the browser', async () => {
  const spies = await mountPalette();
  await pressKey({ key: '9', metaKey: true });
  assert.deepEqual(spies.onNavigate.calls, []);
  assert.deepEqual(spies.onClose.calls, []);
  assert.ok(dialog(), 'palette stays open');
});

test('(e) plain digits (no ⌘/Ctrl) never jump rows — typing 1 just types', async () => {
  const spies = await mountPalette();
  await pressKey({ key: '1' });
  assert.deepEqual(spies.onNavigate.calls, []);
  assert.deepEqual(spies.onClose.calls, []);
  assert.ok(dialog(), 'palette stays open');
});

test('(f) Ctrl+1 is equivalent to ⌘1 (Vue matches metaKey || ctrlKey)', async () => {
  const spies = await mountPalette();
  await pressKey({ key: '1', ctrlKey: true });
  assert.deepEqual(spies.onNavigate.calls, ['/platform/creatChat']);
});

test('(g) with an active query ⌘1 runs the first filtered command and records the search', async () => {
  const spies = await mountPalette({ initialQuery: 'agent' });
  assert.ok(document.body.textContent?.includes('打开智能体'), 'expected only the agents command to remain visible');
  await pressKey({ key: '1', metaKey: true });
  assert.deepEqual(spies.onNavigate.calls, ['/platform/agents']);
  assert.deepEqual(spies.onSearch.calls, ['agent'], 'running a command from a searched query records the recent');
  assert.equal(spies.onClose.calls.length, 1);
});

test('(h) the first nine visible rows show ⌘1-⌘9 kbd badges in flat order (ResultItem.vue:32-34)', async () => {
  await mountPalette({ recentQueries: ['r1', 'r2', 'r3', 'r4'] }); // 4 recents + 5 commands = 9 items
  const badges = [...document.querySelectorAll<HTMLElement>('.cmdk__item .cmdk__item-shortcut')];
  assert.equal(badges.length, 9, 'one badge per visible row while there are at most nine');
  const digits = badges.map((badge) => badge.querySelector('kbd:last-child')?.textContent);
  assert.deepEqual(digits, ['1', '2', '3', '4', '5', '6', '7', '8', '9']);
  assert.equal(badges[0]?.querySelector('kbd:first-child')?.textContent, '⌘');
  // Badges land on recent rows too (Vue passes shortcutFor() to recents).
  const firstItem = document.querySelector('.cmdk__item');
  assert.ok(firstItem?.textContent?.includes('r1'));
  assert.ok(firstItem?.querySelector('.cmdk__item-shortcut'));
});

test('(i) a filtered list renumbers badges from 1 (flat order, not catalogue position)', async () => {
  // 'set' matches only open-settings (keyword 'settings') — catalogue
  // position 5, but the filtered view must badge it ⌘1 (Vue flatIndexFor).
  await mountPalette({ initialQuery: 'set' });
  const badges = [...document.querySelectorAll<HTMLElement>('.cmdk__item .cmdk__item-shortcut')];
  assert.equal(badges.length, 1, 'expected exactly the settings command to survive the filter');
  assert.equal(badges[0]?.querySelector('kbd:last-child')?.textContent, '1', 'badges restart at 1 after filtering');
});

test('(j) REGRESSION PIN: with the palette closed ⌘1 is completely inert', async () => {
  const spies = await mountPalette({ open: false });
  assert.equal(dialog(), null, 'closed palette renders nothing (no dialog, no listener host)');
  // Round N+3 ruling: the rejected slice bound ⌘1 app-wide. Guard the seam:
  // window-level ⌘1 while closed must not navigate, open, or record anything.
  await windowKey({ key: '1', metaKey: true });
  await windowKey({ key: '1', ctrlKey: true });
  await settle(5);
  assert.deepEqual(spies.onNavigate.calls, [], 'no navigation while closed');
  assert.deepEqual(spies.onClose.calls, []);
  assert.deepEqual(spies.onSearch.calls, []);
  assert.equal(dialog(), null, '⌘1 must not even open the palette (Vue onGlobalKey only knows ⌘K and "/")');
});

test('(k) in-range ⌘ presses are cancelable; out-of-range is left un-prevented (Vue parity)', async () => {
  await mountPalette();
  const inRange = new window.KeyboardEvent('keydown', { key: '1', metaKey: true, bubbles: true, cancelable: true });
  await act(async () => { input()?.dispatchEvent(inRange); });
  assert.equal(inRange.defaultPrevented, true, 'in-range jump prevents the browser default (Vue e.preventDefault())');
  const outOfRange = new window.KeyboardEvent('keydown', { key: '9', metaKey: true, bubbles: true, cancelable: true });
  await act(async () => { input()?.dispatchEvent(outOfRange); });
  assert.equal(outOfRange.defaultPrevented, false, 'out-of-range digit falls through like Vue (if (item) guard)');
});
