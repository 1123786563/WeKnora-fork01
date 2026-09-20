import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import * as nodeModule from 'node:module';
import test from 'node:test';

import {
  buildHeaderUtilityItems,
  buildSessionMarkdown,
  collectAllSessionMessages,
  copyTextToClipboard,
  currentSessionLink,
  headerUtilityCopy,
} from './header-menu-actions.ts';

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, { window: dom.window, document: dom.window.document });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

function stubClipboard(writeText?: (text: string) => Promise<void>): void {
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    // value must be spelled out: a partial descriptor keeps the previous value.
    value: writeText ? { writeText } : undefined,
  });
}

function stubExecCommand(exec: (command: string) => boolean): void {
  Object.defineProperty(document, 'execCommand', { configurable: true, value: exec });
}

/*
 * R483 D16 (R482 report-B2): the Vue chat header ⋯ menu carries a utility
 * block between 修改标题 and 清空消息 — 复制会话 ID / 复制对话链接 /
 * 复制为 Markdown / 在新窗口中打开 (ChatHeader.vue:61-77). The behaviors are
 * ported from the Vue sources: clipboard.ts (async API + execCommand
 * fallback), sessionMarkdown.ts (paginated collect + export builder) and
 * currentSessionLink (URL minus search/hash).
 */

test('the utility copy table carries the four Vue menu labels byte-exact per locale', () => {
  assert.deepEqual(
    [headerUtilityCopy('zh-CN').copySessionId, headerUtilityCopy('zh-CN').copyLink, headerUtilityCopy('zh-CN').copyMarkdown, headerUtilityCopy('zh-CN').openNewWindow],
    ['复制会话 ID', '复制对话链接', '复制为 Markdown', '在新窗口中打开'],
  );
  assert.deepEqual(
    [headerUtilityCopy('en-US').copySessionId, headerUtilityCopy('en-US').copyLink, headerUtilityCopy('en-US').copyMarkdown, headerUtilityCopy('en-US').openNewWindow],
    ['Copy Session ID', 'Copy Conversation Link', 'Copy as Markdown', 'Open in New Window'],
  );
  // Toast + markdown-builder strings exist for every supported locale.
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    const copy = headerUtilityCopy(locale);
    assert.ok(copy.sessionIdCopied && copy.linkCopied && copy.copyFailed && copy.markdownCopied && copy.markdownCopyFailed);
    assert.ok(copy.markdown.sessionId && copy.markdown.exportedAt && copy.markdown.user && copy.markdown.assistant && copy.markdown.attachments && copy.markdown.references);
  }
});

test('the session link strips query and hash like the Vue currentSessionLink', () => {
  const link = currentSessionLink(new URL('http://localhost:5175/platform/chat/abc-123?q=deep&kbIds=k1#frag'));
  assert.equal(link, 'http://localhost:5175/platform/chat/abc-123');
});

test('copyTextToClipboard prefers the async clipboard API and falls back to execCommand', async () => {
  const calls: string[] = [];
  stubClipboard(async (text) => { calls.push('api:' + text); });
  assert.equal(await copyTextToClipboard('via-api'), true);
  assert.deepEqual(calls, ['api:via-api']);

  // Clipboard API unavailable (non-secure origin) → hidden textarea + execCommand.
  stubClipboard(undefined);
  stubExecCommand((command) => { calls.push('exec:' + command); return true; });
  assert.equal(await copyTextToClipboard('via-exec'), true);
  assert.deepEqual(calls, ['api:via-api', 'exec:copy']);
});

test('collectAllSessionMessages pages backwards, dedupes and sorts like the Vue collector', async () => {
  const pages = new Map<string, Array<{ id: string; role: string; content: string; created_at: string }>>([
    ['', [
      { id: 'u2', role: 'user', content: 'second', created_at: '2026-09-20T10:01:00Z' },
      { id: 'a2', role: 'assistant', content: 'answer two', created_at: '2026-09-20T10:02:00Z' },
    ]],
    ['2026-09-20T10:01:00Z', [
      { id: 'u1', role: 'user', content: 'first', created_at: '2026-09-20T10:00:00Z' },
      // Overlapping row re-served by the older page (dedupe by id).
      { id: 'a2', role: 'assistant', content: 'answer two', created_at: '2026-09-20T10:02:00Z' },
    ]],
  ]);
  const collected = await collectAllSessionMessages(async (beforeTime) => pages.get(beforeTime) ?? [], 2);
  assert.deepEqual(collected.map((message) => message.id), ['u1', 'u2', 'a2']);
});

test('buildSessionMarkdown renders the Vue export shape with labels, attachments and references', () => {
  const markdown = buildSessionMarkdown({
    sessionId: 'session-1',
    title: '导出的对话',
    messages: [
      { id: 'u1', role: 'user', content: '第一问 <kb rid="k1"/>', created_at: '2026-09-20T10:00:00Z', attachments: [{ file_name: 'spec.pdf' }] },
      { id: 'a1', role: 'assistant', content: '引用回答', created_at: '2026-09-20T10:02:00Z', knowledge_references: [{ knowledge_title: '手册', knowledge_source: 'https://example.com/manual' }] },
    ],
    labels: headerUtilityCopy('zh-CN').markdown,
    exportedAt: '2026-09-20T00:00:00.000Z',
  });
  assert.match(markdown, /^# 导出的对话/);
  assert.match(markdown, /> 会话 ID: session-1  /);
  assert.match(markdown, /> 导出时间: 2026-09-20T00:00:00\.000Z/);
  assert.match(markdown, /## 用户\n\n第一问(?![\s\S]*<kb)/);
  assert.match(markdown, /### 附件\n\n- spec\.pdf/);
  assert.match(markdown, /## 助手\n\n引用回答/);
  assert.match(markdown, /### 引用\n\n- \[手册\]\(https:\/\/example\.com\/manual\)/);
  // The <kb/> citation tag is stripped from the export exactly like the Vue
  // cleanMessageContent.
  assert.doesNotMatch(markdown, /<kb/);
});

test('buildHeaderUtilityItems returns the four Vue actions in the Vue order', async () => {
  const activated: string[] = [];
  const copied: string[] = [];
  stubClipboard(async (text) => { copied.push(text); });
  const toasts: string[] = [];
  const items = buildHeaderUtilityItems({
    locale: 'zh-CN',
    sessionId: 'session-9',
    sessionTitle: '标题',
    currentUrl: new URL('http://localhost:5175/platform/chat/session-9?x=1'),
    loadMessagesPage: async () => [{ id: 'u1', role: 'user', content: '问', created_at: '2026-09-20T10:00:00Z' }],
    now: () => new Date('2026-09-20T00:00:00.000Z'),
    toast: (message) => { toasts.push(message); },
    openWindow: (url) => { activated.push('open:' + url); },
  });
  assert.deepEqual(items.map((item) => item.id), ['copySessionId', 'copyLink', 'copyMarkdown', 'openNewWindow']);
  assert.deepEqual(items.map((item) => item.label), ['复制会话 ID', '复制对话链接', '复制为 Markdown', '在新窗口中打开']);
  for (const item of items) item.onActivate();
  await new Promise((resolve) => setTimeout(resolve, 0));
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(copied, ['session-9', 'http://localhost:5175/platform/chat/session-9', '# 标题\n\n> 会话 ID: session-9  \n\n> 导出时间: 2026-09-20T00:00:00.000Z\n\n## 用户\n\n问\n']);
  assert.deepEqual(activated, ['open:http://localhost:5175/platform/chat/session-9']);
  assert.deepEqual(toasts, ['会话 ID 已复制', '对话链接已复制', '完整对话已复制为 Markdown']);
});

test('chat route wires the utility items into the chat page header menu', () => {
  const routeSource = readFileSync(new URL('./ChatRoutePage.tsx', import.meta.url), 'utf8');
  assert.match(routeSource, /buildHeaderUtilityItems/);
  assert.match(routeSource, /headerUtilityItems=\{headerUtilityItems\}/);
});
