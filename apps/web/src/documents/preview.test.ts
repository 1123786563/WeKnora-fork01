import assert from 'node:assert/strict';
import test from 'node:test';
import { createElement, act } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import * as XLSX from 'xlsx';

// jsdom globals for the mermaid fullscreen viewer tests (the viewer mounts on
// document.body at click time, mirroring the embed mermaid test setup).
import { JSDOM } from 'jsdom';
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'http://localhost/' });
(globalThis as typeof globalThis & { window: unknown; document: unknown }).window = dom.window;
(globalThis as typeof globalThis & { window: unknown; document: unknown }).document = dom.window.document;

import { DocumentPreviewContent, buildDocumentPreview, canPreviewDocument, isInlinePreviewKind, openDocumentMermaidFullscreen, readCurrentPreviewText, readPreviewText, readSpreadsheetPreview, type DocumentMermaidLabels, type DocumentMermaidLoader } from './preview.ts';

test('builds an authenticated preview model without treating download URLs as public', () => {
  assert.deepEqual(buildDocumentPreview({ id: 'doc/a', type: 'file', file_name: 'guide.pdf', parse_status: 'completed' }, '/api/v1/knowledge/doc%2Fa/preview'), {
    kind: 'pdf',
    ready: true,
    path: '/api/v1/knowledge/doc%2Fa/preview',
    fileName: 'guide.pdf',
  });
});

test('preview readiness ignores parse_status exactly like the Vue canPreview gate', () => {
  assert.equal(buildDocumentPreview({ id: 'doc-1', type: 'file', file_name: 'guide.pdf', parse_status: 'pending' }, '/preview').ready, true, 'Vue never consults parse_status before loading preview content');
  assert.equal(buildDocumentPreview({ id: 'doc-1', type: 'file', file_name: 'guide.pdf', parse_status: 'finalizing' }, '/preview').ready, true);
  assert.equal(buildDocumentPreview({ id: 'doc-1', type: 'file', file_name: 'guide.pdf', parse_status: 'failed' }, '/preview').ready, true);
  assert.equal(buildDocumentPreview({ id: 'doc-1', type: 'file', file_name: 'Interview.mp3', parse_status: 'pending' }, '/preview').ready, true, 'audio stays fetchable for the embedded player regardless of processing state');
  assert.equal(buildDocumentPreview({ id: 'doc-1', file_name: 'guide.pdf', type: 'manual' }, '/preview').ready, false, 'Vue gates on details.type === "file"');
  assert.equal(buildDocumentPreview({ id: 'doc-1', type: 'file', file_name: 'brief.docx', parse_status: 'completed' }, '/preview').ready, false, 'unresolvable inline extensions never fetch');
  assert.equal(buildDocumentPreview({ id: 'doc-1', type: 'file', file_name: 'guide.' }, '/preview').ready, false);
});

test('marks spreadsheet documents as inline previewable like the Vue document preview', () => {
  assert.equal(isInlinePreviewKind('text'), true);
  assert.equal(isInlinePreviewKind('markdown'), true);
  assert.equal(isInlinePreviewKind('image'), true);
  assert.equal(isInlinePreviewKind('pdf'), true);
  assert.equal(isInlinePreviewKind('audio'), true);
  assert.equal(isInlinePreviewKind('video'), true);
  assert.equal(isInlinePreviewKind('spreadsheet'), true);
  assert.equal(isInlinePreviewKind('mermaid'), true);
  assert.equal(isInlinePreviewKind('docx'), false);
  assert.equal(isInlinePreviewKind('pptx'), false);
  assert.equal(isInlinePreviewKind('unsupported'), false);
});

test('canPreviewDocument mirrors the Vue canPreview gate: file type, resolvable extension, never audio', () => {
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'Guide.pdf', source: '' }), true, 'real backend payloads carry source:"" for file knowledge — the gate keys off type');
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'Guide.pdf' }), true, 'absent source must not matter; Vue checks type');
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'architecture.mmd' }), true);
  assert.equal(canPreviewDocument({ type: 'file', file_type: 'pdf' }), true, 'Vue resolveFilePreviewExt falls back to file_type when the title has no extension');
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'Interview.mp3' }), false, 'Vue keeps audio out of the preview tab; the player is embedded instead');
  assert.equal(canPreviewDocument({ type: 'url', file_name: 'Guide.pdf' }), false);
  assert.equal(canPreviewDocument({ type: 'manual', file_name: 'Guide.pdf' }), false);
  assert.equal(canPreviewDocument({ file_name: 'Guide.pdf', source: '' }), false, 'missing type never previews, matching Vue details?.type !== "file"');
  assert.equal(canPreviewDocument({ type: '', file_name: 'Guide.pdf' }), false, 'empty-string type is not file');
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'notes.txt', file_type: 'pdf' }), true, 'Vue prefers an explicit file_type over the filename suffix');
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'guide.pdf', file_type: 'txt' }), true, 'an explicit file_type wins even when the filename suffix differs — txt is text-previewable');
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'clip.mp4', file_type: 'exe' }), false, 'an explicit unpreviewable file_type wins over the filename suffix');
  assert.equal(canPreviewDocument({ type: 'file', file_name: 'Guide.' }), false, 'trailing dot resolves to no extension');
  assert.equal(canPreviewDocument({ type: 'file' }), false);
});

test('recognizes Mermaid files as inline previews like the Vue document preview', () => {
  const preview = buildDocumentPreview({ id: 'doc-1', type: 'file', file_name: 'architecture.mmd', parse_status: 'completed' }, '/preview');

  assert.equal(preview.kind, 'mermaid');
  assert.equal(preview.ready, true);
});

test('reads every Vue-supported spreadsheet sheet into safe table rows', async () => {
  const workbook = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(workbook, XLSX.utils.aoa_to_sheet([
    ['Name', 'Count'],
    ['Alpha', 2],
  ]), 'Summary');
  XLSX.utils.book_append_sheet(workbook, XLSX.utils.aoa_to_sheet([
    ['Status'],
    ['<ready>'],
  ]), 'Details');
  const bytes = XLSX.write(workbook, { bookType: 'xlsx', type: 'array' });

  assert.deepEqual(await readSpreadsheetPreview(bytes as ArrayBuffer, 'report.xlsx'), {
    sheets: [
      { name: 'Summary', rows: [['Name', 'Count'], ['Alpha', '2']] },
      { name: 'Details', rows: [['Status'], ['<ready>']] },
    ],
  });
});

test('renders markdown as escaped text instead of interpreting HTML', async () => {
  const text = await readPreviewText(new Blob(['# Guide\n<script>alert(1)</script>'], { type: 'text/markdown' }));
  const markup = renderToStaticMarkup(createElement(DocumentPreviewContent, { kind: 'markdown', text }));

  assert.match(markup, /&lt;script&gt;alert\(1\)&lt;\/script&gt;/);
  assert.doesNotMatch(markup, /<script>/);
});

test('drops delayed text from a closed preview after a new preview opens', async () => {
  let firstIsActive = true;
  const delayedFirstText = readCurrentPreviewText('first document', () => firstIsActive);

  firstIsActive = false;
  const secondText = await readCurrentPreviewText('second document', () => true);

  assert.equal(await delayedFirstText, undefined);
  assert.equal(secondText, 'second document');
});

// ─── R465/A1 — mermaid fullscreen viewer (Vue document-preview parity) ───────
// Vue contract (frontend/src/components/document-preview.vue):
//   - the .preview-mermaid surface is clickable (@click="openMermaid"), which
//     mounts openMermaidFullscreen from utils/mermaidViewer.ts: a fixed overlay
//     with the toolbar zoomIn, zoomOut, reset, download, close (Vue order) —
//     zoom steps by 0.2, reset restores scale 1, Escape/overlay/close dismiss.
//   - clicking with no rendered svg is a no-op (`if (!svg) return`).
// Labels are the Vue i18n mermaid.* strings (zh-CN shown here).

const zhMermaidLabels: DocumentMermaidLabels = {
  zoomIn: '放大',
  zoomOut: '缩小',
  reset: '重置',
  download: '下载图片',
  downloading: '下载中...',
  close: '关闭',
  expand: '全屏查看',
};

const SVG_HTML = '<svg><path d="M0 0"></path></svg>';

function dismissOpenViewers(): void {
  for (const viewer of [...document.body.querySelectorAll('.wk-document-mermaid-viewer')]) viewer.remove();
}

test('the document mermaid fullscreen viewer carries the Vue toolbar in the Vue order', () => {
  openDocumentMermaidFullscreen(SVG_HTML, zhMermaidLabels);
  const viewer = document.body.querySelector<HTMLElement>('.wk-document-mermaid-viewer');
  assert.ok(viewer, 'openDocumentMermaidFullscreen mounts the viewer on body');
  assert.equal(viewer.getAttribute('role'), 'dialog');
  assert.equal(viewer.getAttribute('aria-label'), '全屏查看');
  assert.ok(viewer.querySelector('.wk-document-mermaid-viewer__stage svg path'), 'the viewer shows the diagram svg');

  const buttons = [...viewer.querySelectorAll<HTMLButtonElement>('.wk-mermaid-viewer-toolbar button')];
  assert.deepEqual(
    buttons.map((button) => button.title),
    ['放大', '缩小', '重置', '下载图片', '关闭'],
    'zoomIn, zoomOut, reset, download, then close — the Vue order',
  );

  const stage = viewer.querySelector<HTMLElement>('.wk-document-mermaid-viewer__stage')!;
  buttons[0]!.click();
  buttons[0]!.click();
  assert.match(stage.style.transform, /scale\(1\.4\)/, 'zoomIn steps by the Vue 0.2 increment');
  buttons[2]!.click();
  assert.equal(stage.style.transform, 'translate(0px, 0px) scale(1)', 'reset restores the initial view');

  buttons[4]!.click();
  assert.equal(document.body.querySelector('.wk-document-mermaid-viewer'), null, 'close dismisses the viewer');
  dismissOpenViewers();
});

test('Escape and an overlay click dismiss the document mermaid viewer', () => {
  const KeyboardEventCtor = (window as unknown as { KeyboardEvent: typeof KeyboardEvent }).KeyboardEvent;

  openDocumentMermaidFullscreen(SVG_HTML, zhMermaidLabels);
  document.dispatchEvent(new KeyboardEventCtor('keydown', { key: 'Escape' }));
  assert.equal(document.body.querySelector('.wk-document-mermaid-viewer'), null, 'Escape dismisses the viewer');

  openDocumentMermaidFullscreen(SVG_HTML, zhMermaidLabels);
  const overlay = document.body.querySelector<HTMLElement>('.wk-document-mermaid-viewer')!;
  overlay.click();
  assert.equal(document.body.querySelector('.wk-document-mermaid-viewer'), null, 'an overlay click dismisses the viewer');
  dismissOpenViewers();
});

test('clicking a hydrated mermaid preview opens the viewer; no svg stays a no-op (Vue openMermaid)', async () => {
  const { createRoot } = await import('react-dom/client') as typeof import('react-dom/client');
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

  const svgLoader: DocumentMermaidLoader = async (root: HTMLElement) => {
    for (const block of [...root.querySelectorAll('[data-markdown-diagram="mermaid"]')]) {
      const wrapper = document.createElement('div');
      wrapper.className = 'wk-chat-mermaid';
      wrapper.setAttribute('role', 'img');
      wrapper.innerHTML = SVG_HTML;
      block.replaceWith(wrapper);
    }
  };

  const host = document.createElement('div');
  document.body.appendChild(host);
  const root = createRoot(host);
  await act(async () => {
    root.render(createElement(DocumentPreviewContent, {
      kind: 'mermaid',
      text: 'graph TD; A-->B',
      fileName: 'flow.mmd',
      mermaidLabels: zhMermaidLabels,
      mermaidLoader: svgLoader,
    }));
  });

  const preview = host.querySelector<HTMLElement>('.wk-preview-mermaid')!;
  assert.ok(preview, 'the mermaid preview surface renders');
  assert.ok(preview.querySelector('.wk-chat-mermaid svg'), 'the injected loader hydrated the diagram');

  await act(async () => {
    preview.dispatchEvent(new (window as unknown as { MouseEvent: typeof MouseEvent }).MouseEvent('click', { bubbles: true }));
  });
  const viewer = document.body.querySelector<HTMLElement>('.wk-document-mermaid-viewer');
  assert.ok(viewer, 'clicking the diagram opens the fullscreen viewer (Vue openMermaid)');
  assert.equal(
    viewer.querySelector<HTMLButtonElement>('.wk-mermaid-viewer-toolbar button')!.title,
    '放大',
    'the shared engine toolbar mounts with the passed labels',
  );
  dismissOpenViewers();

  // Vue openMermaid guard: no rendered svg means the click does nothing.
  await act(async () => {
    root.render(createElement(DocumentPreviewContent, {
      kind: 'mermaid',
      text: 'graph TD; A-->B',
      fileName: 'flow.mmd',
      mermaidLabels: zhMermaidLabels,
      mermaidLoader: async () => {},
    }));
  });
  const barePreview = host.querySelector<HTMLElement>('.wk-preview-mermaid')!;
  await act(async () => {
    barePreview.dispatchEvent(new (window as unknown as { MouseEvent: typeof MouseEvent }).MouseEvent('click', { bubbles: true }));
  });
  assert.equal(document.body.querySelector('.wk-document-mermaid-viewer'), null, 'no svg → no viewer (Vue `if (!svg) return`)');

  await act(async () => {
    root.unmount();
  });
  host.remove();
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
});
