import assert from 'node:assert/strict';
import test from 'node:test';

// R444/A2 — embed chat answer markdown rendering. Vue contract (EmbedBotMessage.vue
// renderedHTML -> frontend/src/utils/chatMarkdownRenderer.ts renderChatMarkdown):
//   1. stripIncompleteCitationTag hides a trailing partial <kb / <web tag
//   2. citation tags are converted to pill HTML and shielded from marked as
//      @@WEKNORA_HTML_PLACEHOLDER_n@@ tokens (extractCitationHtmlPlaceholders)
//   3. marked parses the remaining text with { breaks: true, gfm: true }
//   4. pill HTML is restored after parse and citation-only paragraphs are
//      collapsed into the preceding one (collapseStandaloneCitationParagraphs)
//   5. the final HTML goes through sanitizeMarkdownHTML (markdownDomPurifyConfig
//      + markdownDomPurifySecurityHooks), so raw model HTML is stripped but the
//      citation pill attributes (data-chunk-id/data-doc/data-url) survive.
// DOMPurify binds to the global window at import time — install jsdom globals
// before loading the renderer (mirrors the browser binding in vite builds).
import { JSDOM } from 'jsdom';
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'http://localhost/' });
(globalThis as typeof globalThis & { window: unknown; document: unknown }).window = dom.window;
(globalThis as typeof globalThis & { window: unknown; document: unknown }).document = dom.window.document;

const { renderEmbedChatMarkdown } = await import('./markdown.ts');

test('markdown basic blocks render like the Vue embed chain (breaks + gfm on)', () => {
  const html = renderEmbedChatMarkdown(
    [
      '# 标题',
      '',
      'Para **bold** *em* `code`',
      '',
      '- item one',
      '- item two',
      '',
      '| a | b |',
      '| --- | --- |',
      '| 1 | 2 |',
      '',
      'line one',
      'line two',
    ].join('\n'),
  );
  assert.match(html, /<h1[^>]*>标题<\/h1>/);
  assert.match(html, /<p>Para <strong>bold<\/strong> <em>em<\/em> <code>code<\/code><\/p>/);
  assert.match(html, /<ul>\s*<li>item one<\/li>\s*<li>item two<\/li>\s*<\/ul>/);
  assert.match(html, /<table>/);
  assert.match(html, /<td>1<\/td>/);
  // Vue configures marked with breaks: true — single newlines become <br>.
  assert.match(html, /line one<br>line two/);
});

test('citation pills ride inside the markdown pipeline like Vue (order: placeholder -> parse -> restore)', () => {
  const refs = [{ id: 'chunk-uuid-1', chunk_type: 'document', knowledge_title: '产品手册' }];
  const html = renderEmbedChatMarkdown('**结论**如下 <kb doc="产品手册" chunk_id="DOC-1"/>，完毕。', refs);
  // Markdown around the tag still parses…
  assert.match(html, /<strong>结论<\/strong>如下/);
  // …and the pill comes out as Vue's citation HTML with the resolved chunk id.
  assert.match(
    html,
    /<span class="citation citation-kb" data-kb-id="" data-chunk-id="chunk-uuid-1" data-doc="产品手册" role="button" tabindex="0"><span class="citation-icon citation-icon--book" aria-hidden="true"><\/span><span class="citation-text">产品手册<\/span>/,
  );
  // The pill stays inline in the same paragraph (joinCitationTagsToPreviousLine).
  assert.match(html, /如下 <span class="citation citation-kb"/);
});

test('web citation tags become external link pills with the domain label', () => {
  const html = renderEmbedChatMarkdown('来源 <web url="https://docs.example.com/path" title="Docs"/>', []);
  // The pill HTML carries target=_blank, but USE_PROFILES rejects `target`
  // during sanitization (same as Vue — only rel survives; wiki markdown.test.tsx
  // documents the identical finding).
  assert.match(
    html,
    /<a class="citation citation-web" data-url="https:\/\/docs\.example\.com\/path" href="https:\/\/docs\.example\.com\/path" rel="noopener noreferrer"><span class="citation-icon citation-icon--web" aria-hidden="true"><\/span><span class="citation-domain">example\.com<\/span>/,
  );
  assert.doesNotMatch(html, /target=/);
});

test('a citation-only paragraph merges into the previous paragraph like Vue', () => {
  const refs = [{ id: 'chunk-uuid-2', chunk_type: 'document', knowledge_title: '手册' }];
  const html = renderEmbedChatMarkdown('第一段。\n\n<kb doc="手册" chunk_id="chunk-uuid-2"/>', refs);
  assert.match(html, /<p>第一段。 <span class="citation citation-kb"/);
  assert.doesNotMatch(html, /<p><span class="citation citation-kb"/);
});

test('xss payloads are stripped while the pill HTML survives sanitization', () => {
  const refs = [{ id: 'chunk-uuid-3', chunk_type: 'document', knowledge_title: '手册' }];
  const html = renderEmbedChatMarkdown(
    '前文\n\n<script>alert(1)</script>\n\n<img src=x onerror="alert(1)">\n\n[js](javascript:alert(2))\n\n<kb doc="手册" chunk_id="chunk-uuid-3"/>',
    refs,
  );
  assert.doesNotMatch(html, /<script/);
  assert.doesNotMatch(html, /onerror/);
  assert.doesNotMatch(html, /href="javascript:/);
  assert.match(html, /citation-kb/);
});

test('citation attribute injection cannot break out of the pill markup', () => {
  const html = renderEmbedChatMarkdown('<kb doc="x&quot;&gt;&lt;script&gt;alert(1)&lt;/script&gt;" chunk_id="abc"/>', []);
  // Vue escapeHtml escapes the & first, so the attribute keeps the entity text
  // (double-escaped) but stays inside its quotes — no markup break-out.
  assert.match(html, /^<p><span class="citation citation-kb" data-kb-id="" data-chunk-id="abc" data-doc="[^"]*" role="button" tabindex="0">/);
  assert.doesNotMatch(html, /<script/);
});

test('a trailing incomplete citation tag is hidden while streaming like Vue', () => {
  assert.equal(renderEmbedChatMarkdown('答案 <kb doc="手册" chunk_id="c1"'), renderEmbedChatMarkdown('答案 '));
  // A complete tag after other text still renders.
  assert.match(renderEmbedChatMarkdown('答案 <kb doc="手册" chunk_id="c1"/>'), /citation-kb/);
});

test('markdown syntax inside a citation tag is never parsed as markdown', () => {
  const html = renderEmbedChatMarkdown('# 标题 <kb doc="**手册**" chunk_id="c1"/>', []);
  assert.match(html, /<h1[^>]*>标题 <span class="citation citation-kb"/);
  assert.match(html, /data-doc="\*\*手册\*\*"/);
});
