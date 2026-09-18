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
const { EMBED_MERMAID_PREFIX, defaultEmbedMermaidLoader, hydrateEmbedAnswerMermaid, decorateEmbedMermaidChrome } = await import('./mermaid.ts');
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

// ─── R449/A3 — badge/fullscreen chrome (EmbedBotMessage.vue parity) ──────────
// Vue embed face renders every mermaid block through buildMermaidBlockHtml
// (frontend/src/utils/markdownEnhancements.ts L105): a header with a
// mermaid.diagram badge and a mermaid.expand fullscreen button, and
// attachMarkdownEnhancementListeners opens the fullscreen viewer. The React
// face must decorate the hydrated .wk-chat-mermaid figure with the same chrome.

const zhLabels = {
  badge: '图表',
  expand: '全屏查看',
  close: '关闭',
  zoomIn: '放大',
  zoomOut: '缩小',
  reset: '重置',
  download: '下载图片',
  downloading: '下载中...',
};

function hydratedRoot(): {
  root: HTMLElement;
  figure: HTMLElement;
  engine: (node: HTMLElement, prefix?: string) => Promise<void>;
} {
  const root = document.createElement('div');
  root.innerHTML = '<pre data-markdown-diagram="mermaid"><code class="language-mermaid">graph TD; A--&gt;B</code></pre>';
  document.body.appendChild(root);
  const figure = document.createElement('figure');
  figure.className = 'wk-chat-mermaid';
  figure.setAttribute('role', 'img');
  figure.innerHTML = '<svg width="10"><path d="M0 0"></path></svg>';
  // Fake views engine: swap the escaped source block for the hydrated figure.
  const engine = async () => { root.querySelector('pre')?.replaceWith(figure); };
  return { root, figure, engine };
}

test('hydration decorates the svg figure with the Vue badge/expand header', async () => {
  const { root, figure, engine } = hydratedRoot();
  await hydrateEmbedAnswerMermaid(root, true, engine, zhLabels);
  const block = root.querySelector('.embed-mermaid-block');
  assert.ok(block, 'the hydrated figure gets the mermaid block wrapper');
  assert.ok(block?.contains(figure), 'the svg figure stays inside the block');
  const badge = block?.querySelector('.embed-mermaid-block__badge');
  assert.equal(badge?.textContent, '图表', 'the header badge carries the mermaid.diagram label');
  const expand = block?.querySelector<HTMLButtonElement>('.embed-mermaid-block__expand');
  assert.ok(expand, 'the header carries the fullscreen expand button');
  assert.equal(expand?.getAttribute('aria-label'), '全屏查看', 'expand matches the mermaid.expand label');
  assert.equal(expand?.getAttribute('title'), '全屏查看');
});

test('hydration without labels (or a failed loader) leaves no chrome behind', async () => {
  const { root: bare, engine } = hydratedRoot();
  await hydrateEmbedAnswerMermaid(bare, true, engine);
  assert.equal(bare.querySelector('.embed-mermaid-block'), null, 'no chrome without labels');

  const { root: failed } = hydratedRoot();
  await hydrateEmbedAnswerMermaid(failed, true, async () => { throw new Error('boom'); }, zhLabels);
  assert.equal(failed.querySelector('.embed-mermaid-block'), null, 'no chrome when hydration fails');
});

test('expand opens a fullscreen viewer dialog, close and Escape dismiss it', async () => {
  const { root, engine } = hydratedRoot();
  await hydrateEmbedAnswerMermaid(root, true, engine, zhLabels);
  const expand = root.querySelector<HTMLButtonElement>('.embed-mermaid-block__expand')!;
  expand.click();
  const viewer = document.body.querySelector('.embed-mermaid-viewer');
  assert.ok(viewer, 'expand opens the fullscreen viewer');
  assert.equal(viewer?.getAttribute('role'), 'dialog');
  assert.equal(viewer?.getAttribute('aria-label'), '全屏查看');
  assert.ok(viewer?.querySelector('svg path[d="M0 0"]'), 'the viewer shows the diagram svg');

  const closeBtn = viewer?.querySelector<HTMLButtonElement>('.embed-mermaid-viewer__close');
  assert.ok(closeBtn, 'the viewer offers a close control');
  assert.equal(closeBtn?.getAttribute('aria-label'), '关闭');
  closeBtn!.click();
  assert.equal(document.body.querySelector('.embed-mermaid-viewer'), null, 'close dismisses the viewer');

  expand.click();
  assert.ok(document.body.querySelector('.embed-mermaid-viewer'), 'expand reopens');
  const EscapeEvent = (window as unknown as { KeyboardEvent: typeof KeyboardEvent }).KeyboardEvent;
  document.dispatchEvent(new EscapeEvent('keydown', { key: 'Escape' }));
  assert.equal(document.body.querySelector('.embed-mermaid-viewer'), null, 'Escape dismisses the viewer');
});

test('the expand button is disabled until an svg exists (Vue syncMermaidExpandButtons)', () => {
  const root = document.createElement('div');
  root.innerHTML = '<figure class="wk-chat-mermaid" role="img"></figure>';
  document.body.appendChild(root);
  decorateEmbedMermaidChrome(root, zhLabels);
  const expand = root.querySelector<HTMLButtonElement>('.embed-mermaid-block__expand')!;
  assert.ok(expand.disabled, 'no svg means the fullscreen action stays disabled');
});

test('figures already decorated are not wrapped twice', async () => {
  const { root, engine } = hydratedRoot();
  await hydrateEmbedAnswerMermaid(root, true, engine, zhLabels);
  decorateEmbedMermaidChrome(root, zhLabels);
  assert.equal(root.querySelectorAll('.embed-mermaid-block').length, 1, 'decoration is idempotent');
  assert.equal(root.querySelectorAll('.embed-mermaid-block__badge').length, 1);
});

// ─── R463/A3 — fullscreen viewer toolbar (Vue openMermaidFullscreen parity) ──
// Vue ships a zoomIn/zoomOut/reset/download toolbar beside the close control
// (frontend/src/utils/mermaidViewer.ts L98-103); the React face mounts the
// shared views-engine toolbar (packages/views/src/chat/mermaid-viewer.ts).

test('the fullscreen viewer carries the Vue toolbar with zoom stepping', async () => {
  const { root, engine } = hydratedRoot();
  await hydrateEmbedAnswerMermaid(root, true, engine, zhLabels);
  root.querySelector<HTMLButtonElement>('.embed-mermaid-block__expand')!.click();
  const viewer = document.body.querySelector('.embed-mermaid-viewer')!;
  const stage = viewer.querySelector<HTMLElement>('.embed-mermaid-viewer__stage')!;
  assert.ok(stage, 'the zoom transform targets the viewer stage');

  const buttons = [...viewer.querySelectorAll<HTMLButtonElement>('.wk-mermaid-viewer-toolbar button')];
  assert.deepEqual(
    buttons.map((button) => button.title),
    ['放大', '缩小', '重置', '下载图片', '关闭'],
    'zoomIn, zoomOut, reset, download, then close — the Vue order',
  );

  buttons[0]!.click();
  buttons[0]!.click();
  assert.match(stage.style.transform, /scale\(1\.4\)/, 'zoomIn steps by the Vue 0.2 increment');
  buttons[2]!.click();
  assert.equal(stage.style.transform, 'translate(0px, 0px) scale(1)', 'reset restores the initial view');
});

test('the embed labels type the shared viewer toolbar contract', async () => {
  const { attachMermaidViewerToolbar } = await import('@weknora/views/chat/mermaid');
  const overlay = document.createElement('div');
  const stage = document.createElement('div');
  const handle = attachMermaidViewerToolbar(overlay, stage, zhLabels);
  overlay.appendChild(handle.toolbar);
  assert.ok(overlay.contains(handle.toolbar), 'the shared engine mounts the toolbar for the embed face');
  handle.detach();
  assert.equal(overlay.querySelector('.wk-mermaid-viewer-toolbar'), null, 'detach clears the toolbar');
});
