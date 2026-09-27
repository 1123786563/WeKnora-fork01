// T10 (#127) front-end domain rule: a source citation NEVER carries an
// openable URL. The panel's only resolution path is the assembly's
// onOpenSource(citationId, durableRef), which re-authorizes through the
// viewer's own permission chain on every click; a citation whose access was
// revoked keeps the citable placeholder (id stays, digest/bytes leave, the
// open button disables) so denial never erases evidence that a source existed.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (h: {resolve: (specifier: string, context: unknown, nextResolve: (s: string, c: unknown) => unknown) => unknown}) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit:true, url:'data:text/javascript,export default {}' } : nextResolve(specifier,context) });
const React = await import('react');
const { renderToStaticMarkup } = await import('../../../../apps/web/node_modules/react-dom/server.js');
const { CraftSources } = await import('./sources.tsx');
const { projectKnowledgeSources } = await import('./presentation.ts');

const source = {
  citationId: 'kc_0123456789abcdef01234567',
  ref: 'craftkb://kb/kb-a/knowledge/k-a/chunk/c-a',
  title: 'Region Sales',
  digest: '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
  excerptBytes: 4096,
  tenantId: 1,
  acquiredAt: '2026-09-25T08:00:00Z',
};

test('a citation row never renders a provider or storage URL — the durable ref is the panel’s only currency', () => {
  const markup = renderToStaticMarkup(React.createElement(CraftSources, {
    locale: 'en', sources: [source], truncated: false, openingCitation: null, onOpenSource: () => {},
  }));
  assert.doesNotMatch(markup, /https?:\/\//, 'no URL is ever rendered for a citation');
  assert.doesNotMatch(markup, /href=/, 'a citation is never a link the browser could open directly');
  assert.match(markup, /kc_0123456789abcdef01234567/, 'the citation placeholder stays visible');
  assert.match(markup, /0123456789ab…/, 'the integrity digest stays visible');
  assert.match(markup, /Open source/, 'the authorized open affordance is the button, not a URL');
});

test('a revoked citation keeps the citable placeholder while hiding the fact bytes', () => {
  const markup = renderToStaticMarkup(React.createElement(CraftSources, {
    locale: 'en', sources: [source], truncated: false, openingCitation: null,
    onOpenSource: () => {}, revokedCitationIds: [source.citationId],
  }));
  assert.match(markup, /kc_0123456789abcdef01234567/, 'denial never erases the citation id');
  assert.doesNotMatch(markup, /0123456789ab…/, 'the digest is hidden once access is revoked');
  assert.doesNotMatch(markup, /2026-09-25T08:00:00Z/, 'the observation time is hidden once access is revoked');
  assert.match(markup, /disabled=""/, 'the open button is disabled, not removed');
  assert.doesNotMatch(markup, /https?:\/\//, 'still no URL, especially not on denial');
});

test('the data plane projects only durable refs — the projection never fabricates or rewrites a citation', () => {
  const projection = projectKnowledgeSources([
    { kind: 'knowledge.built', data: { sources: [
      { citation_id: source.citationId, ref: source.ref, digest: source.digest, excerpt_bytes: 4096, tenant_id: 1 },
      { citation_id: '', ref: source.ref },
      { citation_id: 'kc_ffffffffffffffffffffffff', ref: '' },
    ], truncated: false } },
  ]);
  assert.equal(projection.sources.length, 1, 'rows without a citation id or a durable ref are dropped');
  assert.equal(projection.sources[0]?.citationId, source.citationId);
  assert.equal(projection.sources[0]?.ref, source.ref, 'the durable ref travels verbatim to the opener');
  assert.equal(projection.truncated, false);
});
