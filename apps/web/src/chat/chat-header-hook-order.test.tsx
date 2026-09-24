import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, HTMLInputElement: dom.window.HTMLInputElement, IS_REACT_ACT_ENVIRONMENT: true });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
// B1 question-minimap（ChatPage 渲染路径）经 window.requestAnimationFrame
// 调度测量；jsdom 无 raf，window 侧与 globalThis 侧都垫 setTimeout 帧垫片
// （同 settings 域判例 SkillSettingsPanel.test）。
const w = dom.window as unknown as { requestAnimationFrame?: unknown; cancelAnimationFrame?: unknown };
w.requestAnimationFrame = w.requestAnimationFrame ?? ((cb: (t: number) => void) => setTimeout(() => cb(Date.now()), 16));
w.cancelAnimationFrame = w.cancelAnimationFrame ?? ((id: ReturnType<typeof setTimeout>) => clearTimeout(id));
(globalThis as unknown as { requestAnimationFrame?: unknown }).requestAnimationFrame
  = (globalThis as unknown as { requestAnimationFrame?: unknown }).requestAnimationFrame ?? w.requestAnimationFrame;
(globalThis as unknown as { cancelAnimationFrame?: unknown }).cancelAnimationFrame
  = (globalThis as unknown as { cancelAnimationFrame?: unknown }).cancelAnimationFrame ?? w.cancelAnimationFrame;
// B1 question-minimap 指针粗细探测（question-minimap.tsx setIsCoarsePointer）：
// jsdom 无 matchMedia，垫 coarse=false 的最小桩。
w.matchMedia = w.matchMedia ?? ((query: string) => ({ matches: false, media: query, addEventListener: () => {}, removeEventListener: () => {}, addListener: () => {}, removeListener: () => {}, onchange: null, dispatchEvent: () => false }));
const { createRoot } = await import('react-dom/client');
const { ChatPage, resolveChatCopy } = await import('@weknora/views');

let root: ReturnType<typeof createRoot> | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
});

const baseProps = {
  draft: '',
  locale: 'zh-CN' as const,
  onSelectSession: () => undefined,
  onCreateSession: () => undefined,
  onDraftChange: () => undefined,
  send: async () => undefined,
};

/**
 * Regression: navigating to a session that is not yet in the loaded sessions
 * array (the immediate post-send jump, or a cold deep link) first renders
 * ChatHeaderMenu without a session — it early-returns before its rename
 * useEffect. When the session list lands, the effect mounts and React aborts
 * with "Rendered more hooks than during the previous render", white-screening
 * the whole app. The header menu must keep a stable hook count regardless of
 * session presence (D6 agent-chat browser evidence, 2026-09-18).
 */
test('ChatHeaderMenu keeps a stable hook count when the selected session arrives after mount', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);

  const session = { id: 'session-new', title: '新会话', is_pinned: false };
  const withMessages = (messages: unknown[]) => ({ ...baseProps, sessions: [] as typeof session[], selectedSessionId: 'session-new', messages: messages as never[], copy: resolveChatCopy('zh-CN') });

  // First render: selected session id set, sessions list still empty/stale.
  await act(async () => root?.render(<ChatPage {...withMessages([])} />));
  // Second render: the sessions list lands (and the stream may already have
  // produced an empty-content assistant message, as with the aborted agent
  // run) — previously this render threw the hooks-order violation.
  const messages = [
    { id: 'm1', session_id: 'session-new', role: 'user', content: '智能推理测试：请介绍一下 WeKnora 的检索能力', is_completed: true },
    { id: 'm2', session_id: 'session-new', role: 'assistant', content: '', is_completed: true },
  ];
  await act(async () => root?.render(<ChatPage {...withMessages(messages)} sessions={[session]} />));

  const headerMenu = container.querySelector('.wk-chat-header-menu');
  assert.ok(headerMenu, 'header menu renders once the session appears');
  assert.equal(container.textContent?.includes('Something went wrong'), false);
});
