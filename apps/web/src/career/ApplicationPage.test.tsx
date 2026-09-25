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
import type { CareerView, EvaluationReceipt } from '../../../../packages/career-core/src/contracts.ts'
import type { ApplicationReceipt } from '../../../../packages/api-client/src/career.ts'

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void }
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) })

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } }
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1' })
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Element: dom.window.Element, MutationObserver: dom.window.MutationObserver, getComputedStyle: dom.window.getComputedStyle.bind(dom.window), Event: dom.window.Event, IS_REACT_ACT_ENVIRONMENT: true })
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator })
const { createRoot } = await import('react-dom/client')
const { ApplicationPage } = await import('./ApplicationPage.tsx')

const view: CareerView = { revision: 4, facts: [], proposals: [] }
const eligible: EvaluationReceipt = { kind: 'evaluation_created', requestId: 'evaluation-request-1', evaluationId: 'eval-eligible', opportunityId: 'opp/1', snapshotId: 'snapshot ?1', profileRevision: 4, status: 'eligible' }
const ineligible: EvaluationReceipt = { kind: 'evaluation_created', requestId: 'evaluation-request-2', evaluationId: 'eval-ineligible', opportunityId: 'opp/1', snapshotId: 'snapshot ?1', profileRevision: 4, status: 'ineligible' }
const eligiblePin = { opportunityId: 'opp/1', snapshotId: 'snapshot ?1', evaluationId: 'eval-eligible', profileRevision: 4, evaluationStatus: 'eligible' as const, batchIdentity: '2026 秋招 A 批' }
const readyReceipt: ApplicationReceipt = { applicationId: 'app-1', requestId: 'apply-1', linkState: 'ready', taskId: 'task-9', runId: 'run-9', qualified: true, pinnedEvidence: eligiblePin }
const linkingReceipt: ApplicationReceipt = { applicationId: 'app-2', requestId: 'apply-2', linkState: 'linking', qualified: true, pinnedEvidence: eligiblePin }
const failedReceipt: ApplicationReceipt = { applicationId: 'app-4', requestId: 'apply-4', linkState: 'link_failed', qualified: true, pinnedEvidence: eligiblePin }
const warnedReceipt: ApplicationReceipt = { applicationId: 'app-5', requestId: 'apply-5', linkState: 'ready', taskId: 'task-5', qualified: false, warning: { evaluationId: 'eval-ineligible', evaluationStatus: 'ineligible', hardRuleId: 'graduation_year', reasonCode: 'graduation_year_mismatch' }, pinnedEvidence: { ...eligiblePin, evaluationId: 'eval-ineligible', evaluationStatus: 'ineligible' } }

type CareerStub = Record<string, (...args: any[]) => unknown>
let root: Root | undefined
let host: HTMLDivElement | undefined
afterEach(async () => {
 if (root) await act(async () => root?.unmount())
 root = undefined; host?.remove(); host = undefined; document.body.replaceChildren()
 window.history.replaceState({}, '', '/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1')
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
function toggle(check: HTMLInputElement) {
 const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(check), 'checked')?.set
 setter?.call(check, !check.checked)
 check.dispatchEvent(new dom.window.Event('click', { bubbles: true }))
}
function pick(container: HTMLElement, evaluationId: string) {
 const radio = container.querySelector<HTMLInputElement>(`input[type="radio"][value="${evaluationId}"]`)!
 const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(radio), 'checked')?.set
 setter?.call(radio, true)
 radio.dispatchEvent(new dom.window.Event('click', { bubbles: true }))
}
async function mountApplication(career: CareerStub, options: { evaluations?: EvaluationReceipt[]; scopeController?: ReturnType<typeof createScopeController>; batchHint?: { state: 'known'; value: string } | { state: 'unknown' } } = {}) {
 const scopeController = options.scopeController ?? createScopeController({ origin: 'https://weknora.test', userId: 'u', tenantId: 't' })
 const element = React.createElement(ApplicationPage, {
  client: { career } as unknown as WeKnoraClient, scopeController,
  opportunityId: 'opp/1', snapshotId: 'snapshot ?1',
  evaluations: options.evaluations ?? [eligible, ineligible],
  batchHint: options.batchHint ?? { state: 'known', value: '2026 秋招 A 批' },
 })
 const container = render(element)
 await act(async () => { await settle(); await settle() })
 return { container, scopeController }
}
async function submitApplication(container: HTMLElement, evaluationId: string, batch: string, career: CareerStub & { createApplication: (input: any) => Promise<ApplicationReceipt> }) {
 await act(async () => { pick(container, evaluationId); await settle() })
 const batchInput = container.querySelector<HTMLInputElement>('[aria-label="招聘批次标识"]')!
 await act(async () => { setInput(batchInput, batch); await settle() })
 await act(async () => { byLabel(container, 'button', '创建申请').click(); await settle(); await settle() })
}

test('creates an application from the job card that pins snapshot, profile revision and evaluation, then shows the ready task', async () => {
 const sent: unknown[] = []
 const career: CareerStub & { createApplication: (input: unknown) => Promise<ApplicationReceipt> } = {
  open: async () => view,
  createApplication: async (input: unknown) => { sent.push(input); return { ...readyReceipt, requestId: (input as { requestId: string }).requestId } },
 }
 const { container } = await mountApplication(career)
 assert.match(container.textContent ?? '', /当前档案修订 4/)
 assert.equal(container.querySelector<HTMLInputElement>('[aria-label="招聘批次标识"]')?.value, '2026 秋招 A 批')
 await submitApplication(container, 'eval-eligible', '2026 秋招 A 批', career)
 assert.equal(sent.length, 1)
 assert.deepEqual(sent[0], { requestId: (sent[0] as { requestId: string }).requestId, opportunityId: 'opp/1', snapshotId: 'snapshot ?1', evaluationId: 'eval-eligible', batchIdentity: '2026 秋招 A 批', continueDespiteHardFailure: false, expectedRevision: 4 })
 assert.ok((sent[0] as { requestId: string }).requestId)
 assert.match(container.textContent ?? '', /申请编号 app-1/)
 assert.match(container.textContent ?? '', /Task 已就绪/)
 assert.match(container.textContent ?? '', /task-9/)
 assert.match(container.textContent ?? '', /合格申请/)
 const pinned = container.querySelector('.wk-application__pinned')?.textContent ?? ''
 assert.match(pinned, /岗位/)
 assert.match(pinned, /opp\/1/)
 assert.match(pinned, /快照/)
 assert.match(pinned, /snapshot \?1/)
 assert.match(pinned, /评估/)
 assert.match(pinned, /eval-eligible/)
 assert.match(pinned, /档案修订/)
 assert.match(pinned, /档案修订4/)
 assert.match(pinned, /批次/)
 assert.match(pinned, /2026 秋招 A 批/)
 assert.match(window.location.search, /application=app-1/)
})

test('hard-ineligible evaluation blocks by default, keeps the warning resident after explicit continuation, and shows qualified=false', async () => {
 const sent: unknown[] = []
 const career: CareerStub & { createApplication: (input: unknown) => Promise<ApplicationReceipt> } = {
  open: async () => view,
  createApplication: async (input: unknown) => { sent.push(input); return { ...warnedReceipt, requestId: (input as { requestId: string }).requestId } },
 }
 const { container } = await mountApplication(career)
 await act(async () => { pick(container, 'eval-ineligible'); await settle() })
 const warning = container.querySelector('[role="alert"]')
 assert.ok(warning)
 assert.match(warning.textContent ?? '', /硬性条件不符/)
 const submit = byLabel(container, 'button', '创建申请') as HTMLButtonElement
 assert.equal(submit.disabled, true)
 const acknowledge = container.querySelector<HTMLInputElement>('[aria-label="我已知晓硬性条件不符，仍要显式继续申请"]')!
 await act(async () => { toggle(acknowledge); await settle() })
 assert.equal((byLabel(container, 'button', '创建申请') as HTMLButtonElement).disabled, false)
 await act(async () => { byLabel(container, 'button', '创建申请').click(); await settle(); await settle() })
 assert.equal((sent[0] as { continueDespiteHardFailure: boolean }).continueDespiteHardFailure, true)
 assert.equal((sent[0] as { evaluationId: string }).evaluationId, 'eval-ineligible')
 assert.match(container.textContent ?? '', /不计合格申请/)
 assert.ok(container.querySelector('[role="alert"]'))
 assert.match(container.textContent ?? '', /硬性条件警示/)
})

test('linking state stays visibly unready and reconciles with the original request ID into ready', async () => {
 const reconcileCalls: string[] = []
 let first = true
 const career: CareerStub = {
  open: async () => view,
  createApplication: async (input: { requestId: string }) => { if (first) { first = false; return { ...linkingReceipt, requestId: input.requestId } } return { ...readyReceipt, requestId: input.requestId } },
  reconcileApplicationLink: async (requestId: string) => { reconcileCalls.push(requestId); return { ...readyReceipt, requestId } },
 }
 const { container } = await mountApplication(career)
 await submitApplication(container, 'eval-eligible', '2026 秋招 A 批', career as CareerStub & { createApplication: (input: any) => Promise<ApplicationReceipt> })
 assert.match(container.textContent ?? '', /Task 关联中/)
 assert.match(container.textContent ?? '', /未就绪/)
 assert.doesNotMatch(container.textContent ?? '', /Task 已就绪/)
 await act(async () => { byLabel(container, 'button', '用原请求编号对账').click(); await settle(); await settle() })
 assert.equal(reconcileCalls.length, 1)
 assert.match(container.textContent ?? '', /Task 已就绪/)
 assert.match(container.textContent ?? '', /task-9/)
})

test('unknown create outcome recovers by the original request ID and an exact replay returns the one stored receipt', async () => {
 const creates: unknown[] = []
 const receiptReads: string[] = []
 let first = true
 const career: CareerStub & { createApplication: (input: any) => Promise<ApplicationReceipt> } = {
  open: async () => view,
  createApplication: async (input: { requestId: string }) => {
   creates.push(input)
   if (first) { first = false; throw Object.assign(new Error('gateway timeout'), { code: 'outcome_unknown', requestId: input.requestId }) }
   return { ...readyReceipt, requestId: input.requestId }
  },
  applicationReceipt: async (requestId: string) => { receiptReads.push(requestId); throw Object.assign(new Error('not found'), { code: 'not_found' }) },
 }
 const { container } = await mountApplication(career)
 await submitApplication(container, 'eval-eligible', '2026 秋招 A 批', career as CareerStub & { createApplication: (input: any) => Promise<ApplicationReceipt> })
 assert.match(container.textContent ?? '', /无法确认申请是否已创建/)
 await act(async () => { byLabel(container, 'button', '查询申请回执').click(); await settle(); await settle() })
 assert.deepEqual(receiptReads, [(creates[0] as { requestId: string }).requestId])
 assert.match(container.textContent ?? '', /尚未找到申请回执/)
 await act(async () => { byLabel(container, 'button', '用原请求编号重试').click(); await settle(); await settle() })
 assert.equal(creates.length, 2)
 assert.deepEqual(creates[1], creates[0])
 assert.match(container.textContent ?? '', /申请编号 app-1/)
 assert.match(container.textContent ?? '', /task-9/)
})

test('revision conflict shows the current revision and a refreshed resubmit uses a new request ID', async () => {
 const sent: Array<{ requestId: string; expectedRevision: number }> = []
 let first = true
 const career: CareerStub & { createApplication: (input: { requestId: string; expectedRevision: number }) => Promise<ApplicationReceipt> } = {
  open: async () => ({ ...view, revision: first ? 4 : 7 }),
  createApplication: async (input: { requestId: string; expectedRevision: number }) => {
   if (first) { first = false; throw Object.assign(new Error('revision conflict'), { code: 'revision_conflict', currentRevision: 7 }) }
   sent.push(input)
   return { ...readyReceipt, requestId: input.requestId }
  },
 }
 const { container } = await mountApplication(career)
 await submitApplication(container, 'eval-eligible', '2026 秋招 A 批', career)
 assert.match(container.textContent ?? '', /档案已更新/)
 assert.match(container.textContent ?? '', /当前修订 7/)
 await act(async () => { byLabel(container, 'button', '重新读取档案修订').click(); await settle(); await settle() })
 assert.match(container.textContent ?? '', /当前档案修订 7/)
 await act(async () => { byLabel(container, 'button', '创建申请').click(); await settle(); await settle() })
 assert.equal(sent.length, 1)
 assert.equal(sent[0]?.expectedRevision, 7)
 assert.notEqual(sent[0]?.requestId, undefined)
})

test('link_failed renders the definite failure and reconciles again with the original request ID', async () => {
 const reconcileCalls: string[] = []
 const career: CareerStub = {
  open: async () => view,
  createApplication: async (input: { requestId: string }) => ({ ...failedReceipt, requestId: input.requestId }),
  reconcileApplicationLink: async (requestId: string) => { reconcileCalls.push(requestId); return { ...failedReceipt, requestId } },
 }
 const { container } = await mountApplication(career)
 await submitApplication(container, 'eval-eligible', '2026 秋招 A 批', career as CareerStub & { createApplication: (input: any) => Promise<ApplicationReceipt> })
 assert.match(container.textContent ?? '', /Task 关联失败/)
 await act(async () => { byLabel(container, 'button', '重试对账').click(); await settle(); await settle() })
 assert.equal(reconcileCalls.length, 1)
 assert.match(container.textContent ?? '', /Task 关联失败/)
})

test('same job and batch already has an application and the UI reports the definite conflict', async () => {
 const career: CareerStub = {
  open: async () => view,
  createApplication: async () => { throw Object.assign(new Error('already applied'), { code: 'application_conflict' }) },
 }
 const { container } = await mountApplication(career)
 await submitApplication(container, 'eval-eligible', '2026 秋招 A 批', career as CareerStub & { createApplication: (input: any) => Promise<ApplicationReceipt> })
 assert.match(container.textContent ?? '', /已存在申请/)
 assert.doesNotMatch(container.textContent ?? '', /Task 已就绪/)
})

test('cross-tenant forbidden application surfaces a clear inaccessible state and keeps no receipt', async () => {
 const career: CareerStub = {
  open: async () => view,
  createApplication: async () => { throw Object.assign(new Error('forbidden'), { code: 'forbidden' }) },
 }
 const { container } = await mountApplication(career)
 await submitApplication(container, 'eval-eligible', '2026 秋招 A 批', career as CareerStub & { createApplication: (input: any) => Promise<ApplicationReceipt> })
 assert.match(container.textContent ?? '', /当前空间不可访问/)
 assert.doesNotMatch(container.textContent ?? '', /申请编号/)
})

test('a page reload restores the stored application from the URL without creating anything', async () => {
 window.history.replaceState({}, '', '/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1&application=app-9')
 const reads: string[] = []
 const career: CareerStub = {
  open: async () => view,
  application: async (applicationId: string) => { reads.push(applicationId); return { ...readyReceipt, applicationId, requestId: 'apply-9' } },
  createApplication: async () => { throw new Error('must not create on restore') },
 }
 const { container } = await mountApplication(career)
 assert.deepEqual(reads, ['app-9'])
 assert.match(container.textContent ?? '', /申请编号 app-9/)
 assert.match(container.textContent ?? '', /task-9/)
})

test('the same job applies to a different batch and a fresh request, while each receipt stays pinned', async () => {
 const sent: Array<{ requestId: string; batchIdentity: string }> = []
 let calls = 0
 const career: CareerStub & { createApplication: (input: { requestId: string; batchIdentity: string }) => Promise<ApplicationReceipt> } = {
  open: async () => view,
  createApplication: async (input: { requestId: string; batchIdentity: string }) => {
   sent.push(input); calls += 1
   return { ...readyReceipt, applicationId: `app-${calls}`, requestId: input.requestId, pinnedEvidence: { ...eligiblePin, batchIdentity: input.batchIdentity } }
  },
 }
 const { container } = await mountApplication(career)
 await submitApplication(container, 'eval-eligible', '2026 秋招 A 批', career)
 await act(async () => { byLabel(container, 'button', '为其他批次创建新申请').click(); await settle() })
 assert.equal(container.querySelector<HTMLInputElement>('[aria-label="招聘批次标识"]')?.value, '')
 await submitApplication(container, 'eval-eligible', '2026 春招补录', career)
 assert.equal(sent.length, 2)
 assert.equal(sent[0]?.batchIdentity, '2026 秋招 A 批')
 assert.equal(sent[1]?.batchIdentity, '2026 春招补录')
 assert.notEqual(sent[0]?.requestId, sent[1]?.requestId)
 assert.match(container.querySelector('.wk-application__pinned')?.textContent ?? '', /2026 春招补录/)
})

test('after a reload the stored application re-pins its evaluation so another batch can apply', async () => {
 window.history.replaceState({}, '', '/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1&application=app-9')
 const sent: Array<{ requestId: string; batchIdentity: string }> = []
 const career: CareerStub & { createApplication: (input: { requestId: string; batchIdentity: string }) => Promise<ApplicationReceipt> } = {
  open: async () => view,
  application: async () => ({ ...warnedReceipt, applicationId: 'app-9', requestId: 'apply-9' }),
  createApplication: async (input: { requestId: string; batchIdentity: string }) => { sent.push(input); return { ...readyReceipt, applicationId: 'app-10', requestId: input.requestId, pinnedEvidence: { ...eligiblePin, batchIdentity: input.batchIdentity } } },
 }
 // evaluations prop is empty: the page was reloaded and the in-memory history is gone.
 const { container } = await mountApplication(career, { evaluations: [] })
 assert.match(container.textContent ?? '', /申请编号 app-9/)
 await act(async () => { byLabel(container, 'button', '为其他批次创建新申请').click(); await settle() })
 assert.ok(container.querySelector('input[type="radio"][value="eval-ineligible"]'), 'stored receipt re-pins its evaluation as selectable')
 await act(async () => { pick(container, 'eval-ineligible'); await settle() })
 const acknowledge = container.querySelector<HTMLInputElement>('[aria-label="我已知晓硬性条件不符，仍要显式继续申请"]')!
 await act(async () => { toggle(acknowledge); await settle() })
 await act(async () => { setInput(container.querySelector<HTMLInputElement>('[aria-label="招聘批次标识"]')!, '2026 春招补录'); await settle() })
 await act(async () => { byLabel(container, 'button', '创建申请').click(); await settle(); await settle() })
 assert.equal(sent.length, 1)
 assert.equal(sent[0]?.batchIdentity, '2026 春招补录')
 assert.equal((sent[0] as unknown as { evaluationId: string }).evaluationId, 'eval-ineligible')
})

test('without any evaluation the entry explains that the snapshot must be evaluated first', async () => {
 const { container } = await mountApplication({ open: async () => view }, { evaluations: [] })
 assert.match(container.textContent ?? '', /先.*评估/)
 assert.equal((byLabel(container, 'button', '创建申请') as HTMLButtonElement).disabled, true)
})

test('scope switch clears the in-flight application state', async () => {
 const scope = createScopeController({ origin: 'https://weknora.test', userId: 'u', tenantId: 't' })
 const { container } = await mountApplication({ open: async () => view }, { scopeController: scope })
 await act(async () => { pick(container, 'eval-eligible'); await settle() })
 await act(async () => { scope.switchScope('https://weknora.test', 'other-user', 'other-tenant'); await settle() })
 assert.match(container.textContent ?? '', /空间已切换/)
 assert.equal(container.querySelectorAll('input[type="radio"]').length, 0)
})

test('application styles keep TDesign light surfaces, brand green confirmations and narrow-screen stacking', () => {
 const css = readFileSync(new URL('./application.css', import.meta.url), 'utf8')
 assert.match(css, /\.wk-application \{/)
 assert.match(css, /--td-bg-color-container/)
 assert.match(css, /wk-application__link--ready[\s\S]*?color: #07c05f/)
 assert.match(css, /@media \(max-width: 640px\)/)
 assert.match(css, /overflow-wrap: anywhere/)
})
