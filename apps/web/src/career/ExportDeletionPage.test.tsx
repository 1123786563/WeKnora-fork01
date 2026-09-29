import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import * as nodeModule from 'node:module'
import test, { afterEach } from 'node:test'
import * as React from 'react'
import { act } from 'react'
import type { Root } from 'react-dom/client'
import { createScopeController } from '@weknora/domain/scope'
import type { WeKnoraClient } from '@weknora/api-client'
import type { CareerDeletionBoundaryView, CareerDeletionReceipt, CareerExportReceipt } from '../../../../packages/api-client/src/career.ts'

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void }
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) })

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } }
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/career' })
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Element: dom.window.Element, MutationObserver: dom.window.MutationObserver, getComputedStyle: dom.window.getComputedStyle.bind(dom.window), Event: dom.window.Event, IS_REACT_ACT_ENVIRONMENT: true })
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator })
const { createRoot } = await import('react-dom/client')
const { ExportDeletionPage } = await import('./ExportDeletionPage.tsx')

const ts = '2026-09-25T10:30:00Z'
const digest = 'a'.repeat(64)
const exportedFact = (key: string) => ({ key, value: `${key}值`, revision: 2, source: { kind: 'user', label: '本人确认' }, confirmation: { userId: 'owner-1', confirmedAt: ts }, confirmedAt: ts })
const exportReceipt = (extra: Partial<CareerExportReceipt> = {}): CareerExportReceipt => ({
 kind: 'career_exported', requestId: 'exp-req-1', exportId: 'exp-1', revision: 4, status: 'complete', digest, createdAt: ts,
 archive: {
  profile: { revision: 4, facts: [exportedFact('学历')], proposals: [] },
  factHistory: [exportedFact('学历'), exportedFact('毕业时间')],
  opportunities: [{ opportunityId: 'opp-1', snapshots: [{ snapshotId: 'snap-1', status: 'needs_review', rawText: 'JD 原文', acquiredAt: ts }] }],
  applications: [{ applicationId: 'app-1', opportunityId: 'opp-1', snapshotId: 'snap-1', batchIdentity: 'batch-1', taskId: 'task-9', progressEvents: [{ eventId: 'evt-1', applicationId: 'app-1', seq: 1, eventType: 'submitted', note: '官网已投', occurredAt: ts, source: { kind: 'user' }, confirmer: 'owner-1' }] }],
  materials: [{ materialId: 'mat-1', opportunityId: 'opp-1', status: 'confirmed', versions: [{ version: 3, requestId: 'confirm-1', versionBody: '正文', createdAt: ts }] }],
  submissions: [{ submissionId: 'sub-1', applicationId: 'app-1', channel: 'web', occurredAt: ts, versionConfirmed: true, materialId: 'mat-1', exportId: 'exp-1', version: 3, contentDigest: 'b'.repeat(64), note: '官网已投', confirmer: 'owner-1', createdAt: ts }],
 }, ...extra,
})
const boundaryView = (): CareerDeletionBoundaryView => ({
 inSpace: [
  { section: 'profile', description: '已确认的档案事实与待处理提案', count: 2 },
  { section: 'materials', description: '材料草稿', count: 1 },
 ],
 external: [{ item: 'external_platform_submissions', description: '你在外部招聘平台完成的投递、沟通与账号操作不在本空间控制范围内，本系统无法撤回或修改。', revocable: false }],
 retention: [{ holder: 'career_data_deletions', reason: '删除审计与可恢复状态（法定/技术保留）', status: 'retained' }],
})
const deletionSteps = () => ([
 { name: 'revoke_material_exports' as const, status: 'done' as const },
 { name: 'purge_career_data' as const, status: 'done' as const },
 { name: 'remove_workbench_tasks' as const, status: 'done' as const },
 { name: 'finalize' as const, status: 'done' as const },
])
const deletionReceipt = (extra: Partial<CareerDeletionReceipt> = {}): CareerDeletionReceipt => ({
 kind: 'career_deleted', requestId: 'del-req-1', status: 'deleted', steps: deletionSteps(),
 retention: [{ holder: 'career_data_deletions', reason: '删除审计与可恢复状态（法定/技术保留）', status: 'retained' }],
 revision: 5, startedAt: ts, completedAt: ts, ...extra,
})

type CareerStub = Record<string, (...args: any[]) => unknown>
let root: Root | undefined
let host: HTMLDivElement | undefined
afterEach(async () => {
 if (root) await act(async () => root?.unmount())
 root = undefined; host?.remove(); host = undefined; document.body.replaceChildren()
 window.history.replaceState({}, '', '/platform/career')
})
function render(element: React.ReactNode): HTMLDivElement {
 host = document.createElement('div'); document.body.append(host); root = createRoot(host); act(() => root!.render(element)); return host
}
async function settle() { await new Promise((resolve) => setImmediate(resolve)) }
function byLabel(container: HTMLElement, selector: string, label: string): HTMLElement {
 const found = [...container.querySelectorAll<HTMLElement>(selector)].find((item) => item.textContent?.trim() === label)
 assert.ok(found, `${selector} “${label}” exists`)
 return found
}
function click(el: HTMLElement) { act(() => { el.dispatchEvent(new dom.window.Event('click', { bubbles: true })) }) }
function toggle(checkbox: HTMLInputElement) {
 const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(checkbox), 'checked')?.set
 setter?.call(checkbox, !checkbox.checked)
 act(() => { checkbox.dispatchEvent(new dom.window.Event('click', { bubbles: true })) })
}

async function mountLifecycle(career: CareerStub, options: { onCareerDeleted?: () => void } = {}) {
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 const deleted: number[] = []
 const container = render(React.createElement(ExportDeletionPage, { client: { career } as unknown as WeKnoraClient, scopeController, onCareerDeleted: () => { deleted.push(deleted.length); options.onCareerDeleted?.() } }))
 await act(async () => { await settle(); await settle() })
 return { container, deleted }
}

test('the page exports the whole space, presents every frozen package section and downloads it as a file', async () => {
 const sent: unknown[] = []
 const downloads: Blob[] = []
 Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: (blob: Blob) => { downloads.push(blob); return 'blob:mock' } })
 Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: () => { } })
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  exportCareer: async (input: Record<string, unknown>) => { sent.push(input); return exportReceipt({ requestId: input.requestId as string }) },
  careerDeletionBoundary: async () => boundaryView(),
  deleteCareer: async () => { throw new Error('must not delete in export test') },
 }
 const { container } = await mountLifecycle(career)
 assert.match(container.textContent ?? '', /当前档案修订 4/)
 await act(async () => { click(byLabel(container, 'button', '发起导出')); await settle(); await settle() })
 assert.equal(sent.length, 1)
 assert.equal((sent[0] as Record<string, unknown>).expectedRevision, 4)
 assert.ok((sent[0] as Record<string, unknown>).requestId)
 const panel = container.querySelector('[aria-label="导出包内容"]')
 assert.ok(panel, 'export package panel exists')
 const text = panel?.textContent ?? ''
 assert.match(text, /exp-1/)
 assert.match(text, new RegExp(digest))
 // Inclusive presentation: profile, fact history, original job snapshots,
 // application events, material versions and submission records.
 assert.match(text, /档案事实：1 条/)
 assert.match(text, /事实历史：2 条/)
 assert.match(text, /1 个岗位 \/ 1 份快照/)
 assert.match(text, /1 个申请 \/ 1 条事件/)
 assert.match(text, /1 份材料 \/ 1 个版本/)
 assert.match(text, /投递记录：1 条/)
 await act(async () => { click(byLabel(container, 'button', '下载导出包')); await settle() })
 assert.equal(downloads.length, 1)
 const body = JSON.parse(await downloads[0]!.text())
 assert.equal(body.kind, 'career_exported')
 assert.equal(body.archive.applications.length, 1)
 assert.equal(body.archive.materials[0].versions.length, 1)
})

test('an uncertain export outcome keeps the original request id for receipt lookup and same-id retry', async () => {
 const sent: unknown[] = []
 let failFirst = true
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  exportCareer: async (input: Record<string, unknown>) => { sent.push(input); if (failFirst) { failFirst = false; throw Object.assign(new Error('timed out'), { code: 'TIMEOUT' }) } return exportReceipt({ requestId: input.requestId as string }) },
  careerExportReceipt: async (requestId: string) => { throw Object.assign(new Error('not found'), { code: 'not_found' }) },
  careerDeletionBoundary: async () => boundaryView(),
 }
 const { container } = await mountLifecycle(career)
 await act(async () => { click(byLabel(container, 'button', '发起导出')); await settle(); await settle() })
 assert.match(container.textContent ?? '', /暂时无法确认导出/)
 assert.match(container.textContent ?? '', /原请求编号/)
 await act(async () => { click(byLabel(container, 'button', '查询导出回执')); await settle(); await settle() })
 assert.match(container.textContent ?? '', /尚未找到导出回执/)
 await act(async () => { click(byLabel(container, 'button', '用原请求编号重试')); await settle(); await settle() })
 assert.equal(sent.length, 2)
 assert.equal((sent[0] as Record<string, unknown>).requestId, (sent[1] as Record<string, unknown>).requestId)
 assert.ok(container.querySelector('[aria-label="导出包内容"]'), 'receipt accepted after retry')
})

test('deletion first presents the in-space vs external boundary and requires explicit acknowledgement', async () => {
 const sent: unknown[] = []
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  careerDeletionBoundary: async () => { sent.push('boundary'); return boundaryView() },
  deleteCareer: async () => { throw new Error('must not delete before acknowledgement') },
 }
 const { container } = await mountLifecycle(career)
 assert.equal(container.querySelector('[aria-label="删除边界清单"]'), null, 'boundary is not fabricated client-side')
 const start = byLabel(container, 'button', '发起完整删除')
 assert.equal(start.hasAttribute('disabled'), true, 'deletion is blocked before the boundary is presented')
 await act(async () => { click(byLabel(container, 'button', '查看删除边界')); await settle(); await settle() })
 assert.equal(sent.length, 1)
 const panel = container.querySelector('[aria-label="删除边界清单"]')
 assert.ok(panel, 'boundary panel exists')
 const text = panel?.textContent ?? ''
 assert.match(text, /空间内将删除/)
 assert.match(text, /profile（已确认的档案事实与待处理提案）：2 项/)
 assert.match(text, /外部平台资料（不可撤回）/)
 assert.match(text, /external_platform_submissions/)
 assert.match(text, /不可撤回/)
 assert.match(text, /保留范围/)
 assert.match(text, /career_data_deletions/)
 assert.equal(start.hasAttribute('disabled'), true, 'deletion still blocked until acknowledged')
 const acknowledge = container.querySelector<HTMLInputElement>('input[type="checkbox"]')
 assert.ok(acknowledge)
 await act(async () => { toggle(acknowledge!); await settle() })
 assert.equal(start.hasAttribute('disabled'), false, 'deletion unblocked after explicit acknowledgement')
})

test('a partial deletion keeps a recoverable state, discloses retention and never claims complete deletion', async () => {
 const sent: unknown[] = []
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  careerDeletionBoundary: async () => boundaryView(),
  deleteCareer: async (input: Record<string, unknown>) => { sent.push(input); return deletionReceipt({ requestId: input.requestId as string, status: 'partial', completedAt: undefined, steps: deletionSteps().map((step, index) => index === 2 ? { ...step, status: 'failed' as const, detail: 'workbench remover unavailable' } : step) }) },
 }
 const { container } = await mountLifecycle(career)
 await act(async () => { click(byLabel(container, 'button', '查看删除边界')); await settle(); await settle() })
 await act(async () => { toggle(container.querySelector<HTMLInputElement>('input[type="checkbox"]')!); await settle() })
 await act(async () => { click(byLabel(container, 'button', '发起完整删除')); await settle(); await settle() })
 assert.equal(sent.length, 1)
 assert.equal((sent[0] as Record<string, unknown>).expectedRevision, 4)
 const receipt = container.querySelector('[aria-label="删除结果"]')
 assert.ok(receipt)
 const text = receipt?.textContent ?? ''
 assert.match(text, /部分失败/)
 assert.match(text, /未完全删除/)
 assert.match(text, /移除 Workbench 申请任务投影/)
 assert.match(text, /失败/)
 assert.match(text, /workbench remover unavailable/)
 assert.doesNotMatch(text, /已完全删除/)
 assert.match(text, /保留范围/, 'retention is disclosed on partial receipts too')
 await act(async () => { click(byLabel(container, 'button', '用原请求编号重试删除')); await settle(); await settle() })
 assert.equal(sent.length, 2, 'partial deletion resumes under the same request id')
 assert.equal((sent[0] as Record<string, unknown>).requestId, (sent[1] as Record<string, unknown>).requestId)
})

test('a completed deletion discloses retention, verifies the old export grant is unreadable and clears caches', async () => {
 const exports: string[] = []
 const deletionRequests: string[] = []
 const probed: string[] = []
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  exportCareer: async (input: Record<string, unknown>) => { exports.push(input.requestId as string); return exportReceipt({ requestId: input.requestId as string }) },
  careerExportReceipt: async (requestId: string) => { probed.push(requestId); throw Object.assign(new Error('receipt gone'), { code: 'not_found' }) },
  careerDeletionBoundary: async () => boundaryView(),
  deleteCareer: async (input: Record<string, unknown>) => { deletionRequests.push(input.requestId as string); return deletionReceipt({ requestId: input.requestId as string }) },
 }
 const { container, deleted } = await mountLifecycle(career)
 await act(async () => { click(byLabel(container, 'button', '发起导出')); await settle(); await settle() })
 await act(async () => { click(byLabel(container, 'button', '查看删除边界')); await settle(); await settle() })
 await act(async () => { toggle(container.querySelector<HTMLInputElement>('input[type="checkbox"]')!); await settle() })
 await act(async () => { click(byLabel(container, 'button', '发起完整删除')); await settle(); await settle(); await settle() })
 const receipt = container.querySelector('[aria-label="删除结果"]')
 const text = receipt?.textContent ?? ''
 assert.match(text, /已完全删除/)
 assert.match(text, /落定删除/)
 assert.match(text, /保留范围/)
 assert.equal(deleted.length, 1, 'client caches are cleared through onCareerDeleted')
 assert.deepEqual(probed, [exports[0]], 'the pre-deletion export receipt is re-read to prove old grants are dead')
 assert.match(container.textContent ?? '', /旧授权与旧入口已失效/)
 assert.match(container.textContent ?? '', new RegExp(exports[0]!))
})

test('confirmed deletion immediately removes mounted archive controls and preserves only its receipt', async () => {
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  exportCareer: async (input: Record<string, unknown>) => exportReceipt({ requestId: input.requestId as string }),
  careerExportReceipt: async () => { throw Object.assign(new Error('receipt gone'), { code: 'not_found' }) },
  careerDeletionBoundary: async () => boundaryView(),
  deleteCareer: async (input: Record<string, unknown>) => deletionReceipt({ requestId: input.requestId as string }),
 }
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 let generation = 0
 const client = { career } as unknown as WeKnoraClient
 function TestHarness() {
  const [currentGeneration, setCurrentGeneration] = React.useState(0)
  return React.createElement(ExportDeletionPage, { client, scopeController, deletionGeneration: currentGeneration, onCareerDeleted: () => setCurrentGeneration((value) => value + 1) })
 }
 const container = render(React.createElement(TestHarness))
 await act(async () => { await settle(); await settle() })
 await act(async () => { click(byLabel(container, 'button', '发起导出')); await settle(); await settle() })
 assert.ok(container.querySelector('[aria-label="导出包内容"]'))
 await act(async () => { click(byLabel(container, 'button', '查看删除边界')); await settle(); await settle() })
 await act(async () => { toggle(container.querySelector<HTMLInputElement>('input[type="checkbox"]')!); await settle() })
 await act(async () => { click(byLabel(container, 'button', '发起完整删除')); await settle(); await settle(); await settle() })
 assert.equal(container.querySelector('[aria-label="导出包内容"]'), null)
 assert.equal(container.querySelector('[aria-label="删除边界清单"]'), null)
 assert.equal([...container.querySelectorAll('button')].some((item) => item.textContent?.trim() === '下载导出包'), false)
 assert.match(container.querySelector('[aria-label="删除结果"]')?.textContent ?? '', /已完全删除/)
 assert.match(container.textContent ?? '', /保留范围/)
})

test('revision conflicts on deletion surface the current revision and a re-read affordance', async () => {
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  careerDeletionBoundary: async () => boundaryView(),
  deleteCareer: async () => { throw Object.assign(new Error('space moved on'), { code: 'revision_conflict', currentRevision: 6 }) },
 }
 const { container } = await mountLifecycle(career)
 await act(async () => { click(byLabel(container, 'button', '查看删除边界')); await settle(); await settle() })
 await act(async () => { toggle(container.querySelector<HTMLInputElement>('input[type="checkbox"]')!); await settle() })
 await act(async () => { click(byLabel(container, 'button', '发起完整删除')); await settle(); await settle() })
 assert.match(container.textContent ?? '', /档案已更新/)
 assert.match(container.textContent ?? '', /当前修订 6/)
 await act(async () => { click(byLabel(container, 'button', '重新读取档案修订')); await settle(); await settle() })
 assert.match(container.textContent ?? '', /当前档案修订 4/)
})

test('cross-tenant forbidden responses replace the panel with an access notice', async () => {
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  exportCareer: async () => { throw Object.assign(new Error('other tenant'), { code: 'forbidden' }) },
  careerDeletionBoundary: async () => boundaryView(),
 }
 const { container } = await mountLifecycle(career)
 await act(async () => { click(byLabel(container, 'button', '发起导出')); await settle(); await settle() })
 assert.match(container.textContent ?? '', /当前空间不可访问/)
 assert.equal([...container.querySelectorAll('button')].some((item) => item.textContent?.trim() === '发起导出'), false)
 assert.equal([...container.querySelectorAll('button')].some((item) => item.textContent?.trim() === '发起完整删除'), false)
})

test('switching scope resets lifecycle private state and re-reads the new scope revision', async () => {
 let reads = 0
 const career: CareerStub = {
  open: async () => ({ revision: ++reads === 1 ? 4 : 9 }),
  exportCareer: async (input: Record<string, unknown>) => exportReceipt({ requestId: input.requestId as string }),
  careerExportReceipt: async () => { throw Object.assign(new Error('gone'), { code: 'not_found' }) },
  careerDeletionBoundary: async () => boundaryView(),
  deleteCareer: async (input: Record<string, unknown>) => deletionReceipt({ requestId: input.requestId as string }),
 }
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 const container = render(React.createElement(ExportDeletionPage, { client: { career } as unknown as WeKnoraClient, scopeController }))
 await act(async () => { await settle(); await settle() })
 await act(async () => { click(byLabel(container, 'button', '发起导出')); await settle(); await settle() })
 await act(async () => { click(byLabel(container, 'button', '查看删除边界')); await settle(); await settle() })
 await act(async () => { toggle(container.querySelector<HTMLInputElement>('input[type="checkbox"]')!); await settle() })
 await act(async () => { click(byLabel(container, 'button', '发起完整删除')); await settle(); await settle(); await settle() })
 assert.match(container.textContent ?? '', /旧授权与旧入口已失效/)
 await act(async () => { scopeController.switchScope('https://weknora.test', 'owner-2', 'tenant-2'); await settle(); await settle() })
 assert.equal(reads, 2)
 assert.match(container.textContent ?? '', /空间已切换或登录已失效/)
 assert.doesNotMatch(container.textContent ?? '', /旧授权与旧入口已失效/)
 assert.equal(container.querySelector('[aria-label="导出包内容"]'), null)
 assert.equal(container.querySelector('[aria-label="删除结果"]'), null)
})

test('scope change can recover by reloading the new scope revision and deletion boundary', async () => {
 const opened: number[] = []
 const exports: Array<Record<string, unknown>> = []
 let boundaries = 0
 const career: CareerStub = {
  open: async () => ({ revision: ++opened.length === 1 ? 4 : 9 }),
  careerDeletionBoundary: async () => { boundaries++; return boundaryView() },
  exportCareer: async (input: Record<string, unknown>) => { exports.push(input); return exportReceipt({ requestId: input.requestId as string }) },
 }
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 const container = render(React.createElement(ExportDeletionPage, { client: { career } as unknown as WeKnoraClient, scopeController }))
 await act(async () => { await settle(); await settle() })
 await act(async () => { scopeController.switchScope('https://weknora.test', 'owner-2', 'tenant-2'); await settle(); await settle() })
 assert.match(container.textContent ?? '', /空间已切换或登录已失效/)
 await act(async () => { click(byLabel(container, 'button', '重新加载当前空间')); await settle(); await settle() })
 assert.equal(boundaries, 1)
 assert.match(container.textContent ?? '', /当前档案修订 9/)
 await act(async () => { click(byLabel(container, 'button', '发起导出')); await settle(); await settle() })
 assert.equal(exports.length, 1)
 assert.equal(exports[0]?.expectedRevision, 9)
})

test('a deleting receipt keeps a query action and accepts its terminal status for the same request id', async () => {
 const ids: string[] = []
 let writes = 0
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  careerDeletionBoundary: async () => boundaryView(),
  deleteCareer: async (input: Record<string, unknown>) => { writes++; ids.push(input.requestId as string); return deletionReceipt({ requestId: input.requestId as string, status: 'deleting', completedAt: undefined, steps: deletionSteps().map((step) => ({ ...step, status: 'pending' as const })) }) },
  careerDeletionReceipt: async (requestId: string) => { ids.push(requestId); return deletionReceipt({ requestId }) },
 }
 const { container } = await mountLifecycle(career)
 await act(async () => { click(byLabel(container, 'button', '查看删除边界')); await settle(); await settle() })
 await act(async () => { toggle(container.querySelector<HTMLInputElement>('input[type="checkbox"]')!); await settle() })
 await act(async () => { click(byLabel(container, 'button', '发起完整删除')); await settle(); await settle() })
 assert.equal(writes, 1)
 assert.match(container.textContent ?? '', /删除仍在进行中/)
 assert.equal(byLabel(container, 'button', '发起完整删除').hasAttribute('disabled'), true, 'an in-progress deletion cannot be restarted with a new request id')
 assert.equal([...container.querySelectorAll('button')].some((button) => button.textContent?.trim() === '用原请求编号重试'), false)
 await act(async () => { click(byLabel(container, 'button', '查询删除回执')); await settle(); await settle() })
 assert.equal(ids.length, 2)
 assert.equal(ids[0], ids[1])
 assert.match(container.textContent ?? '', /已完全删除/)
})

test('mismatched export receipt lookup is deterministic and clears the recovery attempt', async () => {
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  exportCareer: async () => { throw Object.assign(new Error('timeout'), { code: 'TIMEOUT' }) },
  careerExportReceipt: async () => exportReceipt({ requestId: 'alien-export-request' }),
 }
 const { container } = await mountLifecycle(career)
 await act(async () => { click(byLabel(container, 'button', '发起导出')); await settle(); await settle() })
 await act(async () => { click(byLabel(container, 'button', '查询导出回执')); await settle(); await settle() })
 assert.match(container.textContent ?? '', /导出回执与本次请求不匹配/)
 assert.equal([...container.querySelectorAll('button')].some((button) => button.textContent?.trim() === '查询导出回执'), false)
 assert.equal([...container.querySelectorAll('button')].some((button) => button.textContent?.trim() === '用原请求编号重试'), false)
})

test('mismatched deletion receipt lookup is deterministic and clears the recovery attempt', async () => {
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  careerDeletionBoundary: async () => boundaryView(),
  deleteCareer: async () => { throw Object.assign(new Error('timeout'), { code: 'TIMEOUT' }) },
  careerDeletionReceipt: async () => deletionReceipt({ requestId: 'alien-delete-request' }),
 }
 const { container } = await mountLifecycle(career)
 await act(async () => { click(byLabel(container, 'button', '查看删除边界')); await settle(); await settle() })
 await act(async () => { toggle(container.querySelector<HTMLInputElement>('input[type="checkbox"]')!); await settle() })
 await act(async () => { click(byLabel(container, 'button', '发起完整删除')); await settle(); await settle() })
 await act(async () => { click(byLabel(container, 'button', '查询删除回执')); await settle(); await settle() })
 assert.match(container.textContent ?? '', /删除回执与本次请求不匹配/)
 assert.equal([...container.querySelectorAll('button')].some((button) => button.textContent?.trim() === '查询删除回执'), false)
 assert.equal([...container.querySelectorAll('button')].some((button) => button.textContent?.trim() === '用原请求编号重试'), false)
})

test('a mismatched lookup after deleting keeps the original request quarantined for requery', async () => {
 const ids: string[] = []
 let lookup = 0
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  careerDeletionBoundary: async () => boundaryView(),
  deleteCareer: async (input: Record<string, unknown>) => { ids.push(input.requestId as string); return deletionReceipt({ requestId: input.requestId as string, status: 'deleting', completedAt: undefined, steps: deletionSteps().map((step) => ({ ...step, status: 'pending' as const })) }) },
  careerDeletionReceipt: async (requestId: string) => {
   ids.push(requestId)
   if (++lookup === 1) return deletionReceipt({ requestId: 'alien-delete-request' })
   return deletionReceipt({ requestId })
  },
 }
 const { container } = await mountLifecycle(career)
 await act(async () => { click(byLabel(container, 'button', '查看删除边界')); await settle(); await settle() })
 await act(async () => { toggle(container.querySelector<HTMLInputElement>('input[type="checkbox"]')!); await settle() })
 await act(async () => { click(byLabel(container, 'button', '发起完整删除')); await settle(); await settle() })
 const originalId = ids[0]
 assert.ok(originalId)
 await act(async () => { click(byLabel(container, 'button', '查询删除回执')); await settle(); await settle() })
 assert.match(container.textContent ?? '', /删除回执与本次请求不匹配/)
 assert.equal(byLabel(container, 'button', '发起完整删除').hasAttribute('disabled'), true, 'mismatch cannot unlock a new deletion')
 assert.ok(container.querySelector('[aria-label="恢复删除写入"]'), 'original attempt remains available only for reconciliation')
 await act(async () => { click(byLabel(container, 'button', '查询删除回执')); await settle(); await settle() })
 assert.deepEqual(ids, [originalId, originalId, originalId], 'the write and every lookup use the quarantined original ID')
 assert.match(container.textContent ?? '', /已完全删除/)
})

test('a late automatic same-scope revision read cannot overwrite recovery or the next export revision', async () => {
 let resolveInitialNewScope!: (value: { revision: number }) => void
 let opened = 0
 const exports: Array<Record<string, unknown>> = []
 const career: CareerStub = {
  open: async () => {
   opened++
   if (opened === 1) return { revision: 4 }
   if (opened === 2) return await new Promise((resolve) => { resolveInitialNewScope = resolve })
   return { revision: 9 }
  },
  careerDeletionBoundary: async () => boundaryView(),
  exportCareer: async (input: Record<string, unknown>) => { exports.push(input); return exportReceipt({ requestId: input.requestId as string }) },
 }
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 const container = render(React.createElement(ExportDeletionPage, { client: { career } as unknown as WeKnoraClient, scopeController }))
 await act(async () => { await settle(); await settle() })
 await act(async () => { scopeController.switchScope('https://weknora.test', 'owner-2', 'tenant-2'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '重新加载当前空间')); await settle(); await settle() })
 assert.match(container.textContent ?? '', /当前档案修订 9/)
 await act(async () => { resolveInitialNewScope({ revision: 8 }); await settle(); await settle() })
 assert.match(container.textContent ?? '', /当前档案修订 9/, 'late older same-scope read is ignored')
 await act(async () => { click(byLabel(container, 'button', '发起导出')); await settle(); await settle() })
 assert.equal(exports.length, 1)
 assert.equal(exports[0]?.expectedRevision, 9, 'export uses the recovered revision')
})
