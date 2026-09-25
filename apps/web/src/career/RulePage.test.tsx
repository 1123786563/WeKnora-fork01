import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import * as nodeModule from 'node:module'
import test, { afterEach } from 'node:test'
import * as React from 'react'
import { act } from 'react'
import type { Root } from 'react-dom/client'
import { createScopeController } from '@weknora/domain/scope'
import type { WeKnoraClient } from '@weknora/api-client'

const moduleHooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void }
if (moduleHooks.registerHooks) moduleHooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) })

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } }
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/career/rules' })
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Element: dom.window.Element, MutationObserver: dom.window.MutationObserver, getComputedStyle: dom.window.getComputedStyle.bind(dom.window), Event: dom.window.Event, IS_REACT_ACT_ENVIRONMENT: true })
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator })
const { createRoot } = await import('react-dom/client')
const { CareerRulePage } = await import('./RulePage.tsx')

type RuleWrite = { requestId: string; ruleId?: string; query: string; intervalMinutes: number; status: string; expectedRevision: number }
const view = (revision: number) => ({ revision, facts: [], proposals: [] })
const estimate = { triggersPerDay: 1, sourcesPerTrigger: 0, estimatedSearchesPerDay: 0, basis: 'deterministic projection: triggers_per_day = 1440 / interval_minutes; estimated_searches_per_day = triggers_per_day × vetted sources per trigger; this is an estimate from rule parameters, not a quota balance' }
const createdDisabled = { kind: 'rule_set' as const, requestId: 'rule-create', ruleId: 'rule-1', query: '上海 前端 实习', intervalMinutes: 1440, status: 'disabled' as const, revision: 1, estimate }
const enabledReceipt = { ...createdDisabled, requestId: 'rule-enable', status: 'enabled' as const, revision: 2, nextDueAt: '2026-09-26T08:00:00Z' }
const modifiedReceipt = { ...enabledReceipt, requestId: 'rule-modify', query: '杭州 后端', intervalMinutes: 60, revision: 3, nextDueAt: '2026-09-26T09:00:00Z' }
const pausedReceipt = { ...modifiedReceipt, requestId: 'rule-pause', status: 'paused' as const, revision: 4 }
const ruleView = (overrides: Record<string, unknown> = {}) => ({
 ruleId: 'rule-1', query: '杭州 后端', intervalMinutes: 60, status: 'paused', revision: 4, lastPeriod: 2, estimate,
 runs: [
  { kind: 'rule_run', ruleId: 'rule-1', period: 1, requestId: 'rule:rule-1:1', status: 'no_vetted_sources', note: 'no vetted search source is configured; the trigger was not searched and nothing was fabricated', triggeredAt: '2026-09-25T08:00:00Z' },
  { kind: 'rule_run', ruleId: 'rule-1', period: 2, requestId: 'rule:rule-1:2', status: 'blocked_no_quota', note: 'the search quota gate refused admission for this trigger; no search was consumed and nothing was fabricated', triggeredAt: '2026-09-25T22:00:00Z' },
 ],
 todos: [
  { todoId: 'todo-1', ruleId: 'rule-1', runId: 'run-1', searchId: 'search-1', link: 'https://jobs.example.test/1', status: 'open', createdAt: '2026-09-25T08:00:01Z' },
 ],
 createdAt: '2026-09-24T08:00:00Z', updatedAt: '2026-09-25T22:00:00Z',
 ...overrides,
})

let root: Root | undefined
let host: HTMLDivElement | undefined
afterEach(async () => {
 if (root) await act(async () => root?.unmount())
 root = undefined; host?.remove(); host = undefined
 document.body.replaceChildren()
 window.localStorage.clear()
})
function render(element: React.ReactNode): HTMLDivElement {
 host = document.createElement('div'); document.body.append(host); root = createRoot(host); act(() => root!.render(element)); return host
}
type CareerStubs = Record<string, (...args: any[]) => unknown>
async function mountRules(career: CareerStubs, scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'u-1', tenantId: 't-1' })) {
 const container = render(React.createElement(CareerRulePage, { client: { career } as unknown as WeKnoraClient, scopeController }))
 await act(async () => { await new Promise((resolve) => setImmediate(resolve)) })
 return { container, scopeController }
}
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
async function settle() { await new Promise((resolve) => setImmediate(resolve)) }
async function saveRule(container: HTMLElement, query: string, interval: string, status: 'enabled' | 'paused' | 'disabled') {
 const queryBox = container.querySelector<HTMLTextAreaElement>('[aria-label="找岗条件"]')!
 const intervalBox = container.querySelector<HTMLInputElement>('[aria-label="触发间隔（分钟）"]')!
 const radio = container.querySelector<HTMLInputElement>(`input[type="radio"][value="${status}"]`)!
 await act(async () => {
  setInput(queryBox, query)
  setInput(intervalBox, interval)
  radio.click()
 })
 await act(async () => { byLabel(container, 'button', '保存规则').click(); await settle() })
}

test('creates a rule disabled by default and shows conditions, frequency and the backend estimate before enabling', async () => {
 const writes: RuleWrite[] = []
 const { container } = await mountRules({
  open: async () => view(5),
  setRule: async (input: RuleWrite) => { writes.push(input); return { ...createdDisabled, requestId: input.requestId } },
  getRule: async () => ruleView(),
 })
 await saveRule(container, '上海 前端 实习', '1440', 'disabled')
 assert.equal(writes.length, 1)
 assert.equal(writes[0]?.status, 'disabled')
 assert.equal(writes[0]?.query, '上海 前端 实习')
 assert.equal(writes[0]?.intervalMinutes, 1440)
 assert.equal(writes[0]?.expectedRevision, 5)
 const panel = container.querySelector('[aria-label="规则状态"]')!
 assert.match(panel.textContent ?? '', /上海 前端 实习/)
 assert.match(panel.textContent ?? '', /每 1440 分钟/)
 const estimateSection = container.querySelector('[aria-label="预计消耗"]')!
 assert.match(estimateSection.textContent ?? '', /每天预计触发 1 次/)
 assert.match(estimateSection.textContent ?? '', /每次触发检索 0 个已核验来源/)
 assert.match(estimateSection.textContent ?? '', /deterministic projection/)
 assert.match(estimateSection.textContent ?? '', /not a quota balance/)
 const plan = container.querySelector('[aria-label="下次运行计划"]')!
 assert.match(plan.textContent ?? '', /规则未启用/)
 assert.match(container.textContent ?? '', /默认不开启/)
 assert.match(container.textContent ?? '', /不会在后台运行/)
})

test('enabling schedules the next run; modifying the rule updates the displayed run plan', async () => {
 const writes: RuleWrite[] = []
 let phase = 0
 const { container } = await mountRules({
  open: async () => view(5),
  setRule: async (input: RuleWrite) => {
   writes.push(input)
   phase += 1
   return phase === 1 ? { ...createdDisabled, requestId: input.requestId } : phase === 2 ? { ...enabledReceipt, requestId: input.requestId } : { ...modifiedReceipt, requestId: input.requestId }
  },
  getRule: async () => ruleView(),
 })
 await saveRule(container, '上海 前端 实习', '1440', 'disabled')
 await saveRule(container, '上海 前端 实习', '1440', 'enabled')
 const plan = container.querySelector('[aria-label="下次运行计划"]')!
 assert.match(plan.textContent ?? '', /2026-09-26 08:00 UTC/)
 assert.equal(writes[1]?.status, 'enabled')
 assert.equal(writes[1]?.ruleId, 'rule-1')
 await saveRule(container, '杭州 后端', '60', 'enabled')
 const updated = container.querySelector('[aria-label="下次运行计划"]')!
 assert.match(updated.textContent ?? '', /2026-09-26 09:00 UTC/)
 assert.doesNotMatch(updated.textContent ?? '', /08:00 UTC/)
 assert.match(container.querySelector('[aria-label="规则状态"]')!.textContent ?? '', /杭州 后端/)
 assert.match(container.querySelector('[aria-label="规则状态"]')!.textContent ?? '', /每 60 分钟/)
})

test('pausing cancels the next trigger visibly and a paused plan defers on resume', async () => {
 const writes: RuleWrite[] = []
 let phase = 0
 const { container } = await mountRules({
  open: async () => view(5),
  setRule: async (input: RuleWrite) => {
   writes.push(input)
   phase += 1
   return phase === 1 ? { ...createdDisabled, requestId: input.requestId } : phase === 2 ? { ...enabledReceipt, requestId: input.requestId } : { ...pausedReceipt, requestId: input.requestId }
  },
  getRule: async () => ruleView(),
 })
 await saveRule(container, '上海 前端 实习', '1440', 'disabled')
 await saveRule(container, '上海 前端 实习', '1440', 'enabled')
 await saveRule(container, '杭州 后端', '60', 'paused')
 const plan = container.querySelector('[aria-label="下次运行计划"]')!
 assert.match(plan.textContent ?? '', /已暂停/)
 assert.match(plan.textContent ?? '', /下一次触发已取消/)
 assert.match(plan.textContent ?? '', /恢复启用后.*重新排程/)
 assert.doesNotMatch(plan.textContent ?? '', /UTC/)
 assert.equal(writes[2]?.status, 'paused')
})

test('run history shows blocked statuses with their backend notes, never silent skips', async () => {
 window.localStorage.setItem('weknora:career:rule-id:u-1:t-1', 'rule-1')
 const { container } = await mountRules({
  open: async () => view(3),
  getRule: async () => ruleView(),
 })
 const history = container.querySelector('[aria-label="执行历史"]')!
 const rows = history.querySelectorAll('li')
 assert.equal(rows.length, 2)
 assert.match(rows[0]!.textContent ?? '', /第 1 次/)
 assert.match(rows[0]!.textContent ?? '', /暂无已核验来源/)
 assert.match(rows[0]!.textContent ?? '', /no vetted search source is configured/)
 assert.match(rows[1]!.textContent ?? '', /额度不足/)
 assert.match(rows[1]!.textContent ?? '', /the search quota gate refused admission/)
 assert.doesNotMatch(history.textContent ?? '', /静默/)
})

test('discovery todos render one row per todo with the job link', async () => {
 window.localStorage.setItem('weknora:career:rule-id:u-1:t-1', 'rule-1')
 const { container } = await mountRules({
  open: async () => view(3),
  getRule: async () => ruleView({ todos: [
   { todoId: 'todo-1', ruleId: 'rule-1', runId: 'run-1', searchId: 'search-1', sourceId: 'board-1', link: 'https://jobs.example.test/1', status: 'open', createdAt: '2026-09-25T08:00:01Z' },
   { todoId: 'todo-2', ruleId: 'rule-1', runId: 'run-2', searchId: 'search-2', link: 'https://jobs.example.test/2', status: 'open', createdAt: '2026-09-25T22:00:01Z' },
  ] }),
 })
 const todos = container.querySelector('[aria-label="发现待办"]')!
 const rows = todos.querySelectorAll('li')
 assert.equal(rows.length, 2)
 const links = [...todos.querySelectorAll<HTMLAnchorElement>('a')].map((anchor) => anchor.getAttribute('href'))
 assert.deepEqual(links, ['https://jobs.example.test/1', 'https://jobs.example.test/2'])
 assert.match(todos.textContent ?? '', /同一岗位链接只生成一条/)
})

test('an unknown write outcome recovers through the original request ID without a duplicate rule write', async () => {
 const writes: RuleWrite[] = []
 const receiptReads: string[] = []
 let first = true
 const { container } = await mountRules({
  open: async () => view(5),
  setRule: async (input: RuleWrite) => {
   writes.push(input)
   if (first) { first = false; throw Object.assign(new Error('gateway timeout'), { code: 'outcome_unknown', requestId: input.requestId }) }
   return { ...createdDisabled, requestId: input.requestId }
  },
  ruleReceipt: async (requestId: string) => { receiptReads.push(requestId); throw Object.assign(new Error('not found'), { code: 'not_found' }) },
  getRule: async () => ruleView(),
 })
 await saveRule(container, '上海 前端 实习', '1440', 'disabled')
 const originalId = writes[0]?.requestId
 assert.ok(originalId)
 const unknownPanel = container.querySelector('[role="status"]')!
 assert.match(unknownPanel.textContent ?? '', /保存结果暂时未知/)
 assert.match(unknownPanel.textContent ?? '', new RegExp(originalId))
 await act(async () => { byLabel(container, 'button', '查询回执').click(); await settle() })
 assert.deepEqual(receiptReads, [originalId])
 assert.match(container.textContent ?? '', /暂未找到回执/)
 await act(async () => { byLabel(container, 'button', '用原请求编号重试').click(); await settle() })
 assert.equal(writes.length, 2)
 assert.equal(writes[1]?.requestId, originalId)
 assert.equal(writes[1]?.status, 'disabled')
 assert.ok(container.querySelector('[aria-label="规则状态"]'))
})

test('revision conflict surfaces the current revision and retries the same request refreshed', async () => {
 const writes: RuleWrite[] = []
 let openCalls = 0
 let first = true
 const { container } = await mountRules({
  open: async () => { openCalls += 1; return view(openCalls === 1 ? 5 : 6) },
  setRule: async (input: RuleWrite) => {
   writes.push(input)
   if (first) { first = false; throw Object.assign(new Error('revision conflict'), { code: 'revision_conflict', currentRevision: 6 }) }
   return { ...createdDisabled, requestId: input.requestId }
  },
  getRule: async () => ruleView(),
 })
 await saveRule(container, '上海 前端 实习', '1440', 'disabled')
 const originalId = writes[0]?.requestId
 const alert = container.querySelector('[role="alert"]')!
 assert.match(alert.textContent ?? '', /档案已更新/)
 assert.match(alert.textContent ?? '', /当前修订 6/)
 await act(async () => { byLabel(container, 'button', '按当前修订重试（原请求编号）').click(); await settle() })
 assert.equal(writes.length, 2)
 assert.equal(writes[1]?.requestId, originalId)
 assert.equal(writes[1]?.expectedRevision, 6)
 assert.ok(container.querySelector('[aria-label="规则状态"]'))
})

test('a stored rule that is no longer visible is reported and the stale reference cleared', async () => {
 window.localStorage.setItem('weknora:career:rule-id:u-1:t-1', 'rule-gone')
 const reads: string[] = []
 const { container } = await mountRules({
  open: async () => view(3),
  getRule: async (ruleId: string) => { reads.push(ruleId); throw Object.assign(new Error('not found'), { code: 'not_found' }) },
 })
 assert.deepEqual(reads, ['rule-gone'])
 assert.match(container.textContent ?? '', /这条规则在服务端已不可见/)
 assert.equal(window.localStorage.getItem('weknora:career:rule-id:u-1:t-1'), null)
 assert.ok(container.querySelector('[aria-label="找岗条件"]'))
})

test('cross-scope denial shows the forbidden state and a scope switch clears cached rule state', async () => {
 const forbiddenPage = await mountRules({ open: async () => { throw Object.assign(new Error('forbidden'), { code: 'forbidden' }) } })
 assert.match(forbiddenPage.container.textContent ?? '', /当前空间不可访问/)
 assert.equal(forbiddenPage.container.querySelector('[aria-label="找岗条件"]'), null)

 const live = await mountRules({
  open: async () => view(4),
  setRule: async (input: RuleWrite) => ({ ...createdDisabled, requestId: input.requestId }),
  getRule: async () => ruleView(),
 })
 await saveRule(live.container, '上海 前端 实习', '1440', 'disabled')
 assert.ok(live.container.querySelector('[aria-label="规则状态"]'))
 assert.ok(window.localStorage.getItem('weknora:career:rule-id:u-1:t-1'))
 await act(async () => { live.scopeController.switchScope('https://weknora.test', 'u-2', 't-2'); await settle() })
 assert.match(live.container.textContent ?? '', /空间已切换/)
 assert.equal(live.container.querySelector('[aria-label="规则状态"]'), null)
 assert.equal(window.localStorage.getItem('weknora:career:rule-id:u-1:t-1'), null)
})
