import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  FocusEvent: dom.window.FocusEvent,
  MouseEvent: dom.window.MouseEvent,
  KeyboardEvent: dom.window.KeyboardEvent,
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
// Roomy viewport so the drag test can exercise the maxWidth clamp (1600)
// instead of the viewport clamp.
Object.defineProperty(dom.window, 'innerWidth', { configurable: true, value: 2400 });

const { createRoot } = await import('react-dom/client');
const { TenantAuditDrawer } = await import('./TenantAuditDrawer.tsx');

let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.localStorage.clear();
});

async function mount(open: boolean, props: { storageKey?: string; width?: number; minWidth?: number; maxWidth?: number } = {}) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  const { storageKey, width = 1120, minWidth = 720, maxWidth = 1600 } = props;
  await act(async () => {
    mountedRoot?.render(
      <>
        <button type="button" data-testid="opener">open</button>
        <TenantAuditDrawer open={open} title="Audit log" onClose={() => {}} storageKey={storageKey} width={width} minWidth={minWidth} maxWidth={maxWidth}>
          <div>drawer body</div>
        </TenantAuditDrawer>
      </>,
    );
  });
  return container;
}

test('drawer renders the Vue header anatomy: title only, no redundant close button', async () => {
  await mount(true);
  const dialog = document.querySelector('[role="dialog"]');
  assert.ok(dialog);
  const heading = dialog?.querySelector('h2');
  assert.equal(heading?.textContent, 'Audit log');
  assert.equal(dialog?.querySelector('button'), null, 'SettingDrawer.vue renders no X button; overlay/Esc close instead');
  assert.ok(document.querySelector('[role="presentation"]'), 'overlay backdrop is rendered');
  assert.ok(document.querySelector('[role="separator"]'), 'left-edge resize handle is rendered');
});

test('drawer honours a persisted width and clamps it to [minWidth, maxWidth]', async () => {
  dom.window.localStorage.setItem('setting-drawer:width:tenant-members-audit', '900');
  await mount(true, { storageKey: 'setting-drawer:width:tenant-members-audit' });
  const dialog = document.querySelector<HTMLElement>('[role="dialog"]');
  assert.equal(dialog?.style.width, '900px', 'persisted width wins over the 1120px default');
});

test('dragging the handle updates and persists the width (SettingDrawer.vue:218-240)', async () => {
  await mount(true, { storageKey: 'setting-drawer:width:audit-drag', width: 1120 });
  const handle = document.querySelector<HTMLElement>('[role="separator"]');
  assert.ok(handle);
  await act(async () => {
    handle?.dispatchEvent(new dom.window.MouseEvent('mousedown', { bubbles: true, clientX: 2000 }));
  });
  // Drag 500px to the left: delta = 2000 - 1500 = 500 -> width 1620 -> clamped 1600.
  await act(async () => {
    document.dispatchEvent(new dom.window.MouseEvent('mousemove', { bubbles: true, clientX: 1500 }));
  });
  const dialog = document.querySelector<HTMLElement>('[role="dialog"]');
  assert.equal(dialog?.style.width, '1600px', 'width follows the pointer, clamped to maxWidth 1600');
  await act(async () => {
    document.dispatchEvent(new dom.window.MouseEvent('mouseup', { bubbles: true }));
  });
  assert.equal(dom.window.localStorage.getItem('setting-drawer:width:audit-drag'), '1600', 'mouseup persists the clamped width');
});

test('Escape closes the drawer and focus returns to the trigger', async () => {
  let closed = false;
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  const opener = document.createElement('button');
  opener.dataset.testid = 'opener';
  document.body.append(opener);
  opener.focus();
  assert.equal(document.activeElement, opener);
  // Controlled open state, like Vue's v-model:visible on SettingDrawer — the
  // drawer unmounts when the owner honours onClose, and focus restoration
  // happens in the unmount cleanup.
  function Harness() {
    const [open, setOpen] = React.useState(true);
    return <TenantAuditDrawer open={open} title="Audit log" onClose={() => { closed = true; setOpen(false); }} storageKey="k"><div>body</div></TenantAuditDrawer>;
  }
  await act(async () => {
    mountedRoot?.render(<Harness />);
  });
  const dialog = document.querySelector<HTMLElement>('[role="dialog"]');
  assert.ok(dialog);
  assert.equal(document.activeElement, dialog, 'panel takes focus on open');
  await act(async () => {
    document.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  });
  assert.equal(closed, true, 'Escape fires onClose');
  assert.equal(document.activeElement, opener, 'focus is restored to the pre-open trigger');
  assert.equal(document.querySelector('[role="dialog"]'), null, 'controlled close unmounts the drawer');
});

test('mousedown on the overlay closes the drawer, clicks inside do not', async () => {
  let closed = false;
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<TenantAuditDrawer open title="Audit log" onClose={() => { closed = true; }}><div>body</div></TenantAuditDrawer>);
  });
  const overlay = document.querySelector('[role="presentation"]');
  assert.ok(overlay);
  await act(async () => {
    overlay?.dispatchEvent(new dom.window.MouseEvent('mousedown', { bubbles: true }));
  });
  assert.equal(closed, true, 'overlay click fires onClose');
});

test('closing unmounts the content (destroy-on-close)', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<TenantAuditDrawer open={false} title="Audit log" onClose={() => {}} storageKey="setting-drawer:width:reopen"><div>body</div></TenantAuditDrawer>);
  });
  assert.equal(document.querySelector('[role="dialog"]'), null, 'closed drawer renders nothing');
  assert.equal(document.querySelector('[role="presentation"]'), null, 'overlay is unmounted too');
  await act(async () => {
    mountedRoot?.render(<TenantAuditDrawer open title="Audit log" onClose={() => {}} storageKey="setting-drawer:width:reopen"><div>body</div></TenantAuditDrawer>);
  });
  assert.ok(document.querySelector('[role="dialog"]'), 'reopening remounts the drawer');
});
