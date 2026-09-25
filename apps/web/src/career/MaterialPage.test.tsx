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
import type { CareerView } from '../../../../packages/career-core/src/contracts.ts'
import type { MaterialBody, MaterialReceipt, MaterialView, MaterialVersionComparison, MaterialVersionView } from '../../../../packages/api-client/src/career.ts'

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void }
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) })

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } }
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1' })
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Element: dom.window.Element, MutationObserver: dom.window.MutationObserver, getComputedStyle: dom.window.getComputedStyle.bind(dom.window), Event: dom.window.Event, IS_REACT_ACT_ENVIRONMENT: true })
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator })
const { createRoot } = await import('react-dom/client')
const { MaterialPage } = await import('./MaterialPage.tsx')

const profileView: CareerView = { revision: 4, facts: [], proposals: [] }
const pin = { opportunityId: 'opp/1', snapshotId: 'snapshot ?1', snapshotSha256: 'a'.repeat(64), profileRevision: 4 }
const claimEdu = { claimId: 'claim-edu', text: '最高学历为本科', factKey: '学历', needsReview: false }
const claimIntern = { claimId: 'claim-intern', text: '实习经历待补充', needsReview: true, reviewNote: '缺少实习证明' }
const storedBody: MaterialBody = { sections: [{ heading: '教育经历', content: '计算机科学与技术本科', claims: [claimEdu, claimIntern] }] }
const risks = [{ code: 'missing_placeholder' as const, message: '缺失或待补充信息占位（实习经历待补充）：不得由系统补造', claimId: 'claim-intern' }]
const ts = '2026-09-25T08:00:00Z'
const editedReceipt = (requestId: string): MaterialReceipt => ({ kind: 'material_edited', requestId, materialId: 'mat-1', status: 'draft', pinnedEvidence: pin, body: storedBody, reviewRisks: risks })
const materialView = (versions: number[]): MaterialView => ({ materialId: 'mat-1', status: 'draft', pinnedEvidence: pin, body: storedBody, reviewRisks: risks, versionCount: versions.length, versions: versions.map((version) => ({ version, createdAt: ts })), createdAt: ts, updatedAt: ts })
const versionView = (version: number): MaterialVersionView => ({ version, pinnedEvidence: pin, factBasisRevision: 4, body: storedBody, reviewRisks: risks, requestId: `confirm-${version}`, createdAt: ts })
const comparison = (baseline: number, target: number, targetContent = '软件工程硕士'): MaterialVersionComparison => ({ materialId: 'mat-1', baseline: versionView(baseline), target: { ...versionView(target), body: { sections: [{ heading: '教育经历', content: targetContent, claims: [claimEdu, claimIntern] }] } }, changes: [{ kind: 'section_changed', heading: '教育经历', baseline: '计算机科学与技术本科', target: targetContent }] })

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
function click(el: HTMLElement) { act(() => { el.dispatchEvent(new dom.window.Event('click', { bubbles: true })) }) }

async function mountMaterial(career: CareerStub, options: { scopeController?: ReturnType<typeof createScopeController> } = {}) {
 const scopeController = options.scopeController ?? createScopeController({ origin: 'https://weknora.test', userId: 'u', tenantId: 't' })
 const container = render(React.createElement(MaterialPage, { client: { career } as unknown as WeKnoraClient, scopeController, opportunityId: 'opp/1', snapshotId: 'snapshot ?1' }))
 await act(async () => { await settle(); await settle() })
 return { container, scopeController }
}

// Fill one section with one fact-linked claim through the visible editor.
async function fillFirstSection(container: HTMLElement, options: { factKey?: string; keepPlaceholder?: boolean } = {}) {
 await act(async () => { click(byLabel(container, 'button', '新增章节')); await settle() })
 await act(async () => { setInput(container.querySelector<HTMLInputElement>('[aria-label="章节 1 标题"]')!, '教育经历'); await settle() })
 await act(async () => { setInput(container.querySelector<HTMLTextAreaElement>('[aria-label="章节 1 正文"]')!, '计算机科学与技术本科'); await settle() })
 if (options.factKey !== undefined || options.keepPlaceholder) {
  await act(async () => { click(container.querySelector<HTMLButtonElement>('[aria-label="章节 1 添加缺失占位主张"]')!); await settle() })
  await act(async () => { setInput(container.querySelector<HTMLTextAreaElement>('[aria-label="章节 1 主张 1 内容"]')!, '最高学历为本科'); await settle() })
  if (options.factKey !== undefined) {
   await act(async () => { setInput(container.querySelector<HTMLInputElement>('[aria-label="章节 1 主张 1 事实编号"]')!, options.factKey ?? ''); await settle() })
   await act(async () => { toggle(container.querySelector<HTMLInputElement>('[aria-label="章节 1 主张 1 标记待审阅"]')!); await settle() })
  }
 }
}

test('creating a draft from the job card pins the frozen evidence and saves the structured body', async () => {
 const sent: unknown[] = []
 const reads: string[] = []
 const career: CareerStub = {
  open: async () => profileView,
  editMaterial: async (input: { requestId: string }) => { sent.push(input); return editedReceipt(input.requestId) },
  material: async (id: string) => { reads.push(id); return materialView([]) },
 }
 const { container } = await mountMaterial(career)
 assert.match(container.textContent ?? '', /当前档案修订 4（材料将按此修订固定）/)
 await fillFirstSection(container, { factKey: '学历' })
 await act(async () => { click(byLabel(container, 'button', '保存草稿')); await settle(); await settle() })
 assert.equal(sent.length, 1)
 const input = sent[0] as { requestId: string; opportunityId: string; snapshotId: string; body: MaterialBody; expectedRevision: number }
 assert.equal(input.opportunityId, 'opp/1')
 assert.equal(input.snapshotId, 'snapshot ?1')
 assert.equal(input.expectedRevision, 4)
 assert.deepEqual(input.body, { sections: [{ heading: '教育经历', content: '计算机科学与技术本科', claims: [{ claimId: 'claim-1', text: '最高学历为本科', factKey: '学历', needsReview: false }] }] })
 assert.ok(input.requestId)
 assert.deepEqual(reads, ['mat-1'])
 assert.match(window.location.search, /material=mat-1/)
 const pinned = container.querySelector('[aria-label="材料固定的证据"]')?.textContent ?? ''
 assert.match(pinned, /岗位/)
 assert.match(pinned, /opp\/1/)
 assert.match(pinned, /快照/)
 assert.match(pinned, /snapshot \?1/)
 assert.match(pinned, /快照摘要/)
 assert.match(pinned, new RegExp(pin.snapshotSha256))
 assert.match(pinned, /档案修订/)
 assert.match(container.textContent ?? '', /草稿已保存（材料编号 mat-1）/)
})

test('a stored material shows fact-linked claims, explicit missing placeholders and the review risks', async () => {
 window.history.replaceState({}, '', '/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1&material=mat-1')
 const reads: string[] = []
 const career: CareerStub = {
  open: async () => profileView,
  material: async (id: string) => { reads.push(id); return materialView([1]) },
 }
 const { container } = await mountMaterial(career)
 assert.deepEqual(reads, ['mat-1'])
 assert.match(container.textContent ?? '', /已链接确认事实：学历/)
 const missing = container.querySelector('.wk-material__claim--needs-review')?.textContent ?? ''
 assert.match(missing, /缺失\/待补充（needs_review）：不得补造/)
 assert.match(missing, /实习经历待补充/)
 assert.match(missing, /缺少实习证明/)
 const risksSection = container.querySelector('[aria-label="审阅风险清单"]')?.textContent ?? ''
 assert.match(risksSection, /审阅风险（1 项）/)
 assert.match(risksSection, /missing_placeholder/)
 assert.match(risksSection, /不得由系统补造/)
 assert.doesNotMatch(container.textContent ?? '', /实习经历：某公司/)
 const versions = container.querySelector('[aria-label="不可变版本列表"]')?.textContent ?? ''
 assert.match(versions, /不可变版本（1）/)
 assert.match(versions, /V1/)
})

test('confirming publishes immutable versions and V1/V2 open side by side with the visible diff', async () => {
 const confirms: unknown[] = []
 const compares: string[][] = []
 const versions: number[] = []
 const career: CareerStub & { editMaterial: (input: any) => Promise<MaterialReceipt>; confirmMaterial: (input: any) => Promise<MaterialReceipt> } = {
  open: async () => profileView,
  editMaterial: async (input: { requestId: string; body: MaterialBody }) => ({ ...editedReceipt(input.requestId), body: input.body }),
  confirmMaterial: async (input: { requestId: string; materialId: string; expectedRevision: number }) => {
   confirms.push(input)
   versions.push(versions.length + 1)
   return { kind: 'material_confirmed', requestId: input.requestId, materialId: input.materialId, status: 'confirmed', version: versions.length, pinnedEvidence: pin, body: storedBody, reviewRisks: risks }
  },
  material: async () => materialView([...versions]),
  compareMaterialVersions: async (id: string, baseline: number, target: number) => { compares.push([id, String(baseline), String(target)]); return comparison(baseline, target) },
 }
 const { container } = await mountMaterial(career)
 await fillFirstSection(container)
 await act(async () => { click(byLabel(container, 'button', '保存草稿')); await settle(); await settle() })
 await act(async () => { click(byLabel(container, 'button', '确认发布不可变版本')); await settle(); await settle() })
 assert.match(container.textContent ?? '', /已发布不可变版本 V1/)
 assert.match(container.querySelector('[aria-label="不可变版本列表"]')?.textContent ?? '', /V1/)
 assert.equal((byLabel(container, 'button', '确认发布不可变版本') as HTMLButtonElement).disabled, false)
 await act(async () => { setInput(container.querySelector<HTMLTextAreaElement>('[aria-label="章节 1 正文"]')!, '软件工程硕士'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '保存草稿')); await settle(); await settle() })
 await act(async () => { click(byLabel(container, 'button', '确认发布不可变版本')); await settle(); await settle() })
 assert.match(container.textContent ?? '', /已发布不可变版本 V2/)
 assert.match(container.querySelector('[aria-label="不可变版本列表"]')?.textContent ?? '', /V2/)
 assert.equal(confirms.length, 2)
 assert.notEqual((confirms[0] as { requestId: string }).requestId, (confirms[1] as { requestId: string }).requestId)
 assert.equal((confirms[0] as { materialId: string }).materialId, 'mat-1')
 assert.equal((confirms[0] as { expectedRevision: number }).expectedRevision, 4)
 await act(async () => { click(byLabel(container, 'button', '与 V1 比较')); await settle(); await settle() })
 assert.deepEqual(compares, [['mat-1', '1', '2']])
 const compareSection = container.querySelector('[aria-label="版本并排比较"]')?.textContent ?? ''
 assert.match(compareSection, /版本 V1 与 V2 并排比较/)
 const panes = [...container.querySelectorAll('.wk-material__compare-pane')]
 assert.equal(panes.length, 2)
 assert.match(panes[0]?.textContent ?? '', /版本 V1（只读）/)
 assert.match(panes[0]?.textContent ?? '', /计算机科学与技术本科/)
 assert.match(panes[1]?.textContent ?? '', /版本 V2（只读）/)
 assert.match(panes[1]?.textContent ?? '', /软件工程硕士/)
 const changes = container.querySelector('[aria-label="版本差异"]')?.textContent ?? ''
 assert.match(changes, /章节「教育经历」变更/)
 assert.match(changes, /计算机科学与技术本科 → 软件工程硕士/)
})

test('an old immutable version opens read-only without any editable field', async () => {
 window.history.replaceState({}, '', '/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1&material=mat-1')
 const reads: string[][] = []
 const career: CareerStub = {
  open: async () => profileView,
  material: async () => materialView([1, 2]),
  materialVersion: async (id: string, version: number) => { reads.push([id, String(version)]); return versionView(version) },
 }
 const { container } = await mountMaterial(career)
 await act(async () => { click(container.querySelector<HTMLButtonElement>('[aria-label="只读查看 V1"]')!); await settle(); await settle() })
 assert.deepEqual(reads, [['mat-1', '1']])
 const detail = container.querySelector('[aria-label="版本只读回看"]')
 assert.ok(detail)
 assert.match(detail.textContent ?? '', /版本 V1（只读，不可修改）/)
 assert.match(detail.textContent ?? '', /计算机科学与技术本科/)
 assert.equal(detail.querySelectorAll('input, textarea').length, 0)
 assert.match(detail.textContent ?? '', /不可变版本不能修改/)
})

test('a claim-unconfirmed refusal keeps the local draft and the stored reason without publishing', async () => {
 window.history.replaceState({}, '', '/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1&material=mat-1')
 const confirms: unknown[] = []
 const career: CareerStub = {
  open: async () => profileView,
  editMaterial: async () => { throw Object.assign(new Error('career material claim references an unconfirmed fact: 城市'), { code: 'material_claim_unconfirmed' }) },
  confirmMaterial: async (input: unknown) => { confirms.push(input); throw new Error('must not confirm a refused draft') },
  material: async () => ({ ...materialView([1]), status: 'failed' as const, failureCode: 'claim_unconfirmed', failureMessage: 'career material claim references an unconfirmed fact: 城市' }),
 }
 const { container } = await mountMaterial(career)
 await act(async () => { setInput(container.querySelector<HTMLTextAreaElement>('[aria-label="章节 1 正文"]')!, '未确认城市：杭州'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '保存草稿')); await settle(); await settle() })
 assert.equal(confirms.length, 0)
 assert.match(container.textContent ?? '', /草稿保留未发布/)
 assert.match(container.textContent ?? '', /未确认事实/)
 assert.match(container.textContent ?? '', /claim_unconfirmed/)
 assert.match(container.querySelector('[aria-label="不可变版本列表"]')?.textContent ?? '', /不可变版本（1）/)
 assert.doesNotMatch(container.querySelector('[aria-label="不可变版本列表"]')?.textContent ?? '', /V2/)
 assert.equal(container.querySelector<HTMLTextAreaElement>('[aria-label="章节 1 正文"]')?.value, '未确认城市：杭州')
 assert.equal((byLabel(container, 'button', '确认发布不可变版本') as HTMLButtonElement).disabled, true)
})

test('an unknown write outcome recovers by the original request ID', async () => {
 const edits: Array<{ requestId: string }> = []
 const receiptReads: string[] = []
 let first = true
 const career: CareerStub & { editMaterial: (input: any) => Promise<MaterialReceipt> } = {
  open: async () => profileView,
  editMaterial: async (input: { requestId: string; body: MaterialBody }) => {
   edits.push(input)
   if (first) { first = false; throw Object.assign(new Error('gateway timeout'), { code: 'TIMEOUT' }) }
   return { ...editedReceipt(input.requestId), body: input.body }
  },
  materialReceipt: async (requestId: string) => { receiptReads.push(requestId); return editedReceipt(edits[0]!.requestId) },
  material: async () => materialView([]),
 }
 const { container } = await mountMaterial(career)
 await fillFirstSection(container)
 await act(async () => { click(byLabel(container, 'button', '保存草稿')); await settle(); await settle() })
 assert.match(container.textContent ?? '', /无法确认材料写入是否完成/)
 await act(async () => { click(byLabel(container, 'button', '查询材料回执')); await settle(); await settle() })
 assert.deepEqual(receiptReads, [edits[0]!.requestId])
 assert.match(container.textContent ?? '', /草稿已保存（材料编号 mat-1）/)
 assert.match(window.location.search, /material=mat-1/)
})

test('a revision conflict shows the current revision and a refreshed resave uses a new request ID', async () => {
 window.history.replaceState({}, '', '/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1&material=mat-1')
 const edits: Array<{ requestId: string; expectedRevision: number }> = []
 let head = 4
 let first = true
 const career: CareerStub & { editMaterial: (input: any) => Promise<MaterialReceipt> } = {
  open: async () => ({ ...profileView, revision: head }),
  editMaterial: async (input: { requestId: string; expectedRevision: number; body: MaterialBody }) => {
   if (first) { first = false; head = 7; throw Object.assign(new Error('revision conflict'), { code: 'revision_conflict', currentRevision: 7 }) }
   edits.push(input)
   return { ...editedReceipt(input.requestId), body: input.body }
  },
  material: async () => materialView([1]),
 }
 const { container } = await mountMaterial(career)
 await act(async () => { setInput(container.querySelector<HTMLTextAreaElement>('[aria-label="章节 1 正文"]')!, '更新后的正文'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '保存草稿')); await settle(); await settle() })
 assert.match(container.textContent ?? '', /档案已更新/)
 assert.match(container.textContent ?? '', /当前修订 7/)
 await act(async () => { click(byLabel(container, 'button', '重新读取档案修订')); await settle(); await settle() })
 assert.match(container.textContent ?? '', /当前档案修订 7/)
 await act(async () => { click(byLabel(container, 'button', '保存草稿')); await settle(); await settle() })
 assert.equal(edits.length, 1)
 assert.equal(edits[0]?.expectedRevision, 7)
 assert.notEqual(edits[0]?.requestId, undefined)
})

test('cross-tenant forbidden load clears the editor and a missing material reports a clear error', async () => {
 window.history.replaceState({}, '', '/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1&material=mat-1')
 const forbidden = await mountMaterial({ open: async () => profileView, material: async () => { throw Object.assign(new Error('forbidden'), { code: 'forbidden' }) } })
 assert.match(forbidden.container.textContent ?? '', /当前空间不可访问此材料，已清除编辑内容。/)
 assert.equal(forbidden.container.querySelector<HTMLButtonElement>('[aria-label="新增章节"]') ?? null, null)
 assert.doesNotMatch(window.location.search, /material=/)

 window.history.replaceState({}, '', '/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1&material=mat-1')
 const missing: CareerStub = {
  open: async () => profileView,
  material: async () => materialView([1]),
  editMaterial: async () => { throw Object.assign(new Error('career material not found'), { code: 'not_found' }) },
 }
 const second = await mountMaterial(missing)
 await act(async () => { setInput(second.container.querySelector<HTMLTextAreaElement>('[aria-label="章节 1 正文"]')!, '再次编辑'); await settle() })
 await act(async () => { click(byLabel(second.container, 'button', '保存草稿')); await settle(); await settle() })
 assert.match(second.container.textContent ?? '', /材料不存在/)
 assert.match(second.container.querySelector<HTMLTextAreaElement>('[aria-label="章节 1 正文"]')?.value ?? '', /再次编辑/)
})

test('a scope switch clears the in-flight material editing state', async () => {
 const scope = createScopeController({ origin: 'https://weknora.test', userId: 'u', tenantId: 't' })
 const { container } = await mountMaterial({ open: async () => profileView }, { scopeController: scope })
 await fillFirstSection(container)
 await act(async () => { scope.switchScope('https://weknora.test', 'other-user', 'other-tenant'); await settle(); await settle() })
 assert.match(container.textContent ?? '', /空间已切换或登录已失效，已清除材料编辑内容。/)
 assert.equal(container.querySelectorAll('input, textarea, button').length, 0)
})

test('material styles keep TDesign light surfaces, brand green confirmations, side-by-side panes and narrow-screen stacking', () => {
 const css = readFileSync(new URL('./material.css', import.meta.url), 'utf8')
 assert.match(css, /\.wk-material \{/)
 assert.match(css, /--td-bg-color-container/)
 assert.match(css, /#07c05f/)
 assert.match(css, /\.wk-material__compare \{[\s\S]*?grid-template-columns: 1fr 1fr/)
 assert.match(css, /@media \(max-width: 640px\)/)
 assert.match(css, /overflow-wrap: anywhere/)
})
