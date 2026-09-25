// T06 (#125): the Workbench sources panel renders the delivered web
// artifact's citation manifest — facts link to recorded sources, model
// inference carries an explicit distinct marker and is never presented as a
// source fact, and missing/revoked sources keep a non-leaking placeholder.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (h: {resolve: (specifier: string, context: unknown, nextResolve: (s: string, c: unknown) => unknown) => unknown}) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier: string, context: unknown, nextResolve) => specifier.endsWith('.css') ? { shortCircuit:true, url:'data:text/javascript,export default {}' } : nextResolve(specifier,context) });
const React = await import('react');
const { renderToStaticMarkup } = await import('../../../../apps/web/node_modules/react-dom/server.js');
const { CraftSources } = await import('./sources.tsx');

const source = {
  citationId: 'kc_0123456789abcdef01234567',
  ref: 'craftkb://kb/a/knowledge/k/chunk/c',
  title: 'Regional sales report',
  digest: 'digestsecret0123456789',
  excerptBytes: 32,
  tenantId: 1,
  acquiredAt: '2026-09-25T01:02:03Z',
};

function render(extra: Record<string, unknown> = {}): string {
  return renderToStaticMarkup(React.createElement(CraftSources, {
    locale: 'zh',
    sources: [source],
    truncated: false,
    openingCitation: null,
    onOpenSource: () => {},
    ...extra,
  }));
}

test('citation facts render bound to their recorded source row and agree with the evidence table', () => {
  const html = render({
    citations: [
      { kind: 'fact', citationId: source.citationId, claim: '华东区销售额来自来源' },
      { kind: 'inference', claim: '预计 Q4 延续增长' },
    ],
  });
  // The fact entry is visible with its claim and its stable citation marker.
  assert.match(html, /data-craft-citation="kc_0123456789abcdef01234567"/);
  assert.match(html, /华东区销售额来自来源/);
  assert.match(html, /\[事实来源\]/);
  // The recorded source row is marked as cited by the delivered artifact.
  assert.match(html, /data-cited="true"/);
  // The open control still routes through the durable ref seam.
  assert.match(html, /查看来源/);
});

test('model inference carries an explicit distinct marker and is never presented as a source fact', () => {
  const html = render({
    citations: [
      { kind: 'inference', claim: '预计 Q4 延续增长' },
    ],
  });
  assert.match(html, /data-craft-inference="true"/);
  assert.match(html, /\[模型推断\]/);
  assert.match(html, /预计 Q4 延续增长/);
  // No source row is cited and no citation marker links inference to a source.
  assert.doesNotMatch(html, /data-cited="true"/);
  assert.doesNotMatch(html, /data-craft-citation=/);
  // The sources table itself is untouched: inference is not a source fact.
  assert.doesNotMatch(html, /data-craft-inference[^>]*data-craft-citation/);
});

test('a missing or revoked cited source keeps a non-leaking placeholder', () => {
  const html = render({
    revokedCitationIds: [source.citationId],
    citations: [
      { kind: 'fact', citationId: source.citationId, claim: '华东区销售额来自来源' },
    ],
  });
  assert.match(html, /来源不可用/);
  assert.match(html, /data-craft-citation-unavailable="kc_0123456789abcdef01234567"/);
  // The placeholder leaks neither the digest nor the acquisition time.
  assert.doesNotMatch(html, /digestsecret/);
  assert.doesNotMatch(html, /2026-09-25T01:02:03Z/);
});

test('a cited fact with no matching recorded row renders the placeholder, not an invented source', () => {
  const html = render({
    citations: [
      { kind: 'fact', citationId: 'kc_ffffffffffffffffffffffff', claim: '孤儿引用' },
    ],
  });
  assert.match(html, /来源不可用/);
  assert.match(html, /data-craft-citation-unavailable="kc_ffffffffffffffffffffffff"/);
  assert.doesNotMatch(html, /data-craft-citation-open=/, 'an orphan citation must not render an open control');
});
