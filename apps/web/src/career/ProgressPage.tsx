import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ScopeController } from '@weknora/domain/scope'
import type { AppendProgressInput, CorrectProgressInput, ProgressEventView, ProgressEventType, ProgressReceipt, ProgressView } from '../../../../packages/api-client/src/career.ts'
import './progress.css'
import { SubmissionPage } from './SubmissionPage.tsx'
import { ReceiptMismatchError, errorDetails, isUncertainWrite as baseIsUncertainWrite, newRequestId } from './protocol.ts'
// ocr3-054/055：ReceiptMismatchError / errorDetails / newRequestId 统一改用
// protocol.ts 共享实现——本地副本与共享类同名但 instanceof 不互通；本页
// 端点特定的确定性失败码在基础契约之上叠加。
const endpointDefiniteCodes: readonly string[] = ['revision_conflict']
const isUncertainWrite = (cause: unknown): boolean => endpointDefiniteCodes.includes(errorDetails(cause).code ?? '') ? false : baseIsUncertainWrite(cause)


type WritePhase = 'idle' | 'busy' | 'unknown' | 'error'
type ReadState = 'loading' | 'ready' | 'error' | 'forbidden' | 'scope-changed'
type WriteAttempt = { requestId: string; input: AppendProgressInput | CorrectProgressInput }
// Frozen backend enums rendered verbatim (internal/modules/career/progress.go):
// the seven-stage projection and the event dictionary are closed vocabularies,
// the UI never invents a stage, an event type, or a provenance kind.
const stageLabels: Record<ProgressView['stage'], string> = { preparing: '准备中', pending_submission: '待投递', submitted: '已投递', assessment: '测评或笔试', interview: '面试', offer: 'Offer', closed: '已结束' }
const eventTypeLabels: Record<ProgressEventType, string> = { pending_submission: '待投递', submitted: '已投递', assessment: '测评或笔试', interview: '面试', offer: 'Offer', resubmitted: '重新投递', rejected: '未通过', withdrawn: '已撤回', retracted: '招聘方撤回' }
const eventTypeOptions = (Object.keys(eventTypeLabels) as ProgressEventType[]).filter((type) => type !== 'submitted' && type !== 'resubmitted')
const sourceKindLabels: Record<string, string> = { manual: '用户录入', user: '用户录入', system_import: '系统导入' }


function ProgressEventRow({ item, onCorrect }: { item: ProgressEventView; onCorrect: (eventId: string) => void }): ReactNode {
 return <li className={item.corrected ? 'wk-progress__event wk-progress__event--corrected' : item.kind === 'progress_corrected' ? 'wk-progress__event wk-progress__event--correction' : 'wk-progress__event'} aria-label={`进展事件 ${item.eventId}`}>
  <div className="wk-progress__event-head">
   <time dateTime={item.occurredAt}>{item.occurredAt}</time>
   <span className="wk-progress__seq">#{item.seq}</span>
   <span>来源：{sourceKindLabels[item.source.kind] ?? item.source.kind}{item.source.label ? `（${item.source.label}）` : ''}</span>
   <span>确认者：<code>{item.confirmer}</code></span>
  </div>
  <p className="wk-progress__event-body"><strong>{eventTypeLabels[item.eventType]}</strong>{item.note ? `：${item.note}` : ''}</p>
  {item.kind === 'progress_corrected' ? <p className="wk-progress__event-flag">更正事件（更正 <code>{item.correctsEventId}</code>）</p> : null}
  {item.corrected ? <p className="wk-progress__event-flag">已更正：原文保留，投影采用更正语义</p> : null}
  {item.kind === 'progress_appended' ? <button type="button" aria-label={`纠错事件 ${item.eventId}`} onClick={() => onCorrect(item.eventId)}>纠错</button> : null}
 </li>
}

// From the application detail (one fixed application) the user keeps one
// append-only progress timeline. Events are recorded with a request ID and the
// expected event revision; the current stage is the deterministic projection
// of the stored events, so filtering, refreshing, and reopening always agree.
// A correction appends a referencing event — the original stays visible — and
// an unknown write outcome is recovered through the receipt of the original
// request ID, never by silently minting a new one.
export function ProgressPage({ client, scopeController, applicationId, materialId }: { client: WeKnoraClient; scopeController: ScopeController; applicationId: string; materialId?: string }): ReactNode {
 const scope = scopeController.current()
 const [view, setView] = useState<ProgressView>()
 const [readState, setReadState] = useState<ReadState>('loading')
 const [readMessage, setReadMessage] = useState('')
 const [reload, setReload] = useState(0)
 const [eventType, setEventType] = useState('')
 const [note, setNote] = useState('')
 const [attempt, setAttempt] = useState<WriteAttempt>()
 const [writePhase, setWritePhase] = useState<WritePhase>('idle')
 const [message, setMessage] = useState('')
 const [correcting, setCorrecting] = useState<string>()
 const [submissionOpen, setSubmissionOpen] = useState(false)
 const [correctionType, setCorrectionType] = useState('')
 const [correctionNote, setCorrectionNote] = useState('')
 const writeInFlight = useRef(false)

 const clearPrivate = useCallback((notice: string, nextState: 'forbidden' | 'scope-changed' = 'forbidden') => {
  setView(undefined); setReadState(nextState); setReadMessage(notice)
  setAttempt(undefined); setWritePhase('idle'); setMessage('')
  setEventType(''); setNote(''); setCorrecting(undefined); setCorrectionType(''); setCorrectionNote('')
 }, [])
 useEffect(() => {
  const requestScope = scopeController.current()
  const clear = () => clearPrivate('空间已切换或登录已失效，已清除申请进展。', 'scope-changed')
  requestScope.signal?.addEventListener('abort', clear, { once: true })
  return () => requestScope.signal?.removeEventListener('abort', clear)
 }, [clearPrivate, scopeController, scope.scope.generation])

 useEffect(() => {
  let active = true
  const requestScope = scopeController.current()
  setView(undefined); setReadState('loading'); setReadMessage('')
  if (!applicationId.trim()) { setReadState('error'); setReadMessage('缺少申请编号，无法读取进展。'); return () => { active = false } }
  const read = async (): Promise<void> => {
   try {
    const next = await client.career.applicationProgress(applicationId, requestScope.signal)
    if (!active || !scopeController.isCurrent(requestScope.scope)) return
    if (next.applicationId !== applicationId) { setReadState('error'); setReadMessage('进展读取与当前申请不匹配。'); return }
    setView(next); setReadState('ready')
   } catch (cause) {
    if (!active || !scopeController.isCurrent(requestScope.scope)) return
    const parsed = errorDetails(cause)
    setView(undefined)
    if (parsed.code === 'forbidden') { setReadState('forbidden'); setReadMessage('当前空间不可访问此申请的进展。'); return }
    setReadState('error')
    setReadMessage(parsed.code === 'not_found' ? '未找到此申请（可能不属于当前空间）。可刷新重试。' : '申请进展暂时无法读取，可刷新重试。')
   }
  }
  void read()
  return () => { active = false }
 }, [applicationId, client, reload, scopeController, scope.scope.generation])

 const refresh = (): void => setReload((value) => value + 1)
 const resetCompose = (): void => { setEventType(''); setNote(''); setCorrecting(undefined); setCorrectionType(''); setCorrectionNote('') }
 const acceptReceipt = (next: ProgressReceipt, expected: WriteAttempt): void => {
  if (next.requestId !== expected.requestId || next.applicationId !== applicationId) throw new ReceiptMismatchError('进展回执与本次请求不匹配')
  setAttempt(undefined); setWritePhase('idle'); setMessage(''); resetCompose(); refresh()
 }
 const runWrite = async (fixed?: WriteAttempt): Promise<void> => {
  if (writeInFlight.current) return
  let current = fixed
  if (!current) {
   if (writePhase === 'busy' || writePhase === 'unknown') return
   const revision = view?.revision
   if (revision === undefined || !eventType) return
   const requestId = newRequestId()
   const input: AppendProgressInput = { requestId, applicationId, eventType: eventType as ProgressEventType, ...(note.trim() ? { note: note.trim() } : {}), expectedRevision: revision }
   current = { requestId, input }
  }
  writeInFlight.current = true
  const requestScope = scopeController.current()
  setAttempt(current); setWritePhase('busy'); setMessage('正在记录进展…')
  try {
   const next = 'correctsEventId' in current.input
    ? await client.career.correctProgress(current.input as CorrectProgressInput, requestScope.signal)
    : await client.career.appendProgress(current.input as AppendProgressInput, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   acceptReceipt(next, current)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此申请的进展，已清除进展内容。'); return }
   if (parsed.code === 'revision_conflict') {
    setAttempt(undefined); setWritePhase('error')
    setMessage(`进展已被其他记录更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请刷新进展后重新提交；新提交会使用新的请求编号。`)
    return
   }
   if (['invalid_request', 'idempotency_conflict', 'not_found', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
    setAttempt(undefined); setWritePhase('error')
    setMessage(parsed.code === 'idempotency_conflict' ? '请求编号已对应其他内容，服务器拒绝了本次提交。请刷新后重新提交。' : parsed.code === 'not_found' ? '目标事件或申请不存在（可能不属于当前空间）。请刷新进展后重试。' : `进展未被接受：${parsed.message}`)
    return
   }
   if (cause instanceof ReceiptMismatchError) { setAttempt(undefined); setWritePhase('error'); setMessage(`进展未完成：${cause.message}`); return }
   if (!isUncertainWrite(cause)) { setAttempt(undefined); setWritePhase('error'); setMessage(`进展未完成：${parsed.message}`); return }
   setWritePhase('unknown')
   setMessage(`暂时无法确认进展是否已保存（原请求编号 ${current.requestId}）。请先用原请求编号查询回执，或用同一编号重试；不会自动更换请求编号。`)
  } finally { writeInFlight.current = false }
 }
 const submitCorrection = async (): Promise<void> => {
  if (!correcting || writePhase === 'busy' || writePhase === 'unknown') return
  const revision = view?.revision
  if (revision === undefined || !correctionType) return
  const requestId = newRequestId()
  const input: CorrectProgressInput = { requestId, applicationId, correctsEventId: correcting, eventType: correctionType as ProgressEventType, ...(correctionNote.trim() ? { note: correctionNote.trim() } : {}), expectedRevision: revision }
  await runWrite({ requestId, input })
 }
 const lookupReceipt = async (): Promise<void> => {
  if (!attempt || writePhase === 'busy') return
  const current = attempt
  const requestScope = scopeController.current()
  setWritePhase('busy'); setMessage('正在查询原进展回执…')
  try {
   const next = await client.career.progressReceipt(current.requestId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   acceptReceipt(next, current)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此申请的进展，已清除进展内容。'); return }
   if (cause instanceof ReceiptMismatchError) { setAttempt(undefined); setWritePhase('error'); setMessage(`进展未完成：${cause.message}`); return }
   setWritePhase('unknown')
   setMessage(parsed.code === 'not_found' ? `尚未找到进展回执（原请求编号 ${current.requestId}）。可以继续查询，或使用原请求编号重试。` : '进展回执暂时无法读取。原请求编号已保留，可稍后重试查询。')
  }
 }

 const composeBlocked = writePhase === 'busy' || writePhase === 'unknown'
 return <section className="wk-progress" aria-labelledby="wk-progress-title">
  <h3 id="wk-progress-title">申请进展时间线</h3>
  <p>事件按发生顺序追加且不可覆盖；纠错会追加更正事件，原文与更正都保留。当前阶段由已确认事件确定性投影，刷新或重新打开结果一致。</p>
  {readState === 'forbidden' || readState === 'scope-changed' ? <p className="wk-progress__message wk-progress__message--error" role="alert">{readMessage}</p> : <>
   {readState === 'loading' ? <p className="wk-progress__state" role="status" aria-busy="true">正在读取申请进展…</p> : readState === 'error' ? <p className="wk-progress__message wk-progress__message--error" role="alert">{readMessage}</p> : view ? <>
    <p className="wk-progress__stage" aria-label="当前阶段投影">当前阶段：<strong>{stageLabels[view.stage]}</strong>（确定性投影 · 事件修订 {view.revision}）</p>
    <div className="wk-progress__actions"><button type="button" onClick={refresh}>刷新</button><button type="button" onClick={() => setSubmissionOpen((open) => !open)}>{submissionOpen ? '收起投递确认与回看' : '投递确认与回看'}</button></div>
    {submissionOpen ? <SubmissionPage client={client} scopeController={scopeController} applicationId={applicationId} materialId={materialId} /> : null}
    <fieldset className="wk-progress__compose"><legend>记录新的进展事件</legend>
     <label className="wk-progress__label" htmlFor="wk-progress-type">事件类型</label>
     <select id="wk-progress-type" aria-label="事件类型" value={eventType} disabled={composeBlocked} onChange={(event) => setEventType(event.target.value)}>
      <option value="">请选择事件类型</option>
      {eventTypeOptions.map((type) => <option key={type} value={type}>{eventTypeLabels[type]}</option>)}
     </select>
     <label className="wk-progress__label" htmlFor="wk-progress-note">进展备注（可选）</label>
     <textarea id="wk-progress-note" aria-label="进展备注" value={note} disabled={composeBlocked} onChange={(event) => setNote(event.target.value)} />
     <div className="wk-progress__actions"><button type="button" className="wk-progress__submit" disabled={composeBlocked || !eventType} onClick={() => void runWrite()}>记录进展</button></div>
    </fieldset>
    {writePhase === 'unknown' && attempt ? <div className="wk-progress__actions" role="group" aria-label="恢复进展写入">
     <button type="button" onClick={() => void lookupReceipt()}>查询进展回执</button>
     <button type="button" onClick={() => void runWrite(attempt)}>用原请求编号重试</button>
    </div> : null}
    {message && writePhase !== 'idle' ? <p className={writePhase === 'error' ? 'wk-progress__message wk-progress__message--error' : 'wk-progress__message'} role={writePhase === 'error' ? 'alert' : 'status'} aria-live="polite">{message}</p> : null}
    {correcting ? <fieldset className="wk-progress__correct"><legend>纠错事件 <code>{correcting}</code>（追加更正事件，原文保留）</legend>
     <label className="wk-progress__label" htmlFor="wk-progress-correct-type">更正事件类型</label>
     <select id="wk-progress-correct-type" aria-label="更正事件类型" value={correctionType} disabled={composeBlocked} onChange={(event) => setCorrectionType(event.target.value)}>
      <option value="">请选择更正后的事件类型</option>
      {eventTypeOptions.map((type) => <option key={type} value={type}>{eventTypeLabels[type]}</option>)}
     </select>
     <label className="wk-progress__label" htmlFor="wk-progress-correct-note">更正备注（可选）</label>
     <textarea id="wk-progress-correct-note" aria-label="更正备注" value={correctionNote} disabled={composeBlocked} onChange={(event) => setCorrectionNote(event.target.value)} />
     <div className="wk-progress__actions">
      <button type="button" disabled={composeBlocked || !correctionType} onClick={() => void submitCorrection()}>提交更正</button>
      <button type="button" disabled={composeBlocked} onClick={() => { setCorrecting(undefined); setCorrectionType(''); setCorrectionNote('') }}>取消</button>
     </div>
    </fieldset> : null}
    <ol className="wk-progress__events" aria-label="进展事件历史">
     {view.events.map((item) => <ProgressEventRow key={item.eventId} item={item} onCorrect={(eventId) => { setCorrecting(correcting === eventId ? undefined : eventId); setCorrectionType(''); setCorrectionNote('') }} />)}
    </ol>
   </> : null}
  </>}
 </section>
}
