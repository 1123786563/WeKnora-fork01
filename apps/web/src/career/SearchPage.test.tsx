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
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/career/search' })
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Element: dom.window.Element, MutationObserver: dom.window.MutationObserver, getComputedStyle: dom.window.getComputedStyle.bind(dom.window), Event: dom.window.Event, IS_REACT_ACT_ENVIRONMENT: true })
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator })
const { createRoot } = await import('react-dom/client')
const { CareerSearchPage } = await import('./SearchPage.tsx')

type SearchCall = { requestId: string; query: string; expectedRevision: number }
const view = (revision: number) => ({ revision, facts: [], proposals: [] })
const completedReceipt = {
  kind: 'search_once' as const, requestId: 'r-1', searchId: 'search-1', status: 'completed' as const, query: '上海 前端 实习',
  coverage: { sources: [{ sourceId: 'board-1', label: '校园招聘看板', accessMethods: ['public_listing'], cities: ['上海'], available: true }] },
  scopeNotes: [], failureCode: undefined,
  results: [
    { resultId: 'result-1', sourceId: 'board-1', link: 'https://jobs.example.test/1', checkedAt: '2026-09-25T08:00:00Z', qualification: 'needs_review' as const, uncertainty: 'low_confidence' as const },
    { resultId: 'result-2', sourceId: 'board-1', link: 'https://jobs.example.test/2', checkedAt: '2026-09-25T08:00:01Z', qualification: 'qualified' as const, uncertainty: 'low_confidence' as const },
  ],
  checkedAt: '2026-09-25T08:00:00Z',
}
const failedReceipt = {
  kind: 'search_once' as const, requestId: 'r-fail', searchId: 'search-fail', status: 'failed' as const, query: '找岗',
  coverage: { sources: [] }, scopeNotes: ['no vetted search source is configured; nothing was fetched and no result is fabricated', 'coverage reflects only actually vetted sources'],
  failureCode: 'no_vetted_sources' as const, results: [], checkedAt: '2026-09-25T08:00:00Z',
}
const importReceipt = { kind: 'opportunity_url_imported', requestId: 'imp-1', opportunityId: 'opp-9', observationId: 'obs-9', snapshotId: 'snap ?9', status: 'needs_review', sourceStatus: 'policy_unverified', completeness: 'unknown', submittedUrl: 'https://jobs.example.test/1', acquiredAt: '2026-09-25T08:05:00Z', needsUserJD: true }

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
async function mountSearch(career: CareerStubs, scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'u-1', tenantId: 't-1' })) {
  const container = render(React.createElement(CareerSearchPage, { client: { career } as unknown as WeKnoraClient, scopeController }))
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
// TDesign renders a disabled submit Button as <div type="submit"
// class="t-button t-is-disabled"> (same as CareerPage); locate the submit
// control by its type attribute instead of the button tag.
function submitControl(container: HTMLElement): HTMLElement {
 const found = [...container.querySelectorAll<HTMLElement>('[type="submit"]')].find((item) => item.textContent?.includes('找岗'))
 assert.ok(found, 'submit control exists')
 return found
}
async function submitQuery(container: HTMLElement, query: string) {
  const textarea = container.querySelector<HTMLTextAreaElement>('[aria-label="找岗指令"]')!
  await act(async () => { setInput(textarea, query) })
  await act(async () => { byLabel(container, 'button', '找岗（一次性）').click(); await settle() })
}

test('runs one search per instruction and renders truthful coverage with per-row evidence', async () => {
  const searches: SearchCall[] = []
  const { container } = await mountSearch({
    open: async () => view(7),
    searchOnce: async (input: SearchCall) => { searches.push(input); return { ...completedReceipt, requestId: input.requestId } },
  })
  await submitQuery(container, '上海 前端 实习')
  assert.equal(searches.length, 1)
  assert.equal(searches[0]?.query, '上海 前端 实习')
  assert.equal(searches[0]?.expectedRevision, 7)
  assert.match(searches[0]?.requestId ?? '', /.+/)
  const coverage = container.querySelector('[aria-label="来源覆盖"]')!
  assert.match(coverage.textContent ?? '', /校园招聘看板/)
  assert.match(coverage.textContent ?? '', /上海/)
  const results = container.querySelector('[aria-label="找岗结果"]')!
  const rows = results.querySelectorAll('li')
  assert.equal(rows.length, 2)
  const link = rows[0]!.querySelector<HTMLAnchorElement>('a[href="https://jobs.example.test/1"]')!
  assert.equal(link.getAttribute('target'), '_blank')
  assert.match(rows[0]!.textContent ?? '', /2026-09-25 08:00/)
  assert.match(rows[0]!.textContent ?? '', /待人工判断/)
  assert.match(rows[0]!.textContent ?? '', /低置信度/)
  assert.match(rows[1]!.textContent ?? '', /符合/)
})

test('shows the honest empty-coverage failure without nationwide claims or demo rows', async () => {
  const { container } = await mountSearch({
    open: async () => view(3),
    searchOnce: async (input: SearchCall) => ({ ...failedReceipt, requestId: input.requestId }),
  })
  await submitQuery(container, '找岗')
  const coverage = container.querySelector('[aria-label="来源覆盖"]')!
  assert.match(coverage.textContent ?? '', /暂无已核验来源/)
  assert.match(coverage.textContent ?? '', /no vetted search source is configured/)
  assert.match(container.textContent ?? '', /没有生成演示/)
  const alert = container.querySelector('[role="alert"]')!
  assert.match(alert.textContent ?? '', /暂无已核验来源/)
  const results = container.querySelector('[aria-label="找岗结果"]')!
  assert.match(results.textContent ?? '', /没有结果/)
  assert.equal(results.querySelectorAll('a').length, 0)
  assert.doesNotMatch(container.textContent ?? '', /全国/)
})

test('recovers an unknown outcome through the original request ID without duplicating the search', async () => {
  const searches: SearchCall[] = []
  const receiptReads: string[] = []
  let first = true
  const { container } = await mountSearch({
    open: async () => view(2),
    searchOnce: async (input: SearchCall) => {
      searches.push(input)
      if (first) { first = false; throw Object.assign(new Error('gateway timeout'), { code: 'outcome_unknown', requestId: input.requestId }) }
      return { ...completedReceipt, requestId: input.requestId }
    },
    searchReceipt: async (requestId: string) => { receiptReads.push(requestId); throw Object.assign(new Error('not found'), { code: 'not_found' }) },
  })
  await submitQuery(container, '上海 前端 实习')
  const originalId = searches[0]?.requestId
  assert.ok(originalId)
  const unknownPanel = container.querySelector('[role="status"]')!
  assert.match(unknownPanel.textContent ?? '', /找岗结果暂时未知/)
  assert.match(unknownPanel.textContent ?? '', new RegExp(originalId))
  await act(async () => { byLabel(container, 'button', '查询回执').click(); await settle() })
  assert.deepEqual(receiptReads, [originalId])
  assert.match(container.textContent ?? '', /暂未找到回执/)
  await act(async () => { byLabel(container, 'button', '用原请求编号重试').click(); await settle() })
  assert.equal(searches.length, 2)
  assert.equal(searches[1]?.requestId, originalId)
  assert.equal(searches[1]?.query, '上海 前端 实习')
  assert.ok(container.querySelector('[aria-label="找岗结果"]')!.querySelector('a[href="https://jobs.example.test/1"]'))
})

test('a terminal search is never duplicated; a fresh attempt gets a fresh request ID', async () => {
  const searches: SearchCall[] = []
  const { container } = await mountSearch({
    open: async () => view(1),
    searchOnce: async (input: SearchCall) => { searches.push(input); return { ...completedReceipt, requestId: input.requestId } },
  })
  await submitQuery(container, 'first query')
  const savedButton = submitControl(container)
  assert.match(savedButton.textContent ?? '', /已找岗（一次性）/)
  assert.ok(savedButton.classList.contains('t-is-disabled') || (savedButton as HTMLButtonElement).disabled === true)
  await act(async () => { savedButton.click(); await settle() })
  assert.equal(searches.length, 1)
  await act(async () => { byLabel(container, 'button', '开始新的一次找岗').click(); await settle() })
  const textarea = container.querySelector<HTMLTextAreaElement>('[aria-label="找岗指令"]')!
  assert.equal(textarea.value, '')
  await submitQuery(container, 'second query')
  assert.equal(searches.length, 2)
  assert.equal(searches[1]?.requestId === searches[0]?.requestId, false)
  assert.equal(searches[1]?.query, 'second query')
})

test('revision conflict surfaces the current revision and retries the same request under the refreshed revision', async () => {
  const searches: SearchCall[] = []
  let openCalls = 0
  let first = true
  const { container } = await mountSearch({
    open: async () => { openCalls += 1; return view(openCalls === 1 ? 5 : 6) },
    searchOnce: async (input: SearchCall) => {
      searches.push(input)
      if (first) { first = false; throw Object.assign(new Error('revision conflict'), { code: 'revision_conflict', currentRevision: 6 }) }
      return { ...completedReceipt, requestId: input.requestId }
    },
  })
  await submitQuery(container, '上海 前端 实习')
  const originalId = searches[0]?.requestId
  const alert = container.querySelector('[role="alert"]')!
  assert.match(alert.textContent ?? '', /档案已更新/)
  assert.match(alert.textContent ?? '', /当前修订 6/)
  await act(async () => { byLabel(container, 'button', '按当前修订重试（原请求编号）').click(); await settle() })
  assert.equal(searches.length, 2)
  assert.equal(searches[1]?.requestId, originalId)
  assert.equal(searches[1]?.expectedRevision, 6)
  assert.ok(container.querySelector('[aria-label="找岗结果"]'))
})

test('quota refusal stays typed and recoverable with the same request ID', async () => {
  const searches: SearchCall[] = []
  let first = true
  const { container } = await mountSearch({
    open: async () => view(0),
    searchOnce: async (input: SearchCall) => {
      searches.push(input)
      if (first) { first = false; throw Object.assign(new Error('quota refused'), { code: 'search_quota_refused' }) }
      return { ...completedReceipt, requestId: input.requestId }
    },
  })
  await submitQuery(container, '找岗')
  const alert = container.querySelector('[role="alert"]')!
  assert.match(alert.textContent ?? '', /找岗额度受限/)
  const originalId = searches[0]?.requestId
  await act(async () => { byLabel(container, 'button', '稍后用原请求编号重试').click(); await settle() })
  assert.equal(searches.length, 2)
  assert.equal(searches[1]?.requestId, originalId)
  assert.ok(container.querySelector('[aria-label="找岗结果"]'))
})

test('cross-scope denial clears the page and a scope switch clears cached search state', async () => {
  const forbiddenPage = await mountSearch({ open: async () => { throw Object.assign(new Error('forbidden'), { code: 'forbidden' }) } })
  assert.match(forbiddenPage.container.textContent ?? '', /当前空间不可访问/)
  assert.equal(forbiddenPage.container.querySelector('[aria-label="找岗指令"]'), null)

  const live = await mountSearch({
    open: async () => view(4),
    searchOnce: async (input: SearchCall) => ({ ...completedReceipt, requestId: input.requestId }),
    search: async () => completedReceipt,
  })
  await submitQuery(live.container, '上海 前端 实习')
  assert.ok(live.container.querySelector('[aria-label="找岗结果"]')!.querySelector('a'))
  await act(async () => { live.scopeController.switchScope('https://weknora.test', 'u-2', 't-2'); await settle() })
  assert.match(live.container.textContent ?? '', /空间已切换/)
  assert.equal(live.container.querySelector('[aria-label="找岗结果"]'), null)
})

test('history survives reload and reopens the stored search by its search ID', async () => {
  const { container } = await mountSearch({
    open: async () => view(9),
    searchOnce: async (input: SearchCall) => ({ ...completedReceipt, requestId: input.requestId, query: input.query }),
  })
  await submitQuery(container, '历史指令 A')
  await act(async () => { root?.unmount(); await settle() })
  document.body.replaceChildren()

  const reads: string[] = []
  const reopened = await mountSearch({
    open: async () => view(9),
    search: async (searchId: string) => { reads.push(searchId); return { ...completedReceipt, query: '历史指令 A' } },
  })
  const history = reopened.container.querySelector('[aria-label="历史找岗"]')!
  assert.match(history.textContent ?? '', /历史指令 A/)
  await act(async () => { byLabel(reopened.container, 'button', '历史指令 A').click(); await settle() })
  // One read on mount (auto-reload of the newest stored search) and one for
  // the explicit reopen — both under the original search ID, never a re-run.
  assert.deepEqual(reads, ['search-1', 'search-1'])
  assert.ok(reopened.container.querySelector('[aria-label="找岗结果"]')!.querySelector('a[href="https://jobs.example.test/1"]'))
})

test('importing a result row opens the existing fixed evidence page without auto-fetching', async () => {
  const imports: Array<{ requestId: string; url: string }> = []
  const { container } = await mountSearch({
    open: async () => view(2),
    searchOnce: async (input: SearchCall) => ({ ...completedReceipt, requestId: input.requestId }),
    importUrl: async (input: { requestId: string; url: string }) => { imports.push(input); return importReceipt },
  })
  await submitQuery(container, '上海 前端 实习')
  assert.equal(imports.length, 0)
  await act(async () => { byLabel(container, 'button', '导入为岗位证据').click(); await settle() })
  assert.equal(imports.length, 1)
  assert.equal(imports[0]?.url, 'https://jobs.example.test/1')
  const evidence = byLabel(container, 'a', '查看岗位证据') as HTMLAnchorElement
  assert.equal(evidence.getAttribute('href'), '/platform/career/opportunities/opp-9?snapshotId=snap%20%3F9')
})

test('offers no continuous-search control of any kind', async () => {
  const { container } = await mountSearch({ open: async () => view(0) })
  assert.match(container.textContent ?? '', /一次性/)
  const ruleControls = container.querySelectorAll('input[type="checkbox"], input[type="radio"], [role="switch"], [role="checkbox"]')
  assert.equal(ruleControls.length, 0)
})
