import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import * as nodeModule from 'node:module'
import { readFileSync } from 'node:fs'
import test, { afterEach } from 'node:test'
import * as React from 'react'
import { act } from 'react'
import type { Root } from 'react-dom/client'
import { createScopeController } from '@weknora/domain/scope'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ProgressEventView, ProgressReceipt, ProgressView } from '../../../../packages/api-client/src/career.ts'

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void }
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) })

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } }
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1&application=app-1' })
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Element: dom.window.Element, MutationObserver: dom.window.MutationObserver, getComputedStyle: dom.window.getComputedStyle.bind(dom.window), Event: dom.window.Event, IS_REACT_ACT_ENVIRONMENT: true })
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator })
const { createRoot } = await import('react-dom/client')
const { ProgressPage } = await import('./ProgressPage.tsx')

const ts = '2026-09-25T08:00:00Z'
const ts2 = '2026-09-25T10:30:00Z'
const manual = { kind: 'manual' as const }
const event = (seq: number, eventType: ProgressEventView['eventType'], note: string, extra: Partial<ProgressEventView> = {}): ProgressEventView => ({ eventId: `evt-${seq}`, seq, kind: 'progress_appended', eventType, note, occurredAt: seq === 1 ? ts : ts2, source: manual, confirmer: 'owner-1', corrected: false, requestId: `req-${seq}`, createdAt: ts, ...extra })
const view = (stage: ProgressView['stage'], events: ProgressEventView[], applicationId = 'app-1'): ProgressView => ({ applicationId, revision: events.length, stage, events })
const receipt = (seq: number, eventType: ProgressReceipt['eventType'], requestId: string, stage: ProgressReceipt['stage'], extra: Partial<ProgressReceipt> = {}): ProgressReceipt => ({ kind: 'progress_appended', requestId, applicationId: 'app-1', eventId: `evt-${seq}`, seq, revision: seq, eventType, stage, occurredAt: ts2, source: manual, confirmer: 'owner-1', createdAt: ts2, ...extra })

type CareerStub = Record<string, (...args: any[]) => unknown>
let root: Root | undefined
let host: HTMLDivElement | undefined
afterEach(async () => {
 if (root) await act(async () => root?.unmount())
 root = undefined; host?.remove(); host = undefined; document.body.replaceChildren()
 window.history.replaceState({}, '', '/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1&application=app-1')
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
function setInput(input: HTMLInputElement | HTMLTextAreaElement, value: string) {
 const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(input), 'value')?.set
 setter?.call(input, value)
 input.dispatchEvent(new dom.window.Event('input', { bubbles: true }))
}
function choose(select: HTMLSelectElement, value: string) {
 const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(select), 'value')?.set
 setter?.call(select, value)
 select.dispatchEvent(new dom.window.Event('change', { bubbles: true }))
}
function click(el: HTMLElement) { act(() => { el.dispatchEvent(new dom.window.Event('click', { bubbles: true })) }) }

async function mountProgress(career: CareerStub, applicationId = 'app-1') {
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 const container = render(React.createElement(ProgressPage, { client: { career } as unknown as WeKnoraClient, scopeController, applicationId }))
 await act(async () => { await settle(); await settle() })
 return { container, scopeController }
}

test('the timeline lists events in order with source, confirmer, content and a prominent stage projection', async () => {
 const reads: string[] = []
 const career: CareerStub = { applicationProgress: async (id: string) => { reads.push(id); return view('interview', [event(1, 'submitted', '已通过官网投递'), event(2, 'interview', '一面通过')]) } }
 const { container } = await mountProgress(career)
 assert.deepEqual(reads, ['app-1'])
 assert.match(container.querySelector('[aria-label="当前阶段投影"]')?.textContent ?? '', /面试/)
 const items = [...container.querySelectorAll<HTMLElement>('[aria-label="进展事件历史"] > li')]
 assert.equal(items.length, 2)
 assert.match(items[0]!.textContent ?? '', /已投递/)
 assert.match(items[0]!.textContent ?? '', /已通过官网投递/)
 assert.match(items[0]!.textContent ?? '', /用户录入/)
 assert.match(items[0]!.textContent ?? '', /owner-1/)
 assert.match(items[0]!.textContent ?? '', new RegExp(ts))
 assert.match(items[1]!.textContent ?? '', /面试/)
 assert.match(items[1]!.textContent ?? '', /一面通过/)
 assert.match(items[1]!.textContent ?? '', new RegExp(ts2))
})

test('generic timeline omits submission event choices and routes to dedicated submission confirmation', async () => {
 const career: CareerStub = { applicationProgress: async () => view('preparing', []), applicationSubmissions: async () => ({ applicationId: 'app-1', submissions: [] }), open: async () => ({ revision: 1 }), materialExports: async () => ({ exports: [] }) }
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 const container = render(React.createElement(ProgressPage, { client: { career } as unknown as WeKnoraClient, scopeController, applicationId: 'app-1' }))
 await act(async () => { await settle(); await settle() })
 const options = [...container.querySelectorAll<HTMLOptionElement>('[aria-label="事件类型"] option')].map((option) => option.value)
 assert.equal(options.includes('submitted'), false)
 assert.equal(options.includes('resubmitted'), false)
 const correctionOptions = [...container.querySelectorAll<HTMLOptionElement>('[aria-label="更正事件类型"] option')].map((option) => option.value)
 assert.equal(correctionOptions.includes('submitted'), false)
 assert.equal(correctionOptions.includes('resubmitted'), false)
 click(byLabel(container, 'button', '投递确认与回看'))
 await act(async () => { await settle(); await settle() })
 assert.match(container.textContent ?? '', /记录投递确认/)
})

test('progress submission confirmation loads and records the exact known material export', async () => {
 const digest = 'a'.repeat(64)
 const sent: unknown[] = []
 const exportReceipt = { kind: 'material_published', requestId: 'publish-1', exportId: 'export-known', materialId: 'material-current', version: 7, status: 'submittable', submittable: true, contentDigest: digest, files: [
  { format: 'pdf', materialId: 'material-current', version: 7, contentDigest: digest, verified: true },
  { format: 'docx', materialId: 'material-current', version: 7, contentDigest: digest, verified: true },
 ], createdAt: ts }
 const career: CareerStub = {
  applicationProgress: async () => view('preparing', []),
  applicationSubmissions: async () => ({ applicationId: 'app-1', submissions: [] }),
  open: async () => ({ revision: 9 }),
  materialExports: async (materialId: string) => ({ materialId, exports: [exportReceipt] }),
  recordSubmission: async (input: unknown) => { sent.push(input); return { requestId: 'record-1' } },
 }
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 const container = render(React.createElement(ProgressPage, { client: { career } as unknown as WeKnoraClient, scopeController, applicationId: 'app-1', materialId: 'material-current' }))
 await act(async () => { await settle(); await settle() })
 click(byLabel(container, 'button', '投递确认与回看'))
 await act(async () => { await settle(); await settle() })
 const version = container.querySelector<HTMLSelectElement>('[aria-label="投递版本"]')!
 assert.ok([...version.options].some((option) => option.value === 'export-known'))
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="投递渠道"]')!, 'web'); choose(version, 'export-known'); await settle() })
 click(byLabel(container, 'button', '确认投递'))
 await act(async () => { await settle(); await settle() })
 assert.equal(sent.length, 1)
 assert.deepEqual(sent[0], { requestId: (sent[0] as { requestId: string }).requestId, applicationId: 'app-1', channel: 'web', materialId: 'material-current', exportId: 'export-known', versionUnknown: false, expectedRevision: 9 })
})

test('progress submission confirmation keeps absent material explicit and offers unknown version', async () => {
 let exportReads = 0
 const career: CareerStub = { applicationProgress: async () => view('preparing', []), applicationSubmissions: async () => ({ applicationId: 'app-1', submissions: [] }), open: async () => ({ revision: 1 }), materialExports: async () => { exportReads += 1; return { exports: [] } } }
 const { container } = await mountProgress(career)
 click(byLabel(container, 'button', '投递确认与回看'))
 await act(async () => { await settle(); await settle() })
 assert.equal(exportReads, 0)
 assert.match(container.textContent ?? '', /尚无可投递版本/)
 assert.equal(container.querySelector<HTMLOptionElement>('[aria-label="投递版本"] option[value="__unknown__"]')?.disabled, false)
 assert.equal(container.textContent?.includes('版本 V'), false)
})

test('recording an interview event updates the projection and reopening shows the same stage', async () => {
 let current = view('submitted', [event(1, 'submitted', '已通过官网投递')])
 const sent: unknown[] = []
 const career: CareerStub = {
  applicationProgress: async () => current,
  appendProgress: async (input: { requestId: string }) => {
   sent.push(input)
   current = view('interview', [event(1, 'submitted', '已通过官网投递'), event(2, 'interview', '一面通过')])
   return receipt(2, 'interview', input.requestId, 'interview')
  },
 }
 const { container } = await mountProgress(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="事件类型"]')!, 'interview'); await settle() })
 await act(async () => { setInput(container.querySelector<HTMLTextAreaElement>('[aria-label="进展备注"]')!, '一面通过'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '记录进展')); await settle(); await settle() })
 assert.equal(sent.length, 1)
 const input = sent[0] as { requestId: string; applicationId: string; eventType: string; note: string; expectedRevision: number }
 assert.ok(input.requestId)
 assert.equal(input.applicationId, 'app-1')
 assert.equal(input.eventType, 'interview')
 assert.equal(input.note, '一面通过')
 assert.equal(input.expectedRevision, 1)
 assert.match(container.querySelector('[aria-label="当前阶段投影"]')?.textContent ?? '', /面试/)
 assert.equal(container.querySelectorAll('[aria-label="进展事件历史"] > li').length, 2)
 await act(async () => { click(byLabel(container, 'button', '刷新')); await settle(); await settle() })
 assert.match(container.querySelector('[aria-label="当前阶段投影"]')?.textContent ?? '', /面试/)
 await act(async () => { root?.unmount() })
 document.body.replaceChildren()
 const reopened = await mountProgress(career)
 assert.match(reopened.container.querySelector('[aria-label="当前阶段投影"]')?.textContent ?? '', /面试/)
 assert.equal(reopened.container.querySelectorAll('[aria-label="进展事件历史"] > li').length, 2)
})

test('a correction keeps the original visible, marks it corrected and re-projects the stage', async () => {
 let current = view('submitted', [event(1, 'submitted', '已通过官网投递')])
 const sent: unknown[] = []
 const career: CareerStub = {
  applicationProgress: async () => current,
  correctProgress: async (input: { requestId: string }) => {
   sent.push(input)
   current = view('assessment', [
    event(1, 'submitted', '已通过官网投递', { corrected: true }),
    event(2, 'assessment', '更正：此前为测评，尚未正式投递', { kind: 'progress_corrected', correctsEventId: 'evt-1' }),
   ])
   return receipt(2, 'assessment', input.requestId, 'assessment', { kind: 'progress_corrected', correctsEventId: 'evt-1' })
  },
 }
 const { container } = await mountProgress(career)
 await act(async () => { click(container.querySelector<HTMLButtonElement>('[aria-label="纠错事件 evt-1"]')!); await settle() })
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="更正事件类型"]')!, 'assessment'); await settle() })
 await act(async () => { setInput(container.querySelector<HTMLTextAreaElement>('[aria-label="更正备注"]')!, '更正：此前为测评，尚未正式投递'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '提交更正')); await settle(); await settle() })
 assert.equal(sent.length, 1)
 const input = sent[0] as { requestId: string; applicationId: string; correctsEventId: string; eventType: string; expectedRevision: number }
 assert.equal(input.applicationId, 'app-1')
 assert.equal(input.correctsEventId, 'evt-1')
 assert.equal(input.eventType, 'assessment')
 assert.equal(input.expectedRevision, 1)
 const items = [...container.querySelectorAll<HTMLElement>('[aria-label="进展事件历史"] > li')]
 assert.equal(items.length, 2)
 assert.match(items[0]!.textContent ?? '', /已通过官网投递/)
 assert.match(items[0]!.textContent ?? '', /已更正/)
 assert.match(items[1]!.textContent ?? '', /更正：此前为测评，尚未正式投递/)
 assert.match(items[1]!.textContent ?? '', /更正 evt-1/)
 assert.match(container.querySelector('[aria-label="当前阶段投影"]')?.textContent ?? '', /测评或笔试/)
})

test('an unknown write recovers through the receipt lookup with the original request id', async () => {
 let current = view('submitted', [event(1, 'submitted', '已通过官网投递')])
 const appends: Array<{ requestId: string }> = []
 const receiptReads: string[] = []
 const career: CareerStub = {
  applicationProgress: async () => current,
  appendProgress: async (input: { requestId: string }) => {
   appends.push(input)
   throw Object.assign(new Error('写入结果未知'), { code: 'outcome_unknown', requestId: input.requestId })
  },
  progressReceipt: async (requestId: string) => {
   receiptReads.push(requestId)
   current = view('interview', [event(1, 'submitted', '已通过官网投递'), event(2, 'interview', '一面通过')])
   return receipt(2, 'interview', appends[0]!.requestId, 'interview')
  },
 }
 const { container } = await mountProgress(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="事件类型"]')!, 'interview'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '记录进展')); await settle(); await settle() })
 assert.equal(appends.length, 1)
 const originalRequestId = appends[0]!.requestId
 assert.match(container.textContent ?? '', new RegExp(`原请求编号 ${originalRequestId}`))
 assert.ok(byLabel(container, 'button', '查询进展回执'), 'receipt recovery action exists')
 await act(async () => { click(byLabel(container, 'button', '查询进展回执')); await settle(); await settle() })
 assert.deepEqual(receiptReads, [originalRequestId])
 assert.match(container.querySelector('[aria-label="当前阶段投影"]')?.textContent ?? '', /面试/)
 assert.equal(container.querySelectorAll('[aria-label="进展事件历史"] > li').length, 2)
 assert.ok(!byLabelOrNull(container, 'button', '查询进展回执'), 'recovery state cleared after the receipt replay')
})

test('a mismatched receipt lookup ends recovery with a deterministic error', async () => {
 const career: CareerStub = {
  applicationProgress: async () => view('submitted', [event(1, 'submitted', '已投递')]),
  appendProgress: async (input: { requestId: string }) => { throw Object.assign(new Error('unknown'), { code: 'outcome_unknown', requestId: input.requestId }) },
  progressReceipt: async () => receipt(2, 'interview', 'other-request', 'interview'),
 }
 const { container } = await mountProgress(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="事件类型"]')!, 'interview'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '记录进展')); await settle(); await settle() })
 await act(async () => { click(byLabel(container, 'button', '查询进展回执')); await settle(); await settle() })
 assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /回执与本次请求不匹配/)
 assert.equal(byLabelOrNull(container, 'button', '查询进展回执'), undefined)
 assert.equal(byLabelOrNull(container, 'button', '用原请求编号重试'), undefined)
})

function byLabelOrNull(container: HTMLElement, selector: string, label: string): HTMLElement | undefined {
 return [...container.querySelectorAll<HTMLElement>(selector)].find((item) => item.textContent?.trim() === label)
}

test('retrying an unknown write reuses the same request id and produces no second event', async () => {
 let current = view('submitted', [event(1, 'submitted', '已通过官网投递')])
 const appends: Array<{ requestId: string }> = []
 let failures = 1
 const career: CareerStub = {
  applicationProgress: async () => current,
  appendProgress: async (input: { requestId: string }) => {
   appends.push(input)
   if (failures > 0) { failures -= 1; throw Object.assign(new Error('写入结果未知'), { code: 'outcome_unknown', requestId: input.requestId }) }
   current = view('interview', [event(1, 'submitted', '已通过官网投递'), event(2, 'interview', '一面通过')])
   return receipt(2, 'interview', input.requestId, 'interview')
  },
 }
 const { container } = await mountProgress(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="事件类型"]')!, 'interview'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '记录进展')); await settle(); await settle() })
 assert.equal(appends.length, 1)
 await act(async () => { click(byLabel(container, 'button', '用原请求编号重试')); await settle(); await settle() })
 assert.equal(appends.length, 2)
 assert.equal(appends[1]!.requestId, appends[0]!.requestId)
 assert.match(container.querySelector('[aria-label="当前阶段投影"]')?.textContent ?? '', /面试/)
 assert.equal(container.querySelectorAll('[aria-label="进展事件历史"] > li').length, 2)
})

test('a network-failed write stays unknown and recovers by the original receipt', async () => {
 let current = view('submitted', [event(1, 'submitted', '已通过官网投递')])
 const appends: Array<{ requestId: string }> = []
 let failNetwork = true
 const career: CareerStub = {
  applicationProgress: async () => current,
  appendProgress: async (input: { requestId: string }) => {
   appends.push(input)
   if (failNetwork) { failNetwork = false; throw new TypeError('Failed to fetch') }
   current = view('interview', [event(1, 'submitted', '已通过官网投递'), event(2, 'interview', '一面通过')])
   return receipt(2, 'interview', input.requestId, 'interview')
  },
  progressReceipt: async (requestId: string) => {
   current = view('interview', [event(1, 'submitted', '已通过官网投递'), event(2, 'interview', '一面通过')])
   return receipt(2, 'interview', requestId, 'interview')
  },
 }
 const { container } = await mountProgress(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="事件类型"]')!, 'interview'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '记录进展')); await settle(); await settle() })
 assert.match(container.textContent ?? '', new RegExp(`原请求编号 ${appends[0]!.requestId}`))
 await act(async () => { click(byLabel(container, 'button', '查询进展回执')); await settle(); await settle() })
 assert.match(container.querySelector('[aria-label="当前阶段投影"]')?.textContent ?? '', /面试/)
 assert.equal(container.querySelectorAll('[aria-label="进展事件历史"] > li').length, 2)
})

test('a revision conflict surfaces the current revision', async () => {
 const career: CareerStub = {
  applicationProgress: async () => view('submitted', [event(1, 'submitted', '已通过官网投递')]),
  appendProgress: async () => { throw Object.assign(new Error('进展已被更新'), { code: 'revision_conflict', currentRevision: 3 }) },
 }
 const { container } = await mountProgress(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="事件类型"]')!, 'interview'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '记录进展')); await settle(); await settle() })
 assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /当前修订 3/)
})

test('a cross-tenant or missing application shows a clear error state', async () => {
 const forbidden: CareerStub = { applicationProgress: async () => { throw Object.assign(new Error('forbidden'), { code: 'forbidden' }) } }
 const first = await mountProgress(forbidden)
 assert.match(first.container.querySelector('[role="alert"]')?.textContent ?? '', /当前空间不可访问/)
 await act(async () => { root?.unmount() })
 document.body.replaceChildren()
 const missing: CareerStub = { applicationProgress: async () => { throw Object.assign(new Error('not found'), { code: 'not_found' }) } }
 const second = await mountProgress(missing)
 assert.match(second.container.querySelector('[role="alert"]')?.textContent ?? '', /未找到此申请/)
})

test('switching applications never mixes their timelines', async () => {
 const reads: string[] = []
 const career: CareerStub = {
  applicationProgress: async (id: string) => {
   reads.push(id)
   if (id === 'app-1') return view('submitted', [event(1, 'submitted', '申请一的投递事件')])
   return view('interview', [event(1, 'interview', '申请二的面试事件')], 'app-2')
  },
 }
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 const client = { career } as unknown as WeKnoraClient
 const first = render(React.createElement(ProgressPage, { key: 'app-1', client, scopeController, applicationId: 'app-1' }))
 await act(async () => { await settle(); await settle() })
 assert.match(first.textContent ?? '', /申请一的投递事件/)
 await act(async () => { root?.unmount() })
 document.body.replaceChildren()
 const second = render(React.createElement(ProgressPage, { key: 'app-2', client, scopeController, applicationId: 'app-2' }))
 await act(async () => { await settle(); await settle() })
 assert.deepEqual(reads, ['app-1', 'app-2'])
 assert.doesNotMatch(second.textContent ?? '', /申请一/)
 assert.match(second.textContent ?? '', /申请二的面试事件/)
})

test('progress styles keep TDesign light surfaces and the brand green projection', () => {
 const css = readFileSync(new URL('./progress.css', import.meta.url), 'utf8')
 assert.match(css, /\.wk-progress \{/)
 assert.match(css, /--td-bg-color-container/)
 assert.match(css, /#07c05f/)
})
