import assert from 'node:assert/strict';
import test from 'node:test';

// R446/A3 — embed answer mermaid hydration. Vue contract (R445 gap):
//   - frontend/src/views/embed/EmbedBotMessage.vue renders ```mermaid blocks
//     through createMermaidCodeRenderer('mermaid-embed-botmsg') and hydrates
//     them only after session.is_completed (renderMermaidDiagrams ->
//     enhanceMarkdownContainer -> renderMermaidInContainer):
//       * streaming answers keep the escaped source block (data-mermaid="false")
//       * completed answers get the SVG injected; engine failures are caught
//         and the escaped code block stays visible (normal-code fallback)
//   - the React face reuses the shared mermaid engine from
//     packages/views/src/chat/mermaid.ts (mermaid 11.15.0 pinned there):
//     markdown emits <pre data-markdown-diagram="mermaid" ...> and the views
//     hydrator swaps it for the sanitized SVG container (DOMPurify svg profile
//     inside the engine's browser defaults).
// DOMPurify binds to the global window at import time — install jsdom globals
// before loading the renderer (mirrors embed-markdown.test.ts).
import * as nodeModule from 'node:module';
type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
hooks.registerHooks?.({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });
import { JSDOM } from 'jsdom';
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'http://localhost/' });
(globalThis as typeof globalThis & { window: unknown; document: unknown }).window = dom.window;
(globalThis as typeof globalThis & { window: unknown; document: unknown }).document = dom.window.document;

const { renderEmbedChatMarkdown } = await import('./markdown.ts');
const { EMBED_MERMAID_PREFIX, defaultEmbedMermaidLoader, hydrateEmbedAnswerMermaid } = await import('./mermaid.ts');
const { hydrateMermaidBlocks, hydrateMermaidBlocksWithBrowserDefaults } = await import('@weknora/views/chat/mermaid');

function mermaidRoot(): { root: HTMLElement; pre: HTMLElement } {
  const root = document.createElement('div');
  root.innerHTML = '<pre data-markdown-diagram="mermaid" data-mermaid="false" id="mermaid-embed-botmsg-1"><code class="language-mermaid">graph TD; A--&gt;B</code></pre>';
  document.body.appendChild(root);
  return { root, pre: root.querySelector('pre') as HTMLElement };
}

// ─── markdown pipeline: the mermaid container + data attributes ──────────────

test('answers with a ```mermaid block render the Vue mermaid container and data attributes', () => {
  const html = renderEmbedChatMarkdown('看这张图\n\n```mermaid\ngraph TD; A-->B\n```\n');
  // Vue createMermaidCodeRenderer('mermaid-embed-botmsg') stamps the id prefix
  // and data-mermaid="false" until the answer completes (buildMermaidBlockHtml).
  // SANITIZE_NAMED_PROPS (Vue markdownDomPurify L17) prefixes ids with
  // user-content-, so the rendered id is user-content-mermaid-embed-botmsg-N.
  assert.match(
    html,
    /<pre data-markdown-diagram="mermaid" data-mermaid="false" id="(user-content-)?mermaid-embed-botmsg-\d+"><code class="language-mermaid">graph TD; A--&gt;B<\/code><\/pre>/,
  );
});

test('mermaid source stays escaped inside the fenced block', () => {
  const html = renderEmbedChatMarkdown('```mermaid\nflowchart TD\n  A["<img src=x onerror=y>"] --> B\n```');
  assert.doesNotMatch(html, /<img src=x/);
  assert.match(html, /&lt;img src=x onerror=y&gt;/);
});

test('non-mermaid code blocks keep the plain code shape and are not marked for hydration', () => {
  const html = renderEmbedChatMarkdown('```js\nconst a = 1\n```');
  assert.doesNotMatch(html, /data-markdown-diagram/);
  assert.doesNotMatch(html, /data-mermaid/);
  assert.match(html, /<pre><code class="language-js">const a = 1/);
});

// ─── hydration gating: completed answers only, failures contained ────────────

test('streaming answers (is_completed false) are never handed to the mermaid loader', async () => {
  const { root, pre } = mermaidRoot();
  let loaderCalls = 0;
  await hydrateEmbedAnswerMermaid(root, false, async () => { loaderCalls += 1; });
  assert.equal(loaderCalls, 0);
  assert.ok(root.contains(pre), 'the escaped source block stays while streaming');
});

test('completed answers without mermaid blocks skip the loader', async () => {
  const root = document.createElement('div');
  root.innerHTML = '<p>plain answer</p>';
  document.body.appendChild(root);
  let loaderCalls = 0;
  await hydrateEmbedAnswerMermaid(root, true, async () => { loaderCalls += 1; });
  assert.equal(loaderCalls, 0);
});

test('completed answers with mermaid blocks call the embed loader once', async () => {
  const { root } = mermaidRoot();
  const seen: Array<[HTMLElement, string | undefined]> = [];
  await hydrateEmbedAnswerMermaid(root, true, async (node, prefix) => { seen.push([node, prefix]); });
  assert.equal(seen.length, 1);
  assert.equal(seen[0]![1], EMBED_MERMAID_PREFIX);
  assert.equal(seen[0]![0], root);
});

test('loader rejections never escape hydration (fallback keeps the code block)', async () => {
  const { root, pre } = mermaidRoot();
  await hydrateEmbedAnswerMermaid(root, true, async () => { throw new Error('mermaid exploded'); });
  assert.ok(root.contains(pre), 'the escaped source block remains visible on failure');
});

test('the default embed loader is the shared views browser-default hydrator', () => {
  assert.equal(defaultEmbedMermaidLoader, hydrateMermaidBlocksWithBrowserDefaults);
});

// ─── the shared engine swaps the block for the sanitized SVG (controlled
// promises — no real mermaid, no flake) ──────────────────────────────────────

test('hydration waits for the engine promise, sanitizes, then swaps in the SVG container', async () => {
  const { root } = mermaidRoot();
  let release!: (value: { svg: string }) => void;
  const engine = {
    initialize: () => undefined,
    render: () => new Promise<{ svg: string }>((resolve) => { release = resolve; }),
  };
  const sanitized: string[] = [];
  const done = hydrateMermaidBlocks(
    root,
    engine,
    (svg) => { sanitized.push(svg); return svg.replace(' onload="bad()"', ''); },
    EMBED_MERMAID_PREFIX,
  );
  // While the render promise is pending the escaped source block is what the
  // visitor sees (Vue: data-mermaid="false" until the SVG exists).
  assert.match(root.querySelector('pre')?.textContent || '', /graph TD; A-->B/);
  assert.equal(root.querySelector('.wk-chat-mermaid'), null);

  release({ svg: '<svg width="10"><path d="M0 0"></path></svg>' });
  await done;

  const wrapper = root.querySelector('.wk-chat-mermaid');
  assert.ok(wrapper, 'the views engine swaps the pre for its SVG container');
  assert.equal(wrapper?.getAttribute('role'), 'img');
  assert.match(wrapper?.innerHTML || '', /<path d="M0 0"/);
  assert.doesNotMatch(wrapper?.innerHTML || '', /onerror|<script/i);
  assert.equal(sanitized.length, 1, 'the SVG passes through the sanitizer before injection');
  assert.equal(root.querySelector('pre[data-markdown-diagram="mermaid"]'), null);
});

test('engine failures keep the escaped code block as the readable fallback', async () => {
  const { root, pre } = mermaidRoot();
  await hydrateMermaidBlocks(
    root,
    { initialize: () => undefined, render: async () => { throw new Error('invalid diagram'); } },
    (svg) => svg,
    EMBED_MERMAID_PREFIX,
  );
  assert.ok(root.contains(pre), 'the block is preserved for the normal code display');
  assert.equal(pre.dataset.mermaidError, 'true');
  assert.equal(root.querySelector('.wk-chat-mermaid'), null);
});

test('an empty sanitized SVG is treated as a failure and keeps the code block', async () => {
  const { root, pre } = mermaidRoot();
  await hydrateMermaidBlocks(
    root,
    { initialize: () => undefined, render: async () => ({ svg: '<svg><path></path></svg>' }) },
    () => '',
    EMBED_MERMAID_PREFIX,
  );
  assert.ok(root.contains(pre));
  assert.equal(root.querySelector('.wk-chat-mermaid'), null);
});
