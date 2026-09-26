import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ScopeController } from '@weknora/domain/scope'
import type { MaterialBody, PreparationFocus, PreparationReceipt } from '../../../../packages/api-client/src/career.ts'
import './preparation.css'

type WritePhase = 'idle' | 'busy' | 'unknown' | 'error'
type ReadState = 'loading' | 'ready' | 'error' | 'forbidden' | 'scope-changed'
type GenerateAttempt = { requestId: string; focus: PreparationFocus; expectedRevision: number }
type ReviseAttempt = { requestId: string; materialId: string; body: MaterialBody; expectedRevision: number }
const newRequestId = (): string => typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`
// Frozen backend enums rendered verbatim (internal/modules/career/preparation.go):
// the two T19 drafts are the closed focus vocabulary, and the version anchor
// is always the actually submitted version — never the latest one.
const focusLabels: Record<PreparationFocus, string> = { cover_letter: '求职信', interview_prep: '面试准备' }
const focusOptions = Object.keys(focusLabels) as PreparationFocus[]

// A receipt that does not match the request it answers is a definite protocol
// error, unlike a network TypeError (a failed fetch), which leaves the write
// outcome genuinely unknown and must route into receipt recovery.
class ReceiptMismatchError extends Error {}

function errorDetails(cause: unknown): { code?: string; requestId?: string; currentRevision?: number; status?: number; message: string } {
 const error = cause as { code?: string; requestId?: string; currentRevision?: string | number; status?: number; message?: string }
 const currentRevision = typeof error?.currentRevision === 'number' ? error.currentRevision : undefined
 return { code: error?.code, requestId: error?.requestId, currentRevision, status: error?.status, message: error?.message || '请求未完成' }
}
function isUncertainWrite(cause: unknown): boolean {
 const error = errorDetails(cause)
 if (error.code === 'TIMEOUT' || error.code === 'outcome_unknown') return true
 if (['forbidden', 'invalid_request', 'idempotency_conflict', 'revision_conflict', 'preparation_version_unknown', 'preparation_generation_failed', 'request_too_large', 'PAYLOAD_TOO_LARGE', 'not_found', 'unauthorized'].includes(error.code ?? '')) return false
 if (error.status !== undefined) return error.status >= 500 || error.status < 400
 return true
}
const cloneBody = (body: MaterialBody): MaterialBody => ({ sections: body.sections.map((section) => ({ heading: section.heading, content: section.content, claims: section.claims.map((claim) => ({ ...claim })) })) })

function PreparationRow({ item, revising, onRevise }: { item: PreparationReceipt; revising: boolean; onRevise: (receipt: PreparationReceipt) => void }): ReactNode {
 const failed = item.status !== 'draft'
 return <li className={`wk-preparation__item${failed ? ' wk-preparation__item--failed' : ''}`} aria-label={`准备 ${item.preparationId}`}>
  <div className="wk-preparation__item-head">
   <strong>{focusLabels[item.focus]}</strong>
   <span>{failed ? (item.status === 'failed' ? '生成失败' : '生成中（可恢复）') : '草稿（可审阅、可修订）'}</span>
   <span>请求编号 <code>{item.requestId}</code></span>
   <time dateTime={item.createdAt}>{item.createdAt}</time>
  </div>
  <p className="wk-preparation__anchor">基于实际投递版本 V{item.anchor.version}（材料 <code>{item.anchor.materialId}</code> · 导出 <code>{item.anchor.exportId}</code> · 提交 <code>{item.anchor.submissionId}</code>）</p>
  {failed ? <>
   {item.failureCode ? <p className="wk-preparation__failure" role="alert">生成失败（{item.failureCode}）：{item.failureMessage ?? '生成未完成'}。请求与锚定版本已保留，可用原请求编号重试；不会留下空白成功产物。</p> : <p className="wk-preparation__failure" role="alert">生成中断（结果未知）。请求与锚定版本已保留，可用原请求编号恢复。</p>}
   {item.sources.snapshot.snapshotId ? <p className="wk-preparation__sources-line">拟锚定投递版本 V{item.sources.submittedVersion.version} · 快照 <code>{item.sources.snapshot.snapshotId}</code> · 档案修订 {item.sources.profileRevision}</p> : null}
  </> : <>
   <div className="wk-preparation__sources" aria-label="准备来源链">
    <h4>来源链（可追溯）</h4>
    <ul>
     <li>投递版本 V{item.sources.submittedVersion.version}：提交 <code>{item.sources.submittedVersion.submissionId}</code> · 材料 <code>{item.sources.submittedVersion.materialId}</code> · 内容摘要 <code>{item.sources.submittedVersion.contentDigest.slice(0, 12)}…</code></li>
     {item.sources.snapshot.snapshotId ? <li>岗位快照 <code>{item.sources.snapshot.snapshotId}</code>{item.sources.snapshot.snapshotSha256 ? <>（SHA-256 <code>{item.sources.snapshot.snapshotSha256.slice(0, 12)}…</code>）</> : null}</li> : null}
     <li>已确认事实：{item.sources.factKeys.length ? item.sources.factKeys.map((key) => <code key={key}>{key}</code>) : '（本草稿未引用事实键）'}</li>
     <li>档案修订 {item.sources.profileRevision} · 材料草稿 <code>{item.materialId}</code></li>
    </ul>
   </div>
   <div className="wk-preparation__body">
    {item.body.sections.map((section, index) => <div className="wk-preparation__section" key={`${section.heading}-${index}`}>
     <h5>{section.heading}</h5>
     <p>{section.content}</p>
     {section.claims.length ? <ul>{section.claims.map((claim) => <li key={claim.claimId} className={claim.needsReview ? 'wk-preparation__claim--needs-review' : undefined}>{claim.text}{claim.factKey ? `（已链接确认事实：${claim.factKey}）` : claim.needsReview ? '（缺失/待补充，不得补造）' : ''}</li>)}</ul> : null}
    </div>)}
    {item.reviewRisks.length ? <p className="wk-preparation__risks">审阅提示：{item.reviewRisks.map((risk) => risk.message).join('；')}</p> : null}
    <div className="wk-preparation__actions"><button type="button" className="wk-preparation__revise" aria-label={`修订草稿 ${item.materialId}`} disabled={revising} onClick={() => onRevise(item)}>修订草稿（保存为材料草稿）</button></div>
   </div>
  </>}
 </li>
}

// From the application detail the user generates the cover letter / interview
// preparation draft of exactly the version they actually submitted. The draft
// cites its full source chain (submitted version, frozen snapshot, confirmed
// facts), stays a material-domain draft the user reviews and revises, and
// nothing here ever sends, mails, or promises anything on the user's behalf.
export function PreparationPage({ client, scopeController, applicationId }: { client: WeKnoraClient; scopeController: ScopeController; applicationId: string }): ReactNode {
 const scope = scopeController.current()
 const [items, setItems] = useState<PreparationReceipt[]>()
 const [readState, setReadState] = useState<ReadState>('loading')
 const [readMessage, setReadMessage] = useState('')
 const [reload, setReload] = useState(0)
 const [revision, setRevision] = useState<number | undefined>()
 const [revisionState, setRevisionState] = useState<'loading' | 'ready' | 'error'>('loading')
 const [focus, setFocus] = useState('')
 const [attempt, setAttempt] = useState<GenerateAttempt>()
 const [writePhase, setWritePhase] = useState<WritePhase>('idle')
 const [message, setMessage] = useState('')
 const [revisionConflict, setRevisionConflict] = useState<number>()
 const [editing, setEditing] = useState<{ materialId: string; body: MaterialBody; source: PreparationReceipt }>()
 const [revised, setRevised] = useState<{ materialId: string; body: MaterialBody }>()
 const [reviseBusy, setReviseBusy] = useState(false)
 const [reviseMessage, setReviseMessage] = useState('')
 const [reviseConflict, setReviseConflict] = useState<number>()
 const writeInFlight = useRef(false)

 const clearPrivate = useCallback((notice: string, nextState: 'forbidden' | 'scope-changed' = 'forbidden') => {
  setItems(undefined); setReadState(nextState); setReadMessage(notice)
  setAttempt(undefined); setWritePhase('idle'); setMessage(''); setFocus('')
  setEditing(undefined); setRevised(undefined); setReviseMessage(''); setReviseConflict(undefined)
 }, [])
 useEffect(() => {
  const requestScope = scopeController.current()
  const clear = () => clearPrivate('空间已切换或登录已失效，已清除面试准备内容。', 'scope-changed')
  requestScope.signal?.addEventListener('abort', clear, { once: true })
  return () => requestScope.signal?.removeEventListener('abort', clear)
 }, [clearPrivate, scopeController, scope.scope.generation])

 useEffect(() => {
  let active = true
  const requestScope = scopeController.current()
  setItems(undefined); setReadState('loading'); setReadMessage('')
  if (!applicationId.trim()) { setReadState('error'); setReadMessage('缺少申请编号，无法读取面试准备。'); return () => { active = false } }
  const read = async (): Promise<void> => {
   try {
    const next = await client.career.applicationPreparations(applicationId, requestScope.signal)
    if (!active || !scopeController.isCurrent(requestScope.scope)) return
    setItems(next.preparations); setReadState('ready')
   } catch (cause) {
    if (!active || !scopeController.isCurrent(requestScope.scope)) return
    const parsed = errorDetails(cause)
    setItems(undefined)
    if (parsed.code === 'forbidden') { setReadState('forbidden'); setReadMessage('当前空间不可访问此申请的面试准备。'); return }
    setReadState('error')
    setReadMessage(parsed.code === 'not_found' ? '未找到此申请（可能不属于当前空间）。可刷新重试。' : '面试准备暂时无法读取，可刷新重试。')
   }
  }
  void read()
  return () => { active = false }
 }, [applicationId, client, reload, scopeController, scope.scope.generation])

 // The preparation CAS houses against the profile head revision, so the panel
 // reads the same revision the application flow reads.
 const readRevision = useCallback(async (): Promise<void> => {
  const requestScope = scopeController.current()
  setRevisionState('loading')
  try {
   const view = await client.career.open(requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   setRevision(view.revision); setRevisionState('ready')
  } catch {
   if (!scopeController.isCurrent(requestScope.scope)) return
   setRevision(undefined); setRevisionState('error')
  }
 }, [client, scopeController])
 useEffect(() => { void readRevision() }, [readRevision])

 const refresh = (): void => setReload((value) => value + 1)
 const acceptReceipt = (next: PreparationReceipt, expected: GenerateAttempt): void => {
  if (next.requestId !== expected.requestId || next.applicationId !== applicationId || next.focus !== expected.focus) throw new ReceiptMismatchError('准备回执与本次请求不匹配')
  if (next.status === 'draft') {
   setAttempt(undefined); setWritePhase('idle'); setFocus('')
   setMessage('准备草稿已生成：正文与来源链已呈现，可审阅并修订。系统不会自动发送任何内容。')
   refresh()
   return
  }
  // FindPreparationReceipt also answers with the durable row of a failed or
  // still-generating attempt: only status 'draft' is a success product. A
  // failed receipt is definite, a generating one is undecided — both keep the
  // same request number recoverable and never narrate success.
  if (next.status === 'failed') {
   setWritePhase('error')
   setMessage(`生成失败（${next.failureCode ?? '未提供代码'}）：${next.failureMessage ?? '生成未完成'}。已保留本次请求（原请求编号 ${next.requestId}），可用原请求编号重试；不会留下空白成功产物。`)
   refresh()
   return
  }
  setWritePhase('unknown')
  setMessage(`生成仍在进行（原请求编号 ${next.requestId}）。请稍后用原请求编号查询回执，或用同一编号重试；不会自动更换请求编号。`)
  refresh()
 }
 const runGenerate = async (fixed?: GenerateAttempt): Promise<void> => {
  if (writeInFlight.current) return
  let current = fixed
  if (!current) {
   if (writePhase === 'busy' || writePhase === 'unknown') return
   if (revision === undefined || !focus) return
   current = { requestId: newRequestId(), focus: focus as PreparationFocus, expectedRevision: revision }
  }
  writeInFlight.current = true
  const requestScope = scopeController.current()
  setAttempt(current); setWritePhase('busy'); setMessage('正在生成准备草稿…'); setRevisionConflict(undefined)
  try {
   const next = await client.career.generatePreparation({ requestId: current.requestId, applicationId, focus: current.focus, expectedRevision: current.expectedRevision }, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   acceptReceipt(next, current)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此申请的面试准备，已清除准备内容。'); return }
   if (parsed.code === 'preparation_version_unknown') {
    // The typed prompt state: the application has no confirmed submitted
    // version. The panel guides the user to record the actual submission (or
    // the explicit unknown marker) first — it never falls back to the latest
    // material version by itself.
    setAttempt(undefined); setWritePhase('error')
    setMessage('未确认实际投递版本：此申请还没有已确认的投递版本，系统不会自行改用最新材料版本。请先记录投递（在「投递确认」中绑定实际投递的版本，或显式选择未知口径），再生成面试准备。')
    return
   }
   if (parsed.code === 'revision_conflict') {
    setAttempt(undefined); setWritePhase('error'); setRevisionConflict(parsed.currentRevision)
    setMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订后再次生成；新生成会使用新的请求编号。`)
    return
   }
   if (parsed.code === 'preparation_generation_failed') {
    // A typed failure is definite (the backend persisted the failed row) but
    // keeps the request durable and recoverable: the same request ID may
    // retry, and no blank success product is shown.
    setWritePhase('error')
    setMessage(`生成失败（${parsed.message}）。已保留本次请求（原请求编号 ${current.requestId}）与锚定信息，可用原请求编号重试；不会留下空白成功产物。`)
    return
   }
   if (['invalid_request', 'idempotency_conflict', 'not_found', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
    setAttempt(undefined); setWritePhase('error')
    setMessage(parsed.code === 'idempotency_conflict' ? '请求编号已对应其他内容，服务器拒绝了本次生成。请检查后重新生成。' : parsed.code === 'not_found' ? '未找到此申请（可能不属于当前空间）。请刷新后重试。' : `准备生成未被接受：${parsed.message}`)
    return
   }
   if (cause instanceof ReceiptMismatchError) { setAttempt(undefined); setWritePhase('error'); setMessage(`准备生成未完成：${cause.message}`); return }
   if (!isUncertainWrite(cause)) { setAttempt(undefined); setWritePhase('error'); setMessage(`准备生成未完成：${parsed.message}`); return }
   setWritePhase('unknown')
   setMessage(`暂时无法确认准备是否已生成（原请求编号 ${current.requestId}）。请先用原请求编号查询回执，或用同一编号重试；不会自动更换请求编号。`)
  } finally { writeInFlight.current = false }
 }
 const lookupReceipt = async (): Promise<void> => {
  if (!attempt || writePhase === 'busy') return
  const current = attempt
  const requestScope = scopeController.current()
  setWritePhase('busy'); setMessage('正在查询原准备回执…')
  try {
   const next = await client.career.preparationReceipt(current.requestId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   acceptReceipt(next, current)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此申请的面试准备，已清除准备内容。'); return }
   if (parsed.code === 'invalid_request') { setAttempt(undefined); setWritePhase('error'); setMessage(`准备回执无法读取：${parsed.message}`); return }
   setWritePhase('unknown')
   setMessage(parsed.code === 'not_found' ? `尚未找到准备回执（原请求编号 ${current.requestId}）。可以继续查询，或使用原请求编号重试。` : '准备回执暂时无法读取。原请求编号已保留，可稍后重试查询。')
  }
 }

 // The revision seam is the frozen edit_material intent: a non-empty
 // materialId edits that material's draft in place (never its frozen
 // evidence), so revising keeps the same anchored draft. The preparation
 // receipt stays the snapshot of what generation produced; the saved
 // revision is read back from the material domain so the user sees exactly
 // what is stored.
 const startRevise = (receipt: PreparationReceipt): void => {
  if (!receipt.materialId) return
  setEditing({ materialId: receipt.materialId, body: cloneBody(receipt.body), source: receipt })
  setReviseMessage(''); setReviseConflict(undefined)
 }
 const saveRevision = async (): Promise<void> => {
  if (!editing || reviseBusy) return
  if (revision === undefined) { setReviseMessage('暂时无法读取当前档案修订，修订保存会被暂缓。请稍后重试。'); return }
  const current: ReviseAttempt = { requestId: newRequestId(), materialId: editing.materialId, body: editing.body, expectedRevision: revision }
  const requestScope = scopeController.current()
  setReviseBusy(true); setReviseMessage('正在保存修订…'); setReviseConflict(undefined)
  try {
   const next = await client.career.editMaterial({ requestId: current.requestId, materialId: current.materialId, body: current.body, expectedRevision: current.expectedRevision }, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   if (next.requestId !== current.requestId || next.materialId !== current.materialId) throw new ReceiptMismatchError('材料回执与本次修订不匹配')
   // 写入已成功且回执已验证：关闭编辑会话，随后只读回读（material 视图是
   // 持久化事实的展示来源）单独兜底——ocr1-075：回读失败不得落回下方共享
   // catch 的「修订未保存/结果暂时未知」分支，也不得引导用原请求编号恢复
   // （写入结果已确定，回读按 materialId 查询并不消费请求编号）。
   setReviseBusy(false); setEditing(undefined)
   try {
    const view = await client.career.material(current.materialId, requestScope.signal)
    if (!scopeController.isCurrent(requestScope.scope)) return
    if (view.materialId !== current.materialId) throw new ReceiptMismatchError('材料视图与本次修订不匹配')
    setRevised({ materialId: current.materialId, body: view.body })
    setReviseMessage('修订已保存：材料草稿已更新（仍未确认成版本，可继续修订或到材料区确认）。来源链与锚定投递版本保持不变。')
   } catch {
    if (!scopeController.isCurrent(requestScope.scope)) return
    setRevised({ materialId: current.materialId, body: next.body })
    setReviseMessage('修订已保存：材料草稿已更新（仍未确认成版本，可继续修订或到材料区确认）；内容回显暂时失败，可稍后重新打开。')
   }
   refresh()
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   setReviseBusy(false)
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此材料，已清除面试准备内容。'); return }
   if (parsed.code === 'revision_conflict') {
    setReviseConflict(parsed.currentRevision)
    setReviseMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订后再次保存；本次编辑内容已保留。`)
    return
   }
   if (parsed.code === 'material_claim_unconfirmed') { setReviseMessage(`修订未保存：正文引用了未确认事实（${parsed.message}）。请只保留已确认事实或使用待补充占位。`); return }
   if (['invalid_request', 'idempotency_conflict', 'not_found', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) { setReviseMessage(`修订未保存：${parsed.message}`); return }
   if (cause instanceof ReceiptMismatchError) { setReviseMessage(`修订未完成：${cause.message}`); return }
   setReviseMessage(`修订结果暂时未知（原请求编号 ${current.requestId}）。本次编辑内容已保留；可稍后在材料区用原请求编号查询回执，或重试保存。`)
  }
 }

 const composeBlocked = writePhase === 'busy' || writePhase === 'unknown'
 const generateBlocked = composeBlocked || revision === undefined || !focus
 return <section className="wk-preparation" aria-labelledby="wk-preparation-title">
  <h3 id="wk-preparation-title">面试准备与来源（基于实际投递）</h3>
  <p className="wk-preparation__notice">求职信与面试准备固定自你实际投递的版本与岗位快照，仅引用已确认事实；生成结果可审阅、修订，每条内容都能追溯到来源。系统不自动发送任何邮件或投递，也不代为承诺事实；发送与否、何时发送完全由你本人决定。</p>
  {readState === 'forbidden' || readState === 'scope-changed' ? <p className="wk-preparation__message wk-preparation__message--error" role="alert">{readMessage}</p> : <>
   {readState === 'loading' ? <p className="wk-preparation__state" role="status" aria-busy="true">正在读取面试准备…</p> : readState === 'error' ? <p className="wk-preparation__message wk-preparation__message--error" role="alert">{readMessage}</p> : <>
    <p className="wk-preparation__revision" role="status">{revisionState === 'loading' ? '正在读取当前档案修订…' : revisionState === 'error' ? '暂时无法读取当前档案修订，可稍后重试；准备生成会被暂缓。' : revision !== undefined ? `当前档案修订 ${revision}（准备生成将按此修订提交）` : ''}</p>
    <fieldset className="wk-preparation__compose" aria-label="生成准备表单">
     <legend>生成准备草稿</legend>
     <label className="wk-preparation__label" htmlFor="wk-preparation-focus">准备类型（固定自实际投递版本）</label>
     <select id="wk-preparation-focus" aria-label="准备焦点" value={focus} disabled={composeBlocked} onChange={(event) => setFocus(event.target.value)}>
      <option value="">请选择准备类型</option>
      {focusOptions.map((option) => <option key={option} value={option}>{focusLabels[option]}</option>)}
     </select>
     <p className="wk-preparation__hint">生成前请确认此申请已记录实际投递并绑定投递版本；未确认投递版本时系统会提示，不会改用最新材料版本。</p>
     <div className="wk-preparation__actions"><button type="button" className="wk-preparation__submit" disabled={generateBlocked} onClick={() => void runGenerate()}>生成准备草稿</button><button type="button" disabled={composeBlocked} onClick={refresh}>刷新准备列表</button></div>
    </fieldset>
    {attempt && (writePhase === 'unknown' || writePhase === 'error') ? <div className="wk-preparation__actions" role="group" aria-label="恢复准备写入">
     <button type="button" onClick={() => void lookupReceipt()}>查询准备回执</button>
     <button type="button" onClick={() => void runGenerate(attempt)}>用原请求编号重试</button>
    </div> : null}
    {message ? <p className={writePhase === 'error' ? 'wk-preparation__message wk-preparation__message--error' : 'wk-preparation__message'} role={writePhase === 'error' ? 'alert' : 'status'} aria-live="polite">{message}</p> : null}
    {writePhase === 'error' && revisionConflict !== undefined ? <div className="wk-preparation__actions"><button type="button" onClick={() => { setRevisionConflict(undefined); void readRevision() }}>重新读取档案修订</button></div> : null}
    {items && items.length ? <ol className="wk-preparation__list" aria-label="准备列表">{items.map((item) => <PreparationRow key={item.preparationId} item={item} revising={reviseBusy || writePhase === 'busy'} onRevise={startRevise} />)}</ol> : <p className="wk-preparation__hint">此申请还没有面试准备草稿。</p>}
    {editing ? <fieldset className="wk-preparation__edit" aria-label="修订准备草稿">     <legend>修订草稿 <code>{editing.materialId}</code>（保存为材料草稿，不改变已锚定的投递版本）</legend>
     {editing.body.sections.map((section, sectionIndex) => <div className="wk-preparation__edit-section" key={`${section.heading}-${sectionIndex}`}>
      <label className="wk-preparation__label">段落标题</label>
      <input aria-label="段落标题" value={section.heading} disabled={reviseBusy} onChange={(event) => setEditing((current) => {
       if (!current) return current
       const body = cloneBody(current.body)
       body.sections[sectionIndex]!.heading = event.target.value
       return { ...current, body }
      })} />
      <label className="wk-preparation__label">段落正文</label>
      <textarea aria-label="段落正文" value={section.content} disabled={reviseBusy} onChange={(event) => setEditing((current) => {
       if (!current) return current
       const body = cloneBody(current.body)
       body.sections[sectionIndex]!.content = event.target.value
       return { ...current, body }
      })} />
      {section.claims.length ? <div className="wk-preparation__edit-claims">
       <p className="wk-preparation__label">主张（引用已确认事实；缺失处保留待补充占位，不得补造）</p>
       {section.claims.map((claim, claimIndex) => <div className="wk-preparation__edit-claim" key={claim.claimId}>
        <textarea aria-label="主张文本" value={claim.text} disabled={reviseBusy} onChange={(event) => setEditing((current) => {
         if (!current) return current
         const body = cloneBody(current.body)
         body.sections[sectionIndex]!.claims[claimIndex]!.text = event.target.value
         return { ...current, body }
        })} />
        <p className="wk-preparation__claim-meta">{claim.factKey ? `已链接确认事实：${claim.factKey}（改动文本仍须符合该事实）` : claim.needsReview ? '待补充占位（不得写成事实）' : '自由文本（不引用事实键）'}</p>
       </div>)}
      </div> : null}
     </div>)}
     <div className="wk-preparation__actions">
      <button type="button" className="wk-preparation__submit" disabled={reviseBusy} onClick={() => void saveRevision()}>保存修订</button>
      <button type="button" disabled={reviseBusy} onClick={() => { setEditing(undefined); setReviseMessage(''); setReviseConflict(undefined) }}>放弃修订</button>
     </div>
    </fieldset> : null}
    {revised ? <section className="wk-preparation__revised" aria-label="修订后草稿">
     <h4>修订后草稿（已保存为材料草稿 <code>{revised.materialId}</code>）</h4>
     {revised.body.sections.map((section, index) => <div className="wk-preparation__section" key={`${section.heading}-${index}`}>
      <h5>{section.heading}</h5>
      <p>{section.content}</p>
      {section.claims.length ? <ul>{section.claims.map((claim) => <li key={claim.claimId} className={claim.needsReview ? 'wk-preparation__claim--needs-review' : undefined}>{claim.text}{claim.factKey ? `（已链接确认事实：${claim.factKey}）` : claim.needsReview ? '（缺失/待补充，不得补造）' : ''}</li>)}</ul> : null}
     </div>)}
     <p className="wk-preparation__hint">这是材料域当前保存的草稿正文；上方列表行保留生成时的回执快照与锚定投递版本，两者都以同一材料草稿为准继续演进。</p>
    </section> : null}
    {reviseConflict !== undefined ? <div className="wk-preparation__actions"><button type="button" onClick={() => { setReviseConflict(undefined); void readRevision() }}>重新读取档案修订</button></div> : null}
    {reviseMessage ? <p className={reviseMessage.startsWith('修订已保存') ? 'wk-preparation__message' : 'wk-preparation__message wk-preparation__message--error'} role={reviseMessage.startsWith('修订已保存') ? 'status' : 'alert'} aria-live="polite">{reviseMessage}</p> : null}
   </>}
  </>}
 </section>
}
