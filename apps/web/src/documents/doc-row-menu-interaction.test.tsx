import '../test-tdom-harness.ts'; // jsdom 全局（tdesign Popup 运行时）
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';

// Rendered between tests: node:test runs the file's tests sequentially on one
// document, so each test unmounts its page before the next one mounts.

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge/kb-1/documents' });
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'zh-CN' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  MouseEvent: dom.window.MouseEvent,
  CustomEvent: dom.window.CustomEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});

const { createRoot } = await import('react-dom/client');
const { KnowledgeDocumentsPage } = await import('./KnowledgeDocumentsPage.tsx');

// --- fixture: one failed .md document with a real 17ms trace -------------------

const failedDocument = {
  id: 'doc-failed',
  file_name: 'mermaid-arch-demo.md',
  title: 'mermaid-arch-demo.md',
  // Live API shape: type carries the Vue item.type signal; source is the
  // empty string (not nullish) on normal file documents.
  type: 'file',
  source: '',
  file_type: 'md',
  parse_status: 'failed',
  summary_status: '',
  updated_at: '2026-09-19T02:35:00Z',
  created_at: '2026-09-18T10:00:00Z',
};

const failedSpans = {
  knowledge_id: 'doc-failed',
  parse_status: 'failed',
  current_attempt: 1,
  current_stage: 'docreader',
  trace: {
    span_id: 'span-root',
    name: 'knowledge_processing',
    kind: 'root',
    status: 'failed',
    duration_ms: 17,
    started_at: '2026-09-19T02:35:00.000Z',
    finished_at: '2026-09-19T02:35:00.017Z',
    children: [
      { name: 'docreader', kind: 'stage', status: 'failed', duration_ms: 17 },
    ],
  },
};

interface MoveCall { knowledge_ids: string[]; source_kb_id: string; target_kb_id: string; mode: string }

function menuClient(calls: { spans: number; moveTargets: number; move: MoveCall[] }): WeKnoraClient {
  const knowledgeBase = {
    id: 'kb-1',
    name: 'parity-kb',
    description: '',
    type: 'knowledge',
    created_at: '2030-01-01T00:00:00Z',
    my_permission: 'owner',
  };
  const documents = {
    list: async () => ({ data: [failedDocument], total: 1, page: 1, page_size: 20 }),
    tagsPage: async () => ({ data: [], total: 0, page: 1, page_size: 50 }),
    tags: async () => [],
    folders: async () => ({ folders: [], root_document_count: 1, total_document_count: 1 }),
    spans: async () => { calls.spans += 1; return failedSpans; },
    moveTargets: async () => {
      calls.moveTargets += 1;
      return [
        { id: 'kb-2', name: 'Wiki Parity Fixture', knowledge_count: 0 },
        { id: 'kb-3', name: 'Parity KB Demo', knowledge_count: 2 },
      ];
    },
    move: async (input: MoveCall) => {
      calls.move.push(input);
      return { taskId: 'task-move-1', knowledgeCount: input.knowledge_ids.length };
    },
    moveProgress: async () => ({ taskId: 'task-move-1', status: 'completed', processed: 1, failed: 0, total: 1, progress: 100 }),
  };
  return {
    knowledgeBases: {
      settings: {
        get: async () => knowledgeBase,
        parserEngines: async () => ({ data: [] }),
      },
      list: async () => [knowledgeBase],
      documents,
    },
    knowledge: { documents },
    auth: { me: async () => ({ user: { id: 'u1', role: 'admin', is_superuser: true }, memberships: [{ tenant_id: 1, role: 'admin' }] }) },
    identity: { organizations: { knowledgeBaseShares: { listShared: async () => null } } },
    configuration: { models: { list: async () => [] } },
    settings: { system: { info: async () => ({}) } },
  } as unknown as WeKnoraClient;
}

async function renderPage(client: WeKnoraClient): Promise<Root> {
  const host = document.createElement('div');
  document.body.append(host);
  const root = createRoot(host);
  await act(async () => {
    root.render(React.createElement(KnowledgeDocumentsPage, { client, knowledgeBaseId: 'kb-1' }));
  });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });
  return root;
}

function openRowMenu(): HTMLElement {
  // tdesign 平移后（Vue DocumentCardView DOM）：卡片三点触发器是
  // .more-wrap（t-popup trigger），菜单 portal 到 body 的 .card-menu。
  const trigger = document.querySelector('.knowledge-card .more-wrap') as HTMLElement | null;
  assert.ok(trigger, 'document row more trigger is mounted');
  act(() => { trigger.click(); });
  const menus = [...document.querySelectorAll('.card-menu')] as HTMLElement[];
  const menu = menus.find((candidate) => candidate.querySelector('[role="menuitem"]'));
  assert.ok(menu, 'row action menu opened');
  return menu;
}

function menuLabels(menu: HTMLElement): string[] {
  return [...menu.querySelectorAll('[role="menuitem"]')].map((item) => item.textContent?.trim() ?? '');
}

let mountedRoot: Root | undefined;

import { afterEach } from 'node:test';
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

test('the document row menu renders the seven Vue items in Vue order after the trace probe', async () => {
  const calls = { spans: 0, moveTargets: 0, move: [] as MoveCall[] };
  mountedRoot = await renderPage(menuClient(calls));
  const menu = openRowMenu();
  // The probe is async — let the spans call land before asserting the trace item.
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });
  assert.deepEqual(menuLabels(menu), [
    '下载',
    '查看 Trace',
    '重建知识',
    '移动到目录',
    '移动到...',
    '批量管理',
    '删除文档',
  ]);
  assert.ok(calls.spans >= 1, 'opening the menu probes /spans for the trace item');
});

test('移动到... opens the Vue target-KB picker inside the row menu', async () => {
  const calls = { spans: 0, moveTargets: 0, move: [] as MoveCall[] };
  mountedRoot = await renderPage(menuClient(calls));
  const menu = openRowMenu();
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });
  const moveItem = [...menu.querySelectorAll('[role="menuitem"]')].find((item) => item.textContent?.trim() === '移动到...');
  assert.ok(moveItem, '移动到... renders in the row menu');
  act(() => { (moveItem as HTMLElement).click(); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });
  assert.equal(calls.moveTargets, 1, 'opening the picker loads the move targets');
  const bodyText = document.body.textContent ?? '';
  assert.ok(bodyText.includes('移动到知识库'), 'picker header renders the Vue copy');
  assert.ok(bodyText.includes('Wiki Parity Fixture'), 'target KB names render');
  assert.ok(bodyText.includes('Parity KB Demo'), 'second target KB renders');
});

test('selecting a target shows the Vue confirm panel and posts the move on 确认移动', async () => {
  const calls = { spans: 0, moveTargets: 0, move: [] as MoveCall[] };
  mountedRoot = await renderPage(menuClient(calls));
  const menu = openRowMenu();
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });
  const moveItem = [...menu.querySelectorAll('[role="menuitem"]')].find((item) => item.textContent?.trim() === '移动到...');
  assert.ok(moveItem);
  act(() => { (moveItem as HTMLElement).click(); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });
  // Vue move targets render as .card-menu-item rows (divs, not buttons).
  const target = [...document.querySelectorAll('.card-menu-item')].find((row) => row.textContent?.includes('Wiki Parity Fixture')) as HTMLElement | undefined;
  assert.ok(target, 'target row is clickable');
  act(() => { target.click(); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 10)); });
  const bodyText = document.body.textContent ?? '';
  assert.ok(bodyText.includes('确认移动设置'), 'confirm header renders');
  assert.ok(bodyText.includes('复用向量（快速）'), 'reuse-vectors mode renders');
  assert.ok(bodyText.includes('重新解析'), 'reparse mode renders');
  const confirm = [...document.querySelectorAll('button')].find((button) => button.textContent?.trim() === '确认移动');
  assert.ok(confirm, '确认移动 button renders');
  act(() => { confirm.click(); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 30)); });
  assert.deepEqual(calls.move, [
    { knowledge_ids: ['doc-failed'], source_kb_id: 'kb-1', target_kb_id: 'kb-2', mode: 'reuse_vectors' },
  ]);
  assert.ok((document.body.textContent ?? '').includes('移动任务已提交'), 'move started notice renders');
});

test('the hover popover shows the trace summary block with the total duration', async () => {
  const calls = { spans: 0, moveTargets: 0, move: [] as MoveCall[] };
  mountedRoot = await renderPage(menuClient(calls));
  const card = document.querySelector('.knowledge-card') as HTMLElement | null;
  assert.ok(card, 'document card renders in the grid view');
  await act(async () => {
    // React synthesizes onMouseEnter from the delegated mouseover/mouseout pair.
    card.dispatchEvent(new dom.window.MouseEvent('mouseover', { bubbles: true }));
    await new Promise((resolve) => setTimeout(resolve, 400));
  });
  const popover = document.querySelector('.knowledge-card-hover-popover') as HTMLElement | null;
  assert.ok(popover, 'hover popover mounted after the hover delay');
  const text = popover.textContent ?? '';
  assert.ok(text.includes('总耗时：17ms'), 'compact trace caption renders the total duration: ' + text);
  assert.ok(text.includes('更新：'), 'popover keeps the updated-at meta line');
  assert.ok(text.includes('点击卡片查看全文与分段'), 'popover keeps the click-to-view hint');
});

test.after(async () => {
  document.body.replaceChildren();
});
