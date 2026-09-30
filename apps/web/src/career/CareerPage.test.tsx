import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import * as nodeModule from 'node:module'
import test, { afterEach } from 'node:test'
import * as React from 'react'
import { act } from 'react'
import type { Root } from 'react-dom/client'
import { createScopeController } from '@weknora/domain/scope'
import type { WeKnoraClient } from '@weknora/api-client'
import type { CareerAction, CareerReceipt, CareerView } from '../../../../packages/career-core/src/contracts.ts'

// CareerPage now mounts the T22 export/deletion surface, which carries its
// own stylesheet; resolve .css imports to a stub like the sibling page tests.
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void }
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) })

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
 const client = { career: { ...api, sources: api.sources ?? (async () => []), upload: api.upload ?? (async () => { throw new Error('unused upload') }), reminders: api.reminders ?? (async () => ({ reminders: [] })) } } as unknown as WeKnoraClient
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'u', tenantId: 't' })
 await act(async () => { root!.render(React.createElement(CareerPage, { client, scopeController, userId: 'u' })); await new Promise((resolve) => setImmediate(resolve)); await new Promise((resolve) => setImmediate(resolve)) })
 return host
}
const button = (container: HTMLElement, label: string): HTMLElement => {
 const found = [...container.querySelectorAll('.t-button')].find((item) => item.textContent?.trim() === label)
 assert.ok(found, `button ${label} exists`); return found as HTMLElement
}
const nativeButton = (container: HTMLElement, label: string): HTMLButtonElement => {
 const found = [...container.querySelectorAll('button')].find((item) => item.textContent?.trim() === label)
 assert.ok(found, `button ${label} exists`); return found as HTMLButtonElement
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
  for (const label of ['保存为提案', '直接确认', '确认', '忽略']) assert.equal([...container.querySelectorAll('.t-button')].some((item) => item.textContent?.trim() === label), false, `${label} is hidden`)
 })

test('forbidden receipt after an ambiguous action hides cached facts in the mounted page', async () => {
 const container = await mount({
  open: async () => profile, list: async () => profile, changes: async () => ({ revision: 1, changes: [] }),
  act: async () => { throw Object.assign(new Error('timed out'), { code: 'TIMEOUT' }) },
  receipt: async () => { throw Object.assign(new Error('access revoked'), { code: 'forbidden' }) },
 } as never)
 assert.match(container.textContent ?? '', /本科/)
 await act(async () => { button(container, '确认').click(); await new Promise((resolve) => setImmediate(resolve)) })
 assert.match(container.textContent ?? '', /当前空间不可访问/)
 assert.doesNotMatch(container.textContent ?? '', /本科/)
 assert.equal([...container.querySelectorAll('.t-button')].some((item) => ['保存为提案', '直接确认', '确认', '忽略'].includes(item.textContent?.trim() ?? '')), false)
})

test('resume upload renders six category review, exact evidence and confirmed facts stay separate', async () => {
 const documentSource = { id: 'source-1', revision: 2, fileName: 'resume.pdf', mimeType: 'application/pdf', size: 12, digest: 'digest', status: 'ready' as const, missingCategories: ['education.graduation_date'], reviewFlags: ['experience.date_conflict'], createdAt: 'now' }
 const intakeProposals = [
  { id: 'e1', key: 'education.school', value: 'Example University', evidence: '教育背景：Example University', source: { kind: 'resume_extraction', referenceId: 'source-1' }, status: 'pending' as const, createdAt: 'now' },
  { id: 'e2', key: 'experience.company', value: 'Example Co', source: { kind: 'resume_extraction', referenceId: 'source-1' }, status: 'pending' as const, createdAt: 'now' },
  { id: 'e3', key: 'project.name', value: 'Search Engine', source: { kind: 'resume_extraction', referenceId: 'source-1' }, status: 'pending' as const, createdAt: 'now' },
  { id: 'e4', key: 'skill.language', value: 'Go', source: { kind: 'resume_extraction', referenceId: 'source-1' }, status: 'pending' as const, createdAt: 'now' },
  { id: 'e5', key: 'achievement.metric', value: 'Reduced latency 20%', source: { kind: 'resume_extraction', referenceId: 'source-1' }, status: 'pending' as const, createdAt: 'now' },
  { id: 'e6', key: 'certificate.name', value: 'Cloud certificate', source: { kind: 'resume_extraction', referenceId: 'source-1' }, status: 'pending' as const, createdAt: 'now' },
 ]
 let uploadedExpectedRevision = -1
 const container = await mount({
  open: async () => profile, list: async () => ({ ...profile, revision: 2, proposals: [...profile.proposals, ...intakeProposals] }), changes: async () => ({ revision: 1, changes: [] }), act: async () => { throw new Error('unused') }, receipt: async () => { throw new Error('unused') },
  sources: async () => [documentSource], upload: async (_file: Blob, _name: string, _requestId: string, revision: number) => { uploadedExpectedRevision = revision; return { source: documentSource, receipt: { kind: 'intake_completed', requestId: 'source-1:batch', revision: 2, proposals: intakeProposals } } },
 } as never)
 assert.match(container.textContent ?? '', /来源版本/)
 assert.match(container.textContent ?? '', /缺失类别：education\.graduation_date/)
 assert.match(container.textContent ?? '', /experience\.date_conflict/)
 const fileInput = container.querySelector<HTMLInputElement>('#career-resume-file')!
 const file = new dom.window.File(['resume'], 'resume.pdf', { type: 'application/pdf' })
 Object.defineProperty(fileInput, 'files', { configurable: true, value: [file] })
 await act(async () => { fileInput.dispatchEvent(new dom.window.Event('change', { bubbles: true })) })
 await act(async () => { button(container, '开始上传').click(); await new Promise((resolve) => setImmediate(resolve)) })
 assert.equal(uploadedExpectedRevision, 1)
 assert.match(container.textContent ?? '', /简历已处理/)
 assert.match(container.textContent ?? '', /本科/)
 assert.match(container.textContent ?? '', /Example University/)
 assert.match(container.textContent ?? '', /教育背景：Example University/)
 assert.match(container.textContent ?? '', /待确认/)
 for (const value of ['Example Co', 'Search Engine', 'Go', 'Reduced latency 20%', 'Cloud certificate']) assert.match(container.textContent ?? '', new RegExp(value))
})

test('manual project, internship and skill entries submit exact keys and values without a resume', async () => {
 const sent: CareerAction[] = []
 const container = await mount({ open: async () => profile, list: async () => profile, changes: async () => ({ revision: 1, changes: [] }), act: async (action: CareerAction) => { sent.push(action); if (action.action !== 'confirm') throw new Error('expected direct confirmation'); return { kind: 'confirmed', requestId: action.requestId!, revision: sent.length + 1, fact: { key: action.key, value: action.value, revision: sent.length + 1, source: { kind: 'user', label: '本人确认' }, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' } } }, receipt: async () => { throw new Error('unused') } } as never)
 const cases = [
  { label: '项目经历', key: 'project.name', value: 'Search Engine' },
  { label: '实习经历', key: 'internship.company', value: 'Example Co' },
  { label: '技能', key: 'skill.name', value: 'Go' },
 ]
 assert.match(container.textContent ?? '', /已确认档案/, `loaded profile visible: ${container.textContent}`)
 const selector = container.querySelector('.t-select input') as HTMLElement
 assert.ok(selector, 'manual field selector is rendered')
 for (const [index, item] of cases.entries()) {
  await act(async () => { selector.click(); await new Promise((resolve) => setImmediate(resolve)) })
  const option = [...document.querySelectorAll<HTMLElement>('.t-select-option')].find((candidate) => candidate.textContent?.trim() === item.label)
  assert.ok(option, `${item.label} is selectable without a resume`)
  await act(async () => { option!.click(); await new Promise((resolve) => setImmediate(resolve)) })
  const valueInput = container.querySelector('input[placeholder="填写待确认内容"]') as HTMLInputElement
  const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(valueInput), 'value')?.set
  setter?.call(valueInput, item.value)
  await act(async () => { valueInput.dispatchEvent(new dom.window.Event('input', { bubbles: true })); button(container, '直接确认').click(); await new Promise((resolve) => setImmediate(resolve)); await new Promise((resolve) => setImmediate(resolve)) })
  assert.equal(sent[index]?.action, 'confirm')
  assert.equal(sent[index]?.key, item.key)
  assert.equal(sent[index]?.value, item.value)
  assert.deepEqual(sent[index]?.source, { kind: 'user', label: '本人确认' })
  assert.match(container.textContent ?? '', new RegExp(item.value))
 }
})

test('confirmed deletion keeps parent profile and source list cleared when reload rejects', async () => {
 let opens = 0
 const documentSource = { id: 'source-private', revision: 2, fileName: 'private-resume.pdf', mimeType: 'application/pdf', size: 12, digest: 'private-digest', status: 'ready' as const, createdAt: 'now' }
 const container = await mount({
  open: async () => { opens += 1; if (opens > 3) throw Object.assign(new Error('reload offline'), { code: 'NETWORK_ERROR' }); return profile },
  list: async () => profile, changes: async () => ({ revision: 1, changes: [] }),
  sources: async () => [documentSource], reminders: async () => ({ reminders: [] }),
  act: async () => { throw new Error('unused') }, receipt: async () => { throw new Error('unused') },
  careerDeletionBoundary: async () => ({
   inSpace: [{ section: 'profile', description: '档案事实', count: 1 }],
   external: [{ item: 'external_platform_submissions', description: '不可撤回', revocable: false }],
   retention: [{ holder: 'career_data_deletions', reason: '删除审计', status: 'retained' }],
  }),
  deleteCareer: async (input: Record<string, unknown>) => ({ kind: 'career_deleted', requestId: input.requestId, status: 'deleted', steps: ['revoke_material_exports','purge_career_data','remove_workbench_tasks','finalize'].map((name) => ({ name, status: 'done' })), retention: [{ holder: 'career_data_deletions', reason: '删除审计', status: 'retained' }], revision: 2, startedAt: 'now', completedAt: 'now' }),
 } as never)
 assert.match(container.textContent ?? '', /本科/)
 assert.match(container.textContent ?? '', /private-resume\.pdf/)
 await act(async () => { nativeButton(container, '查看删除边界').click(); await new Promise((resolve) => setImmediate(resolve)); await new Promise((resolve) => setImmediate(resolve)) })
 const checkbox = container.querySelector<HTMLInputElement>('.wk-lifecycle__ack input[type="checkbox"]')!
 const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(checkbox), 'checked')?.set
 setter?.call(checkbox, true)
 await act(async () => { checkbox.dispatchEvent(new dom.window.Event('click', { bubbles: true })) })
 await act(async () => { nativeButton(container, '发起完整删除').click(); await new Promise((resolve) => setImmediate(resolve)); await new Promise((resolve) => setImmediate(resolve)); await new Promise((resolve) => setImmediate(resolve)) })
 assert.ok(opens > 3, 'the post-deletion profile reload was attempted and rejected')
 assert.doesNotMatch(container.textContent ?? '', /本科|2027|private-resume\.pdf/)
 assert.match(container.textContent ?? '', /已完全删除/)
 assert.match(container.textContent ?? '', /删除审计/)
 assert.equal(container.querySelector('[aria-label="简历来源版本"]'), null)
 assert.equal(container.querySelector<HTMLInputElement>('#career-resume-file')?.disabled, false)
})

test('failed resume upload preserves confirmed facts and offers a fresh attempt', async () => {
 const failedSource = { id: 'failed-1', revision: 2, fileName: 'bad.pdf', mimeType: 'application/pdf', size: 3, digest: 'd', status: 'failed' as const, errorMessage: '无法解析简历', createdAt: 'now' }
 const container = await mount({
  open: async () => profile, list: async () => profile, changes: async () => ({ revision: 1, changes: [] }), act: async () => { throw new Error('unused') }, receipt: async () => { throw new Error('unused') },
  sources: async () => [failedSource], upload: async () => ({ source: failedSource }),
 } as never)
 const fileInput = container.querySelector<HTMLInputElement>('#career-resume-file')!
 Object.defineProperty(fileInput, 'files', { configurable: true, value: [new dom.window.File(['bad'], 'bad.pdf', { type: 'application/pdf' })] })
 await act(async () => { fileInput.dispatchEvent(new dom.window.Event('change', { bubbles: true })) })
 await act(async () => { button(container, '开始上传').click(); await new Promise((resolve) => setImmediate(resolve)) })
 assert.match(container.textContent ?? '', /无法解析简历/)
 assert.match(container.textContent ?? '', /本科/)
 assert.match(container.textContent ?? '', /不会覆盖已确认档案/)
 assert.equal(fileInput.disabled, false)
 Object.defineProperty(fileInput, 'files', { configurable: true, value: [new dom.window.File(['new'], 'new.pdf', { type: 'application/pdf' })] })
 await act(async () => { fileInput.dispatchEvent(new dom.window.Event('change', { bubbles: true })) })
 assert.equal(button(container, '开始上传').hasAttribute('disabled'), false)
})

test('definitive upload rejections release the retained file so the user can start a fresh request', async () => {
 for (const code of ['invalid_request', 'idempotency_conflict'] as const) {
  const attempts: Array<{ file: Blob; requestId: string; revision: number }> = []
  let callCount = 0
  const terminalSource = { id: `terminal-${code}`, revision: 2, fileName: 'new.pdf', mimeType: 'application/pdf', size: 3, digest: 'd', status: 'failed' as const, errorMessage: 'new attempt failed', createdAt: 'now' }
  const container = await mount({
   open: async () => profile, list: async () => profile, changes: async () => ({ revision: 1, changes: [] }), act: async () => { throw new Error('unused') }, receipt: async () => { throw new Error('unused') },
   upload: async (file: Blob, _name: string, requestId: string, revision: number) => { attempts.push({ file, requestId, revision }); callCount += 1; if (callCount === 1) throw Object.assign(new Error(`definitive ${code}`), { code }); return { source: terminalSource } },
  } as never)
  const fileInput = container.querySelector<HTMLInputElement>('#career-resume-file')!
  Object.defineProperty(fileInput, 'files', { configurable: true, value: [new dom.window.File(['old'], 'old.pdf', { type: 'application/pdf' })] })
  await act(async () => { fileInput.dispatchEvent(new dom.window.Event('change', { bubbles: true })) })
  await act(async () => { button(container, '开始上传').click(); await new Promise((resolve) => setImmediate(resolve)) })
  assert.match(container.textContent ?? '', new RegExp(`definitive ${code}`))
  assert.match(container.textContent ?? '', /本科/)
  assert.equal(fileInput.disabled, false)
  assert.equal(container.querySelector('[aria-label="恢复简历上传"]'), null)
  Object.defineProperty(fileInput, 'files', { configurable: true, value: [new dom.window.File(['new'], 'new.pdf', { type: 'application/pdf' })] })
  await act(async () => { fileInput.dispatchEvent(new dom.window.Event('change', { bubbles: true })) })
  await act(async () => { button(container, '开始上传').click(); await new Promise((resolve) => setImmediate(resolve)) })
  assert.equal(attempts.length, 2)
  assert.notEqual(attempts[0]?.requestId, attempts[1]?.requestId)
  await act(async () => { root?.unmount() }); root = undefined; host?.remove(); host = undefined
 }
})

test('known ready upload stays successful when source or profile refresh fails', async () => {
 for (const failedRead of ['sources', 'profile'] as const) {
  const readySource = { id: `ready-${failedRead}`, revision: 2, fileName: 'resume.pdf', mimeType: 'application/pdf', size: 6, digest: 'd', status: 'ready' as const, createdAt: 'now' }
  const intakeProposal = { id: `proposal-${failedRead}`, key: 'education.school', value: 'Example University', evidence: 'Exact source line', source: { kind: 'resume_extraction', referenceId: readySource.id }, status: 'pending' as const, createdAt: 'now' }
  let sourceReads = 0
  let profileReads = 0
  const container = await mount({
   open: async () => profile, list: async () => { profileReads += 1; if (failedRead === 'profile' && profileReads > 0) throw Object.assign(new Error('profile refresh offline'), { code: 'NETWORK_ERROR' }); return failedRead === 'sources' ? { ...profile, revision: 2, proposals: [...profile.proposals, intakeProposal] } : profile },
   changes: async () => ({ revision: 1, changes: [] }), act: async () => { throw new Error('unused') }, receipt: async () => { throw new Error('unused') },
   sources: async () => { sourceReads += 1; if (failedRead === 'sources' && sourceReads === 2) throw Object.assign(new Error('source refresh offline'), { code: 'NETWORK_ERROR' }); return sourceReads > 2 ? [readySource] : [] },
   upload: async () => ({ source: readySource, receipt: { kind: 'intake_completed', requestId: `${readySource.id}:batch`, revision: 2, proposals: [intakeProposal] } }),
  } as never)
  const fileInput = container.querySelector<HTMLInputElement>('#career-resume-file')!
  Object.defineProperty(fileInput, 'files', { configurable: true, value: [new dom.window.File(['resume'], 'resume.pdf', { type: 'application/pdf' })] })
  await act(async () => { fileInput.dispatchEvent(new dom.window.Event('change', { bubbles: true })) })
  await act(async () => { button(container, '开始上传').click(); await new Promise((resolve) => setImmediate(resolve)) })
  assert.match(container.textContent ?? '', /简历已处理，生成 1 条待确认提案/)
  assert.match(container.textContent ?? '', /Example University/)
  assert.match(container.textContent ?? '', /Exact source line/)
  assert.match(container.textContent ?? '', /本科/)
  assert.match(container.textContent ?? '', new RegExp(failedRead === 'sources' ? 'source refresh offline' : 'profile refresh offline'))
  assert.equal(container.querySelector('[aria-label="恢复简历上传"]'), null)
  assert.equal(fileInput.disabled, false)
  if (failedRead === 'sources') {
   await act(async () => { button(container, '刷新来源').click(); await new Promise((resolve) => setImmediate(resolve)) })
   assert.match(container.textContent ?? '', /来源列表已刷新/)
   assert.match(container.textContent ?? '', /resume\.pdf/)
  }
  await act(async () => { root?.unmount() }); root = undefined; host?.remove(); host = undefined
 }
})

test('revision conflict refreshes and requires a deliberate new upload attempt', async () => {
 let viewReads = 0
 const container = await mount({
  open: async () => profile, list: async () => { viewReads += 1; return { ...profile, revision: viewReads > 1 ? 4 : 1 } }, changes: async () => ({ revision: 1, changes: [] }), act: async () => { throw new Error('unused') }, receipt: async () => { throw new Error('unused') },
  upload: async () => { throw Object.assign(new Error('profile changed'), { code: 'revision_conflict', currentRevision: 4 }) },
 } as never)
 const fileInput = container.querySelector<HTMLInputElement>('#career-resume-file')!
 Object.defineProperty(fileInput, 'files', { configurable: true, value: [new dom.window.File(['resume'], 'resume.pdf', { type: 'application/pdf' })] })
 await act(async () => { fileInput.dispatchEvent(new dom.window.Event('change', { bubbles: true })) })
 await act(async () => { button(container, '开始上传').click(); await new Promise((resolve) => setImmediate(resolve)) })
 assert.match(container.textContent ?? '', /请检查当前修订后重新选择文件并开始一次新的上传/)
 assert.match(container.textContent ?? '', /当前修订 4/)
 assert.equal(fileInput.disabled, false)
 Object.defineProperty(fileInput, 'files', { configurable: true, value: [new dom.window.File(['new resume'], 'new-resume.pdf', { type: 'application/pdf' })] })
 await act(async () => { fileInput.dispatchEvent(new dom.window.Event('change', { bubbles: true })) })
 assert.equal(button(container, '开始上传').hasAttribute('disabled'), false)
})

test('ambiguous resume upload retains the original identity and processing prevents another claim', async () => {
 const pendingSource = { id: 'source-1', revision: 2, fileName: 'resume.pdf', mimeType: 'application/pdf', size: 12, digest: 'digest', status: 'processing' as const, createdAt: 'now' }
 const attempts: Array<{ id: string; revision: number; file: Blob }> = []
 const container = await mount({
  open: async () => profile, list: async () => profile, changes: async () => ({ revision: 1, changes: [] }), act: async () => { throw new Error('unused') }, receipt: async () => { throw new Error('unused') },
  sources: async () => [pendingSource], upload: async (file: Blob, _name: string, id: string, revision: number) => { attempts.push({ file, id, revision }); throw Object.assign(new Error('network timeout'), { code: 'TIMEOUT' }) },
 } as never)
 const fileInput = container.querySelector<HTMLInputElement>('#career-resume-file')!
 const file = new dom.window.File(['resume'], 'resume.pdf', { type: 'application/pdf' })
 Object.defineProperty(fileInput, 'files', { configurable: true, value: [file] })
 await act(async () => { fileInput.dispatchEvent(new dom.window.Event('change', { bubbles: true })) })
 await act(async () => { button(container, '开始上传').click(); await new Promise((resolve) => setImmediate(resolve)) })
 assert.equal(attempts.length, 1)
 assert.match(container.textContent ?? '', /上传结果暂时未知/)
 await act(async () => { button(container, '查询来源状态').click(); await new Promise((resolve) => setImmediate(resolve)) })
 assert.match(container.textContent ?? '', /无法确认本次上传结果/)
 assert.equal(button(container, '开始上传').hasAttribute('disabled'), true)
})

test('same-name source history cannot resolve an unknown upload; exact replay keeps its original tuple', async () => {
 for (const status of ['ready', 'failed', 'processing'] as const) {
  const oldSource = { id: `old-${status}`, revision: 1, fileName: 'resume.pdf', mimeType: 'application/pdf', size: 12, digest: 'old', status, errorMessage: status === 'failed' ? 'old failure' : undefined, createdAt: 'yesterday' }
  const replayed = { ...oldSource, id: 'new-source', revision: 2, digest: 'new', status: 'ready' as const, errorMessage: undefined }
  const attempts: Array<{ file: Blob; name: string; requestId: string; expectedRevision: number }> = []
  let calls = 0
  const container = await mount({
   open: async () => profile, list: async () => profile, changes: async () => ({ revision: 1, changes: [] }), act: async () => { throw new Error('unused') }, receipt: async () => { throw new Error('unused') },
   sources: async () => [oldSource], upload: async (file: Blob, name: string, requestId: string, expectedRevision: number) => {
    attempts.push({ file, name, requestId, expectedRevision }); calls += 1
    if (calls === 1) throw Object.assign(new Error('timeout'), { code: 'TIMEOUT' })
    return { source: replayed }
   },
  } as never)
  const fileInput = container.querySelector<HTMLInputElement>('#career-resume-file')!
  const originalFile = new dom.window.File(['new resume'], 'resume.pdf', { type: 'application/pdf' })
  Object.defineProperty(fileInput, 'files', { configurable: true, value: [originalFile] })
  await act(async () => { fileInput.dispatchEvent(new dom.window.Event('change', { bubbles: true })) })
  await act(async () => { button(container, '开始上传').click(); await new Promise((resolve) => setImmediate(resolve)) })
  const first = attempts[0]!
  await act(async () => { button(container, '查询来源状态').click(); await new Promise((resolve) => setImmediate(resolve)) })
  assert.match(container.textContent ?? '', /无法确认本次上传结果/)
  assert.ok(container.querySelector('[aria-label="恢复简历上传"]'))
  await act(async () => { button(container, '用原文件和请求编号重试').click(); await new Promise((resolve) => setImmediate(resolve)) })
  assert.equal(attempts.length, 2)
  assert.equal(attempts[1]?.file, first.file)
  assert.equal(attempts[1]?.name, first.name)
  assert.equal(attempts[1]?.requestId, first.requestId)
  assert.ok(first.requestId.length > 0)
  assert.equal(attempts[1]?.expectedRevision, first.expectedRevision)
  assert.equal(container.querySelector('[aria-label="恢复简历上传"]'), null)
  await act(async () => { root?.unmount() }); root = undefined; host?.remove(); host = undefined
 }
})

test('forbidden source read invalidates an outstanding upload and fences its late response', async () => {
 let rejectInitialSources: ((error: unknown) => void) | undefined
 let resolveUpload: ((value: unknown) => void) | undefined
 let resolveLateSources: ((value: unknown) => void) | undefined
 let sourceCalls = 0
 const container = await mount({
  open: async () => profile, list: async () => profile, changes: async () => ({ revision: 1, changes: [] }), act: async () => { throw new Error('unused') }, receipt: async () => { throw new Error('unused') },
  sources: () => ++sourceCalls === 1 ? new Promise((_resolve, reject) => { rejectInitialSources = reject }) : new Promise((resolve) => { resolveLateSources = resolve }),
  upload: () => new Promise((resolve) => { resolveUpload = resolve }),
 } as never)
 const fileInput = container.querySelector<HTMLInputElement>('#career-resume-file')!
 Object.defineProperty(fileInput, 'files', { configurable: true, value: [new dom.window.File(['resume'], 'private.pdf', { type: 'application/pdf' })] })
 await act(async () => { fileInput.dispatchEvent(new dom.window.Event('change', { bubbles: true })) })
 await act(async () => { button(container, '开始上传').click(); await new Promise((resolve) => setImmediate(resolve)) })
 assert.ok(resolveUpload, 'upload request is still pending when authorization is lost')
 rejectInitialSources?.(Object.assign(new Error('access revoked'), { code: 'forbidden' }))
 await act(async () => { await new Promise((resolve) => setImmediate(resolve)) })
 assert.match(container.textContent ?? '', /当前空间不可访问/)
 assert.doesNotMatch(container.textContent ?? '', /本科/)
 resolveUpload?.({ source: { id: 'private-source', revision: 2, fileName: 'private.pdf', mimeType: 'application/pdf', size: 6, digest: 'd', status: 'ready', createdAt: 'now' }, receipt: { kind: 'intake_completed', requestId: 'private-batch', revision: 2, proposals: [{ id: 'private-proposal', key: 'education.school', value: 'Secret University', source: { kind: 'resume_extraction', referenceId: 'private-source' }, status: 'pending', createdAt: 'now' }] } })
 await act(async () => { await new Promise((resolve) => setImmediate(resolve)) })
 assert.equal(sourceCalls, 1, 'the invalidated upload must not start a follow-up source request')
 assert.doesNotMatch(container.textContent ?? '', /private\.pdf|Secret University|简历已处理/)
 assert.doesNotMatch(container.textContent ?? '', /来源状态已更新/)
 resolveLateSources?.([])
 assert.equal(container.querySelector('[aria-label="恢复简历上传"]'), null)
})

test('forbidden recovery source read does not write a late notice or retain the upload', async () => {
 let rejectRecoverySources: ((error: unknown) => void) | undefined
 let sourcesCall = 0
 const container = await mount({
  open: async () => profile, list: async () => profile, changes: async () => ({ revision: 1, changes: [] }), act: async () => { throw new Error('unused') }, receipt: async () => { throw new Error('unused') },
  sources: () => ++sourcesCall === 1 ? Promise.resolve([]) : new Promise((_resolve, reject) => { rejectRecoverySources = reject }),
  upload: async () => { throw Object.assign(new Error('timeout'), { code: 'TIMEOUT' }) },
 } as never)
 const fileInput = container.querySelector<HTMLInputElement>('#career-resume-file')!
 Object.defineProperty(fileInput, 'files', { configurable: true, value: [new dom.window.File(['resume'], 'resume.pdf', { type: 'application/pdf' })] })
 await act(async () => { fileInput.dispatchEvent(new dom.window.Event('change', { bubbles: true })) })
 await act(async () => { button(container, '开始上传').click(); await new Promise((resolve) => setImmediate(resolve)) })
 await act(async () => { button(container, '查询来源状态').click() })
 rejectRecoverySources?.(Object.assign(new Error('access revoked'), { code: 'forbidden' }))
 await act(async () => { await new Promise((resolve) => setImmediate(resolve)) })
 assert.match(container.textContent ?? '', /当前空间不可访问/)
 assert.equal(container.querySelector('[aria-label="恢复简历上传"]'), null)
})

test('scope switch clears resume source and upload state before old async response renders', async () => {
 let finishOld: ((value: unknown) => void) | undefined
 let sourceCall = 0
 const api = { open: async () => profile, list: async () => profile, changes: async () => ({ revision: 1, changes: [] }), act: async () => { throw new Error('unused') }, receipt: async () => { throw new Error('unused') }, sources: () => ++sourceCall === 1 ? new Promise((resolve) => { finishOld = resolve }) : Promise.resolve([]) }
 host = document.createElement('div'); document.body.append(host); root = createRoot(host)
 const client = { career: api } as unknown as WeKnoraClient
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'u', tenantId: 't' })
 await act(async () => { root!.render(React.createElement(CareerPage, { client, scopeController, userId: 'u' })); await new Promise((resolve) => setImmediate(resolve)); await new Promise((resolve) => setImmediate(resolve)) })
 scopeController.switchScope('https://weknora.test', 'other', 'other-tenant')
 await act(async () => { root!.render(React.createElement(CareerPage, { client, scopeController, userId: 'other' })); await new Promise((resolve) => setImmediate(resolve)); await new Promise((resolve) => setImmediate(resolve)) })
 finishOld?.([{ id: 'private-old', revision: 1, fileName: 'private.pdf', mimeType: 'application/pdf', size: 1, digest: 'd', status: 'ready', createdAt: 'now' }])
 await act(async () => { await new Promise((resolve) => setImmediate(resolve)) })
 assert.doesNotMatch(host.textContent ?? '', /private\.pdf/)
})
