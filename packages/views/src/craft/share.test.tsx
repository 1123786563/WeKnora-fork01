// T11 (#128) front-end domain rule: sharing a restricted-source result is a
// server authority the panel only PROJECTS. The owner sees the exact
// immutable Version ID and evidence digest they are consenting to and the
// consent controls; everyone else sees the state without controls. The
// projection never derives authority client-side: a decision that does not
// bind the current version and evidence digest is stale and never renders
// as consented, and no original-source material ever enters the share panel.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import { JSDOM } from 'jsdom';
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (h: {resolve: (specifier: string, context: unknown, nextResolve: (s: string, c: unknown) => unknown) => unknown}) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit:true, url:'data:text/javascript,export default {}' } : nextResolve(specifier,context) });
const React = await import('react');
const { act } = await import('react');
const { renderToStaticMarkup } = await import('../../../../apps/web/node_modules/react-dom/server.js');
const { CraftSharePanel, projectShareView } = await import('./share.tsx');

const raw = {
  version_id: 'v-1',
  restricted: true,
  evidence_digest: '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
  status: 'private',
  decision: null,
};

test('the owner sees the exact version and evidence digest with the consent controls', () => {
  const view = projectShareView(raw);
  assert.ok(view);
  const markup = renderToStaticMarkup(React.createElement(CraftSharePanel, {
    locale: 'en', role: 'owner', view,
    onDecide: () => {}, onRevoke: () => {},
  }));
  assert.match(markup, /v-1/, 'the exact immutable version is shown before any consent');
  assert.match(markup, /0123456789abcdef…/, 'the evidence digest is shown before any consent');
  assert.match(markup, /Confirm share/, 'the explicit approval affordance');
  assert.match(markup, /Decline/, 'the explicit rejection affordance');
  assert.match(markup, /restricted/i, 'the summary says why consent is required');
  assert.doesNotMatch(markup, /craftkb:\/\//, 'no original-source ref is ever rendered');
});

test('a non-owner projects the same state without any consent control', () => {
  const view = projectShareView(raw);
  assert.ok(view);
  const markup = renderToStaticMarkup(React.createElement(CraftSharePanel, {
    locale: 'en', role: 'viewer', view,
    onDecide: () => {}, onRevoke: () => {},
  }));
  assert.match(markup, /v-1/, 'the viewer still sees the summary facts');
  assert.match(markup, /Private/, 'the viewer sees the current state');
  assert.doesNotMatch(markup, /Confirm share/, 'a viewer never holds consent controls');
  assert.doesNotMatch(markup, /Decline/, 'a viewer never holds rejection controls');
  assert.doesNotMatch(markup, /<button/, 'no controls at all for a non-owner');
});

test('a live consented state shows the binding and its expiry, with only the revoke control', () => {
  const view = projectShareView({
    ...raw,
    status: 'consented',
    decision: { version_id: 'v-1', evidence_digest: raw.evidence_digest, owner_id: 'u-owner', decision: 'approved' },
    expires_at: '2026-09-27T09:00:00Z',
  });
  assert.ok(view);
  const markup = renderToStaticMarkup(React.createElement(CraftSharePanel, {
    locale: 'en', role: 'owner', view,
    onDecide: () => {}, onRevoke: () => {},
  }));
  assert.match(markup, /Consented/, 'the live sharing authority is projected');
  assert.match(markup, /u-owner/, 'the decision names the consenting owner');
  assert.match(markup, /Revoke/, 'the owner can end the consent');
  assert.doesNotMatch(markup, /Confirm share/, 'no duplicate approval while a consent is live');
});

test('the projection never derives authority client-side — a stale decision is not consent', () => {
  const stale = projectShareView({
    ...raw,
    status: 'consented',
    decision: { version_id: 'v-1', evidence_digest: 'f'.repeat(64), owner_id: 'u-owner', decision: 'approved' },
  });
  assert.ok(stale);
  assert.equal(stale.status, 'private', 'a decision bound to other evidence never projects as authority');
  assert.equal(stale.decision, null, 'the stale decision is dropped entirely');

  const otherVersion = projectShareView({
    ...raw,
    status: 'consented',
    decision: { version_id: 'v-2', evidence_digest: raw.evidence_digest, owner_id: 'u-owner', decision: 'approved' },
  });
  assert.ok(otherVersion);
  assert.equal(otherVersion.status, 'private', 'another version never inherits a consent');
});

test('the projection drops malformed payloads instead of guessing', () => {
  assert.equal(projectShareView(null), null);
  assert.equal(projectShareView({}), null);
  assert.equal(projectShareView({ ...raw, status: 'public' }), null, 'an unknown status is rejected, not defaulted');
  assert.equal(projectShareView({ ...raw, version_id: '' }), null);
  assert.equal(projectShareView({ ...raw, evidence_digest: '' }), null);
  // A restricted flag must be a boolean; the consent requirement is never guessed.
  assert.equal(projectShareView({ ...raw, restricted: 'yes' }), null);
});

test('an unrestricted version is never downgraded by the decision-less consent guard', () => {
  // F7: the downgrade to private applies to restricted versions only — an
  // unrestricted version is consented by construction and carries no
  // decision, so downgrading it would mislabel a shareable version private.
  const unrestricted = projectShareView({ ...raw, restricted: false, status: 'consented', decision: null });
  assert.ok(unrestricted);
  assert.equal(unrestricted.status, 'consented', 'unrestricted consent needs no bound decision');
  // The restricted counterpart still requires a bound decision.
  const restricted = projectShareView({ ...raw, status: 'consented', decision: null });
  assert.ok(restricted);
  assert.equal(restricted.status, 'private', 'a restricted consent is still only projected from a bound decision');
});

test('a declined restricted state still offers the owner both decision controls', () => {
  // F8: a rejected decision is not final — the server upserts one row per
  // task+version, so the owner can decide again at any time.
  const view = projectShareView({
    ...raw,
    status: 'declined',
    decision: { version_id: 'v-1', evidence_digest: raw.evidence_digest, owner_id: 'u-owner', decision: 'rejected' },
  });
  assert.ok(view);
  const markup = renderToStaticMarkup(React.createElement(CraftSharePanel, {
    locale: 'en', role: 'owner', view, onDecide: () => {}, onRevoke: () => {},
  }));
  assert.match(markup, /<button[^>]*wk-craft-share-confirm[^>]*>Confirm share<\/button>/, 'the owner can re-consent after declining');
  assert.match(markup, /<button[^>]*wk-craft-share-decline[^>]*>Decline<\/button>/, 'the owner can re-decline');
  assert.doesNotMatch(markup, /Revoke consent/, 'a declined state holds no live consent to revoke');
});

// mountShare renders the panel into a real DOM so callback rejections can
// be observed as visible feedback (F5).
async function mountShare(overrides: {
  view: import('./share.tsx').CraftShareView;
  onDecide?: (decision: 'approved' | 'rejected') => void | Promise<void>;
  onRevoke?: () => void | Promise<void>;
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
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => root.render(React.createElement(CraftSharePanel, {
    locale: 'en', role: 'owner',
    onDecide: async () => {}, onRevoke: async () => {}, ...overrides,
  })));
  return {
    container,
    async unmount() { await act(async () => root.unmount()); dom.window.close(); },
  };
}

test('a rejected decide callback surfaces visible feedback instead of an unhandled rejection', async () => {
  let rejectDecide!: (error: Error) => void;
  const view = projectShareView(raw);
  assert.ok(view);
  const panel = await mountShare({
    view,
    onDecide: () => new Promise<void>((_resolve, reject) => { rejectDecide = reject; }),
  });
  try {
    const confirm = panel.container.querySelector('button.wk-craft-share-confirm') as HTMLButtonElement;
    assert.ok(confirm, 'the confirm control is rendered');
    await act(async () => confirm.dispatchEvent(new Event('click', { bubbles: true })));
    assert.equal(confirm.disabled, true, 'the control is inert while the decision is in flight');
    assert.equal(panel.container.querySelector('[role="alert"]'), null, 'no error before the rejection settles');
    await act(async () => rejectDecide(new Error('network down')));
    assert.match(panel.container.querySelector('[role="alert"]')?.textContent ?? '', /failed/i, 'F5: the rejection becomes visible feedback');
    assert.equal(confirm.disabled, false, 'the owner can retry after the failure');
  } finally { await panel.unmount(); }
});

test('a rejected revoke callback surfaces visible feedback instead of an unhandled rejection', async () => {
  let rejectRevoke!: (error: Error) => void;
  const view = projectShareView({
    ...raw,
    status: 'consented',
    decision: { version_id: 'v-1', evidence_digest: raw.evidence_digest, owner_id: 'u-owner', decision: 'approved' },
    expires_at: '2026-09-27T09:00:00Z',
  });
  assert.ok(view);
  const panel = await mountShare({
    view,
    onRevoke: () => new Promise<void>((_resolve, reject) => { rejectRevoke = reject; }),
  });
  try {
    const revoke = panel.container.querySelector('button.wk-craft-share-revoke') as HTMLButtonElement;
    assert.ok(revoke, 'the revoke control is rendered');
    await act(async () => revoke.dispatchEvent(new Event('click', { bubbles: true })));
    await act(async () => rejectRevoke(new Error('network down')));
    assert.match(panel.container.querySelector('[role="alert"]')?.textContent ?? '', /failed/i, 'F5: the revocation rejection becomes visible feedback');
    assert.equal(revoke.disabled, false, 'the owner can retry the revocation');
  } finally { await panel.unmount(); }
});
