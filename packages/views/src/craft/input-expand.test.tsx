// T20 (#139) front-end gate for the T02 (#121) deferred item: the archive
// expansion entrance projects the EXISTING guarded endpoint only. The
// affordance appears for archive-named associated inputs; in-flight progress
// is honest (aria-busy + disabled controls); success lists exactly the
// members the SERVER published (all-or-nothing); a refusal keeps the
// entrance and never fabricates members.
import assert from 'node:assert/strict';
import test from 'node:test';
import React, { act } from 'react';
import { JSDOM } from 'jsdom';
import * as nodeModule from 'node:module';
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (h: { resolve: (specifier: string, context: unknown, nextResolve: (s: string, c: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });
const { CraftInputExpandPanel, isExpandableArchiveInput } = await import('./input-expand.tsx');

const archiveInput = {
  ref: 'resource://archive-1', name: 'region-sales.zip', sha256: 'a'.repeat(64), bytes: 2048,
  citation_id: '', recognition: { accepted: true, understood: true, reason: '' },
} as const;
const textInput = {
  ref: 'resource://text-1', name: 'notes.txt', sha256: 'b'.repeat(64), bytes: 12,
  citation_id: '', recognition: { accepted: true, understood: true, reason: '' },
} as const;
const members = [
  { ref: 'resource://member-1', name: 'q1.csv', sha256: 'c'.repeat(64), bytes: 10, citation_id: '', recognition: { accepted: true, understood: true, reason: '' } },
  { ref: 'resource://member-2', name: 'readme.md', sha256: 'd'.repeat(64), bytes: 20, citation_id: '', recognition: { accepted: true, understood: true, reason: '' } },
];

async function mount(onExpand: (ref: string) => Promise<readonly typeof members[number][]>) {
  const dom = new JSDOM('<!doctype html><html><body></body></html>');
  Object.assign(globalThis, {
    window: dom.window, document: dom.window.document,
    HTMLElement: dom.window.HTMLElement, Element: dom.window.Element, Node: dom.window.Node, Event: dom.window.Event,
  });
  const { createRoot } = await import('../../../../apps/web/node_modules/react-dom/client.js');
  const root = createRoot(dom.window.document.body.appendChild(dom.window.document.createElement('div')));
  await act(async () => {
    root.render(React.createElement(CraftInputExpandPanel, {
      locale: 'zh',
      inputs: [archiveInput, textInput] as unknown as readonly import('@weknora/contracts').CraftInputView[],
      onExpand,
    }));
  });
  return {
    dom,
    clickExpand: async () => {
      const button = [...dom.window.document.querySelectorAll('button')].find((b) => (b.textContent ?? '').includes('展开归档'));
      assert.ok(button !== undefined, 'the expand affordance must be rendered');
      await act(async () => {
        button.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
      });
    },
    unmount: async () => { await act(async () => root.unmount()); dom.window.close(); },
  };
}

test('the expansion entrance appears only for archive-named inputs — projection never invents the affordance', async () => {
  const app = await mount(async () => members);
  const markup = app.dom.window.document.body.innerHTML;
  assert.match(markup, /region-sales\.zip/, 'the archive row is listed');
  assert.match(markup, /展开归档/, 'the entrance is offered for the archive');
  assert.doesNotMatch(markup, /notes\.txt/, 'a non-archive input never offers expansion');
  assert.match(markup, /一次性发布为不可变输入/, 'the all-or-nothing contract is stated');
  await app.unmount();
});

test('in-flight expansion shows honest progress (aria-busy, pending label, disabled control) until the server publishes', async () => {
  let release: ((value: readonly typeof members[number][]) => void) | null = null;
  const app = await mount(() => new Promise((resolve) => { release = resolve; }));
  const pending = app.clickExpand();
  await new Promise((resolve) => setTimeout(resolve, 0));
  const busy = app.dom.window.document.querySelector('[data-archive-ref="resource://archive-1"]');
  assert.equal(busy?.getAttribute('aria-busy'), 'true', 'the row marks itself busy while the server extracts');
  assert.match(app.dom.window.document.body.innerHTML, /展开中…/, 'the pending label is visible');
  release?.(members);
  await pending;
  assert.equal(busy?.getAttribute('aria-busy'), null, 'the busy mark clears once the server answered');
  await app.unmount();
});

test('success lists exactly the server-published members; a refusal keeps the entrance and fabricates nothing', async () => {
  const ok = await mount(async () => members);
  await ok.clickExpand();
  const markup = ok.dom.window.document.body.innerHTML;
  assert.match(markup, /已展开 2 个成员/, 'the published member count projects the server answer');
  assert.match(markup, /q1\.csv/);
  assert.match(markup, /readme\.md/);
  assert.doesNotMatch(markup, /展开归档/, 'an expanded archive does not offer a second expansion');
  await ok.unmount();

  const refused = await mount(async () => { throw new Error('403'); });
  await refused.clickExpand();
  const refusedMarkup = refused.dom.window.document.body.innerHTML;
  assert.match(refusedMarkup, /role="alert"/, 'the refusal is a member-visible alert');
  assert.match(refusedMarkup, /未发布任何成员/, 'the all-or-nothing failure is stated');
  assert.match(refusedMarkup, /展开归档/, 'the entrance stays for a deliberate retry');
  assert.doesNotMatch(refusedMarkup, /q1\.csv/, 'a refusal never lists fabricated members');
  await refused.unmount();
});

test('the archive vocabulary mirrors the server extension list', () => {
  assert.equal(isExpandableArchiveInput({ ...archiveInput, name: 'bundle.tar.gz' } as import('@weknora/contracts').CraftInputView), true);
  assert.equal(isExpandableArchiveInput({ ...archiveInput, name: 'bundle.tgz' } as import('@weknora/contracts').CraftInputView), true);
  assert.equal(isExpandableArchiveInput({ ...archiveInput, name: 'photo.png' } as import('@weknora/contracts').CraftInputView), false);
});
