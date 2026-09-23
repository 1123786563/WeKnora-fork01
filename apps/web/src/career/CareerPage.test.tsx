import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import test, { afterEach } from 'node:test'
import * as React from 'react'
import { act } from 'react'
import type { Root } from 'react-dom/client'
import { createScopeController } from '@weknora/domain/scope'
import type { WeKnoraClient } from '@weknora/api-client'
import type { CareerAction, CareerReceipt, CareerView } from '../../../../packages/career-core/src/contracts.ts'

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } }
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/career' })
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Element: dom.window.Element, MutationObserver: dom.window.MutationObserver, getComputedStyle: dom.window.getComputedStyle.bind(dom.window), Event: dom.window.Event, requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)), cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? ((id: number) => clearTimeout(id)), IS_REACT_ACT_ENVIRONMENT: true })
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator })
const { createRoot } = await import('react-dom/client')
const { CareerPage } = await import('./CareerPage.tsx')

const profile: CareerView = { revision: 1, facts: [{ key: '学历', value: '本科', revision: 1, source: { kind: 'user', label: '本人填写' }, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' }], proposals: [{ id: 'p', key: '毕业时间', value: '2027', source: { kind: 'user' }, status: 'pending', createdAt: 'now' }] }
let root: Root | undefined
let host: HTMLDivElement | undefined
afterEach(async () => { if (root) await act(async () => root?.unmount()); root = undefined; host?.remove(); host = undefined; document.body.replaceChildren() })
async function mount(api: Record<string, (...args: never[]) => unknown>) {
 host = document.createElement('div'); document.body.append(host); root = createRoot(host)
 const client = { career: api } as unknown as WeKnoraClient
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'u', tenantId: 't' })
 await act(async () => { root!.render(React.createElement(CareerPage, { client, scopeController, userId: 'u' })); await new Promise((resolve) => setImmediate(resolve)) })
 return host
}
const button = (container: HTMLElement, label: string): HTMLElement => {
 const found = [...container.querySelectorAll('.t-button')].find((item) => item.textContent?.trim() === label)
 assert.ok(found, `button ${label} exists`); return found as HTMLElement
}

 test('all mutation controls stay blocked until an unknown result is reconciled, then retry reuses the same action id', async () => {
  const sent: CareerAction[] = []; let tries = 0
  const confirmed: CareerReceipt = { kind: 'confirmed', requestId: 'request-original', revision: 2, fact: { key: '毕业时间', value: '2027', revision: 2, source: { kind: 'user', label: '本人确认' }, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' } }
  const container = await mount({
   open: async () => profile, list: async () => ({ ...profile, revision: 2 }), changes: async () => ({ revision: 1, changes: [] }),
   act: async (action: CareerAction) => { sent.push(action); tries += 1; if (tries === 1) throw Object.assign(new Error('timed out'), { code: 'TIMEOUT' }); return confirmed },
   receipt: async () => { throw Object.assign(new Error('not found'), { code: 'not_found' }) },
  } as never)
  const confirm = button(container, '确认'); await act(async () => { confirm.click(); await new Promise((resolve) => setImmediate(resolve)) })
  assert.match(container.textContent ?? '', /提交结果暂时未知/)
  for (const label of ['保存为提案', '直接确认', '确认', '忽略']) assert.equal(button(container, label).hasAttribute('disabled'), true, `${label} is blocked`)
  const retry = button(container, '用原请求编号安全重试'); await act(async () => { retry.click(); await new Promise((resolve) => setImmediate(resolve)) })
  assert.equal(sent.length, 2)
  assert.equal(sent[0]?.requestId, sent[1]?.requestId)
  assert.equal(sent[0]?.requestId?.length! > 0, true)
  assert.match(container.textContent ?? '', /已确认档案/)
 })

 test('same-scope forbidden refresh clears private facts and hides all mutation controls', async () => {
  let revoked = false
  const container = await mount({
   open: async () => profile,
   list: async () => { if (revoked) throw Object.assign(new Error('space access revoked'), { code: 'forbidden' }); return profile },
   changes: async () => ({ revision: 1, changes: [] }), act: async () => { throw new Error('must not act') }, receipt: async () => { throw new Error('unused') },
  } as never)
  assert.match(container.textContent ?? '', /本科/)
  revoked = true
  await act(async () => { button(container, '刷新').click(); await new Promise((resolve) => setImmediate(resolve)) })
  assert.match(container.textContent ?? '', /当前空间不可访问/)
  assert.doesNotMatch(container.textContent ?? '', /本科/)
  for (const label of ['保存为提案', '直接确认', '确认', '忽略']) assert.equal([...container.querySelectorAll('button')].some((item) => item.textContent?.trim() === label), false, `${label} is hidden`)
 })
