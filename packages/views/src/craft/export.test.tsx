// T13 (#133) front-end domain rule: exporting restricted derived data is a
// server authority this panel only PROJECTS. The server classifies every
// derived file from its recorded origins, and the owner decides against the
// exact manifest digest; the panel never derives authority client-side: a
// decision that does not bind the current version and manifest digest is
// stale and never renders as consented, and no URL ever enters the panel —
// origins render as the opaque identity they are.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import { JSDOM } from 'jsdom';
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (h: {resolve: (specifier: string, context: unknown, nextResolve: (s: string, c: unknown) => unknown) => unknown}) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });
const React = await import('react');
const { renderToStaticMarkup } = await import('../../../../apps/web/node_modules/react-dom/server.js');
const { CraftExportConsentPanel, projectExportConsentView } = await import('./export.tsx');

const digest = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef';
const otherDigest = 'fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210';
const ownOrigin = { kind: 'knowledge', ref: 'craftkb://kb/kb-own/k-own/c-own', sha256: digest, restricted: false };
const sharedOrigin = { kind: 'knowledge', ref: 'craftkb://kb/kb-shared/k-shared/c-shared', sha256: otherDigest, restricted: true };

const raw = {
  version_id: 'ver-1',
  manifest_digest: digest,
  state: 'awaiting',
  restricted_derived: ['index.html'],
  files: [
    { path: 'index.html', sha256: digest, restricted: false, origins: [ownOrigin, sharedOrigin] },
    { path: 'clean.txt', sha256: otherDigest, restricted: false, origins: [ownOrigin] },
  ],
  decision: null,
};

test('the owner sees the exact files, their origins and the classification before any consent', () => {
  const view = projectExportConsentView(raw);
  assert.ok(view);
  const markup = renderToStaticMarkup(React.createElement(CraftExportConsentPanel, {
    locale: 'en', role: 'owner', view, onDecide: () => {},
  }));
  assert.match(markup, /ver-1/, 'the exact immutable version is shown');
  assert.match(markup, /0123456789abcdef…/, 'the manifest digest is shown before any consent');
  assert.match(markup, /index.html/, 'every derived file is listed');
  assert.match(markup, /clean\.txt/, 'the unrestricted member is listed too');
  assert.match(markup, /restricted/i, 'the restricted derived member is marked');
  assert.match(markup, /kb-shared/, 'the restricted origin is named — the owner sees exact origins');
  assert.match(markup, /Approve export/, 'the explicit approval affordance');
  assert.match(markup, /Export without restricted files/, 'the explicit safe-bundle affordance');
});

test('a non-owner projects the same facts without any consent control', () => {
  const view = projectExportConsentView(raw);
  assert.ok(view);
  const markup = renderToStaticMarkup(React.createElement(CraftExportConsentPanel, {
    locale: 'en', role: 'viewer', view, onDecide: () => {},
  }));
  assert.match(markup, /index.html/, 'the viewer still sees the file list');
  assert.doesNotMatch(markup, /<button/, 'a viewer never holds consent controls');
});

test('the panel never renders a URL — origins stay opaque identities, never links', () => {
  const view = projectExportConsentView(raw);
  assert.ok(view);
  const markup = renderToStaticMarkup(React.createElement(CraftExportConsentPanel, {
    locale: 'en', role: 'owner', view, onDecide: () => {},
  }));
  assert.doesNotMatch(markup, /href=/, 'no link is ever rendered');
  assert.doesNotMatch(markup, /https?:\/\//, 'no provider URL is ever rendered');
});

test('a live consented state projects the binding and hides the approval controls', () => {
  const view = projectExportConsentView({
    ...raw,
    state: 'consented',
    decision: { version_id: 'ver-1', manifest_digest: digest, owner_id: 'u-owner', decision: 'approved' },
  });
  assert.ok(view);
  assert.equal(view.state, 'consented');
  const markup = renderToStaticMarkup(React.createElement(CraftExportConsentPanel, {
    locale: 'en', role: 'owner', view, onDecide: () => {},
  }));
  assert.match(markup, /Consented/, 'the live export authority is projected');
  assert.match(markup, /u-owner/, 'the decision names the consenting owner');
  assert.doesNotMatch(markup, /Approve export/, 'no duplicate approval while a consent is live');
});

test('a declined state stays decidable — a fresh decision upserts server-side', () => {
  const view = projectExportConsentView({
    ...raw,
    state: 'declined',
    decision: { version_id: 'ver-1', manifest_digest: digest, owner_id: 'u-owner', decision: 'rejected' },
  });
  assert.ok(view);
  assert.equal(view.state, 'declined');
  const markup = renderToStaticMarkup(React.createElement(CraftExportConsentPanel, {
    locale: 'en', role: 'owner', view, onDecide: () => {},
  }));
  assert.match(markup, /Declined/, 'the rejection is visible');
  assert.match(markup, /Approve export/, 'the owner can decide again');
});

test('the projection never derives authority client-side — a stale decision is not consent', () => {
  const stale = projectExportConsentView({
    ...raw,
    state: 'consented',
    decision: { version_id: 'ver-1', manifest_digest: otherDigest, owner_id: 'u-owner', decision: 'approved' },
  });
  assert.ok(stale);
  assert.equal(stale.state, 'awaiting', 'a decision of another manifest digest downgrades to awaiting');
  assert.equal(stale.decision, null, 'the stale decision never projects');
  const inferred = projectExportConsentView({
    ...raw,
    state: 'consented',
    decision: null,
  });
  assert.ok(inferred);
  assert.equal(inferred.state, 'awaiting', 'a consented claim without a binding decision never projects');
});

test('the fail-closed guards: malformed payloads and disagreements refuse to project', () => {
  assert.equal(projectExportConsentView(null), null);
  assert.equal(projectExportConsentView({}), null);
  assert.equal(projectExportConsentView({ ...raw, manifest_digest: '' }), null);
  assert.equal(projectExportConsentView({ ...raw, files: 'nope' }), null);
  assert.equal(projectExportConsentView({ ...raw, state: 'bogus' }), null);
  // A server state that contradicts the recorded origins resolves closed:
  // restricted derived files with a "none" state await the owner.
  const disagreement = projectExportConsentView({ ...raw, state: 'none' });
  assert.ok(disagreement);
  assert.equal(disagreement.state, 'awaiting');
  // An honestly unrestricted manifest needs no consent.
  const clean = projectExportConsentView({
    version_id: 'ver-1', manifest_digest: digest, state: 'none', restricted_derived: [],
    files: [{ path: 'clean.txt', sha256: digest, restricted: false, origins: [ownOrigin] }], decision: null,
  });
  assert.ok(clean);
  assert.equal(clean.state, 'none');
});

// mountExport renders the panel into a real DOM so callback rejections can
// be observed as visible feedback (the T11 F5 discipline).
async function mountExport(overrides: {
  view: import('./export.tsx').CraftExportConsentView;
  onDecide?: (decision: 'approved' | 'rejected') => void | Promise<void>;
}) {
  const dom = new JSDOM('<!doctype html><html><body></body></html>');
  Object.assign(globalThis, {
    window: dom.window,
    document: dom.window.document,
    HTMLElement: dom.window.HTMLElement,
    Element: dom.window.Element,
    Node: dom.window.Node,
    Event: dom.window.Event,
    IS_REACT_ACT_ENVIRONMENT: true,
  });
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
  const { createRoot } = await import('../../../../apps/web/node_modules/react-dom/client.js');
  const { act } = await import('react');
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => root.render(React.createElement(CraftExportConsentPanel, {
    locale: 'en', role: 'owner', onDecide: async () => {}, ...overrides,
  })));
  return {
    container,
    async unmount() { await act(async () => root.unmount()); dom.window.close(); },
  };
}

test('a rejected decide callback surfaces visible feedback instead of an unhandled rejection', async () => {
  let rejectDecide!: (error: Error) => void;
  const view = projectExportConsentView(raw);
  assert.ok(view);
  const panel = await mountExport({
    view,
    onDecide: () => new Promise<void>((_resolve, reject) => { rejectDecide = reject; }),
  });
  try {
    const approve = panel.container.querySelector('button.wk-craft-export-approve') as HTMLButtonElement;
    assert.ok(approve, 'the approval control is rendered');
    const { act } = await import('react');
    await act(async () => { approve.click(); });
    rejectDecide(new Error('decision endpoint down'));
    await act(async () => {});
    assert.match(panel.container.textContent ?? '', /failed/i, 'the failure is visible');
  } finally {
    await panel.unmount();
  }
});
