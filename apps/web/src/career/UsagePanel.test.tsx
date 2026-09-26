import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import * as nodeModule from 'node:module'
import test, { afterEach } from 'node:test'
import * as React from 'react'
import { act, useEffect, useMemo, useState } from 'react'
import type { Root } from 'react-dom/client'
import { createScopeController } from '@weknora/domain/scope'
import type { WeKnoraClient } from '@weknora/api-client'
import type { UsageEstimateView } from '../../../../packages/api-client/src/career.ts'

const moduleHooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void }
if (moduleHooks.registerHooks) moduleHooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) })

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } }
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/career/usage' })
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Element: dom.window.Element, MutationObserver: dom.window.MutationObserver, getComputedStyle: dom.window.getComputedStyle.bind(dom.window), Event: dom.window.Event, IS_REACT_ACT_ENVIRONMENT: true })
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator })
const { createRoot } = await import('react-dom/client')
const { CareerUsagePanel, useCareerUsageEstimate } = await import('./UsagePanel.tsx')

// T21 pre-execution quota estimate panel. The estimate is the backend's
// frozen read-only projection: the panel displays it verbatim (cost,
// conditions, live monthly balance) and never recomputes anything. When the
// estimate cannot be obtained the panel states the typed reason and offers
// no execute-first path; an exhausted window blocks the next charged run
// while every stored archive stays readable.
const conditions = [
 '额度在执行前预占：每个 search_once（含规则触发的周期 Run）执行前预占 1 个单位，并在执行前向你展示本预估',
 '预占以 requestId 幂等：同一 requestId 重放或重试不会重复预占或收费',
 '预占在搜索终态后结算；已预占但从未执行的请求在租约过期后自动释放，不占余额',
 '额度按 UTC 自然月重置；本期额度耗尽时只阻止新的收费 Run，既有档案、申请、评估与搜索记录永远可读',
 '付费状态不改变岗位排序或资格判定：评估与排序输入不含任何付费维度',
]
const estimate = (overrides: Partial<UsageEstimateView> = {}): UsageEstimateView => ({
 kind: 'usage_estimate', operation: 'search_once', costUnits: 1, conditions,
 periodStart: '2026-09-01T00:00:00Z', periodEnd: '2026-10-01T00:00:00Z',
 limitUnits: 50, reservedUnits: 1, settledUnits: 2, remainingUnits: 47, wouldAdmit: true,
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
function Host({ career, scopeController }: { career: CareerStubs; scopeController: ReturnType<typeof createScopeController> }): React.ReactNode {
 // The real WeKnoraClient is one stable object for the page's lifetime; a
 // fresh literal per render would re-trigger the estimate effect endlessly.
 const client = useMemo(() => ({ career }) as unknown as WeKnoraClient, [career])
 // Real pages re-render when the live scope aborts (their own abort
 // listener clears page state); the host mirrors that so the hook re-reads
 // under the new identity exactly like SearchPage/RulePage do.
 const [, bump] = useState(0)
 useEffect(() => {
  const activeScope = scopeController.current()
  const rerender = () => bump((current) => current + 1)
  activeScope.signal?.addEventListener('abort', rerender, { once: true })
  return () => activeScope.signal?.removeEventListener('abort', rerender)
 }, [scopeController])
 const usage = useCareerUsageEstimate(client, scopeController)
 return <CareerUsagePanel usage={usage.state} onRetry={usage.reload} />
}
async function mountPanel(career: CareerStubs, scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'u-1', tenantId: 't-1' })) {
 const container = render(React.createElement(Host, { career, scopeController }))
 await act(async () => { await new Promise((resolve) => setImmediate(resolve)) })
 return { container, scopeController }
}
function byLabel(container: HTMLElement, selector: string, label: string): HTMLElement {
 const found = [...container.querySelectorAll<HTMLElement>(selector)].find((item) => item.textContent?.trim() === label)
 assert.ok(found, `${selector} “${label}” exists`)
 return found
}
async function settle() { await new Promise((resolve) => setImmediate(resolve)) }

test('shows the pre-execution estimate verbatim: cost, frozen conditions and the live monthly balance', async () => {
 const reads: string[] = []
 const { container } = await mountPanel({ usageEstimate: async (operation: string) => { reads.push(operation); return estimate() } })
 assert.deepEqual(reads, ['search_once'])
 const panel = container.querySelector('[aria-label="额度预估"]')!
 assert.match(panel.textContent ?? '', /将消耗 1 个额度单位/)
 assert.match(panel.textContent ?? '', /额度在执行前预占/)
 assert.match(panel.textContent ?? '', /同一 requestId 重放或重试不会重复预占或收费/)
 assert.match(panel.textContent ?? '', /既有档案、申请、评估与搜索记录永远可读/)
 assert.match(panel.textContent ?? '', /付费状态不改变岗位排序或资格判定/)
 assert.match(panel.textContent ?? '', /本期剩余 47 \/ 50 个额度单位/)
 assert.match(panel.textContent ?? '', /2026-09-01 至 2026-10-01 UTC/)
 assert.match(panel.textContent ?? '', /已预占 1 · 已结算 2/)
 // The balance line comes from the backend projection only — the panel never
 // recomputes remaining = limit - reserved - settled on its own.
 assert.doesNotMatch(panel.textContent ?? '', /46/)
})

test('an exhausted window is the overage state: new charged runs are blocked, archives stay readable', async () => {
 const { container } = await mountPanel({ usageEstimate: async () => estimate({ reservedUnits: 0, settledUnits: 50, remainingUnits: 0, wouldAdmit: false }) })
 const panel = container.querySelector('[aria-label="额度预估"]')!
 assert.match(panel.textContent ?? '', /本期额度已耗尽/)
 assert.match(panel.textContent ?? '', /新的收费找岗已被阻止/)
 assert.match(panel.textContent ?? '', /既有档案、申请、评估与搜索记录仍可完整读取/)
 assert.match(panel.querySelector('[role="alert"]')?.textContent ?? '', /本期额度已耗尽/)
})

test('an unreadable ledger is fail-closed: the typed reason is shown and no execute-first path is offered', async () => {
 const { container } = await mountPanel({ usageEstimate: async () => { throw Object.assign(new Error('career usage admission unavailable'), { code: 'admission_unavailable' }) } })
 const alert = container.querySelector('[role="alert"]')!
 assert.match(alert.textContent ?? '', /额度预估暂不可用/)
 assert.match(alert.textContent ?? '', /career usage admission unavailable/)
 assert.match(alert.textContent ?? '', /在预估恢复前不会发起收费找岗/)
 assert.match(alert.textContent ?? '', /不会先执行后补报/)
 assert.ok(byLabel(container, 'button', '重新获取预估'))
})

test('a forbidden space shows the permission state with a retry, never a charged path', async () => {
 const { container } = await mountPanel({ usageEstimate: async () => { throw Object.assign(new Error('personal career workspace required'), { code: 'forbidden' }) } })
 const alert = container.querySelector('[role="alert"]')!
 assert.match(alert.textContent ?? '', /当前空间不可访问/)
 assert.match(alert.textContent ?? '', /personal career workspace required/)
 assert.ok(byLabel(container, 'button', '重新获取预估'))
 assert.doesNotMatch(container.textContent ?? '', /将消耗/)
})

test('recovery: retrying a failed estimate re-reads the endpoint and shows the live balance again', async () => {
 let fail = true
 const reads: number[] = []
 const { container } = await mountPanel({ usageEstimate: async () => { reads.push(reads.length); if (fail) throw Object.assign(new Error('career usage admission unavailable'), { code: 'admission_unavailable' }); return estimate({ settledUnits: 3, remainingUnits: 46 }) } })
 assert.match(container.textContent ?? '', /额度预估暂不可用/)
 fail = false
 await act(async () => { byLabel(container, 'button', '重新获取预估').click(); await settle() })
 const panel = container.querySelector('[aria-label="额度预估"]')!
 assert.match(panel.textContent ?? '', /本期剩余 46 \/ 50 个额度单位/)
 assert.doesNotMatch(panel.textContent ?? '', /暂不可用/)
 assert.equal(reads.length, 2)
})

test('a scope switch resets the estimate and re-reads under the new identity', async () => {
 const reads: Array<string | null> = []
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'u-1', tenantId: 't-1' })
 let identity = 'u-1'
 const { container } = await mountPanel({ usageEstimate: async () => { reads.push(identity); return estimate() } }, scopeController)
 assert.deepEqual(reads, ['u-1'])
 await act(async () => { scopeController.switchScope('https://weknora.test', 'u-2', 't-2'); identity = 'u-2'; await settle() })
 assert.deepEqual(reads, ['u-1', 'u-2'])
 assert.match(container.querySelector('[aria-label="额度预估"]')?.textContent ?? '', /本期剩余 47 \/ 50 个额度单位/)
})
