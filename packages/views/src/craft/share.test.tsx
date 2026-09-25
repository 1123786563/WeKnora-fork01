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
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (h: {resolve: (specifier: string, context: unknown, nextResolve: (s: string, c: unknown) => unknown) => unknown}) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit:true, url:'data:text/javascript,export default {}' } : nextResolve(specifier,context) });
const React = await import('react');
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
