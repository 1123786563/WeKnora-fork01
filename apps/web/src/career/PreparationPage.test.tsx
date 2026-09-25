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
import type { PreparationReceipt } from '../../../../packages/api-client/src/career.ts'

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void }
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) })

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } }
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/career/opportunities/opp%2F1?application=app-1' })
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Element: dom.window.Element, MutationObserver: dom.window.MutationObserver, getComputedStyle: dom.window.getComputedStyle.bind(dom.window), Event: dom.window.Event, IS_REACT_ACT_ENVIRONMENT: true })
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator })
const { createRoot } = await import('react-dom/client')
const { PreparationPage } = await import('./PreparationPage.tsx')

const ts = '2026-09-26T07:30:00Z'
const digest = 'c'.repeat(64)
const anchor = { submissionId: 'sub-1', materialId: 'mat-1', exportId: 'exp-1', version: 2, contentDigest: digest }
const draftBody = { sections: [
 { heading: '面试准备（草稿）：教育经历', content: '以下要点固定自实际投递版本与岗位快照，仅引用已确认事实。', claims: [
  { claimId: 'claim-1', text: '本科在读（计算机科学与技术，大三）', factKey: '学历', needsReview: false },
  { claimId: 'claim-2', text: '实习经历待补充', needsReview: true, reviewNote: '缺实习经历，不得补造' },
 ] },
] }
const draft = (extra: Partial<PreparationReceipt> = {}): PreparationReceipt => ({
 kind: 'preparation_generated', requestId: 'prep-req-1', applicationId: 'app-1', preparationId: 'prep-1', focus: 'interview_prep', status: 'draft',
 anchor, materialId: 'mat-draft-1', body: draftBody, reviewRisks: [{ code: 'needs_review', message: '缺实习经历，不得补造', claimId: 'claim-2' }],
 sources: { submittedVersion: anchor, snapshot: { opportunityId: 'opp/1', snapshotId: 'snap-1', snapshotSha256: 'a'.repeat(64) }, factKeys: ['学历'], profileRevision: 5 },
 revision: 5, createdAt: ts, ...extra,
})

type CareerStub = Record<string, (...args: any[]) => unknown>
let root: Root | undefined
let host: HTMLDivElement | undefined
afterEach(async () => {
 if (root) await act(async () => root?.unmount())
 root = undefined; host?.remove(); host = undefined; document.body.replaceChildren()
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
function byLabelOrNull(container: HTMLElement, selector: string, label: string): HTMLElement | undefined {
 return [...container.querySelectorAll<HTMLElement>(selector)].find((item) => item.textContent?.trim() === label)
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

async function mountPreparation(career: CareerStub, options: { applicationId?: string } = {}) {
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'owner-1', tenantId: 't' })
 const container = render(React.createElement(PreparationPage, { client: { career } as unknown as WeKnoraClient, scopeController, applicationId: options.applicationId ?? 'app-1' }))
 await act(async () => { await settle(); await settle() })
 return container
}

test('generating a draft anchors visibly to the actually submitted version and cites the source chain', async () => {
 const sent: unknown[] = []
 let stored: PreparationReceipt[] = []
 const career: CareerStub = {
  open: async () => ({ revision: 5 }),
  applicationPreparations: async () => ({ preparations: stored }),
  generatePreparation: async (input: Record<string, unknown>) => { sent.push(input); stored = [draft({ requestId: input.requestId as string })]; return stored[0] },
  editMaterial: async () => { throw new Error('must not edit without an explicit revision action') },
 }
 const container = await mountPreparation(career)
 assert.equal(sent.length, 0, 'nothing is generated until the user asks')
 const focuses = [...container.querySelectorAll<HTMLSelectElement>('[aria-label="准备焦点"] option')].map((option) => option.value)
 assert.deepEqual(focuses, ['', 'cover_letter', 'interview_prep'])
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="准备焦点"]')!, 'interview_prep'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '生成准备草稿')); await settle(); await settle() })
 assert.equal(sent.length, 1)
 const input = sent[0] as Record<string, unknown>
 assert.ok(input.requestId)
 assert.equal(input.applicationId, 'app-1')
 assert.equal(input.focus, 'interview_prep')
 assert.equal(input.expectedRevision, 5)
 const row = container.querySelector<HTMLElement>('[aria-label="准备列表"] > li')
 assert.ok(row, 'the generated draft is listed')
 assert.match(row.textContent ?? '', /基于实际投递版本 V2/)
 assert.match(row.textContent ?? '', /面试准备/)
 const sources = row.querySelector('[aria-label="准备来源链"]')
 assert.ok(sources, 'the source chain is visible')
 assert.match(sources.textContent ?? '', /岗位快照 snap-1/)
 assert.match(sources.textContent ?? '', /已确认事实：学历/)
 assert.match(sources.textContent ?? '', /投递版本 V2/)
 assert.match(sources.textContent ?? '', /提交 sub-1/)
 // Claims stay traceable: linked facts show their key, placeholders stay
 // explicit instead of being fabricated.
 assert.match(row.textContent ?? '', /已链接确认事实：学历/)
 assert.match(row.textContent ?? '', /待补充/)
})

test('the page states it never sends anything and offers no send action', async () => {
 const career: CareerStub = {
  open: async () => ({ revision: 5 }),
  applicationPreparations: async () => ({ preparations: [draft()] }),
  generatePreparation: async () => { throw new Error('must not generate implicitly') },
 }
 const container = await mountPreparation(career)
 assert.match(container.textContent ?? '', /不自动发送/)
 assert.match(container.textContent ?? '', /不代为承诺/)
 const buttons = [...container.querySelectorAll('button')].map((button) => button.textContent?.trim())
 assert.ok(!buttons.some((label) => /发送|投递|提交给/.test(label ?? '')), `no send-flavoured action exists: ${buttons.join(',')}`)
})

test('an unknown submitted version answers the typed prompt state without guessing a version', async () => {
 const sent: Array<{ requestId: string }> = []
 const career: CareerStub = {
  open: async () => ({ revision: 5 }),
  applicationPreparations: async () => ({ preparations: [] }),
  generatePreparation: async (input: { requestId: string }) => { sent.push(input); throw Object.assign(new Error('version unknown'), { code: 'preparation_version_unknown' }) },
 }
 const container = await mountPreparation(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="准备焦点"]')!, 'cover_letter'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '生成准备草稿')); await settle(); await settle() })
 assert.equal(sent.length, 1)
 const alert = container.querySelector('[role="alert"]')
 assert.ok(alert, 'the prompt state is announced')
 assert.match(alert.textContent ?? '', /未确认实际投递版本/)
 assert.match(alert.textContent ?? '', /先记录投递/)
 assert.ok(!container.querySelector('[aria-label="准备列表"] > li'), 'no draft product is shown for the refused generation')
 assert.ok(!/最新版本/.test(container.textContent ?? ''), 'the prompt never suggests falling back to the latest version')
})

test('viewing the sources and revising the draft saves through the material edit seam', async () => {
 const edits: Array<{ requestId: string; materialId?: string; body: unknown; expectedRevision: number }> = []
 let stored: PreparationReceipt[] = [draft()]
 let materialBody: PreparationReceipt['body'] = draftBody
 const career: CareerStub = {
  open: async () => ({ revision: 5 }),
  applicationPreparations: async () => ({ preparations: stored }),
  generatePreparation: async () => { throw new Error('no generation in this flow') },
  editMaterial: async (input: { requestId: string; materialId?: string; body: unknown; expectedRevision: number }) => {
   edits.push(input)
   materialBody = input.body as PreparationReceipt['body']
   return { kind: 'material_edited', requestId: input.requestId, materialId: input.materialId, status: 'draft', pinnedEvidence: { opportunityId: 'opp/1', snapshotId: 'snap-1', snapshotSha256: 'a'.repeat(64), profileRevision: 5 }, body: input.body, reviewRisks: [] }
  },
  // The saved revision is read back from the material domain: the durable
  // truth of what is stored, not the echoed write.
  material: async (materialId: string) => ({ materialId, status: 'draft', pinnedEvidence: { opportunityId: 'opp/1', snapshotId: 'snap-1', snapshotSha256: 'a'.repeat(64), profileRevision: 5 }, body: materialBody, reviewRisks: [], versionCount: 0, versions: [], createdAt: ts, updatedAt: ts }),
 }
 const container = await mountPreparation(career)
 const row = container.querySelector<HTMLElement>('[aria-label="准备列表"] > li')
 assert.ok(row)
 assert.ok(!container.querySelector('[aria-label="修订准备草稿"]'), 'the revision editor stays closed until the user opens it')
 await act(async () => { click(row.querySelector<HTMLButtonElement>('[aria-label="修订草稿 mat-draft-1"]')!); await settle(); await settle() })
 const editor = container.querySelector('[aria-label="修订准备草稿"]')
 assert.ok(editor, 'the revision editor opens with the draft body loaded')
 const heading = editor.querySelectorAll<HTMLInputElement | HTMLTextAreaElement>('[aria-label="段落标题"]')[0] as HTMLInputElement
 assert.equal(heading.value, '面试准备（草稿）：教育经历')
 await act(async () => { setInput(heading, '面试准备（草稿）：教育经历与项目'); await settle() })
 const content = editor.querySelectorAll<HTMLTextAreaElement>('[aria-label="段落正文"]')[0]
 await act(async () => { setInput(content, '以下要点固定自实际投递版本与岗位快照，仅引用已确认事实。（本人补充：项目经历两段）'); await settle() })
 const claim = editor.querySelectorAll<HTMLTextAreaElement>('[aria-label="主张文本"]')[0]
 await act(async () => { setInput(claim, '本科在读（计算机科学与技术，大三，GPA 3.8）'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '保存修订')); await settle(); await settle() })
 assert.equal(edits.length, 1)
 assert.equal(edits[0]!.materialId, 'mat-draft-1')
 assert.ok(edits[0]!.requestId)
 assert.equal(edits[0]!.expectedRevision, 5)
 assert.deepEqual(edits[0]!.body, { sections: [{ heading: '面试准备（草稿）：教育经历与项目', content: '以下要点固定自实际投递版本与岗位快照，仅引用已确认事实。（本人补充：项目经历两段）', claims: [
  { claimId: 'claim-1', text: '本科在读（计算机科学与技术，大三，GPA 3.8）', factKey: '学历', needsReview: false },
  { claimId: 'claim-2', text: '实习经历待补充', needsReview: true, reviewNote: '缺实习经历，不得补造' },
 ] }] })
 assert.match(container.textContent ?? '', /修订已保存/)
 const revisedView = container.querySelector('[aria-label="修订后草稿"]')
 assert.ok(revisedView, 'the saved revision is read back from the material domain')
 assert.match(revisedView.textContent ?? '', /教育经历与项目/)
 assert.match(revisedView.textContent ?? '', /项目经历两段/)
 assert.match(revisedView.textContent ?? '', /已链接确认事实：学历/)
 // The anchored version stays visible after the revision.
 assert.match(container.querySelector('[aria-label="准备列表"]')?.textContent ?? '', /基于实际投递版本 V2/)
})

test('a generation failure keeps the request recoverable and never shows a blank success', async () => {
 const sent: Array<{ requestId: string }> = []
 const receiptReads: string[] = []
 let stored: PreparationReceipt[] = []
 const career: CareerStub = {
  open: async () => ({ revision: 5 }),
  applicationPreparations: async () => ({ preparations: stored }),
  generatePreparation: async (input: { requestId: string }) => { sent.push(input); throw Object.assign(new Error('写入结果未知'), { code: 'outcome_unknown', requestId: input.requestId }) },
  preparationReceipt: async (requestId: string) => { receiptReads.push(requestId); stored = [draft({ requestId })]; return stored[0] },
 }
 const container = await mountPreparation(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="准备焦点"]')!, 'interview_prep'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '生成准备草稿')); await settle(); await settle() })
 assert.equal(sent.length, 1)
 const originalRequestId = sent[0]!.requestId
 assert.match(container.textContent ?? '', new RegExp(`原请求编号 ${originalRequestId}`))
 assert.ok(!container.querySelector('[aria-label="准备列表"] > li'), 'no draft product exists while the outcome is unknown')
 await act(async () => { click(byLabel(container, 'button', '查询准备回执')); await settle(); await settle() })
 assert.deepEqual(receiptReads, [originalRequestId])
 assert.equal(container.querySelectorAll('[aria-label="准备列表"] > li').length, 1)
 assert.ok(!byLabelOrNull(container, 'button', '查询准备回执'), 'recovery state cleared after the receipt replay')
})

test('retrying an unknown generation reuses the same request id, and so does a typed failure', async () => {
 const sent: Array<{ requestId: string }> = []
 let failures = 1
 let stored: PreparationReceipt[] = []
 const career: CareerStub = {
  open: async () => ({ revision: 5 }),
  applicationPreparations: async () => ({ preparations: stored }),
  generatePreparation: async (input: { requestId: string }) => {
   sent.push(input)
   if (failures > 0) { failures -= 1; throw Object.assign(new Error('写入结果未知'), { code: 'outcome_unknown', requestId: input.requestId }) }
   stored = [draft({ requestId: input.requestId })]
   return stored[0]
  },
 }
 const container = await mountPreparation(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="准备焦点"]')!, 'interview_prep'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '生成准备草稿')); await settle(); await settle() })
 await act(async () => { click(byLabel(container, 'button', '用原请求编号重试')); await settle(); await settle() })
 assert.equal(sent.length, 2)
 assert.equal(sent[1]!.requestId, sent[0]!.requestId)
 assert.equal(container.querySelectorAll('[aria-label="准备列表"] > li').length, 1)

 // A typed generation failure also stays recoverable under the same id and
 // never renders a draft product.
 const failureSent: Array<{ requestId: string }> = []
 const failing: CareerStub = {
  open: async () => ({ revision: 5 }),
  applicationPreparations: async () => ({ preparations: [] }),
  generatePreparation: async (input: { requestId: string }) => { failureSent.push(input); throw Object.assign(new Error('生成失败'), { code: 'preparation_generation_failed' }) },
 }
 const failingContainer = await mountPreparation(failing)
 await act(async () => { choose(failingContainer.querySelector<HTMLSelectElement>('[aria-label="准备焦点"]')!, 'interview_prep'); await settle() })
 await act(async () => { click(byLabel(failingContainer, 'button', '生成准备草稿')); await settle(); await settle() })
 assert.equal(failureSent.length, 1)
 assert.match(failingContainer.querySelector('[role="alert"]')?.textContent ?? '', /生成失败/)
 assert.ok(!failingContainer.querySelector('[aria-label="准备列表"] > li'), 'no blank success product for a failed generation')
 await act(async () => { click(byLabel(failingContainer, 'button', '用原请求编号重试')); await settle(); await settle() })
 assert.equal(failureSent.length, 2)
 assert.equal(failureSent[1]!.requestId, failureSent[0]!.requestId)
})

test('a failed preparation row renders its typed failure state without a body', async () => {
 const failed = draft({ status: 'failed', materialId: undefined, body: { sections: [] }, reviewRisks: [], failureCode: 'generation_failed', failureMessage: '模型暂时不可用' })
 const career: CareerStub = {
  open: async () => ({ revision: 5 }),
  applicationPreparations: async () => ({ preparations: [failed] }),
  generatePreparation: async () => { throw new Error('no implicit generation') },
 }
 const container = await mountPreparation(career)
 const row = container.querySelector<HTMLElement>('[aria-label="准备列表"] > li')
 assert.ok(row)
 assert.match(row.textContent ?? '', /生成失败/)
 assert.match(row.textContent ?? '', /模型暂时不可用/)
 assert.ok(!row.querySelector('[aria-label="准备来源链"] .wk-preparation__claim'), 'a failed row renders no draft claims')
 assert.ok(!row.querySelector('[aria-label^="修订草稿"]'), 'a failed row offers no revision entry')
})

test('a revision conflict surfaces the current revision and offers a re-read', async () => {
 const career: CareerStub = {
  open: async () => ({ revision: 5 }),
  applicationPreparations: async () => ({ preparations: [] }),
  generatePreparation: async () => { throw Object.assign(new Error('revision conflict'), { code: 'revision_conflict', currentRevision: 9 }) },
 }
 const container = await mountPreparation(career)
 await act(async () => { choose(container.querySelector<HTMLSelectElement>('[aria-label="准备焦点"]')!, 'interview_prep'); await settle() })
 await act(async () => { click(byLabel(container, 'button', '生成准备草稿')); await settle(); await settle() })
 assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /当前修订 9/)
 assert.ok(byLabel(container, 'button', '重新读取档案修订'), 're-read action exists')
})

test('a cross-tenant application shows a clear inaccessible state', async () => {
 const career: CareerStub = { applicationPreparations: async () => { throw Object.assign(new Error('forbidden'), { code: 'forbidden' }) } }
 const container = await mountPreparation(career)
 assert.match(container.querySelector('[role="alert"]')?.textContent ?? '', /当前空间不可访问/)
 assert.ok(!container.querySelector('[aria-label="生成准备表单"]'), 'no generate form outside the accessible scope')
})

test('preparation styles keep TDesign light surfaces and the brand green confirm action', () => {
 const css = readFileSync(new URL('./preparation.css', import.meta.url), 'utf8')
 assert.match(css, /\.wk-preparation \{/)
 assert.match(css, /--td-bg-color-container/)
 assert.match(css, /#07c05f/)
})
