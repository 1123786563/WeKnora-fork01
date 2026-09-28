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
import type { MaterialExportReceipt, MaterialVersionView, SubmissionReceipt } from '../../../../packages/api-client/src/career.ts'

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void }
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) })

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } }
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1&application=app-1' })
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Element: dom.window.Element, MutationObserver: dom.window.MutationObserver, getComputedStyle: dom.window.getComputedStyle.bind(dom.window), Event: dom.window.Event, IS_REACT_ACT_ENVIRONMENT: true })
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator })
const { createRoot } = await import('react-dom/client')
const { SubmissionPage } = await import('./SubmissionPage.tsx')

const ts = '2026-09-25T09:15:00Z'
const digest = 'b'.repeat(64)
const boundVersion = { materialId: 'mat-1', exportId: 'exp-1', version: 3, contentDigest: digest }
const record = (extra: Partial<SubmissionReceipt> = {}): SubmissionReceipt => ({ kind: 'submission_recorded', requestId: 'sub-req-1', applicationId: 'app-1', submissionId: 'sub-1', channel: 'web', occurredAt: ts, versionConfirmed: true, boundVersion, note: '官网已投', confirmer: 'owner-1', revision: 4, createdAt: ts, ...extra })
const exportReceipt = (): MaterialExportReceipt => ({ kind: 'material_published', requestId: 'pub-1', exportId: 'exp-1', materialId: 'mat-1', version: 3, status: 'submittable', submittable: true, contentDigest: digest, files: [
 { format: 'pdf', materialId: 'mat-1', version: 3, contentDigest: digest, verified: true },
 { format: 'docx', materialId: 'mat-1', version: 3, contentDigest: digest, verified: true },
], createdAt: ts })
const versionView = (): MaterialVersionView => ({ version: 3, pinnedEvidence: { opportunityId: 'opp/1', snapshotId: 'snapshot ?1', snapshotSha256: 'a'.repeat(64), profileRevision: 4 }, factBasisRevision: 4, body: { sections: [{ heading: '教育经历', content: '某大学 计算机科学与技术', claims: [{ claimId: 'claim-1', text: '本科在读', needsReview: false }] }] }, reviewRisks: [], requestId: 'confirm-1', createdAt: ts })

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

async function mountSubmission(career: CareerStub, options: { applicationId?: string; materialId?: string | null } = {}) {
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 const container = render(React.createElement(SubmissionPage, { client: { career } as unknown as WeKnoraClient, scopeController, applicationId: options.applicationId ?? 'app-1', ...(options.materialId === null ? {} : { materialId: options.materialId ?? 'mat-1' }) }))
 await act(async () => { await settle(); await settle() })
 return container
}

test('the confirm form states the user confirms external submissions and never infers from downloads', async () => {
 const sent: unknown[] = []
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  applicationSubmissions: async () => ({ submissions: [] }),
  materialExports: async () => ({ materialId: 'mat-1', exports: [exportReceipt()] }),
  recordSubmission: async (input: unknown) => { sent.push(input); throw new Error('must not auto-record') },
 }
 const container = await mountSubmission(career)
 assert.match(container.textContent ?? '', /系统不代投/)
 assert.match(container.textContent ?? '', /不会被视为投递/)
 assert.equal(sent.length, 0, 'no submission is recorded without an explicit user confirmation')
 const channels = [...container.querySelectorAll<HTMLSelectElement>('[aria-label="投递渠道"] option')].map((option) => option.value)
 assert.deepEqual(channels, ['', 'email', 'web', 'other'])
 const versions = [...container.querySelectorAll<HTMLSelectElement>('[aria-label="投递版本"] option')].map((option) => option.value)
 assert.deepEqual(versions, ['', 'exp-1', '__unknown__'])
 assert.match(container.querySelector('[aria-label="投递版本"]')?.textContent ?? '', /版本 V3/)
})

test('recording sends the confirmed channel, claimed time and bound export, then shows the record in the timeline', async () => {
 const sent: unknown[] = []
 let stored: SubmissionReceipt[] = []
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  applicationSubmissions: async () => ({ submissions: stored }),
  materialExports: async () => ({ materialId: 'mat-1', exports: [exportReceipt()] }),
  recordSubmission: async (input: Record<string, unknown>) => {
   sent.push(input)
   stored = [record({ requestId: input.requestId as string })]
   return stored[0]
  },
 }
 const container = await mountSubmission(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="投递渠道"]')!, 'web'); await settle() })
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="投递版本"]')!, 'exp-1'); await settle() })
 await act(async () => { setInput(container.querySelector<HTMLInputElement>('[aria-label="声明投递时间"]')!, '2026-09-25T17:15'); await settle() })
 await act(async () => { setInput(container.querySelector<HTMLTextAreaElement>('[aria-label="投递备注"]')!, '官网已投'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '确认投递')); await settle(); await settle() })
 assert.equal(sent.length, 1)
 const input = sent[0] as Record<string, unknown>
 assert.ok(input.requestId)
 assert.equal(input.applicationId, 'app-1')
 assert.equal(input.channel, 'web')
 assert.equal(input.versionUnknown, false)
 assert.equal(input.materialId, 'mat-1')
 assert.equal(input.exportId, 'exp-1')
 assert.equal(input.occurredAt, new Date('2026-09-25T17:15').toISOString())
 assert.equal(input.note, '官网已投')
 assert.equal(input.expectedRevision, 4)
 const rows = [...container.querySelectorAll<HTMLElement>('[aria-label="投递记录时间线"] > li')]
 assert.equal(rows.length, 1)
 assert.match(rows[0]!.textContent ?? '', /招聘网站/)
 assert.match(rows[0]!.textContent ?? '', /官网已投/)
 assert.match(rows[0]!.textContent ?? '', /版本 V3/)
 assert.match(rows[0]!.textContent ?? '', /owner-1/)
 assert.match(rows[0]!.textContent ?? '', new RegExp(input.requestId as string))
 assert.match(container.textContent ?? '', /此申请已有投递记录/)
})

test('clicking the bound version reference opens the read-only material version', async () => {
 const versionReads: Array<[string, number]> = []
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  applicationSubmissions: async () => ({ submissions: [record()] }),
  materialVersion: async (materialId: string, version: number) => { versionReads.push([materialId, version]); return versionView() },
 }
 const container = await mountSubmission(career, { materialId: null })
 assert.ok(!container.querySelector('[aria-label="投递版本只读回看"]'), 'version stays closed until the reference is clicked')
 await act(async () => { click(container.querySelector<HTMLButtonElement>('[aria-label="回看版本 V3"]')!); await settle(); await settle() })
 assert.deepEqual(versionReads, [['mat-1', 3]])
 const review = container.querySelector('[aria-label="投递版本只读回看"]')
 assert.ok(review, 'version review opens')
 assert.match(review.textContent ?? '', /版本 V3（只读）/)
 assert.match(review.textContent ?? '', /教育经历/)
 assert.match(review.textContent ?? '', /某大学/)
})

test('an explicit unknown version records unconfirmed with no binding and renders the marker', async () => {
 const sent: unknown[] = []
 let stored: SubmissionReceipt[] = []
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  applicationSubmissions: async () => ({ submissions: stored }),
  materialExports: async () => ({ materialId: 'mat-1', exports: [exportReceipt()] }),
  recordSubmission: async (input: Record<string, unknown>) => {
   sent.push(input)
   stored = [record({ requestId: input.requestId as string, channel: 'email', versionConfirmed: false, boundVersion: undefined, note: undefined })]
   return stored[0]
  },
 }
 const container = await mountSubmission(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="投递渠道"]')!, 'email'); await settle() })
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="投递版本"]')!, '__unknown__'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '确认投递')); await settle(); await settle() })
 const input = sent[0] as Record<string, unknown>
 assert.equal(input.versionUnknown, true)
 assert.equal('materialId' in input, false)
 assert.equal('exportId' in input, false)
 const rows = [...container.querySelectorAll<HTMLElement>('[aria-label="投递记录时间线"] > li')]
 assert.equal(rows.length, 1)
 assert.match(rows[0]!.textContent ?? '', /邮件/)
 assert.match(rows[0]!.textContent ?? '', /版本未确认（显式未知）/)
 assert.ok(!container.querySelector('[aria-label="回看版本 V3"]'), 'no version reference to open for an unknown binding')
})

test('a repeat confirmation is rejected with the typed conflict and never adds a second record', async () => {
 const sent: Array<{ requestId: string }> = []
 let stored: SubmissionReceipt[] = []
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  applicationSubmissions: async () => ({ submissions: stored }),
  materialExports: async () => ({ materialId: 'mat-1', exports: [exportReceipt()] }),
  recordSubmission: async (input: { requestId: string }) => {
   sent.push(input)
   if (stored.length) throw Object.assign(new Error('already confirmed'), { code: 'submission_already_confirmed' })
   stored = [record({ requestId: input.requestId })]
   return stored[0]
  },
 }
 const container = await mountSubmission(career)
 const submit = (): void => {
  choose(container.querySelector<HTMLSelectElement>('[aria-label="投递渠道"]')!, 'web')
  choose(container.querySelector<HTMLSelectElement>('[aria-label="投递版本"]')!, 'exp-1')
  click(byLabel(container, 'button', '确认投递'))
 }
 await act(async () => { submit(); await settle(); await settle() })
 assert.equal(sent.length, 1)
 await act(async () => { submit(); await settle(); await settle() })
 assert.equal(sent.length, 2)
 assert.notEqual(sent[1]!.requestId, sent[0]!.requestId, 'the rejected repeat uses its own fresh request id')
 assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /已有投递记录/)
 assert.equal(container.querySelectorAll('[aria-label="投递记录时间线"] > li').length, 1, 'no second submission record is created')
})

test('an unknown write recovers through the receipt lookup with the original request id', async () => {
 const sent: Array<{ requestId: string }> = []
 const receiptReads: string[] = []
 let stored: SubmissionReceipt[] = []
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  applicationSubmissions: async () => ({ submissions: stored }),
  materialExports: async () => ({ materialId: 'mat-1', exports: [exportReceipt()] }),
  recordSubmission: async (input: { requestId: string }) => {
   sent.push(input)
   throw Object.assign(new Error('写入结果未知'), { code: 'outcome_unknown', requestId: input.requestId })
  },
  submissionReceipt: async (requestId: string) => {
   receiptReads.push(requestId)
   stored = [record({ requestId })]
   return stored[0]
  },
 }
 const container = await mountSubmission(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="投递渠道"]')!, 'web'); await settle() })
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="投递版本"]')!, 'exp-1'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '确认投递')); await settle(); await settle() })
 assert.equal(sent.length, 1)
 const originalRequestId = sent[0]!.requestId
 assert.match(container.textContent ?? '', new RegExp(`原请求编号 ${originalRequestId}`))
 await act(async () => { click(byLabel(container, 'button', '查询投递回执')); await settle(); await settle() })
 assert.deepEqual(receiptReads, [originalRequestId])
 assert.equal(container.querySelectorAll('[aria-label="投递记录时间线"] > li').length, 1)
 assert.ok(!byLabelOrNull(container, 'button', '查询投递回执'), 'recovery state cleared after the receipt replay')
})

test('retrying an unknown write reuses the same request id', async () => {
 const sent: Array<{ requestId: string }> = []
 let failures = 1
 let stored: SubmissionReceipt[] = []
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  applicationSubmissions: async () => ({ submissions: stored }),
  materialExports: async () => ({ materialId: 'mat-1', exports: [exportReceipt()] }),
  recordSubmission: async (input: { requestId: string }) => {
   sent.push(input)
   if (failures > 0) { failures -= 1; throw Object.assign(new Error('写入结果未知'), { code: 'outcome_unknown', requestId: input.requestId }) }
   stored = [record({ requestId: input.requestId })]
   return stored[0]
  },
 }
 const container = await mountSubmission(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="投递渠道"]')!, 'web'); await settle() })
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="投递版本"]')!, 'exp-1'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '确认投递')); await settle(); await settle() })
 await act(async () => { click(byLabel(container, 'button', '用原请求编号重试')); await settle(); await settle() })
 assert.equal(sent.length, 2)
 assert.equal(sent[1]!.requestId, sent[0]!.requestId)
 assert.equal(container.querySelectorAll('[aria-label="投递记录时间线"] > li').length, 1)
})

function byLabelOrNull(container: HTMLElement, selector: string, label: string): HTMLElement | undefined {
 return [...container.querySelectorAll<HTMLElement>(selector)].find((item) => item.textContent?.trim() === label)
}

test('a revision conflict surfaces the current revision and offers a re-read', async () => {
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  applicationSubmissions: async () => ({ submissions: [] }),
  materialExports: async () => ({ materialId: 'mat-1', exports: [exportReceipt()] }),
  recordSubmission: async () => { throw Object.assign(new Error('revision conflict'), { code: 'revision_conflict', currentRevision: 9 }) },
 }
 const container = await mountSubmission(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="投递渠道"]')!, 'web'); await settle() })
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="投递版本"]')!, 'exp-1'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '确认投递')); await settle(); await settle() })
 assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /当前修订 9/)
 assert.ok(byLabel(container, 'button', '重新读取档案修订'), 're-read action exists')
})

test('a cross-tenant application shows a clear inaccessible state', async () => {
 const career: CareerStub = { applicationSubmissions: async () => { throw Object.assign(new Error('forbidden'), { code: 'forbidden' }) } }
 const container = await mountSubmission(career, { materialId: null })
 assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /当前空间不可访问/)
 assert.ok(!container.querySelector('[aria-label="确认投递表单"]'), 'no confirm form outside the accessible scope')
})

test('without submittable exports only the explicit unknown choice is offered', async () => {
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  applicationSubmissions: async () => ({ submissions: [] }),
  materialExports: async () => ({ materialId: 'mat-1', exports: [{ ...exportReceipt(), exportId: 'exp-0', status: 'staged' as const, submittable: false }] }),
 }
 const container = await mountSubmission(career)
 const versions = [...container.querySelectorAll<HTMLSelectElement>('[aria-label="投递版本"] option')].map((option) => option.value)
 assert.deepEqual(versions, ['', '__unknown__'])
 assert.match(container.textContent ?? '', /尚无.*可投递.*版本/)
})

test('an export-list failure is visible and blocks the irreversible unknown-version choice until retry succeeds', async () => {
 let exportReads = 0
 const submissions: unknown[] = []
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  applicationSubmissions: async () => ({ submissions: [] }),
  materialExports: async () => { exportReads += 1; if (exportReads === 1) throw Object.assign(new Error('temporary network failure'), { status: 503 }); return { materialId: 'mat-1', exports: [] } },
  recordSubmission: async (input: unknown) => { submissions.push(input); return record() },
 }
 const container = await mountSubmission(career)
 assert.match(container.textContent ?? '', /可投递导出版本读取失败/)
 assert.equal((container.querySelector('[aria-label="投递版本"] option[value="__unknown__"]') as HTMLOptionElement).disabled, true)
 await act(async () => { click(byLabel(container, 'button', '重试读取导出版本')); await settle(); await settle() })
 assert.equal(exportReads, 2)
 assert.equal((container.querySelector('[aria-label="投递版本"] option[value="__unknown__"]') as HTMLOptionElement).disabled, false)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="投递渠道"]')!, 'web'); choose(container.querySelector<HTMLSelectElement>('[aria-label="投递版本"]')!, '__unknown__'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '确认投递')); await settle(); await settle() })
 assert.equal(submissions.length, 1)
 assert.equal((submissions[0] as { versionUnknown: boolean }).versionUnknown, true)
})

test('a forbidden export refresh clears the loaded submission and composed private form', async () => {
 let exportReads = 0
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  applicationSubmissions: async () => ({ submissions: [record()] }),
  materialExports: async () => { exportReads += 1; if (exportReads > 1) throw Object.assign(new Error('forbidden'), { code: 'forbidden' }); return { materialId: 'mat-1', exports: [exportReceipt()] } },
 }
 const container = await mountSubmission(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="投递渠道"]')!, 'web'); choose(container.querySelector<HTMLSelectElement>('[aria-label="投递版本"]')!, 'exp-1'); setInput(container.querySelector<HTMLTextAreaElement>('[aria-label="投递备注"]')!, 'private note'); await settle() })
 assert.match(container.textContent ?? '', /投递记录时间线|官网已投/)
 await act(async () => { click(byLabel(container, 'button', '刷新投递记录')); await settle(); await settle() })
 assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /当前空间不可访问/)
 assert.equal(container.querySelector('[aria-label="确认投递表单"]'), null)
 assert.equal(container.querySelector('[aria-label="投递记录时间线"]'), null)
 assert.doesNotMatch(container.textContent ?? '', /private note|重试读取导出版本/)
})

test('a late submission read cannot restore private timeline after exports become forbidden', async () => {
 let resolveSubmissions!: (value: { submissions: SubmissionReceipt[] }) => void
 let rejectExports!: (cause: unknown) => void
 const pendingSubmissions = new Promise<{ submissions: SubmissionReceipt[] }>((resolve) => { resolveSubmissions = resolve })
 const pendingExports = new Promise<{ materialId: string; exports: MaterialExportReceipt[] }>((_resolve, reject) => { rejectExports = reject })
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  applicationSubmissions: () => pendingSubmissions,
  materialExports: () => pendingExports,
 }
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 const container = render(React.createElement(SubmissionPage, { client: { career } as unknown as WeKnoraClient, scopeController, applicationId: 'app-1', materialId: 'mat-1' }))
 await act(async () => { await settle() })
 rejectExports(Object.assign(new Error('forbidden'), { code: 'forbidden' }))
 await act(async () => { await settle(); await settle() })
 assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /当前空间不可访问/)
 assert.equal(container.querySelector('[aria-label="投递记录时间线"]'), null)
 resolveSubmissions({ submissions: [record()] })
 await act(async () => { await settle(); await settle() })
 assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /当前空间不可访问/)
 assert.equal(container.querySelector('[aria-label="投递记录时间线"]'), null, 'late private records stay cleared')
 assert.equal(container.querySelector('[aria-label="确认投递表单"]'), null, 'late private data cannot reopen the pane')
})

test('refresh restarts a pending profile revision read so confirmation can recover', async () => {
 let resolveFirst!: (value: { revision: number }) => void
 let resolveSecond!: (value: { revision: number }) => void
 let openCalls = 0
 const firstOpen = new Promise<{ revision: number }>((resolve) => { resolveFirst = resolve })
 const secondOpen = new Promise<{ revision: number }>((resolve) => { resolveSecond = resolve })
 const sent: unknown[] = []
 const career: CareerStub = {
  open: () => ++openCalls === 1 ? firstOpen : secondOpen,
  applicationSubmissions: async () => ({ submissions: [] }),
  materialExports: async () => ({ materialId: 'mat-1', exports: [exportReceipt()] }),
  recordSubmission: async (input: unknown) => { sent.push(input); return record() },
 }
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 const container = render(React.createElement(SubmissionPage, { client: { career } as unknown as WeKnoraClient, scopeController, applicationId: 'app-1', materialId: 'mat-1' }))
 await act(async () => { await settle(); await settle() })
 assert.equal(openCalls, 1)
 assert.match(container.textContent ?? '', /正在读取当前档案修订/)

 await act(async () => { click(byLabel(container, 'button', '刷新投递记录')); await settle(); await settle() })
 assert.equal(openCalls, 2, 'refresh starts a new revision read')
 resolveFirst({ revision: 3 })
 await act(async () => { await settle() })
 assert.match(container.textContent ?? '', /正在读取当前档案修订/, 'the superseded read cannot settle the refreshed loading state')

 resolveSecond({ revision: 4 })
 await act(async () => { await settle(); await settle() })
 assert.match(container.textContent ?? '', /当前档案修订 4/)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="投递渠道"]')!, 'web'); choose(container.querySelector<HTMLSelectElement>('[aria-label="投递版本"]')!, 'exp-1'); await settle() })
 assert.equal((byLabel(container, 'button', '确认投递') as HTMLButtonElement).disabled, false)
 await act(async () => { click(byLabel(container, 'button', '确认投递')); await settle(); await settle() })
 assert.equal(sent.length, 1)
 assert.equal((sent[0] as { expectedRevision: number }).expectedRevision, 4)
})

test('a late version review cannot restore private material state after forbidden exports', async () => {
 let resolveVersion!: (value: MaterialVersionView) => void
 let exportReads = 0
 const pendingVersion = new Promise<MaterialVersionView>((resolve) => { resolveVersion = resolve })
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  applicationSubmissions: async () => ({ submissions: [record()] }),
  materialExports: async () => {
   exportReads += 1
   if (exportReads === 2) throw Object.assign(new Error('forbidden'), { code: 'forbidden' })
   return { materialId: 'mat-1', exports: [exportReceipt()] }
  },
  materialVersion: () => pendingVersion,
 }
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 const container = render(React.createElement(SubmissionPage, { client: { career } as unknown as WeKnoraClient, scopeController, applicationId: 'app-1', materialId: 'mat-1' }))
 await act(async () => { await settle(); await settle() })
 await act(async () => { click(container.querySelector<HTMLButtonElement>('[aria-label="回看版本 V3"]')!); await settle() })
 await act(async () => { click(byLabel(container, 'button', '刷新投递记录')); await settle(); await settle() })
 assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /当前空间不可访问/)
 assert.equal(container.querySelector('[aria-label="投递版本只读回看"]'), null)

 resolveVersion(versionView())
 await act(async () => { await settle(); await settle() })
 assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /当前空间不可访问/)
 assert.equal(container.querySelector('[aria-label="投递版本只读回看"]'), null, 'late material content stays cleared')

 await act(async () => { root!.render(React.createElement(SubmissionPage, { client: { career } as unknown as WeKnoraClient, scopeController, applicationId: 'app-2', materialId: 'mat-1' })) })
 await act(async () => { await settle(); await settle() })
 assert.ok(container.querySelector('[aria-label="投递记录时间线"]'), 'the reused pane has loaded the next application')
 assert.equal(container.querySelector('[aria-label="投递版本只读回看"]'), null, 'a cleared private version does not reappear when the pane is reused')
 assert.doesNotMatch(container.textContent ?? '', /某大学|版本回看暂时无法读取/)
})

test('a refresh that removes the selected export clears the choice and blocks submission', async () => {
 let exportReads = 0
 const submissions: unknown[] = []
 const career: CareerStub = {
  open: async () => ({ revision: 4 }),
  applicationSubmissions: async () => ({ submissions: [] }),
  materialExports: async () => ({ materialId: 'mat-1', exports: ++exportReads === 1 ? [exportReceipt()] : [] }),
  recordSubmission: async (input: unknown) => { submissions.push(input); return record() },
 }
 const container = await mountSubmission(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="投递渠道"]')!, 'web'); choose(container.querySelector<HTMLSelectElement>('[aria-label="投递版本"]')!, 'exp-1'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '刷新投递记录')); await settle(); await settle() })
 assert.equal(container.querySelector<HTMLSelectElement>('[aria-label="投递版本"]')?.value, '')
 assert.equal((byLabel(container, 'button', '确认投递') as HTMLButtonElement).disabled, true)
 await act(async () => { click(byLabel(container, 'button', '确认投递')); await settle(); await settle() })
 assert.equal(submissions.length, 0)
})

test('submission styles keep TDesign light surfaces and the brand green confirm action', () => {
 const css = readFileSync(new URL('./submission.css', import.meta.url), 'utf8')
 assert.match(css, /\.wk-submission \{/)
 assert.match(css, /--td-bg-color-container/)
 assert.match(css, /#07c05f/)
})
