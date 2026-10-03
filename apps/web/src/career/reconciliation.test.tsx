import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import * as nodeModule from 'node:module'
import test, { afterEach } from 'node:test'
import * as React from 'react'
import { act } from 'react'
import type { Root } from 'react-dom/client'
import { createScopeController } from '@weknora/domain/scope'
import type { WeKnoraClient } from '@weknora/api-client'
import { ApiError } from '../../../../packages/api-client/src/errors.ts'
import type { CareerView, OpportunityEvidence } from '../../../../packages/career-core/src/contracts.ts'
import type { ApplicationReceipt, CareerCoverageView, OpportunityStatusView, ReconcileReceipt } from '../../../../packages/api-client/src/career.ts'

const moduleHooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void }
if (moduleHooks.registerHooks) moduleHooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) })

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } }
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/career/opportunities/opp%2F1?snapshotId=snapshot-1' })
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Element: dom.window.Element, MutationObserver: dom.window.MutationObserver, getComputedStyle: dom.window.getComputedStyle.bind(dom.window), Event: dom.window.Event, IS_REACT_ACT_ENVIRONMENT: true })
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator })
const { createRoot } = await import('react-dom/client')
const { CareerCoveragePanel, OpportunityStatusPanel } = await import('./reconciliation.tsx')
const { ApplicationPage } = await import('./ApplicationPage.tsx')

type CareerStub = Record<string, (...args: any[]) => unknown>
let root: Root | undefined
let host: HTMLDivElement | undefined
afterEach(async () => {
 if (root) await act(async () => root?.unmount())
 root = undefined; host?.remove(); host = undefined; document.body.replaceChildren()
 window.history.replaceState({}, '', '/platform/career/opportunities/opp%2F1?snapshotId=snapshot-1')
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
function pick(select: HTMLSelectElement, value: string) {
 const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(select), 'value')?.set
 setter?.call(select, value)
 select.dispatchEvent(new dom.window.Event('change', { bubbles: true }))
}

const scopeController = (): ReturnType<typeof createScopeController> => createScopeController({ origin: 'https://weknora.test', userId: 'u', tenantId: 't' })

const healthyObservation = { observationId: 'observation-1', snapshotId: 'snapshot-1', source: { kind: 'url', label: '招聘页面', referenceId: 'https://example.test/jd/12345' }, sourceStatus: 'complete' as const, completeness: 'complete' as const, needsUserJD: false, acquiredAt: '2026-09-20T08:00:00Z' }
const failedObservation = { observationId: 'observation-2', snapshotId: 'snapshot-2', source: { kind: 'url', referenceId: 'https://example.test/jd/12345' }, sourceStatus: 'fetch_failed' as const, needsUserJD: false, acquiredAt: '2026-09-26T09:00:00Z' }
const staleStatus: OpportunityStatusView = {
 opportunityId: 'opp/1', annotations: ['requirements_changed'], stale: true,
 lastCheckedAt: '2026-09-26T09:00:00Z', lastHealthyAt: '2026-09-20T08:00:00Z',
 observations: [healthyObservation, failedObservation],
}
const oldSnapshotEvidence: OpportunityEvidence = {
 opportunityId: 'opp/1', observationId: 'observation-1', snapshotId: 'snapshot-1',
 rawText: '旧版 JD', rawSha256: 'a'.repeat(64),
 extracted: { title: { state: 'known', value: '前端工程师' }, company: { state: 'known', value: '示例公司' }, location: { state: 'known', value: '上海' }, batch: { state: 'known', value: '2026 秋招' }, requirements: { state: 'known', value: '熟悉 React，本科及以上' } },
 source: { kind: 'url', label: '招聘页面', referenceId: 'https://example.test/jd/12345' }, acquiredAt: '2026-09-20T08:00:00Z', status: 'stored',
}
const newSnapshotEvidence: OpportunityEvidence = {
 ...oldSnapshotEvidence, observationId: 'observation-2', snapshotId: 'snapshot-2', rawText: '新版 JD：要求硕士', rawSha256: 'b'.repeat(64),
 extracted: { ...oldSnapshotEvidence.extracted, requirements: { state: 'known', value: '熟悉 React，硕士及以上' } },
 acquiredAt: '2026-09-26T09:00:00Z',
}

function mountStatus(career: CareerStub, opportunityId = 'opp/1'): HTMLElement {
 return render(React.createElement(OpportunityStatusPanel, { client: { career } as unknown as WeKnoraClient, scopeController: scopeController(), opportunityId }))
}

test('status panel annotates requirement changes, keeps the stale window explicit, and preserves every original link, check time, and permanent snapshot link', async () => {
 const container = mountStatus({ opportunityStatus: async () => staleStatus, opportunityReconciliations: async () => ({ reconciliations: [] }) })
 await act(async () => { await settle() })
 const text = container.textContent ?? ''
 assert.match(text, /要求已变化/)
 assert.match(text, /最近一次来源检查失败/)
 assert.match(text, /最后成功观察/)
 assert.ok(container.querySelector('time[dateTime="2026-09-20T08:00:00Z"]'), 'last healthy check time visible')
 assert.match(text, /https:\/\/example\.test\/jd\/12345/)
 assert.ok(container.querySelector('time[dateTime="2026-09-26T09:00:00Z"]'), 'failed check time still visible')
 const snapshotLinks = [...container.querySelectorAll<HTMLAnchorElement>('a')].filter((anchor) => anchor.textContent?.includes('固定快照'))
 assert.equal(snapshotLinks.length, 2, 'both history snapshots stay permanently openable')
 assert.equal(snapshotLinks[0]!.getAttribute('href'), '/platform/career/opportunities/opp%2F1?snapshotId=snapshot-1')
 assert.equal(snapshotLinks[1]!.getAttribute('href'), '/platform/career/opportunities/opp%2F1?snapshotId=snapshot-2')
})

test('status panel renders the before/after snapshot diff of an updated JD with explicit change markers', async () => {
 const container = mountStatus({
  opportunityStatus: async () => staleStatus,
  opportunityReconciliations: async () => ({ reconciliations: [] }),
  opportunityEvidence: async (_opportunityId: string, snapshotId: string) => snapshotId === 'snapshot-1' ? oldSnapshotEvidence : newSnapshotEvidence,
 })
 await act(async () => { await settle(); await settle() })
 const diff = container.querySelector<HTMLElement>('[aria-label="变化前后对比"]')
 assert.ok(diff, 'diff section exists')
 const text = diff.textContent ?? ''
 assert.match(text, /熟悉 React，本科及以上/)
 assert.match(text, /熟悉 React，硕士及以上/)
 assert.match(text, /已变化/)
 assert.match(text, /未变化/)
 const markers = [...diff.querySelectorAll('.wk-reconciliation__diff-marker')].map((node) => node.textContent?.trim())
 assert.ok(markers.includes('已变化'), 'changed field flagged')
 const openOld = byLabel(diff, 'a', '打开旧快照') as HTMLAnchorElement
 const openNew = byLabel(diff, 'a', '打开新快照') as HTMLAnchorElement
 assert.equal(openOld.getAttribute('href'), '/platform/career/opportunities/opp%2F1?snapshotId=snapshot-1')
 assert.equal(openNew.getAttribute('href'), '/platform/career/opportunities/opp%2F1?snapshotId=snapshot-2')
 // The raw texts of both snapshots render side by side so an updated JD is
 // readable even when field extraction has no known values.
 assert.match(text, /旧版 JD/)
 assert.match(text, /新版 JD：要求硕士/)
 assert.match(text, /原文内容已变化/)
})

test('reconcile action displays a sufficient-evidence merge with the conflicting-batches disclosure and both identity quadruples', async () => {
 const merged: ReconcileReceipt = {
  kind: 'opportunities_reconciled', requestId: 'reconcile-1', decision: 'merged', targetId: 'opp/1', candidateId: 'opp/2', suspectedDuplicate: false,
  evidence: {
   target: { jobCode: '12345', title: '前端工程师', company: '示例公司', location: '上海', batch: '2026 秋招', sourceRef: 'https://example.test/jd/12345', snapshotId: 'snapshot-1' },
   candidate: { jobCode: '12345', title: '前端工程师', company: '示例公司', location: '上海', batch: '2026 秋招', sourceRef: 'https://mirror.test/jd/12345', snapshotId: 'snapshot-9' },
  },
  conflictingBatches: ['2026 秋招'], createdAt: '2026-09-26T10:00:00Z',
 }
 const sent: unknown[] = []
 const container = mountStatus({
  opportunityStatus: async () => staleStatus,
  opportunityReconciliations: async () => ({ reconciliations: [] }),
  reconcileOpportunities: async (input: { requestId: string }) => { sent.push(input); return { ...merged, requestId: input.requestId } },
 })
 await act(async () => { await settle() })
 const input = container.querySelector<HTMLInputElement>('[aria-label="候选记录编号"]')!
 await act(async () => { setInput(input, 'opp/2') })
 await act(async () => { byLabel(container, 'button', '对账判定').click(); await settle() })
 assert.deepEqual(sent, [{ requestId: (sent[0] as { requestId: string }).requestId, targetId: 'opp/1', candidateId: 'opp/2' }])
 const text = container.textContent ?? ''
 assert.match(text, /已合并/)
 assert.match(text, /历史观察与快照全部保留/)
 assert.match(text, /冲突批次/)
 assert.match(text, /2026 秋招/)
 assert.match(text, /12345/)
 assert.match(text, /https:\/\/mirror\.test\/jd\/12345/)
})

test('an uncertain pair stays side by side with the suspected-duplicate hint visible and nothing merged', async () => {
 const sideBySide: ReconcileReceipt = {
  kind: 'opportunities_reconciled', requestId: 'reconcile-2', decision: 'side_by_side', targetId: 'opp/1', candidateId: 'opp/3', suspectedDuplicate: true,
  evidence: {
   target: { jobCode: '12345', title: '前端工程师', company: '示例公司' },
   candidate: { jobCode: '67890', title: '前端工程师', company: '示例公司' },
  },
  createdAt: '2026-09-26T10:05:00Z',
 }
 const container = mountStatus({
  opportunityStatus: async () => staleStatus,
  opportunityReconciliations: async () => ({ reconciliations: [] }),
  reconcileOpportunities: async (input: { requestId: string }) => ({ ...sideBySide, requestId: input.requestId }),
 })
 await act(async () => { await settle() })
 await act(async () => { setInput(container.querySelector<HTMLInputElement>('[aria-label="候选记录编号"]')!, 'opp/3') })
 await act(async () => { byLabel(container, 'button', '对账判定').click(); await settle() })
 const text = container.textContent ?? ''
 assert.match(text, /不确定重复/)
 assert.match(text, /并列保留/)
 assert.match(text, /疑似同一岗位/)
 assert.match(text, /未提供/)
})

test('an unknown reconcile outcome recovers through the original request receipt without resubmitting content', async () => {
 const recovered: ReconcileReceipt = {
  kind: 'opportunities_reconciled', requestId: 'reconcile-3', decision: 'side_by_side', targetId: 'opp/1', candidateId: 'opp/4', suspectedDuplicate: false,
  evidence: { target: { jobCode: '12345' }, candidate: { jobCode: '' } },
  createdAt: '2026-09-26T10:10:00Z',
 }
 const posts: unknown[] = []
 let failOnce = true
 const container = mountStatus({
  opportunityStatus: async () => staleStatus,
  opportunityReconciliations: async () => ({ reconciliations: [] }),
  reconcileOpportunities: async (input: unknown) => { posts.push(input); if (failOnce) { failOnce = false; throw new ApiError({ code: 'TIMEOUT', message: 'Request timed out' }) } throw new Error('must not resubmit after unknown') },
  reconciliationReceipt: async (requestId: string) => ({ ...recovered, requestId }),
 })
 await act(async () => { await settle() })
 await act(async () => { setInput(container.querySelector<HTMLInputElement>('[aria-label="候选记录编号"]')!, 'opp/4') })
 await act(async () => { byLabel(container, 'button', '对账判定').click(); await settle() })
 assert.equal(posts.length, 1, 'exactly one submission attempt')
 assert.match(container.textContent ?? '', /暂时无法确认/)
 await act(async () => { byLabel(container, 'button', '查询原请求回执').click(); await settle() })
 assert.match(container.textContent ?? '', /判定时间/)
 assert.match(container.textContent ?? '', /并列展示/)
 assert.equal(posts.length, 1, 'recovery replayed the receipt instead of resubmitting')
})

test('an idempotency conflict is surfaced as an error while the request stays reviewable', async () => {
 const container = mountStatus({
  opportunityStatus: async () => staleStatus,
  opportunityReconciliations: async () => ({ reconciliations: [] }),
  reconcileOpportunities: async () => { throw new ApiError({ code: 'idempotency_conflict', message: 'request id reused for other content' }) },
 })
 await act(async () => { await settle() })
 await act(async () => { setInput(container.querySelector<HTMLInputElement>('[aria-label="候选记录编号"]')!, 'opp/5') })
 await act(async () => { byLabel(container, 'button', '对账判定').click(); await settle() })
 const alert = container.querySelector('[role="alert"]')
 assert.ok(alert)
 assert.match(alert.textContent ?? '', /请求编号已对应其他对账内容/)
})

test('the merged-away record discloses where its history went', async () => {
 const container = mountStatus({
  opportunityStatus: async (requested: string) => ({ ...staleStatus, opportunityId: requested, mergedInto: 'opp/0', annotations: [], stale: false, observations: [] }),
  opportunityReconciliations: async () => ({ reconciliations: [] }),
 }, 'opp/9')
 await act(async () => { await settle() })
 assert.match(container.textContent ?? '', /此岗位记录已并入/)
 assert.match(container.textContent ?? '', /opp\/0/)
})

test('a never-successful check states that honestly instead of rendering the Go zero timestamp', async () => {
 const container = mountStatus({
  opportunityStatus: async () => ({ ...staleStatus, annotations: [], lastHealthyAt: '0001-01-01T00:00:00Z', observations: [failedObservation] }),
  opportunityReconciliations: async () => ({ reconciliations: [] }),
 })
 await act(async () => { await settle() })
 assert.match(container.textContent ?? '', /尚无成功观察记录/)
 assert.doesNotMatch(container.querySelector('.wk-reconciliation__stale')?.textContent ?? '', /0001/)
})

test('the reconciliation history lists prior decisions with their times and pairs', async () => {
 const prior: ReconcileReceipt = {
  kind: 'opportunities_reconciled', requestId: 'reconcile-0', decision: 'side_by_side', targetId: 'opp/1', candidateId: 'opp/8', suspectedDuplicate: true,
  evidence: { target: { jobCode: '12345' }, candidate: { jobCode: '99999' } },
  createdAt: '2026-09-25T10:00:00Z',
 }
 const container = mountStatus({ opportunityStatus: async () => staleStatus, opportunityReconciliations: async () => ({ reconciliations: [prior] }) })
 await act(async () => { await settle() })
 const history = container.querySelector('[aria-label="对账历史"]')
 assert.ok(history)
 assert.match(history.textContent ?? '', /2026-09-25T10:00:00Z/)
 assert.match(history.textContent ?? '', /opp\/8/)
 assert.match(history.textContent ?? '', /不确定重复/)
})

test('coverage panel discloses configured and observed sources plus observed cities, with empty lists stated honestly', async () => {
 const rich: CareerCoverageView = {
  configuredSources: [{ sourceId: 's1', label: '已核验来源', accessMethods: ['http'], cities: ['上海'], available: true }],
  observedSources: [{ sourceKind: 'url', label: '招聘页面', observations: 2, lastCheckedAt: '2026-09-26T09:00:00Z' }],
  observedCities: ['上海'],
 }
 const container = render(React.createElement(CareerCoveragePanel, { client: { career: { careerCoverage: async () => rich } } as unknown as WeKnoraClient, scopeController: scopeController() }))
 await act(async () => { await settle() })
 const text = container.textContent ?? ''
 assert.match(text, /已核验来源/)
 assert.match(text, /招聘页面/)
 assert.match(text, /2 次观察/)
 assert.match(text, /上海/)
 const empty = render(React.createElement(CareerCoveragePanel, { client: { career: { careerCoverage: async () => ({ configuredSources: [], observedSources: [], observedCities: [] }) } } as unknown as WeKnoraClient, scopeController: scopeController() }))
 await act(async () => { await settle() })
 const emptyText = empty.textContent ?? ''
 assert.match(emptyText, /暂无已核验来源/)
 assert.match(emptyText, /暂无观察记录/)
 assert.match(emptyText, /暂无覆盖城市记录/)
})

test('a restored application links its pinned evidence to the permanently openable old snapshot', async () => {
 window.history.replaceState({}, '', '/platform/career/opportunities/opp%2F1?snapshotId=snapshot-1&application=app-1')
 const pinnedReceipt: ApplicationReceipt = { applicationId: 'app-1', requestId: 'apply-1', linkState: 'ready', qualified: true, pinnedEvidence: { opportunityId: 'opp/1', snapshotId: 'snapshot-1', evaluationId: 'eval-1', profileRevision: 4, evaluationStatus: 'eligible', batchIdentity: '2026 秋招' } }
 const view: CareerView = { revision: 4, facts: [], proposals: [] }
 const container = render(React.createElement(ApplicationPage, { client: { career: { open: async () => view, application: async () => pinnedReceipt } } as unknown as WeKnoraClient, scopeController: scopeController(), opportunityId: 'opp/1', snapshotId: 'snapshot-1', evaluations: [], batchHint: { state: 'unknown' } }))
 await act(async () => { await settle() })
 const pinned = container.querySelector('.wk-application__pinned')
 assert.ok(pinned)
 const snapshotLink = pinned.querySelector<HTMLAnchorElement>('a[href="/platform/career/opportunities/opp%2F1?snapshotId=snapshot-1"]')
 assert.ok(snapshotLink, 'pinned snapshot opens its fixed evidence page')
 assert.match(snapshotLink.textContent ?? '', /snapshot-1/)
})

test('a scope switch clears the whole reconcile surface: receipt, attempt, and candidate draft never leak into the new space', async () => {
 const merged: ReconcileReceipt = {
  kind: 'opportunities_reconciled', requestId: 'reconcile-scope', decision: 'merged', targetId: 'opp/1', candidateId: 'opp/2', suspectedDuplicate: false,
  evidence: {
   target: { jobCode: '12345', title: '前端工程师', company: '示例公司', location: '上海', batch: '2026 秋招' },
   candidate: { jobCode: '12345', title: '前端工程师', company: '示例公司', location: '上海', batch: '2026 秋招' },
  },
  createdAt: '2026-09-26T11:00:00Z',
 }
 const controller = scopeController()
 const sent: { requestId: string; targetId: string; candidateId: string }[] = []
 const container = render(React.createElement(OpportunityStatusPanel, { client: { career: {
  opportunityStatus: async () => staleStatus,
  opportunityReconciliations: async () => ({ reconciliations: [] }),
  reconcileOpportunities: async (input: { requestId: string; targetId: string; candidateId: string }) => { sent.push(input); return { ...merged, requestId: input.requestId } },
 } } as unknown as WeKnoraClient, scopeController: controller, opportunityId: 'opp/1' }))
 await act(async () => { await settle() })
 await act(async () => { setInput(container.querySelector<HTMLInputElement>('[aria-label="候选记录编号"]')!, 'opp/2') })
 await act(async () => { byLabel(container, 'button', '对账判定').click(); await settle() })
 assert.match(container.textContent ?? '', /已合并/)
 assert.ok(container.querySelector('.wk-reconciliation__receipt'), 'receipt rendered in the original space')
 // Switching identity resets the whole reconcile surface together with the
 // read state: the old-space receipt must not render again, and no kept
 // attempt may replay the old reconcile request against the new space.
 await act(async () => { controller.switchScope('https://weknora.test', 'u-2', 't-2'); await settle(); await settle() })
 assert.ok(container.querySelector('[aria-label="候选记录编号"]'), 'panel reloaded for the new space')
 assert.equal(container.querySelector('.wk-reconciliation__receipt'), null, 'old-space receipt no longer rendered')
 assert.equal(container.querySelector<HTMLInputElement>('[aria-label="候选记录编号"]')?.value, '', 'candidate draft cleared')
 await act(async () => { setInput(container.querySelector<HTMLInputElement>('[aria-label="候选记录编号"]')!, 'opp/9') })
 await act(async () => { byLabel(container, 'button', '对账判定').click(); await settle() })
 assert.equal(sent.length, 2)
 assert.notEqual(sent[1]?.requestId, sent[0]?.requestId, 'the new space reconciles under a fresh request ID, never the leaked one')
})

test('an unknown reconcile attempt survives a remount through per-scope storage (CAREER-OCR H11)', async () => {
 const recovered: ReconcileReceipt = {
  kind: 'opportunities_reconciled', requestId: 'reconcile-remount', decision: 'side_by_side', targetId: 'opp/1', candidateId: 'opp/4', suspectedDuplicate: false,
  evidence: { target: { jobCode: '12345' }, candidate: { jobCode: '' } },
  createdAt: '2026-09-26T10:10:00Z',
 }
 const posts: unknown[] = []
 const career: CareerStub = {
  opportunityStatus: async () => staleStatus,
  opportunityReconciliations: async () => ({ reconciliations: [] }),
  reconcileOpportunities: async (input: unknown) => { posts.push(input); throw new ApiError({ code: 'TIMEOUT', message: 'Request timed out' }) },
  reconciliationReceipt: async (requestId: string) => ({ ...recovered, requestId }),
 }
 const first = mountStatus(career)
 await act(async () => { await settle() })
 await act(async () => { setInput(first.querySelector<HTMLInputElement>('[aria-label="候选记录编号"]')!, 'opp/4') })
 await act(async () => { byLabel(first, 'button', '对账判定').click(); await settle() })
 assert.match(first.textContent ?? '', /暂时无法确认/)
 assert.ok([...Object.keys(dom.window.sessionStorage)].some((key) => key.startsWith('weknora:career:reconcile-attempt:')), 'the unknown attempt is persisted per scope')
 await act(async () => { root?.unmount(); root = undefined; host?.remove(); host = undefined; document.body.replaceChildren() })

 const second = mountStatus(career)
 await act(async () => { await settle() })
 assert.match(second.textContent ?? '', /有一次结果未知的对账/, 'the unknown state survives the remount')
 assert.match(second.textContent ?? '', /原请求编号/, 'the original request id is restored')
 await act(async () => { byLabel(second, 'button', '查询原请求回执').click(); await settle() })
 assert.match(second.textContent ?? '', /并列展示/)
 assert.ok(![...Object.keys(dom.window.sessionStorage)].some((key) => key.startsWith('weknora:career:reconcile-attempt:')), 'accepting the receipt clears the persisted attempt')
})
