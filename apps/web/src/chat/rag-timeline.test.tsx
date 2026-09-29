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
const { RagPipelineProgressFace, resolveChatCopy } = await import('@weknora/views');

let root: ReturnType<typeof createRoot> | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
});

/*
 * CHAT-8 对齐 —— Vue RagPipelineProgress.vue：点击折叠根「检索完成 引用了N篇
 * 文档」展开内联 RAG 管线时间线（tree-children-expanded：已完成问题理解 /
 * 检索知识库 + 摘要 / 思考 / 完成），不再切换共享引用面板。历史行无
 * agentEventStream 时按 ensureRagPipelineHistoryStream 从 knowledge_references
 * 合成两步。
 */
test('retrieval-done root expands the inline RAG pipeline timeline', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<RagPipelineProgressFace
    copy={resolveChatCopy('zh-CN')}
    message={{
      id: 'm-1',
      session_id: 's-1',
      role: 'assistant',
      content: '答案',
      is_completed: true,
      knowledge_references: [
        { knowledge_id: 'kb-1', knowledge_title: '产品文档', chunk_type: 'doc' },
        { knowledge_id: 'kb-1', knowledge_title: '产品文档', chunk_type: 'doc' },
        { knowledge_id: 'kb-2', knowledge_title: 'FAQ', chunk_type: 'doc' },
      ],
    }}
    liveStatusText="检索完成"
  />));
  const rootBtn = container.querySelector<HTMLButtonElement>('button.tree-root-expand');
  assert.ok(rootBtn);
  assert.equal(rootBtn.getAttribute('aria-expanded'), 'false');
  assert.match(rootBtn.textContent ?? '', /检索完成/);
  // 折叠根计数沿用既有 groupChatReferences 分组（kb-1 两 chunk 并组 + kb-2）。
  assert.match(rootBtn.textContent ?? '', /引用了2篇文档/);
  assert.equal(container.querySelector('.tree-children'), null);

  await act(async () => rootBtn.click());
  const timeline = container.querySelector('.tree-children.tree-children-expanded');
  assert.ok(timeline);
  const names = [...timeline.querySelectorAll('.action-name')].map((node) => node.textContent);
  assert.deepEqual(names, ['已完成问题理解', '检索知识库', '完成']);
  assert.equal(timeline.querySelector('.results-summary-text')?.textContent, '找到 3 个结果，来自 2 个文件');
  assert.equal(rootBtn.getAttribute('aria-expanded'), 'true');

  await act(async () => rootBtn.click());
  assert.equal(container.querySelector('.tree-children'), null);
  assert.equal(rootBtn.getAttribute('aria-expanded'), 'false');
});

test('timeline keeps a memory row and a thinking row when the payloads exist', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<RagPipelineProgressFace
    copy={resolveChatCopy('zh-CN')}
    message={{
      id: 'm-2',
      session_id: 's-1',
      role: 'assistant',
      content: '答案',
      is_completed: true,
      knowledge_references: [{ knowledge_id: 'kb-1', knowledge_title: '文档', chunk_type: 'doc' }],
      used_memories: [{ id: 'mem-1', kind: 'preference', content: '偏好深色主题' }],
      agentEventStream: [
        { type: 'tool_call', tool_call_id: 't-1', tool_name: 'query_understand', pending: false, success: true },
        { type: 'tool_call', tool_call_id: 't-2', tool_name: 'knowledge_search', pending: false, success: true },
        { type: 'thinking', content: '先检索再回答' },
      ],
    }}
    liveStatusText="检索完成"
  />));
  await act(async () => container.querySelector<HTMLButtonElement>('button.tree-root-expand')?.click());
  const timeline = container.querySelector('.tree-children');
  assert.ok(timeline);
  const memoryHeader = timeline.querySelector('.memory-header .memory-name');
  assert.equal(memoryHeader?.textContent, '本次使用了 1 条记忆');
  const names = [...timeline.querySelectorAll('.action-name')].map((node) => node.textContent);
  assert.deepEqual(names, ['已完成问题理解', '检索知识库', '思考', '完成']);
  assert.equal(timeline.querySelector('.thinking-detail-content')?.textContent, '先检索再回答');
});
