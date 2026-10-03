import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import * as nodeModule from 'node:module'
import test, { afterEach } from 'node:test'
import * as React from 'react'
import { act } from 'react'
import type { Root } from 'react-dom/client'
import { createScopeController } from '@weknora/domain/scope'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ApplicationReceipt, ReminderReceipt, ReminderView, SetReminderInput } from '../../../../packages/api-client/src/career.ts'
import type { CareerAction, CareerReceipt, CareerView } from '../../../../packages/career-core/src/contracts.ts'

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void }
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) })

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } }
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/career' })
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Element: dom.window.Element, MutationObserver: dom.window.MutationObserver, getComputedStyle: dom.window.getComputedStyle.bind(dom.window), Event: dom.window.Event, IS_REACT_ACT_ENVIRONMENT: true })
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator })
const { createRoot } = await import('react-dom/client')
const { InboxPage } = await import('./InboxPage.tsx')

const ts = '2026-09-26T09:15:00Z'
const frozenProgressNotice = '你有新的求职进展，请登录查看。'
const frozenDiscoveryNotice = '持续找岗有新发现，请登录查看。'
const confirmFact = (key: string, value: string, revision: number, requestId = 'sub-req'): CareerReceipt => ({ kind: 'confirmed', requestId, revision, fact: { key, value, revision, source: { kind: 'user', label: '本人确认' }, confirmation: { userId: 'owner-1', confirmedAt: ts }, confirmedAt: ts } })
const view = (revision: number, pushValue?: string): CareerView => ({ revision, facts: pushValue ? [{ key: 'notifications.push', value: pushValue, revision, source: { kind: 'user', label: '本人确认' }, confirmation: { userId: 'owner-1', confirmedAt: ts }, confirmedAt: ts }] : [], proposals: [] })
const progressTodo = (extra: Partial<ReminderView> = {}): ReminderView => ({ reminderId: 'rem-1', sourceKind: 'progress_event', sourceId: 'evt /1', applicationId: 'app /1', opportunityId: 'opp /1', noticeKey: 'progress_updated', notice: frozenProgressNotice, status: 'open', createdAt: ts, ...extra })
const discoveryTodo = (extra: Partial<ReminderView> = {}): ReminderView => ({ reminderId: 'rem-2', sourceKind: 'discovery', sourceId: 'todo /1', noticeKey: 'discovery_found', notice: frozenDiscoveryNotice, status: 'open', createdAt: ts, ...extra })
const application = (extra: Partial<ApplicationReceipt> = {}): ApplicationReceipt => ({ applicationId: 'app /1', requestId: 'app-req', linkState: 'ready', qualified: true, pinnedEvidence: { opportunityId: 'opp /1', snapshotId: 'snap /1', evaluationId: 'eval-1', profileRevision: 3, evaluationStatus: 'eligible', batchIdentity: 'batch-1' }, ...extra })
const reminderReceipt = (extra: Partial<ReminderReceipt> = {}): ReminderReceipt => ({ kind: 'reminder_set', requestId: 'rem-req-1', reminderId: 'rem-1', sourceKind: 'progress_event', sourceId: 'evt /1', deduplicated: false, applicationId: 'app /1', opportunityId: 'opp /1', noticeKey: 'progress_updated', notice: frozenProgressNotice, status: 'open', revision: 4, createdAt: ts, ...extra })

type CareerStub = Record<string, (...args: any[]) => unknown>
let root: Root | undefined
let host: HTMLDivElement | undefined
afterEach(async () => {
 if (root) await act(async () => root?.unmount())
 root = undefined; host?.remove(); host = undefined; document.body.replaceChildren()
})
async function mount(career: CareerStub) {
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 host = document.createElement('div'); document.body.append(host); root = createRoot(host)
 act(() => root!.render(React.createElement(InboxPage, { client: { career } as unknown as WeKnoraClient, scopeController })))
 await act(async () => { await settle(); await settle(); await settle() })
 return host!
}
async function settle() { await new Promise((resolve) => setImmediate(resolve)) }
function byText(container: HTMLElement, selector: string, pattern: RegExp): HTMLElement {
 const found = [...container.querySelectorAll<HTMLElement>(selector)].find((item) => pattern.test(item.textContent ?? ''))
 assert.ok(found, `${selector} matching ${pattern} exists`)
 return found
}
function button(container: HTMLElement, label: string): HTMLButtonElement {
 const found = [...container.querySelectorAll<HTMLButtonElement>('button')].find((item) => item.textContent?.trim() === label)
 assert.ok(found, `button ${label} exists`)
 return found
}
function click(el: HTMLElement) { act(() => { el.dispatchEvent(new dom.window.Event('click', { bubbles: true })) }) }
function setInput(input: HTMLInputElement, value: string) {
 const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(input), 'value')?.set
 setter?.call(input, value)
 input.dispatchEvent(new dom.window.Event('input', { bubbles: true }))
}
function choose(select: HTMLSelectElement, value: string) {
 const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(select), 'value')?.set
 setter?.call(select, value)
 select.dispatchEvent(new dom.window.Event('change', { bubbles: true }))
}
async function submitTodo(container: HTMLElement, sourceId: string, sourceKind = 'progress_event') {
 choose(container.querySelector<HTMLSelectElement>('#wk-inbox-source-kind')!, sourceKind)
 setInput(container.querySelector<HTMLInputElement>('#wk-inbox-source-id')!, sourceId)
 await act(async () => { click(button(container, '登记待办')); await settle(); await settle() })
}

test('inbox lists todos with only the frozen privacy notices and enters the authoritative application', async () => {
 const applicationReads: string[] = []
 const career: CareerStub = {
  open: async () => view(4),
  reminders: async () => ({ reminders: [progressTodo(), discoveryTodo()] }),
  application: async (id: string) => { applicationReads.push(id); return application() },
 }
 const container = await mount(career)
 assert.deepEqual(applicationReads, ['app /1'])
 assert.match(container.textContent ?? '', /推送只是提醒/)
 assert.match(container.textContent ?? '', /站内待办才是事实源/)
 const rows = [...container.querySelectorAll<HTMLElement>('[aria-label="站内待办列表"] > li')]
 assert.equal(rows.length, 2)
 // Privacy property: every todo body is exactly one of the two frozen
 // literals — any interpolated company, job or interview detail breaks this.
 const notices = [...container.querySelectorAll<HTMLElement>('.wk-inbox__notice')].map((item) => item.textContent ?? '')
 assert.deepEqual(notices.sort(), [frozenDiscoveryNotice, frozenProgressNotice].sort())
 assert.doesNotMatch(rows[0]!.textContent ?? '', /公司|岗位|面试|字节|工程师/)
 // The progress-event todo deep-links into the authoritative application
 // detail: the pinned snapshot is resolved through the application receipt.
 const link = rows[0]!.querySelector<HTMLAnchorElement>('a[href*="/platform/career/opportunities/"]')
 assert.ok(link, 'authoritative application link exists')
 assert.equal(link!.getAttribute('href'), '/platform/career/opportunities/opp%20%2F1?snapshotId=snap%20%2F1&application=app%20%2F1')
 assert.match(link!.textContent ?? '', /进入权威申请/)
 // The discovery todo has no application; it points at the search surface.
 assert.equal(rows[1]!.querySelector('a[href*="/opportunities/"]'), null)
 assert.match(rows[1]!.querySelector('a')?.getAttribute('href') ?? '', /^\/platform\/career\/search$/)
})

test('a reminders response already in flight cannot restore private rows after deletion generation advances', async () => {
 let resolveReminders!: (value: { reminders: ReminderView[] }) => void
 let generation = 0
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 const client = { career: {
  open: async () => view(4), reminders: () => new Promise((resolve) => { resolveReminders = resolve }), application: async () => application(),
 } } as unknown as WeKnoraClient
 host = document.createElement('div'); document.body.append(host); root = createRoot(host)
 act(() => root!.render(React.createElement(InboxPage, { client, scopeController, deletionGeneration: generation })))
 await act(async () => { await settle() })
 assert.match(host.textContent ?? '', /正在读取站内待办/)
 generation++
 await act(async () => { root!.render(React.createElement(InboxPage, { client, scopeController, deletionGeneration: generation })); await settle() })
 assert.match(host.textContent ?? '', /个人求职空间已删除/)
 await act(async () => { resolveReminders({ reminders: [progressTodo()] }); await settle() })
 assert.doesNotMatch(host.textContent ?? '', /新的求职进展/)
 assert.equal(host.querySelector('[aria-label="站内待办列表"]'), null)
})

test('a duplicate trigger reports the dedupe and never renders a second row', async () => {
 let todos: ReminderView[] = [progressTodo()]
 const sent: SetReminderInput[] = []
 const career: CareerStub = {
  open: async () => view(4),
  reminders: async () => ({ reminders: todos }),
  application: async () => application(),
  setReminder: async (input: SetReminderInput) => { sent.push(input); return reminderReceipt({ requestId: input.requestId, deduplicated: true }) },
 }
 const container = await mount(career)
 await submitTodo(container, 'evt /1')
 assert.equal(sent.length, 1)
 assert.match(container.textContent ?? '', /该来源已有待办/)
 assert.match(container.textContent ?? '', /未新增第二条/)
 // Re-triggering the same source again still leaves one row in the list.
 await submitTodo(container, 'evt /1')
 const rows = [...container.querySelectorAll<HTMLElement>('[aria-label="站内待办列表"] > li')]
 assert.equal(rows.length, 1)
 assert.notEqual(sent[0]!.requestId, sent[1]!.requestId)
})

test('a push delivery failure never loses the in-station todo', async () => {
 let todos: ReminderView[] = []
 const career: CareerStub = {
  open: async () => view(4),
  reminders: async () => ({ reminders: todos }),
  application: async () => application(),
  setReminder: async (input: SetReminderInput) => {
   todos = [progressTodo()]
   return reminderReceipt({ requestId: input.requestId, push: { attempted: true, delivered: false, reason: 'delivery_failed' } })
  },
 }
 const container = await mount(career)
 await submitTodo(container, 'evt /1')
 assert.match(container.textContent ?? '', /推送投递失败/)
 assert.match(container.textContent ?? '', /站内为准/)
 assert.equal(container.querySelectorAll('[aria-label="站内待办列表"] > li').length, 1)
 assert.match(container.textContent ?? '', /推送只是提醒/)
})

test('unsubscribing stops push, keeps the todos readable and offers resubscription', async () => {
 let pushValue = 'subscribed'
 const acts: CareerAction[] = []
 const todos = [progressTodo(), discoveryTodo()]
 const career: CareerStub = {
  open: async () => view(5, pushValue),
  reminders: async () => ({ reminders: todos }),
  application: async () => application(),
  act: async (action: CareerAction) => { acts.push(action); pushValue = (action as { value: string }).value; return confirmFact('notifications.push', pushValue, 6, action.requestId) },
 }
 const container = await mount(career)
 assert.match(container.querySelector('[aria-label="推送提醒状态"]')?.textContent ?? '', /已订阅推送提醒/)
 await act(async () => { click(button(container, '退订推送提醒')); await settle(); await settle() })
 assert.equal(acts.length, 1)
 assert.equal(acts[0]!.action, 'confirm')
 assert.equal((acts[0] as { key: string }).key, 'notifications.push')
 assert.equal((acts[0] as { value: string }).value, 'unsubscribed')
 assert.match(container.querySelector('[aria-label="推送提醒状态"]')?.textContent ?? '', /已退订/)
 assert.match(container.textContent ?? '', /站内待办不受影响|仍可读取/)
 assert.equal(container.querySelectorAll('[aria-label="站内待办列表"] > li').length, 2)
 assert.ok(button(container, '重新订阅推送提醒'))
})

test('subscription retry reuses the full original action after refresh advances the profile revision', async () => {
 const sent: CareerAction[] = []
 let opens = 0
 const career: CareerStub = {
  open: async () => view(++opens === 1 ? 4 : 9), reminders: async () => ({ reminders: [] }), application: async () => application(),
  act: async (action: CareerAction) => { sent.push(action); if (sent.length === 1) throw Object.assign(new Error('timeout'), { code: 'outcome_unknown' }); return confirmFact('notifications.push', 'unsubscribed', 10, action.requestId) },
  receipt: async () => { throw Object.assign(new Error('not found'), { code: 'not_found' }) },
 }
 const container = await mount(career)
 await act(async () => { click(button(container, '退订推送提醒')); await settle(); await settle() })
 await act(async () => { click(button(container, '刷新待办')); await settle(); await settle(); await settle() })
 assert.equal(opens, 2, 'explicit refresh reopens the profile at the new revision')
 assert.match(container.textContent ?? '', /已订阅推送提醒/, 'the refreshed view is rendered before retry')
 await act(async () => { click(button(container, '查询待办回执')); await settle(); await settle() })
 await act(async () => { click(button(container, '用原请求编号重试')); await settle(); await settle() })
 assert.equal(sent.length, 2)
 assert.deepEqual(sent[1], sent[0], 'retry preserves every action field, including request ID and frozen revision')
 assert.equal(sent[0]!.expectedRevision, 4)
 assert.equal((sent[0] as CareerAction).requestId?.length! > 0, true)
})

test('a revision conflict on the reminder write refreshes the pinned revision', async () => {
 let revision = 4
 const sent: SetReminderInput[] = []
 const career: CareerStub = {
  open: async () => { revision = 9; return view(revision) },
  reminders: async () => ({ reminders: [] }),
  setReminder: async (input: SetReminderInput) => {
   sent.push(input)
   if (sent.length === 1) throw Object.assign(new Error('revision moved'), { code: 'revision_conflict', currentRevision: 9 })
   return reminderReceipt({ requestId: input.requestId, revision: input.expectedRevision })
  },
 }
 const container = await mount(career)
 await submitTodo(container, 'evt /1')
 assert.match(container.textContent ?? '', /档案已更新/)
 assert.match(container.textContent ?? '', /当前修订 9/)
 await submitTodo(container, 'evt /1')
 assert.equal(sent[1]!.expectedRevision, 9)
})

test('an unknown write outcome recovers through the receipt of the original request id', async () => {
 const sent: SetReminderInput[] = []
 const lookedUp: string[] = []
 let todos: ReminderView[] = []
 const career: CareerStub = {
  open: async () => view(4),
  reminders: async () => ({ reminders: todos }),
  setReminder: async (input: SetReminderInput) => {
   sent.push(input)
   if (sent.length === 1) {
    // The write actually committed; only the HTTP answer was lost.
    todos = [progressTodo()]
    throw Object.assign(new Error('timed out'), { code: 'outcome_unknown', requestId: input.requestId })
   }
   todos = [progressTodo()]
   return reminderReceipt({ requestId: input.requestId })
  },
  reminderReceipt: async (requestId: string) => { lookedUp.push(requestId); return reminderReceipt({ requestId }) },
 }
 const container = await mount(career)
 await submitTodo(container, 'evt /1')
 assert.match(container.textContent ?? '', /暂时无法确认/)
 assert.match(container.textContent ?? '', new RegExp(sent[0]!.requestId))
 await act(async () => { click(button(container, '查询待办回执')); await settle(); await settle() })
 assert.deepEqual(lookedUp, [sent[0]!.requestId])
 assert.equal(container.querySelectorAll('[aria-label="站内待办列表"] > li').length, 1)
 assert.doesNotMatch(container.textContent ?? '', /暂时无法确认/)
})

test('cross-tenant reads answer a scoped error and missing sources answer a typed failure', async () => {
 const career: CareerStub = {
  open: async () => view(4),
  reminders: async () => { throw Object.assign(new Error('wrong tenant'), { code: 'forbidden' }) },
 }
 const container = await mount(career)
 assert.match(container.textContent ?? '', /当前空间不可访问/)
 assert.ok(button(container, '重新读取'))

 const careerNotFound: CareerStub = {
  open: async () => view(4),
  reminders: async () => ({ reminders: [] }),
  setReminder: async () => { throw Object.assign(new Error('no such source'), { code: 'not_found' }) },
 }
 const second = await mount(careerNotFound)
 await submitTodo(second, 'evt-x')
 assert.match(second.textContent ?? '', /来源事件不存在/)
 assert.match(second.textContent ?? '', /不属于当前空间|不属于本空间/)
})

test('an unknown write survives a remount through per-scope storage (CAREER-OCR H10)', async () => {
 const sent: SetReminderInput[] = []
 let todos: ReminderView[] = []
 const career: CareerStub = {
  open: async () => view(4),
  reminders: async () => ({ reminders: todos }),
  setReminder: async (input: SetReminderInput) => { sent.push(input); todos = [progressTodo()]; throw Object.assign(new Error('timed out'), { code: 'outcome_unknown', requestId: input.requestId }) },
  reminderReceipt: async (requestId: string) => { todos = [progressTodo()]; return reminderReceipt({ requestId }) },
 }
 const storage = dom.window.sessionStorage
 const first = await mount(career)
 await submitTodo(first, 'evt /1')
 assert.match(first.textContent ?? '', /暂时无法确认/)
 const requestId = sent[0]!.requestId
 assert.ok([...Object.keys(storage)].some((key) => key.startsWith('weknora:career:inbox-write:')), 'the unknown attempt is persisted per scope')
 await act(async () => { root?.unmount(); root = undefined; host?.remove(); host = undefined; document.body.replaceChildren() })

 const second = await mount(career)
 assert.match(second.textContent ?? '', /有一次结果未知的写入/, 'the unknown state survives the remount')
 assert.match(second.textContent ?? '', new RegExp(requestId), 'the original request id is restored')
 await act(async () => { click(button(second, '查询待办回执')); await settle(); await settle() })
 assert.doesNotMatch(second.textContent ?? '', /暂时无法确认/)
 assert.ok(![...Object.keys(storage)].some((key) => key.startsWith('weknora:career:inbox-write:')), 'accepting the receipt clears the persisted attempt')
})
