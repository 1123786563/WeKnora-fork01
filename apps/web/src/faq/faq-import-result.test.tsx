import '../test-tdom-harness.ts'; // jsdom 全局（tdesign 运行时；须首个 import）
// B6: Vue last-result persistence (FAQEntryManager.vue:1263-1303 state/summary,
// :2111-2150 save+load after import success, :2276-2412 storage/close/download,
// :2820-2829 mount-time restore; API knowledge-base/index.ts:602-610).
// The result strip survives reloads until the user closes it (PUT
// last-result/display 'close') or the next import replaces it.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => /\.(css|png|jpe?g|svg|gif|webp)$/.test(specifier)
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });

const require = nodeModule.createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge-bases/kb-1/faq' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});

const { createRoot } = await import('react-dom/client');
const {
  faqLastCompletedTaskKey, saveLastCompletedTaskId, getLastCompletedTaskId,
  faqImportResultFromProgress, faqImportResultSummary, faqImportResultVisible, formatImportTime,
  FAQPage,
} = await import('./FAQPage.tsx');
const { createTranslator } = await import('../i18n.ts');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

const settle = (ms: number) => act(async () => { await new Promise((resolve) => setTimeout(resolve, ms)); });

// --- Vue getLastCompletedTaskKey / save / load (:2276-2296) -------------------------

test('faqLastCompletedTaskKey mirrors the Vue localStorage key per KB', () => {
  assert.equal(faqLastCompletedTaskKey('kb-9'), 'faq_import_last_completed_kb-9');
});

test('save + get round-trip the last completed task id and survive storage failure', () => {
  saveLastCompletedTaskId('kb-1', 'task-42');
  assert.equal(getLastCompletedTaskId('kb-1'), 'task-42');
  assert.equal(getLastCompletedTaskId('kb-other'), null, 'scoped per KB');
  assert.equal(getLastCompletedTaskId(''), null, 'Vue guards an empty kbId');
});

// --- Vue loadImportResult mapping (:2299-2342) --------------------------------------

test('faqImportResultFromProgress accepts only completed + open results', () => {
  const base = { status: 'completed', total: 5, success_count: 4, failed_count: 1, display_status: 'open', task_id: 't1', import_mode: 'append' };
  const view = faqImportResultFromProgress(base);
  assert.ok(view, 'completed + open maps');
  assert.equal(view!.total_entries, 5);
  assert.equal(view!.display_status, 'open');
  assert.equal(faqImportResultFromProgress({ ...base, status: 'processing' }), null, 'in-flight task is not a result');
  assert.equal(faqImportResultFromProgress({ ...base, display_status: 'close' }), null, 'Vue hides closed results (:2313)');
  assert.equal(faqImportResultFromProgress(null), null);
  assert.equal(faqImportResultFromProgress({}), null, 'missing status rejected');
  assert.equal(faqImportResultFromProgress(undefined), null, 'missing payload rejected');
});

test('faqImportResultFromProgress defaults counts to 0 and mode to append', () => {
  const view = faqImportResultFromProgress({ status: 'completed', total: 3, task_id: 't2' });
  assert.ok(view);
  assert.equal(view!.success_count, 0);
  assert.equal(view!.merged_count, 0);
  assert.equal(view!.import_mode, 'append');
});

// --- Vue importResultSummary (:1277-1303) -------------------------------------------

const t = createTranslator('zh-CN');
const result = (over: Record<string, unknown>) => faqImportResultFromProgress({ status: 'completed', total: 0, task_id: 't', ...over })!;

test('summary prefers the backend message when present (Vue :1280-1282)', () => {
  assert.equal(faqImportResultSummary(result({ message: ' 导入完成 ' }), t), '导入完成');
});

test('summary joins total/merged/added/partial/failed/skipped with · like Vue', () => {
  assert.equal(
    faqImportResultSummary(result({ total: 10, merged_count: 3, added_count: 2, partial_failed_count: 1, failed_count: 4, skipped_count: 1 }), t),
    [`导入数据 10`, `新增 2`, `合并更新 3`, `部分失败 1`, `失败 4`, `跳过 1`].join(' · '),
  );
  assert.equal(faqImportResultSummary(result({ total: 6, success_count: 6 }), t), '导入数据 6 · 成功 6', 'success branch when nothing merged');
  assert.equal(faqImportResultSummary(result({ total: 0 }), t), '导入数据 0', 'total-only floor');
});

// --- Vue showImportResultBadge (:1266-1270) ------------------------------------------

test('result visibility: open + no active task (Vue showImportResultBadge)', () => {
  const open = result({});
  assert.equal(faqImportResultVisible(open, false), true);
  assert.equal(faqImportResultVisible(open, true), false, 'hidden while an import task runs');
  assert.equal(faqImportResultVisible({ ...open, display_status: 'close' }, false), false);
  assert.equal(faqImportResultVisible(null, false), false);
});

// --- Vue formatImportTime (:2369-2377) -----------------------------------------------

test('formatImportTime renders YYYY-MM-DD HH:mm in local time', () => {
  const iso = '2026-09-14T08:30:00Z';
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, '0');
  const expected = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  assert.equal(formatImportTime(iso), expected);
  assert.equal(formatImportTime(''), '');
  assert.equal(formatImportTime(undefined), '');
  assert.equal(formatImportTime('not-a-date'), 'not-a-date', 'invalid input passes through');
});

// --- integration: mount restores the persisted strip; close persists -----------------

function fakeClient(options: { progressData?: Record<string, unknown>; onPut?: (path: string, body: unknown) => void }) {
  const calls: Array<{ method: string; path: string; body?: unknown }> = [];
  return {
    calls,
    client: {
      knowledgeBases: {
        settings: { get: async () => ({ id: 'kb-1', name: 'KB One', type: 'faq' }) },
        list: async () => [],
      },
      knowledge: {
        documents: { tags: async () => [] },
        faq: {
          list: async () => ({ data: [], total: 0, page: 1, page_size: 20 }),
          importProgress: async () => ({ task_id: 'task-x', kb_id: 'kb-1', status: 'processing', progress: 1, total: 1, processed: 0 }),
        },
      },
      auth: { me: async () => ({ user: { id: 'u1', roles: [], memberships: [] }, membership: { role: 'owner' }, can_access_all_tenants: false }) },
      identity: { organizations: { knowledgeBaseShares: { listShared: async () => [] } } },
      request: async (input: { method: string; path: string; body?: unknown }) => {
        calls.push({ ...input });
        if (input.method === 'PUT') { options.onPut?.(input.path, input.body); return { success: true }; }
        return { success: true, data: options.progressData ?? null };
      },
    },
  };
}

async function mountPage(client: Record<string, unknown>) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => { mountedRoot?.render(React.createElement(FAQPage, { client: client as never, knowledgeBaseId: 'kb-1' })); });
  await settle(30);
  return container;
}

test('mount restores the persisted result strip from the last completed task', async () => {
  saveLastCompletedTaskId('kb-1', 'task-9');
  const fake = fakeClient({ progressData: { status: 'completed', display_status: 'open', total: 5, success_count: 4, failed_count: 1, merged_count: 1, added_count: 4, import_mode: 'append', imported_at: '2026-09-14T08:30:00Z', task_id: 'task-9', failed_entries_url: '/downloads/task-9.csv' } });
  await mountPage(fake.client);
  const strip = document.querySelector('.faq-import-strip--result') as HTMLElement | null;
  assert.ok(strip, 'persistent result strip rendered on mount');
  assert.match(strip.textContent || '', /导入数据 5/, 'summary present');
  assert.match(strip.textContent || '', /新增 4/, 'added count present (merged branch, Vue :1285-1289)');
  assert.match(strip.textContent || '', /合并更新 1/, 'merged count present');
  assert.match(strip.textContent || '', /失败 1/, 'failed count present');
  assert.ok(strip.querySelector('.faq-import-strip__close'), 'close button present');
  assert.ok(strip.textContent?.includes('下载原因'), 'download link for failed entries');
  assert.match(strip.textContent || '', /追加模式/, 'append mode tag (Vue t-tag)');
  await act(async () => { (strip.querySelector('.faq-import-strip__close') as HTMLButtonElement).click(); });
  await settle(10);
  const put = fake.calls.find((c) => c.method === 'PUT');
  assert.ok(put, 'close hits the last-result display endpoint');
  assert.match(put.path, /\/knowledge-bases\/kb-1\/faq\/import\/last-result\/display$/);
  assert.deepEqual(put.body, { display_status: 'close' });
  assert.equal(document.querySelector('.faq-import-strip--result'), null, 'strip gone after close');
  assert.equal(getLastCompletedTaskId('kb-1'), 'task-9', 'close does not clear the storage key (Vue keeps it)');
});

test('a closed display_status stays hidden across reloads', async () => {
  saveLastCompletedTaskId('kb-1', 'task-9');
  const fake = fakeClient({ progressData: { status: 'completed', display_status: 'close', total: 5, task_id: 'task-9' } });
  await mountPage(fake.client);
  assert.equal(document.querySelector('.faq-import-strip--result'), null, 'Vue :2313 — closed results do not reappear');
});

test('an in-flight stored task does not render the result strip', async () => {
  saveLastCompletedTaskId('kb-1', 'task-9');
  const fake = fakeClient({ progressData: { status: 'processing', progress: 40, total: 5, processed: 2, task_id: 'task-9' } });
  await mountPage(fake.client);
  assert.equal(document.querySelector('.faq-import-strip--result'), null, 'only completed tasks surface the strip');
});
