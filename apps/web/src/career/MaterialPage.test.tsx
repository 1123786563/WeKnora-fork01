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

// Review round 1 F1 (high): after restoring a stored draft whose claims
// already use the local claim-N pattern, adding another placeholder must
// never reuse an existing claim ID (React key duplication and a backend
// invalid_request refusal from validateMaterialShape's seenClaims check).
test('adding a placeholder after restoring a stored draft never collides with existing claim IDs', async () => {
 window.history.replaceState({}, '', '/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1&material=mat-1')
 const sent: MaterialBody[] = []
 const restoredBody: MaterialBody = { sections: [{ heading: '教育经历', content: '计算机科学与技术本科', claims: [
  { claimId: 'claim-1', text: '实习经历待补充', needsReview: true },
  { claimId: 'claim-2', text: '毕业时间为 2026 年', factKey: '毕业时间', needsReview: false },
 ] }] }
 const career: CareerStub & { editMaterial: (input: any) => Promise<MaterialReceipt> } = {
  open: async () => profileView,
  material: async () => ({ ...materialView([1]), body: restoredBody }),
  editMaterial: async (input: { requestId: string; body: MaterialBody }) => { sent.push(input.body); return { ...editedReceipt(input.requestId), body: input.body } },
  confirmMaterial: async () => { throw new Error('not part of this test') },
 }
 const { container } = await mountMaterial(career)
 await act(async () => { click(container.querySelector<HTMLButtonElement>('[aria-label="章节 1 添加缺失占位主张"]')!); await settle() })
 const ids = [...container.querySelectorAll('.wk-material__claim-id code')].map((node) => node.textContent)
 assert.equal(ids.length, 3)
 assert.equal(new Set(ids).size, 3, `claim IDs stay unique across the restored draft and the new placeholder: ${ids.join(', ')}`)
 assert.equal(ids[2], 'claim-3', 'the counter continues after the restored maximum instead of restarting at claim-1')
 await act(async () => { setInput(container.querySelector<HTMLTextAreaElement>('[aria-label="章节 1 主张 3 内容"]')!, '证书待补充'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '保存草稿')); await settle(); await settle() })
 assert.equal(sent.length, 1)
 const savedIds = sent[0]?.sections[0]?.claims.map((claim) => claim.claimId)
 assert.deepEqual(savedIds, ['claim-1', 'claim-2', 'claim-3'])
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
 assert.match(css, /\.wk-material__export \{/)
 assert.match(css, /\.wk-material__export-submittable \{[\s\S]*?#07c05f/)
 assert.match(css, /\.wk-material__export-file--failed/)
})

// T16 export seam: one publish renders the PDF/DOCX pair of one immutable
// version; only the both-verified export is offered for delivery; downloads
// redeem an authenticated grant and verify the SHA-256 before saving; a
// revocation kills already-issued grants while other exports stay live.
const exportTs = '2026-09-25T09:00:00Z'
const exportDigest = 'f'.repeat(64)
const exportFile = (format: 'pdf' | 'docx', verified: boolean, fileDigest: string, size: number) => ({ format, materialId: 'mat-1', version: 1, contentDigest: exportDigest, objectKey: `objects/${format}`, fileDigest, size, verified, ...(verified ? {} : { error: `verify ${format} failed: truncated body` }) })
const exportReceipt = (options: { status?: 'staged' | 'submittable' | 'failed' | 'revoked'; submittable?: boolean; docxVerified?: boolean; pdfVerified?: boolean; kind?: 'material_published' | 'material_export_revoked'; exportId?: string; version?: number } = {}) => {
 const status = options.status ?? 'submittable'
 const pdfVerified = options.pdfVerified ?? true
 const docxVerified = options.docxVerified ?? true
 return {
  kind: options.kind ?? 'material_published', requestId: `publish-${options.exportId ?? 'exp-1'}`, exportId: options.exportId ?? 'exp-1', materialId: 'mat-1', version: options.version ?? 1, status, submittable: options.submittable ?? status === 'submittable', contentDigest: exportDigest,
  files: [exportFile('pdf', pdfVerified, 'a'.repeat(64), 2048), exportFile('docx', docxVerified, 'b'.repeat(64), 4096)],
  failureCode: status === 'submittable' ? undefined : 'export_verification_failed',
  failureMessage: status === 'staged' ? 'verify docx failed: truncated body' : status === 'failed' ? 'verify pdf failed: truncated body verify docx failed: truncated body' : undefined,
  createdAt: exportTs, ...(options.kind === 'material_export_revoked' ? { revokedAt: exportTs } : {}),
 }
}
const grantFixture = (format: 'pdf' | 'docx', digest: string) => ({ exportId: 'exp-1', materialId: 'mat-1', version: 1, format, digest, size: 12, expiresAt: 1790000000, signature: 'c0ffee'.repeat(4), url: `/api/v1/career/materials/mat-1/exports/exp-1/download?format=${format}&expires=1790000000&signature=${'c0ffee'.repeat(4)}` })

async function mountStoredMaterial(career: CareerStub) {
 window.history.replaceState({}, '', '/platform/career/opportunities/opp%2F1?snapshotId=snapshot%20%3F1&material=mat-1')
 const mounted = await mountMaterial(career)
 return mounted.container
}

test('publishing an immutable version renders the submittable export with the shared digest and the version binding', async () => {
 const publishes: unknown[] = []
 const career: CareerStub = {
  open: async () => profileView,
  material: async () => materialView([1]),
  materialExports: async (id: string) => { exportsReads.push(id); return { materialId: id, exports: [exportReceipt()] } },
  publishMaterial: async (input: { requestId: string }) => { publishes.push(input); return { ...exportReceipt(), requestId: input.requestId } },
 }
 const exportsReads: string[] = []
 const container = await mountStoredMaterial(career)
 await act(async () => { click(container.querySelector<HTMLButtonElement>('[aria-label="发布导出 V1"]')!); await settle(); await settle() })
 assert.equal(publishes.length, 1)
 const input = publishes[0] as { requestId: string; materialId: string; version: number; expectedRevision: number }
 assert.equal(input.materialId, 'mat-1')
 assert.equal(input.version, 1)
 assert.equal(input.expectedRevision, 4)
 assert.ok(input.requestId)
 assert.match(container.textContent ?? '', /导出已发布：PDF 与 DOCX 均核验通过，可用于投递/)
 const item = container.querySelector('[aria-label="导出 exp-1"]')?.textContent ?? ''
 assert.match(item, /状态：双格式核验通过（submittable）/)
 assert.match(item, /可用于投递/)
 assert.match(item, /绑定不可变版本 V1/)
 assert.match(item, new RegExp(`正文摘要（两种格式同一摘要）${exportDigest}`))
 const files = container.querySelector('[aria-label="导出 exp-1 文件"]')?.textContent ?? ''
 assert.match(files, /PDF · 核验通过/)
 assert.match(files, new RegExp('a'.repeat(64)))
 assert.match(files, /DOCX · 核验通过/)
 assert.match(files, new RegExp('b'.repeat(64)))
 assert.match(files, /2048 字节/)
 assert.equal((files.match(/与导出正文摘要一致/g) ?? []).length, 2, 'both format files bind the export body digest')
 assert.ok(container.querySelector('[aria-label="下载 PDF exp-1"]'))
 assert.ok(container.querySelector('[aria-label="下载 DOCX exp-1"]'))
 assert.ok(container.querySelector('[aria-label="撤销导出 exp-1"]'))
 assert.deepEqual(exportsReads, ['mat-1', 'mat-1'], 'exports load once on restore and reload after publish')
})

test('a single-format verification failure keeps the export staged with its error and never deliverable', async () => {
 const container = await mountStoredMaterial({
  open: async () => profileView,
  material: async () => materialView([1]),
  materialExports: async (id: string) => ({ materialId: id, exports: [exportReceipt({ status: 'staged', docxVerified: false })] }),
  publishMaterial: async (input: { requestId: string }) => ({ ...exportReceipt({ status: 'staged', docxVerified: false }), requestId: input.requestId }),
 })
 await act(async () => { click(container.querySelector<HTMLButtonElement>('[aria-label="发布导出 V1"]')!); await settle(); await settle() })
 assert.match(container.textContent ?? '', /导出保留暂存/)
 const item = container.querySelector('[aria-label="导出 exp-1"]')?.textContent ?? ''
 assert.match(item, /状态：已暂存（staged）/)
 assert.match(item, /仅一种格式核验通过/)
 assert.match(item, /DOCX · 核验未通过/)
 assert.match(item, /verify docx failed: truncated body/)
 assert.doesNotMatch(item, /可用于投递/)
 assert.equal(container.querySelector('[aria-label="下载 PDF exp-1"]'), null)
 assert.equal(container.querySelector('[aria-label="下载 DOCX exp-1"]'), null)
 assert.equal(container.querySelector('[aria-label="撤销导出 exp-1"]'), null)
})

test('a payload claiming submittable with an unverified format is never offered for delivery', async () => {
 const container = await mountStoredMaterial({
  open: async () => profileView,
  material: async () => materialView([1]),
  materialExports: async (id: string) => ({ materialId: id, exports: [exportReceipt({ status: 'submittable', submittable: true, docxVerified: false })] }),
 })
 await act(async () => { await settle(); await settle() })
 const item = container.querySelector('[aria-label="导出 exp-1"]')?.textContent ?? ''
 assert.doesNotMatch(item, /可用于投递/)
 assert.match(item, /DOCX · 核验未通过/)
 assert.equal(container.querySelector('[aria-label="下载 PDF exp-1"]'), null)
 assert.equal(container.querySelector('[aria-label="撤销导出 exp-1"]'), null)
})

test('downloads redeem an authenticated grant and verify the SHA-256 digest before saving', async () => {
 const bytes = new TextEncoder().encode('%PDF-1.4 career material')
 const digest = [...new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))].map((byte) => byte.toString(16).padStart(2, '0')).join('')
 const grants: unknown[][] = []
 const downloads: unknown[] = []
 const created: Blob[] = []
 const originalCreate = URL.createObjectURL
 ;(URL as unknown as { createObjectURL: (blob: Blob) => string }).createObjectURL = (blob: Blob) => { created.push(blob); return 'blob:stub' }
 try {
  const container = await mountStoredMaterial({
   open: async () => profileView,
   material: async () => materialView([1]),
   materialExports: async (id: string) => ({ materialId: id, exports: [exportReceipt()] }),
   materialExportSignedURL: async (materialId: string, exportId: string, format: 'pdf' | 'docx', ttl: number) => { grants.push([materialId, exportId, format, ttl]); return grantFixture(format, digest) },
   materialExportDownload: async (grant: unknown) => { downloads.push(grant); return { format: 'pdf', digest, size: bytes.byteLength, body: new Blob([bytes], { type: 'application/pdf' }), contentType: 'application/pdf' } },
  })
  await act(async () => { click(container.querySelector<HTMLButtonElement>('[aria-label="下载 PDF exp-1"]')!); await settle(); await settle() })
  assert.deepEqual(grants, [['mat-1', 'exp-1', 'pdf', 600]])
  assert.equal(downloads.length, 1)
  assert.equal((downloads[0] as { digest: string }).digest, digest)
  const note = container.querySelector('[aria-label="导出 exp-1 下载状态"]')?.textContent ?? ''
  assert.match(note, /PDF 已下载，SHA-256 与授权摘要一致/)
  assert.equal(created.length, 1, 'the verified blob is offered as a browser download')
 } finally { (URL as unknown as { createObjectURL: (blob: Blob) => string }).createObjectURL = originalCreate }
})

test('a digest mismatch refuses to save the downloaded bytes and reports the mismatch', async () => {
 const bytes = new TextEncoder().encode('%PDF-1.4 tampered bytes')
 const created: Blob[] = []
 const originalCreate = URL.createObjectURL
 ;(URL as unknown as { createObjectURL: (blob: Blob) => string }).createObjectURL = (blob: Blob) => { created.push(blob); return 'blob:stub' }
 try {
  const container = await mountStoredMaterial({
   open: async () => profileView,
   material: async () => materialView([1]),
   materialExports: async (id: string) => ({ materialId: id, exports: [exportReceipt()] }),
   materialExportSignedURL: async (_materialId: string, _exportId: string, format: 'pdf' | 'docx') => grantFixture(format, '0'.repeat(64)),
   materialExportDownload: async () => ({ format: 'pdf', digest: '0'.repeat(64), size: bytes.byteLength, body: new Blob([bytes], { type: 'application/pdf' }), contentType: 'application/pdf' }),
  })
  await act(async () => { click(container.querySelector<HTMLButtonElement>('[aria-label="下载 PDF exp-1"]')!); await settle(); await settle() })
  const note = container.querySelector('[aria-label="导出 exp-1 下载状态"]')?.textContent ?? ''
  assert.match(note, /下载字节摘要与授权摘要不一致，已拒绝保存/)
  assert.equal(created.length, 0, 'tampered bytes are never handed to the browser')
 } finally { (URL as unknown as { createObjectURL: (blob: Blob) => string }).createObjectURL = originalCreate }
})

test('revoking invalidates one export immediately while the old-version export stays downloadable', async () => {
 const revokes: unknown[] = []
 let exports = [exportReceipt({ exportId: 'exp-1', version: 1 }), exportReceipt({ exportId: 'exp-2', version: 2 })]
 const container = await mountStoredMaterial({
  open: async () => profileView,
  material: async () => materialView([1, 2]),
  materialExports: async (id: string) => ({ materialId: id, exports }),
  revokeMaterialExport: async (input: { requestId: string }) => { revokes.push(input); exports = [exports[0]!, exportReceipt({ kind: 'material_export_revoked', exportId: 'exp-2', version: 2, status: 'revoked', submittable: false })]; return { ...exports[1]!, requestId: input.requestId } },
 })
 await act(async () => { click(container.querySelector<HTMLButtonElement>('[aria-label="撤销导出 exp-2"]')!); await settle(); await settle() })
 assert.equal(revokes.length, 1)
 const input = revokes[0] as { requestId: string; materialId: string; exportId: string; expectedRevision: number }
 assert.equal(input.materialId, 'mat-1')
 assert.equal(input.exportId, 'exp-2')
 assert.equal(input.expectedRevision, 4)
 assert.ok(input.requestId)
 assert.match(container.textContent ?? '', /导出已撤销：已签发的下载授权立即失效/)
 const revoked = container.querySelector('[aria-label="导出 exp-2"]')?.textContent ?? ''
 assert.match(revoked, /状态：已撤销（revoked）/)
 assert.match(revoked, /旧下载授权立即失效/)
 assert.equal(container.querySelector('[aria-label="下载 PDF exp-2"]'), null)
 assert.ok(container.querySelector('[aria-label="下载 PDF exp-1"]'), 'the old-version export stays downloadable')
 assert.ok(container.querySelector('[aria-label="撤销导出 exp-1"]'))
})

test('an uncertain publish outcome recovers by replaying the original request id', async () => {
 const publishes: Array<{ requestId: string }> = []
 let first = true
 const container = await mountStoredMaterial({
  open: async () => profileView,
  material: async () => materialView([1]),
  materialExports: async (id: string) => ({ materialId: id, exports: first ? [] : [exportReceipt()] }),
  publishMaterial: async (input: { requestId: string }) => {
   publishes.push(input)
   if (first) { first = false; throw Object.assign(new Error('gateway timeout'), { code: 'TIMEOUT' }) }
   return { ...exportReceipt(), requestId: input.requestId }
  },
 })
 await act(async () => { click(container.querySelector<HTMLButtonElement>('[aria-label="发布导出 V1"]')!); await settle(); await settle() })
 assert.match(container.textContent ?? '', /无法确认导出是否已发布/)
 await act(async () => { click(byLabel(container, 'button', '用原请求编号重试发布')); await settle(); await settle() })
 assert.equal(publishes.length, 2)
 assert.equal(publishes[0]?.requestId, publishes[1]?.requestId, 'the retry replays the original request id')
 assert.match(container.textContent ?? '', /导出已发布：PDF 与 DOCX 均核验通过/)
})

test('export failures surface revision conflicts, clear on forbidden scope and report refused grants', async () => {
 const container = await mountStoredMaterial({
  open: async () => profileView,
  material: async () => materialView([1]),
  materialExports: async (id: string) => ({ materialId: id, exports: [exportReceipt()] }),
  publishMaterial: async () => { throw Object.assign(new Error('revision conflict'), { code: 'revision_conflict', currentRevision: 9 }) },
 })
 await act(async () => { click(container.querySelector<HTMLButtonElement>('[aria-label="发布导出 V1"]')!); await settle(); await settle() })
 assert.match(container.textContent ?? '', /档案已更新/)
 assert.match(container.textContent ?? '', /当前修订 9/)

 const denied = await mountStoredMaterial({ open: async () => profileView, material: async () => materialView([1]), materialExports: async () => { throw Object.assign(new Error('forbidden'), { code: 'forbidden' }) } })
 assert.match(denied.textContent ?? '', /当前空间不可访问此材料，已清除编辑内容。/)

 const refused = await mountStoredMaterial({
  open: async () => profileView,
  material: async () => materialView([1]),
  materialExports: async (id: string) => ({ materialId: id, exports: [exportReceipt()] }),
  materialExportSignedURL: async () => { throw Object.assign(new Error('career material export grant invalid'), { code: 'export_grant_invalid' }) },
 })
 await act(async () => { click(refused.querySelector<HTMLButtonElement>('[aria-label="下载 DOCX exp-1"]')!); await settle(); await settle() })
 assert.match(refused.querySelector('[aria-label="导出 exp-1 下载状态"]')?.textContent ?? '', /下载授权已失效（导出可能已被撤销），本次下载被拒绝/)
})

// ocr1-072：写入成功且回执已验证后，只读回读（reloadView→client.career.material）
// 失败不得落入未知写入分支——「用原请求编号恢复」对一次已确认成功的写入是误导。
test('a read-back failure after a successful draft save reports saved-with-refresh-failure, not an unknown write', async () => {
 const sent: unknown[] = []
 const career: CareerStub = {
  open: async () => profileView,
  editMaterial: async (input: { requestId: string }) => { sent.push(input); return editedReceipt(input.requestId) },
  // 写入成功后的回读持续 500。
  material: async () => { throw Object.assign(new Error('transient 5xx'), { status: 500 }) },
 }
 const { container } = await mountMaterial(career)
 await fillFirstSection(container, { factKey: '学历' })
 await act(async () => { click(byLabel(container, 'button', '保存草稿')); await settle(); await settle() })
 assert.equal(sent.length, 1, 'the edit write fired once')
 assert.match(container.textContent ?? '', /草稿已保存（材料编号 mat-1）/, 'the save outcome is reported as saved')
 assert.match(container.textContent ?? '', /内容回显暂时失败/, 'the read-back failure is called out separately')
 assert.doesNotMatch(container.textContent ?? '', /暂时无法确认材料写入是否完成/, 'must NOT be classified as an unknown write')
 assert.doesNotMatch(container.textContent ?? '', /原请求编号/, 'no receipt-recovery guidance for an already-confirmed write')
})
