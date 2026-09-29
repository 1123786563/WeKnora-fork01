import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, IS_REACT_ACT_ENVIRONMENT: true });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
const { createRoot } = await import('react-dom/client');
const { SessionSourceFilterFace, resolveChatCopy } = await import('@weknora/views');

let root: ReturnType<typeof createRoot> | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
});

/*
 * CHAT-10 对齐 —— Vue SessionSourceFilter.vue：会话来源筛选是自定义
 * trigger + 弹出面板 + 平台 logo 形态（原生 <select> 已移除）。当前账号
 * 双端均未渲染该控件（无渠道会话），此处为源码级形态契约。
 */
test('session source filter renders a custom trigger and popup listbox with logos', async () => {
  const selected: string[] = [];
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<SessionSourceFilterFace
    copy={resolveChatCopy('zh-CN')}
    current="web"
    options={[
      { value: 'web', label: '我的对话' },
      { value: 'embed:home', label: 'Embed 站点', logo: 'https://weknora.test/embed.png' },
    ]}
    onSelect={(value) => selected.push(value)}
  />));
  assert.equal(container.querySelector('select'), null, 'native select must be gone');
  const trigger = container.querySelector<HTMLButtonElement>('button.session-source-filter__trigger');
  assert.ok(trigger);
  assert.equal(trigger.getAttribute('aria-haspopup'), 'listbox');
  assert.equal(trigger.getAttribute('aria-expanded'), 'false');
  assert.equal(trigger.querySelector('.session-source-filter__label')?.textContent, '我的对话');
  // emphasized：当前值非默认桶时高亮形态。
  assert.equal(container.querySelector('.session-source-filter--emphasized'), null);

  await act(async () => trigger.click());
  assert.equal(trigger.getAttribute('aria-expanded'), 'true');
  const panel = container.querySelector('.session-source-filter__panel');
  assert.ok(panel);
  assert.equal(panel.getAttribute('role'), 'listbox');
  const options = [...panel.querySelectorAll('.session-source-filter__option')];
  assert.deepEqual(options.map((node) => node.getAttribute('aria-selected')), ['true', 'false']);
  assert.ok(options[1]?.querySelector('img.session-source-filter__logo'), 'channel option carries the platform logo');
  assert.ok(options[0]?.querySelector('.session-source-filter__check--visible'), 'current option shows the check');

  await act(async () => options[1]?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })));
  assert.deepEqual(selected, ['embed:home']);
});
