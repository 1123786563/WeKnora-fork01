import assert from 'node:assert/strict';
import test from 'node:test';

// R463/A3 — shared mermaid fullscreen viewer toolbar (Vue openMermaidFullscreen
// parity, frontend/src/utils/mermaidViewer.ts). The Vue viewer ships a
// zoomIn/zoomOut/reset/download toolbar next to the close button with:
//   - zoom STEP 0.2, clamp [0.2, 10] (button + wheel)
//   - reset restores scale 1 and clears the drag translation
//   - wheel zoom anchors on the pointer (preventDefault, no page scroll)
//   - drag pans the diagram (grab/grabbing cursor)
//   - download exports the diagram via a Blob URL + a[download] click and
//     flashes a "downloading" title on the button
// The React engine counterpart lives here so both the embed face and the wiki
// face can mount the same chrome on their fullscreen overlays.
import { JSDOM } from 'jsdom';
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'http://localhost/' });
(globalThis as typeof globalThis & { window: unknown; document: unknown }).window = dom.window;
(globalThis as typeof globalThis & { window: unknown; document: unknown }).document = dom.window.document;

const { attachMermaidViewerToolbar } = await import('./mermaid-viewer.ts');

const labels = {
  zoomIn: '放大',
  zoomOut: '缩小',
  reset: '重置',
  download: '下载图片',
  downloading: '下载中...',
};

function mountedOverlay(): { overlay: HTMLElement; stage: HTMLElement; toolbar: HTMLElement; detach: () => void } {
  const overlay = document.createElement('div');
  overlay.className = 'test-viewer';
  const stage = document.createElement('div');
  stage.className = 'test-viewer__stage';
  stage.innerHTML = '<svg width="10"><path d="M0 0"></path></svg>';
  const { toolbar, detach } = attachMermaidViewerToolbar(overlay, stage, labels);
  overlay.append(toolbar, stage);
  document.body.appendChild(overlay);
  return { overlay, stage, toolbar, detach };
}

function toolbarButton(toolbar: HTMLElement, title: string): HTMLButtonElement {
  const button = toolbar.querySelector<HTMLButtonElement>(`button[title="${title}"]`);
  assert.ok(button, `the toolbar carries the ${title} button`);
  return button;
}

// ─── toolbar rendering (Vue button order: zoomIn, zoomOut, reset, download) ──

test('mounts the Vue toolbar buttons with their i18n titles in order', () => {
  const { toolbar, detach } = mountedOverlay();
  const buttons = [...toolbar.querySelectorAll('button')];
  assert.equal(buttons.length, 4, 'only the four Vue viewer actions are mounted');
  assert.deepEqual(
    buttons.map((button) => button.title),
    ['放大', '缩小', '重置', '下载图片'],
    'zoomIn, zoomOut, reset, download in the Vue order',
  );
  for (const button of buttons) {
    assert.equal(button.getAttribute('aria-label'), button.title, 'each control is labelled for screen readers');
    assert.match(button.innerHTML, /<svg[^>]*aria-hidden="true"/, 'each control carries its Vue icon');
  }
  detach();
});

// ─── zoom stepping: STEP 0.2, clamp [0.2, 10], reset semantics ────────────────

test('zoomIn steps the scale by 0.2 per click', () => {
  const { stage, toolbar, detach } = mountedOverlay();
  const zoomIn = toolbarButton(toolbar, labels.zoomIn);
  zoomIn.click();
  assert.match(stage.style.transform, /scale\(1\.2\)/, 'one zoomIn click lands on 1.2');
  zoomIn.click();
  zoomIn.click();
  assert.match(stage.style.transform, /scale\(1\.6\)/, 'two more clicks land on 1.6');
  detach();
});

test('zoomOut clamps at the Vue 0.2 floor', () => {
  const { stage, toolbar, detach } = mountedOverlay();
  const zoomOut = toolbarButton(toolbar, labels.zoomOut);
  for (let index = 0; index < 10; index += 1) zoomOut.click();
  assert.match(stage.style.transform, /scale\(0\.2\)/, 'ten clicks reach the 0.2 floor');
  zoomOut.click();
  assert.match(stage.style.transform, /scale\(0\.2\)/, 'extra clicks never go below 0.2');
  detach();
});

test('zoomIn clamps at the Vue 10 ceiling', () => {
  const { stage, toolbar, detach } = mountedOverlay();
  const zoomIn = toolbarButton(toolbar, labels.zoomIn);
  for (let index = 0; index < 60; index += 1) zoomIn.click();
  assert.match(stage.style.transform, /scale\(10\)/, 'the ceiling caps the scale at 10');
  detach();
});

test('reset restores scale 1 and clears the drag translation', () => {
  const { overlay, stage, toolbar, detach } = mountedOverlay();
  toolbarButton(toolbar, labels.zoomIn).click();
  toolbarButton(toolbar, labels.zoomIn).click();
  // pan via a drag: mousedown on the overlay, move, mouseup
  overlay.dispatchEvent(new dom.window.MouseEvent('mousedown', { clientX: 100, clientY: 100, bubbles: true }));
  dom.window.dispatchEvent(new dom.window.MouseEvent('mousemove', { clientX: 160, clientY: 130 }));
  dom.window.dispatchEvent(new dom.window.MouseEvent('mouseup', { clientX: 160, clientY: 130 }));
  assert.match(stage.style.transform, /scale\(1\.4\)/);
  assert.match(stage.style.transform, /translate\(60px, 30px\)/, 'the drag pans the stage');

  toolbarButton(toolbar, labels.reset).click();
  assert.equal(
    stage.style.transform,
    'translate(0px, 0px) scale(1)',
    'reset clears the translation and restores scale 1',
  );
  detach();
});

// ─── wheel zoom (pointer anchored) ───────────────────────────────────────────

test('wheel zoom steps the scale and never scrolls the page', () => {
  const { overlay, stage, detach } = mountedOverlay();
  const wheel = new dom.window.WheelEvent('wheel', { deltaY: -100, clientX: 50, clientY: 50, bubbles: true, cancelable: true });
  overlay.dispatchEvent(wheel);
  assert.match(stage.style.transform, /scale\(1\.2\)/, 'wheel-up zooms in by the same 0.2 step');
  assert.ok(wheel.defaultPrevented, 'the viewer prevents the page scroll');

  overlay.dispatchEvent(new dom.window.WheelEvent('wheel', { deltaY: 100, clientX: 50, clientY: 50, bubbles: true, cancelable: true }));
  assert.match(stage.style.transform, /scale\(1\)/, 'wheel-down zooms back out');
  detach();
});

// ─── download: Blob URL + a[download] click, then revoke ─────────────────────

test('download exports the diagram through a Blob URL anchor click', () => {
  const { stage, toolbar, detach } = mountedOverlay();
  const created: string[] = [];
  const revoked: string[] = [];
  const downloads: Array<{ href: string; download: string }> = [];
  // The engine module resolves the bare `URL` global (Node's), so the Blob URL
  // surface is patched there; the anchor is a jsdom element patched via its
  // prototype so the synthetic click is observable.
  const globalUrl = URL as unknown as { createObjectURL?: (blob: Blob) => string; revokeObjectURL?: (url: string) => void };
  const originalCreate = globalUrl.createObjectURL;
  const originalRevoke = globalUrl.revokeObjectURL;
  globalUrl.createObjectURL = (blob: Blob) => {
    created.push(`${blob.type}:${blob.size > 0}`);
    return 'blob:mermaid-test';
  };
  globalUrl.revokeObjectURL = (value: string) => { revoked.push(value); };
  const anchorDescriptor = Object.getOwnPropertyDescriptor(dom.window.HTMLAnchorElement.prototype, 'click');
  Object.defineProperty(dom.window.HTMLAnchorElement.prototype, 'click', {
    configurable: true,
    value(this: HTMLAnchorElement) {
      downloads.push({ href: this.getAttribute('href') || '', download: this.getAttribute('download') || '' });
    },
  });

  try {
    toolbarButton(toolbar, labels.download).click();
    assert.equal(created.length, 1, 'exactly one Blob is created for the export');
    assert.match(created[0]!, /^image\/svg\+xml:/, 'the export carries the svg blob type');
    assert.deepEqual(
      downloads,
      [{ href: 'blob:mermaid-test', download: 'mermaid-diagram.svg' }],
      'the diagram downloads through a Blob URL anchor with the Vue-style filename',
    );
    assert.deepEqual(revoked, ['blob:mermaid-test'], 'the Blob URL is revoked after the click');
    assert.ok(!downloads.some((item) => /^https?:/.test(item.href)), 'no network URL is requested');
  } finally {
    globalUrl.createObjectURL = originalCreate;
    globalUrl.revokeObjectURL = originalRevoke;
    if (anchorDescriptor) Object.defineProperty(dom.window.HTMLAnchorElement.prototype, 'click', anchorDescriptor);
  }
  detach();
});

// ─── toolbar clicks never leak to the overlay behind them ────────────────────

test('toolbar button clicks are contained (no overlay click-through)', () => {
  const { overlay, toolbar, detach } = mountedOverlay();
  let overlayClicks = 0;
  overlay.addEventListener('click', () => { overlayClicks += 1; });
  toolbarButton(toolbar, labels.zoomIn).dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }));
  assert.equal(overlayClicks, 0, 'zoom clicks stop before reaching the overlay');
  detach();
});

test('detach removes the wheel/drag listeners along with the toolbar', () => {
  const { overlay, stage, detach } = mountedOverlay();
  const resetStyle = stage.style.transform;
  detach();
  assert.equal(overlay.querySelector('button'), null, 'the toolbar is gone');
  overlay.dispatchEvent(new dom.window.WheelEvent('wheel', { deltaY: -100, clientX: 50, clientY: 50, bubbles: true, cancelable: true }));
  assert.equal(stage.style.transform, resetStyle, 'wheel no longer zooms after detach');
});
