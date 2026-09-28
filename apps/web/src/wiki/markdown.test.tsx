import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
hooks.registerHooks?.({ resolve: (specifier, context, nextResolve) => (specifier.endsWith('.css') || specifier.endsWith('.svg')) ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

// DOMPurify binds to the global window at import time — install jsdom globals
// before loading the renderer (mirrors the browser binding in vite builds).
// A real URL is required: opaque origins make jsdom's localStorage throw.
import { JSDOM } from 'jsdom';
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'http://localhost/' });
(globalThis as typeof globalThis & { window: unknown; document: unknown }).window = dom.window;
(globalThis as typeof globalThis & { window: unknown; document: unknown }).document = dom.window.document;

import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

// classic JSX 产物在无 React import 的模块（kb-list-icons.tsx 经 markdown.ts
// import 链加载）里按自由标识符落 globalThis 解析——挂全局（settings-error-ux
// 判例同款），否则 kb-list-icons.tsx:44 'React is not defined'。
(Object.assign as (target: unknown, patch: Record<string, unknown>) => unknown)(globalThis, { React });

const { renderWikiMarkdown, stripDuplicateLeadingTitle, wikiSlugDisplayName, handleWikiBodyClick, stripLegacyIndexDirectory, appendWikiIndexDirectoryLines, assembleWikiIndexMarkdown, parseWikiSourceRefs } = await import('./markdown.ts');
const { WikiPage, WikiImagePreview, WikiIndexView, WikiReaderFooter, wikiPreviewStep } = await import('./WikiPage.tsx');

const identity = (slug: string) => wikiSlugDisplayName(slug, []);

test('typical markdown renders the same block DOM as the Vue reader (marked, breaks on)', () => {
  const html = renderWikiMarkdown(
    [
      '# Title',
      '',
      'Para **bold** *em* `code`',
      '',
      '- item one',
      '- item two',
      '',
      '1. first',
      '2. second',
      '',
      '> quoted',
      '',
      '```js',
      'const x = 1;',
      '```',
      '',
      '| a | b |',
      '| --- | --- |',
      '| 1 | 2 |',
      '',
      'line one',
      'line two',
    ].join('\n'),
    { resolveSlugName: identity },
  );
  assert.match(html, /<h1>Title<\/h1>/);
  assert.match(html, /<p>Para <strong>bold<\/strong> <em>em<\/em> <code>code<\/code><\/p>/);
  assert.match(html, /<ul>\s*<li>item one<\/li>\s*<li>item two<\/li>\s*<\/ul>/);
  assert.match(html, /<ol>\s*<li>first<\/li>\s*<li>second<\/li>\s*<\/ol>/);
  assert.match(html, /<blockquote>\s*<p>quoted<\/p>\s*<\/blockquote>/);
  assert.match(html, /<pre><code class="language-js">const x = 1;\n<\/code><\/pre>/);
  assert.match(html, /<table>\s*<thead>/);
  assert.match(html, /<th>a<\/th>/);
  assert.match(html, /<td>1<\/td>/);
  // Vue parses with marked { breaks: true } — single newlines become <br>.
  assert.match(html, /line one<br>line two/);
  // No syntax highlighting in Vue's chain — code stays plain.
  assert.doesNotMatch(html, /class="[^"]*hljs/);
});

test('markdown links and images survive the sanitize allow-list like the Vue reader', () => {
  const html = renderWikiMarkdown('[site](https://example.com)\n\n![alt text](https://example.com/a.png)', { resolveSlugName: identity });
  // Vue's chain hardens http links with rel=noopener noreferrer (the
  // USE_PROFILES allow-list rejects `target`, so only rel survives — same as Vue).
  assert.match(html, /<a href="https:\/\/example\.com" rel="noopener noreferrer">site<\/a>/);
  assert.match(html, /<img src="https:\/\/example\.com\/a\.png" alt="alt text"/);
  // DOMPurify hook guarantees alt on images.
  assert.match(renderWikiMarkdown('![](https://example.com/b.png)', { resolveSlugName: identity }), /<img src="https:\/\/example\.com\/b\.png" alt=""/);
});

test('wiki-links render as wiki-content-link anchors with the same attributes as Vue', () => {
  const html = renderWikiMarkdown('See [[entity/beijing]] and [[entity/beijing|北京]] here.', { resolveSlugName: identity });
  // Plain slug: display falls back to slug minus the type prefix (Vue slugDisplayName).
  assert.match(html, /<a href="#" class="wiki-content-link" data-slug="entity\/beijing">beijing<\/a>/);
  // Piped slug: explicit label wins.
  assert.match(html, /<a href="#" class="wiki-content-link" data-slug="entity\/beijing">北京<\/a>/);
});

test('wikiSlugDisplayName prefers the loaded page title and falls back to the type-stripped slug', () => {
  const pages = [{ slug: 'entity/beijing', title: '北京' }] as Array<{ slug: string; title: string }>;
  assert.equal(wikiSlugDisplayName('entity/beijing', pages), '北京');
  assert.equal(wikiSlugDisplayName('entity/beijing', []), 'beijing');
  assert.equal(wikiSlugDisplayName('plain', []), 'plain');
});

test('stripDuplicateLeadingTitle drops a leading H1 that repeats the page title (Vue parity)', () => {
  assert.equal(stripDuplicateLeadingTitle('# 北京\n\nbody', '北京'), 'body');
  assert.equal(stripDuplicateLeadingTitle('# 其他\n\nbody', '北京'), '# 其他\n\nbody');
  assert.equal(stripDuplicateLeadingTitle('', '北京'), '');
});

test('XSS payloads are neutralized the same way as the Vue sanitize chain', () => {
  const html = renderWikiMarkdown(
    '<script>alert(1)</script>\n\n[x](javascript:alert(1))\n\n<img src="x" onerror="alert(1)">\n\n[["><img src=x onerror=alert(1)>]]',
    { resolveSlugName: identity },
  );
  // Assert on the parsed DOM: no script elements and no event-handler
  // attributes survive anywhere, and javascript: URIs never reach a href.
  const doc = new dom.window.DOMParser().parseFromString(html, 'text/html');
  assert.equal(doc.querySelectorAll('script').length, 0);
  let eventHandlerAttrs = 0;
  doc.querySelectorAll('*').forEach((el) => {
    for (const attr of Array.from(el.attributes)) {
      if (/^on/i.test(attr.name)) eventHandlerAttrs += 1;
    }
  });
  assert.equal(eventHandlerAttrs, 0);
  assert.doesNotMatch(html, /javascript:/i);
  // A raw <img src=x onerror=...> is neutralized (Vue keeps the img element
  // without the handler); the wiki-slug payload must not mint any new element.
  doc.querySelectorAll('img').forEach((img) => {
    assert.equal(img.getAttribute('onerror'), null);
    assert.match(img.getAttribute('src') ?? '', /^(?!data:text\/html)/);
  });
  doc.querySelectorAll('a.wiki-content-link').forEach((link) => {
    assert.equal(link.querySelector('img, script'), null, 'a hostile slug cannot inject elements into the anchor');
  });
});

test('handleWikiBodyClick navigates on wiki-content-link clicks and ignores other targets', () => {
  const navigated: string[] = [];
  const navigate = (slug: string) => navigated.push(slug);
  const link = dom.window.document.createElement('a');
  link.className = 'wiki-content-link';
  link.setAttribute('data-slug', 'entity/beijing');
  const child = dom.window.document.createElement('em');
  child.textContent = 'x';
  link.appendChild(child);
  dom.window.document.body.appendChild(link);

  const clickOn = (target: Element) => {
    let prevented = false;
    handleWikiBodyClick({ target, preventDefault: () => { prevented = true; } }, navigate);
    return prevented;
  };
  assert.equal(clickOn(link), true);
  assert.deepEqual(navigated, ['entity/beijing']);
  assert.equal(clickOn(child), true, 'clicks on inline children of the link still navigate');
  const para = dom.window.document.createElement('p');
  assert.equal(clickOn(para), false);
  assert.equal(navigated.length, 2);
});

test('the React reader replaces the raw pre block with the Vue wiki-reader-body markdown surface', () => {
  const source = WikiPage.toString();
  assert.match(source, /wiki-reader-body/);
  assert.match(source, /dangerouslySetInnerHTML/);
  assert.match(source, /handleWikiBodyClick/);
  assert.match(source, /stripDuplicateLeadingTitle\(/);
  assert.match(source, /wikiSlugDisplayName\(/);
  assert.match(source, /navigateToSlug/);
  // The plain-text pre reader is gone.
  assert.doesNotMatch(source, /<pre className="wk-wiki-reader-content/);
});

// ─── R442 item 1: image preview modal (Vue picture-preview.vue) ───

test('handleWikiBodyClick opens the image preview for img targets like the Vue handleContentClick', () => {
  const navigated: string[] = [];
  const opened: string[] = [];
  const img = dom.window.document.createElement('img');
  img.setAttribute('src', 'http://localhost/a.png');
  dom.window.document.body.appendChild(img);

  let prevented = false;
  handleWikiBodyClick(
    { target: img, preventDefault: () => { prevented = true; } },
    { navigate: (slug: string) => navigated.push(slug), openImage: (src: string) => opened.push(src) },
  );
  assert.equal(prevented, true);
  assert.deepEqual(opened, ['http://localhost/a.png']);
  assert.equal(navigated.length, 0, 'an img click does not navigate');

  // Vue: `if (imagePreviewUrl.value)` — an empty src never opens the viewer.
  const empty = dom.window.document.createElement('img');
  let preventedEmpty = false;
  handleWikiBodyClick(
    { target: empty, preventDefault: () => { preventedEmpty = true; } },
    { navigate: (slug: string) => navigated.push(slug), openImage: (src: string) => opened.push(src) },
  );
  assert.equal(preventedEmpty, true, 'the click is still swallowed like the Vue handler');
  assert.equal(opened.length, 1);

  // Legacy function-form callers (navigate only) keep working without a viewer.
  const link = dom.window.document.createElement('a');
  link.className = 'wiki-content-link';
  link.setAttribute('data-slug', 'entity/x');
  handleWikiBodyClick({ target: link, preventDefault: () => {} }, (slug: string) => navigated.push(slug));
  assert.deepEqual(navigated, ['entity/x']);
  assert.equal(opened.length, 1);
});

test('WikiImagePreview mirrors the t-image-viewer structure: mask, dialog image, zoom controls, close', () => {
  const html = renderToStaticMarkup(
    React.createElement(WikiImagePreview, { src: 'http://localhost/a.png', onClose: () => {} }),
  );
  assert.match(html, /role="dialog"/);
  assert.match(html, /aria-modal="true"/);
  assert.match(html, /wk-wiki-img-preview-mask/);
  assert.match(html, /src="http:\/\/localhost\/a\.png"/);
  assert.match(html, /wk-wiki-img-preview-close/);
  assert.match(html, /wk-wiki-img-preview-zoom-in/);
  assert.match(html, /wk-wiki-img-preview-zoom-out/);
});

test('wikiPreviewStep zooms inside the viewer clamp bounds', () => {
  assert.equal(wikiPreviewStep(1, 0.25), 1.25);
  assert.equal(wikiPreviewStep(4.9, 0.25), 5, 'clamped at the upper bound');
  assert.equal(wikiPreviewStep(1, -0.25), 0.75);
  assert.equal(wikiPreviewStep(0.3, -0.25), 0.2, 'clamped at the lower bound');
});

// ─── R442 item 2: index view reuses the wiki markdown pipeline ───

test('stripLegacyIndexDirectory clips the inline directory like the Vue loader', () => {
  assert.equal(stripLegacyIndexDirectory('# Intro\n\nHello'), '# Intro\n\nHello');
  assert.equal(stripLegacyIndexDirectory('# Intro\n\nHello\n\n## Summary (2)\n[[a|A]]'), '# Intro\n\nHello');
  assert.equal(stripLegacyIndexDirectory(''), '');
});

test('appendWikiIndexDirectoryLines emits Vue path bolding and wiki-link entries', () => {
  const markdown = appendWikiIndexDirectoryLines([
    { slug: 'summary/a', title: 'A', summary: 'first', category_path: ['Guide', 'Deep'] },
    { slug: 'summary/b', title: 'B', summary: '', category_path: ['Guide'] },
    { slug: 'summary/c', title: 'C', summary: '' },
  ]);
  const lines = markdown.split('\n');
  // Entry indent = '  '.repeat(path.length) — a 2-level path nests the entry
  // under both breadcrumb levels (Vue appendIndexDirectoryLines).
  assert.deepEqual(lines.slice(0, 5), ['**Guide**', '  **Deep**', '    [[summary/a|A]] — first', '  [[summary/b|B]]', '[[summary/c|C]]']);
});

test('assembleWikiIndexMarkdown reuses the reader markdown source: ordered sections, empty skipped', () => {
  const labelFor = (type: string) => ({ summary: '摘要', entity: '实体' })[type] ?? type;
  const markdown = assembleWikiIndexMarkdown({
    intro: '# 总览\n\nHi\n\n## Summary (9)\n[[legacy|x]]',
    labelFor,
    groups: [
      { type: 'entity', total: 1, items: [{ slug: 'entity/e1', title: 'E1', summary: '' }] },
      { type: 'summary', total: 2, items: [{ slug: 'summary/a', title: 'A', summary: 'first' }] },
      { type: 'concept', total: 0, items: [] },
    ],
  });
  assert.match(markdown, /^# 总览\n\nHi/);
  assert.doesNotMatch(markdown, /legacy/);
  const summaryIdx = markdown.indexOf('## 摘要 (2)');
  const entityIdx = markdown.indexOf('## 实体 (1)');
  assert.ok(summaryIdx >= 0 && entityIdx > summaryIdx, 'sections follow the Vue Summary→Entity order');
  assert.doesNotMatch(markdown, /## 概念/, 'empty sections are skipped entirely');
  assert.match(markdown, /\[\[summary\/a\|A\]\] — first/);
});

test('WikiIndexView renders the markdown body through the shared reader pipeline', () => {
  const html = renderToStaticMarkup(
    React.createElement(WikiIndexView, {
      indexView: {
        intro: '# 总览',
        version: 1,
        groups: [{ type: 'summary', total: 1, items: [{ slug: 'summary/a', title: 'A', summary: 'first' }] }],
      },
      loading: false,
      error: null,
      hasMore: false,
      labelFor: () => '摘要',
      onLoadMore: () => {},
      onNavigate: () => {},
      onOpenImage: () => {},
      locale: 'zh-CN',
    }),
  );
  assert.match(html, /wiki-index-body/);
  assert.match(html, /wiki-content-link/);
  assert.match(html, /data-slug="summary\/a"/);
  assert.match(html, /<h2[^>]*>摘要 \(1\)<\/h2>/, 'the directory heading survives the shared marked+sanitize chain');
});

// ─── R442 item 3: reader footer backlinks + sources ───

test('parseWikiSourceRefs follows the Vue parseSourceRefEntry fallbacks', () => {
  assert.deepEqual(parseWikiSourceRefs(['doc-1|报告.pdf']), [{ id: 'doc-1', title: '报告.pdf' }]);
  assert.deepEqual(parseWikiSourceRefs(['012345678901234567890']), [{ id: '012345678901234567890', title: '01234567...' }]);
  assert.deepEqual(parseWikiSourceRefs(['short-id']), [{ id: 'short-id', title: 'short-id' }]);
});

test('WikiReaderFooter renders the Vue linkedFrom and sources rows', () => {
  const html = renderToStaticMarkup(
    React.createElement(WikiReaderFooter, {
      page: { in_links: ['summary/a', 'entity/b'], source_refs: ['doc-1|报告.pdf'] },
      resolveSlugName: (slug: string) => slug.toUpperCase(),
      onNavigate: () => {},
      onOpenSourceDoc: () => {},
    }),
  );
  assert.match(html, /wiki-reader-footer/);
  assert.match(html, /wiki-reader-footer-row/);
  assert.match(html, /data-slug="summary\/a"/);
  assert.match(html, /SUMMARY\/A/);
  assert.match(html, /data-source-id="doc-1"/);
  assert.match(html, /报告\.pdf/);
});

test('WikiReaderFooter renders nothing when the page has no in_links or source_refs', () => {
  const html = renderToStaticMarkup(
    React.createElement(WikiReaderFooter, {
      page: {},
      resolveSlugName: (slug: string) => slug,
      onNavigate: () => {},
      onOpenSourceDoc: () => {},
    }),
  );
  assert.equal(html, '');
});

test('the Wiki page wires the preview, footer, and markdown index like the Vue browser', () => {
  // WikiPage.toString only carries its own body — the wiring assertions span
  // the page plus the extracted subcomponents.
  const source = [WikiPage, WikiImagePreview, WikiIndexView, WikiReaderFooter].map(String).join('\n');
  assert.match(source, /WikiImagePreview/);
  assert.match(source, /openImage/);
  assert.match(source, /wiki-reader-footer/);
  assert.match(source, /onOpenSourceDoc/);
  assert.match(source, /assembleWikiIndexMarkdown/);
  assert.match(source, /wiki-index-body/);
});
