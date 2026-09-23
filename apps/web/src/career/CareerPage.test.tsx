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
 const client = { career: { ...api, sources: api.sources ?? (async () => []), upload: api.upload ?? (async () => { throw new Error('unused upload') }) } } as unknown as WeKnoraClient
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
 assert.match(container.textContent ?? '', /请检查当前修订后明确开始一次新的上传/)
 assert.match(container.textContent ?? '', /当前修订 4/)
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
 assert.match(container.textContent ?? '', /仍在处理中/)
 assert.equal(button(container, '开始上传').hasAttribute('disabled'), true)
})

test('scope switch clears resume source and upload state before old async response renders', async () => {
 let finishOld: ((value: unknown) => void) | undefined
 let sourceCall = 0
 const api = { open: async () => profile, list: async () => profile, changes: async () => ({ revision: 1, changes: [] }), act: async () => { throw new Error('unused') }, receipt: async () => { throw new Error('unused') }, sources: () => ++sourceCall === 1 ? new Promise((resolve) => { finishOld = resolve }) : Promise.resolve([]) }
 host = document.createElement('div'); document.body.append(host); root = createRoot(host)
 const client = { career: api } as unknown as WeKnoraClient
 const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'u', tenantId: 't' })
 await act(async () => { root!.render(React.createElement(CareerPage, { client, scopeController, userId: 'u' })); await new Promise((resolve) => setImmediate(resolve)) })
 scopeController.switchScope('https://weknora.test', 'other', 'other-tenant')
 await act(async () => { root!.render(React.createElement(CareerPage, { client, scopeController, userId: 'other' })); await new Promise((resolve) => setImmediate(resolve)) })
 finishOld?.([{ id: 'private-old', revision: 1, fileName: 'private.pdf', mimeType: 'application/pdf', size: 1, digest: 'd', status: 'ready', createdAt: 'now' }])
 await act(async () => { await new Promise((resolve) => setImmediate(resolve)) })
 assert.doesNotMatch(host.textContent ?? '', /private\.pdf/)
})
