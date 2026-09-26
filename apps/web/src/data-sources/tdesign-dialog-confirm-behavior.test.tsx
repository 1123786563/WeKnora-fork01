import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';

/* ocr1-016 行为存证：实证 tdesign-react 1.18.3 Dialog 点击确认按钮只调用
 * onConfirm，不会经 onClose 自动关闭弹窗。因此 DataSourcesPage 删除确认
 * 弹窗（onConfirm={() => void confirmDelete()}）在删除请求进行期间保持
 * 打开，confirmBtn.loading / cancelBtn.disabled / checkbox disabled
 * 三处提交期防护正常出现，弹窗只在 confirmDelete 成功路径
 * （setDeleteSource(null)）或用户主动取消（onClose）时关闭。 */
const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  SVGElement: dom.window.SVGElement,
  DocumentFragment: dom.window.DocumentFragment,
  Event: dom.window.Event,
  KeyboardEvent: dom.window.KeyboardEvent,
  MouseEvent: dom.window.MouseEvent,
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle?.bind(dom.window),
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? clearTimeout,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
(globalThis as typeof globalThis & { React: typeof React }).React = React;

const { Dialog } = await import('tdesign-react');

test('tdesign Dialog: clicking the confirm button does not auto-close via onClose', async () => {
  const events: string[] = [];
  let root: Root | null = null;
  const container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  await act(async () => {
    root!.render(
      <Dialog
        visible
        header="probe"
        confirmBtn={{ content: '确定' }}
        cancelBtn={{ content: '取消' }}
        closeOnOverlayClick={false}
        onClose={() => { events.push('close'); }}
        onCancel={() => { events.push('cancel'); }}
        onConfirm={() => { events.push('confirm'); }}
      >
        body
      </Dialog>,
    );
  });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 30)); });

  const confirmBtn = document.querySelector('.t-dialog__confirm') as HTMLButtonElement | null;
  assert.ok(confirmBtn, 'confirm button renders (portal attached to jsdom body)');

  await act(async () => { confirmBtn!.click(); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 30)); });

  assert.ok(events.includes('confirm'), 'onConfirm fires');
  assert.equal(events.includes('close'), false, `onClose must NOT fire from a confirm click in tdesign-react 1.18.3 — got ${JSON.stringify(events)}`);
  root!.unmount();
});
