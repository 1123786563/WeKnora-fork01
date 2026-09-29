// KBL-5/6/7（functional-sweep-2026-09-29 kb-library）修复回归锁：
//  - KBL-5 根目录网格不渲染子文件夹卡片（Vue currentChildFolders 规则）
//  - KBL-6 目录面板收起/展开（.kb-folder-tree__icon-btn / 面包屑展开按钮）
//  - KBL-7 导入网页弹窗 footer [取消|确认]（确认在右，对齐 Vue t-dialog）
import '../test-tdom-harness.ts';
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge/kb-1/documents' });
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'zh-CN' });
// 覆盖集必须完整（同 test-tdom-harness.ts）：tdesign useTrigger 用
// `element instanceof Element` 解析触发器 DOM，globals 撕裂会导致 Popup 原生
// click 监听不挂载（见 doc-row-menu-interaction.test.tsx 头注）。
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  SVGElement: dom.window.SVGElement,
  Event: dom.window.Event,
  KeyboardEvent: dom.window.KeyboardEvent,
  MouseEvent: dom.window.MouseEvent,
  CustomEvent: dom.window.CustomEvent,
  MutationObserver: dom.window.MutationObserver,
  IS_REACT_ACT_ENVIRONMENT: true,
  getComputedStyle: dom.window.getComputedStyle?.bind(dom.window),
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? clearTimeout,
});

const { createRoot } = await import('react-dom/client');
const { KnowledgeDocumentsPage } = await import('./KnowledgeDocumentsPage.tsx');

const rootDocument = {
  id: 'doc-root',
  file_name: 'root-doc.pdf',
  title: 'root-doc.pdf',
  type: 'file',
  source: '',
  file_type: 'pdf',
  parse_status: 'completed',
  summary_status: 'completed',
  folder_path: '',
  updated_at: '2026-09-19T02:35:00Z',
  created_at: '2026-09-18T10:00:00Z',
};

function folderTreeClient(): WeKnoraClient {
  const knowledgeBase = {
    id: 'kb-1',
    name: 'parity-kb',
    description: '',
    type: 'knowledge',
    created_at: '2030-01-01T00:00:00Z',
    my_permission: 'owner',
    // 有存储引擎：isStorageEngineMissing → false，导入网页弹窗才能打开
    storage_backend_id: 'sb-1',
  };
  const documents = {
    list: async () => ({ data: [rootDocument], total: 1, page: 1, page_size: 20 }),
    tagsPage: async () => ({ data: [], total: 0, page: 1, page_size: 50 }),
    tags: async () => [],
    // 一个子目录（Vue currentChildFolders 判定素材）：根目录网格不应渲染它
    folders: async () => ({
      folders: [{ path: '1-产品与服务详情', name: '1-产品与服务详情', document_count: 45, total_count: 45 }],
      root_document_count: 1,
      total_document_count: 46,
    }),
    spans: async () => null,
  };
  return {
    knowledgeBases: { settings: { get: async () => knowledgeBase, parserEngines: async () => ({ data: [] }) }, list: async () => [knowledgeBase], documents },
    knowledge: { documents },
    auth: { me: async () => ({ user: { id: 'u1', role: 'admin', is_superuser: true }, memberships: [{ tenant_id: 1, role: 'admin' }] }) },
    identity: { organizations: { knowledgeBaseShares: { listShared: async () => null } } },
    configuration: { models: { list: async () => [] } },
    settings: { system: { info: async () => ({}) } },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

import { afterEach } from 'node:test';
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.localStorage.removeItem('weknora.kbFolderTreeCollapsed');
  dom.window.localStorage.removeItem('weknora.kb.docs.viewMode');
});

async function renderPage(client: WeKnoraClient): Promise<Root> {
  const host = document.createElement('div');
  document.body.append(host);
  const root = createRoot(host) as Root;
  await act(async () => {
    root.render(React.createElement(KnowledgeDocumentsPage, { client, knowledgeBaseId: 'kb-1' }));
  });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });
  return root;
}

// jsdom 无布局计算：可见性断言退化为存在性（React 条件渲染已足够区分）。
test('KBL-5: root grid hides child folder cards while the folder tree is open', async () => {
  mountedRoot = await renderPage(folderTreeClient());
  // 目录树面板可见（有真实子目录才分配目录列）
  assert.ok([...document.querySelectorAll('.wk-folder-panel')].length > 0, 'folder panel is mounted');
  // 内容区不渲染文件夹卡片（与左侧目录树重复，Vue folderCards=0）
  assert.equal([...document.querySelectorAll('.folder-card')].length, 0, 'no folder cards in root grid');
  // 根目录直接文档仍以网格卡片渲染
  assert.ok([...document.querySelectorAll('.knowledge-card')].length > 0, 'root documents still render as cards');
});

test('KBL-6: 收起目录 collapses the panel and brings folder cards back, 展开目录 restores', async () => {
  mountedRoot = await renderPage(folderTreeClient());
  const collapseBtn = document.querySelector('.kb-folder-tree__icon-btn') as HTMLButtonElement | null;
  assert.ok(collapseBtn, 'collapse icon button (.kb-folder-tree__icon-btn) is mounted');
  assert.equal(collapseBtn.getAttribute('aria-label'), '收起目录');

  await act(async () => { collapseBtn.click(); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });
  assert.equal([...document.querySelectorAll('.wk-folder-panel')].length, 0, 'panel hidden after collapse');
  // Vue currentChildFolders：树收起后内容区重新显示子文件夹卡片（KBL-5 的对偶面）
  assert.equal([...document.querySelectorAll('.folder-card')].length, 1, 'folder card returns while collapsed');

  const expandToggle = document.querySelector('.doc-folder-path__tree-toggle') as HTMLButtonElement | null;
  assert.ok(expandToggle, 'breadcrumb expand toggle (.doc-folder-path__tree-toggle) appears');
  assert.equal(expandToggle.getAttribute('aria-label'), '展开目录');
  await act(async () => { expandToggle.click(); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });
  assert.ok([...document.querySelectorAll('.wk-folder-panel')].length > 0, 'panel restored after expand');
  assert.equal([...document.querySelectorAll('.folder-card')].length, 0, 'folder cards hidden again once the tree is back');
});

test('KBL-7: import-web dialog footer reads [取消, 确认] with confirm on the right', async () => {
  mountedRoot = await renderPage(folderTreeClient());
  const trigger = document.querySelector('.kb-upload-source-trigger') as HTMLButtonElement | null;
  assert.ok(trigger, 'add-document dropdown trigger is mounted');
  await act(async () => { trigger.click(); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });
  const items = [...document.querySelectorAll('.t-dropdown__item')] as HTMLElement[];
  const webItem = items.find((el) => (el.textContent || '').includes('导入网页'));
  assert.ok(webItem, '导入网页 menu item present in the dropdown');
  await act(async () => { webItem.click(); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });
  const actions = document.querySelector('.wk-list-actions');
  assert.ok(actions, 'import-web dialog footer (.wk-list-actions) mounted');
  const labels = [...actions.querySelectorAll('button')].map((b) => (b.textContent || '').trim()).filter(Boolean);
  // Vue KbUploadSourceDropdown t-dialog：cancel-btn 左、confirm-btn 右
  assert.deepEqual(labels, ['取消', '确认']);
});

// KBL-4：文档列表分页模型对齐 Vue——视口自适应页大小（jsdom innerHeight 768 →
// floor(768/148)*5=25 → 下限 35），无显式分页器，滚动到底追加下一页。
function scrollClient(listCalls: { page: number; page_size: number }[]): WeKnoraClient {
  const client = folderTreeClient();
  const all = Array.from({ length: 45 }, (_, n) => ({
    ...rootDocument,
    id: `doc-${n + 1}`,
    file_name: `doc-${n + 1}.pdf`,
    title: `doc-${n + 1}.pdf`,
  }));
  const list = async (_kbId: string, params?: { page?: number; page_size?: number }) => {
    const page = params?.page ?? 1;
    const pageSize = params?.page_size ?? 35;
    listCalls.push({ page, page_size: pageSize });
    return { data: all.slice((page - 1) * pageSize, page * pageSize), total: 45, page, page_size: pageSize };
  };
  const override = (api: unknown) => { (api as { list: unknown }).list = list; };
  override((client as unknown as { knowledgeBases: { documents: unknown } }).knowledgeBases.documents);
  override((client as unknown as { knowledge: { documents: unknown } }).knowledge.documents);
  return client;
}

test('KBL-4: no explicit paginator; viewport page size 35 loads, scroll appends to 45', async () => {
  const calls: { page: number; page_size: number }[] = [];
  mountedRoot = await renderPage(scrollClient(calls));
  assert.equal(document.querySelectorAll('.wk-pagination').length, 0, 'no explicit paginator (Vue infinite scroll)');
  assert.equal(calls[0].page, 1);
  assert.equal(calls[0].page_size, 35, 'first request uses the Vue viewport-adaptive page size');
  assert.equal(document.querySelectorAll('.knowledge-card').length, 35, 'first slice renders 35 cards');

  // 滚动到底：jsdom 无布局（scrollTop/scrollHeight/clientHeight 全 0），
  // scrollTop+clientHeight >= scrollHeight-10 恒成立，事件即触发加载。
  const container = document.querySelector('.doc-scroll-container') as HTMLDivElement | null;
  assert.ok(container, 'scroll container (.doc-scroll-container) mounted');
  await act(async () => { container.dispatchEvent(new dom.window.Event('scroll', { bubbles: true })); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });
  assert.deepEqual(calls[calls.length - 1], { page: 2, page_size: 35 }, 'bottom scroll requests the next page');
  assert.equal(document.querySelectorAll('.knowledge-card').length, 45, 'appended slice grows the grid to all 45 documents');
});

// KBL-R3：上传文档 input 带 accept 扩展名过滤（与 Vue acceptFileTypes 同源：
// parser engines FileTypes 并集）；folder input 与 Vue 一致不过滤。
test('KBL-R3: upload file input carries the Vue accept filter; folder input stays unfiltered', async () => {
  const client = folderTreeClient();
  const settings = (client as unknown as { knowledgeBases: { settings: { parserEngines: () => Promise<unknown> } } }).knowledgeBases.settings;
  settings.parserEngines = async () => ({
    data: [
      { Name: 'builtin', Available: true, FileTypes: ['pdf', 'docx', 'txt'] },
      { Name: 'ocr', Available: true, FileTypes: ['png', 'pdf'] },
    ],
  });
  mountedRoot = await renderPage(client);
  const fileInput = document.querySelector('input[data-upload-source-input="file"]') as HTMLInputElement | null;
  const folderInput = document.querySelector('input[data-upload-source-input="folder"]') as HTMLInputElement | null;
  assert.ok(fileInput && folderInput, 'both hidden chooser inputs mounted');
  assert.equal(fileInput.getAttribute('accept'), '.pdf,.docx,.txt,.png', 'file input accept mirrors Vue acceptFileTypes');
  assert.equal(folderInput.getAttribute('accept'), null, 'folder input unfiltered exactly like Vue');
});
