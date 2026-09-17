import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

(globalThis as typeof globalThis & { React: typeof React }).React = React;

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

// ─── R443 A4: wiki footer source-title hydration ───
//
// Vue WikiBrowser.vue hydrateSourceRefTitles: source_refs store bare
// knowledge ids (wiki_ingest_batch) and the browser resolves each one
// through GET /api/v1/knowledge/{id} (Vue getKnowledgeDetails), reading
// `title || file_name || fileName`, caching per id, keeping the
// truncated-id fallback on failure, and guarding stale loops with a
// request sequence counter.

const { createSourceRefTitleHydrator } = await import('./source-titles.ts');
const { WikiReaderFooter, WikiPage, WikiImagePreview, WikiIndexView } = await import('./WikiPage.tsx');
const { parseWikiSourceRefs } = await import('./markdown.ts');

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

test('hydrator resolves bare refs through the injected document fetcher with dedup and cache', async () => {
  const fetched: string[] = [];
  const hydrator = createSourceRefTitleHydrator(async (id) => {
    fetched.push(id);
    return id === 'doc-2' ? { file_name: '报告.pdf' } : { title: '会议纪要' };
  });
  const titles = await hydrator.hydrate(['doc-1', 'doc-1', 'doc-2|管道标题', 'doc-2']);
  assert.deepEqual(fetched.sort(), ['doc-1', 'doc-2'], 'pipe refs are skipped and duplicates fetched once');
  assert.equal(titles['doc-1'], '会议纪要');
  assert.equal(titles['doc-2'], '报告.pdf', 'file_name is the Vue fallback field');
  assert.equal(hydrator.titleFor('doc-1'), '会议纪要');

  // Vue caches per id: a second pass with the same refs must not re-fetch.
  const again = await hydrator.hydrate(['doc-1', 'doc-2']);
  assert.deepEqual(fetched.sort(), ['doc-1', 'doc-2'], 'cached ids are not re-fetched');
  assert.equal(again['doc-1'], '会议纪要');
});

test('hydrator keeps the truncated-id fallback when the document fetch fails', async () => {
  const hydrator = createSourceRefTitleHydrator(async (id) => {
    if (id === 'gone') throw new Error('404');
    return { title: 'ok' };
  });
  const titles = await hydrator.hydrate(['gone', 'alive']);
  assert.equal(titles['gone'], undefined, 'failed lookups stay uncached like Vue');
  assert.equal(titles['alive'], 'ok');
  assert.equal(hydrator.titleFor('gone'), null);
});

test('hydrator aborts a superseded pass like the Vue request-sequence guard', async () => {
  const fetched: string[] = [];
  const gate = deferred<{ title: string }>();
  const hydrator = createSourceRefTitleHydrator(async (id) => {
    fetched.push(id);
    if (id === 'a') return gate.promise;
    return { title: `title-${id}` };
  });
  const stale = hydrator.hydrate(['a', 'b']);
  const fresh = hydrator.hydrate(['c']);
  gate.resolve({ title: 'late-title-a' });
  await Promise.all([stale, fresh]);
  assert.ok(!fetched.includes('b'), 'the superseded loop stops issuing further fetches');
  assert.equal(hydrator.titleFor('c'), 'title-c', 'the newest pass still applies');
  assert.equal(hydrator.titleFor('a'), null, 'stale results do not win the cache');
});

test('WikiReaderFooter renders hydrated titles over the truncated-id fallback', () => {
  const html = renderToStaticMarkup(
    React.createElement(WikiReaderFooter, {
      page: { source_refs: ['012345678901234567890', '0123456789ffffffffffz', 'doc-1|管道.pdf'] },      resolveSlugName: (slug: string) => slug,
      onNavigate: () => {},
      onOpenSourceDoc: () => {},
      sourceTitles: { '012345678901234567890': '报告.pdf' },
    }),
  );
  assert.match(html, /报告\.pdf/, 'the hydrated title replaces the truncated fallback');
  assert.match(html, /01234567\.\.\./, 'ids without a hydrated title keep the Vue fallback');
  assert.match(html, /管道\.pdf/, 'pipe-carried titles are untouched');
  assert.match(html, /data-source-id="012345678901234567890"/);
});

test('onOpenSourceDoc stays optional like a Vue emit without a listener', () => {
  const html = renderToStaticMarkup(
    React.createElement(WikiReaderFooter, {
      page: { source_refs: ['doc-1'] },
      resolveSlugName: (slug: string) => slug,
      onNavigate: () => {},
      sourceTitles: { 'doc-1': '报告.pdf' },
    }),
  );
  assert.match(html, /href="#"/);
  const source = [WikiReaderFooter].map(String).join('\n');
  assert.match(source, /onOpenSourceDoc\?\.\(/, 'the click stays a no-op without a host handler');
});

test('the Wiki page wires footer hydration through the shared documents detail endpoint', () => {
  // WikiPage.toString only carries its own body — wiring spans the page
  // plus the extracted subcomponents (same trick as the R442 wiring test).
  const source = [WikiPage, WikiImagePreview, WikiIndexView, WikiReaderFooter].map(String).join('\n');
  assert.match(source, /createSourceRefTitleHydrator/);
  assert.match(source, /knowledge\.documents\.get\(/, 'reuses the existing GET /api/v1/knowledge/{id} client capability');
  // Compiled JSX keeps the prop as shorthand (`sourceTitles`), so anchor the
  // assertion at the footer call site inside WikiPage.
  assert.match(source, /WikiReaderFooter[\s\S]{0,400}?sourceTitles/, 'hydrated titles flow into the reader footer');
  assert.match(source, /hydrate\(/);
  assert.ok(parseWikiSourceRefs(['doc-1|报告.pdf']).length > 0);
});
