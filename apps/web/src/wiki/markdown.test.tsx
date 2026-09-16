import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
hooks.registerHooks?.({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

// DOMPurify binds to the global window at import time — install jsdom globals
// before loading the renderer (mirrors the browser binding in vite builds).
// A real URL is required: opaque origins make jsdom's localStorage throw.
import { JSDOM } from 'jsdom';
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'http://localhost/' });
(globalThis as typeof globalThis & { window: unknown; document: unknown }).window = dom.window;
(globalThis as typeof globalThis & { window: unknown; document: unknown }).document = dom.window.document;

const { renderWikiMarkdown, stripDuplicateLeadingTitle, wikiSlugDisplayName, handleWikiBodyClick } = await import('./markdown.ts');
const { WikiPage } = await import('./WikiPage.tsx');

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
