import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ScopeController } from '@weknora/domain/scope'
import { CareerValidationError, type MaterialExportReceipt, type MaterialVersionView, type RecordSubmissionInput, type SubmissionChannel, type SubmissionReceipt } from '../../../../packages/api-client/src/career.ts'
import './submission.css'
import { ReceiptMismatchError, errorDetails, isUncertainWrite as baseIsUncertainWrite, newRequestId } from './protocol.ts'
// ocr3-054/055：ReceiptMismatchError / errorDetails / newRequestId 统一改用
// protocol.ts 共享实现——本地副本与共享类同名但 instanceof 不互通；本页
// 端点特定的确定性失败码在基础契约之上叠加。
const endpointDefiniteCodes: readonly string[] = ['revision_conflict', 'submission_already_confirmed', 'export_not_submittable']
const isUncertainWrite = (cause: unknown): boolean => endpointDefiniteCodes.includes(errorDetails(cause).code ?? '') ? false : baseIsUncertainWrite(cause)


type WritePhase = 'idle' | 'busy' | 'unknown' | 'error'
type ReadState = 'loading' | 'ready' | 'error' | 'forbidden' | 'scope-changed'
type WriteAttempt = { requestId: string; input: RecordSubmissionInput }
// Frozen backend enums rendered verbatim (internal/modules/career/submission.go):
// the submission channel is a closed vocabulary the UI never extends, and the
// version reference is either the frozen export binding or the explicit
// unknown marker — never an inferred one.
const channelLabels: Record<SubmissionChannel, string> = { email: '邮件', web: '招聘网站', other: '其他渠道' }
const channelOptions = Object.keys(channelLabels) as SubmissionChannel[]
const UNKNOWN_VERSION_CHOICE = '__unknown__'

// The claimed time arrives from a datetime-local input; a blank field leaves
// occurredAt unset so the server stamps the confirmation time, and anything
// unparsable falls back to the same explicit-server-time path.
function occurredAtFromInput(value: string): string | undefined {
 const trimmed = value.trim()
 if (!trimmed) return undefined
 const parsed = new Date(trimmed)
 return Number.isNaN(parsed.getTime()) ? undefined : parsed.toISOString()
}
// The UI gate mirrors the backend rule (and MaterialPage): only a
// both-verified submittable export is ever offered as the bound version.
const deliverableExports = (list: MaterialExportReceipt[]): MaterialExportReceipt[] => list.filter((receipt) => receipt.status === 'submittable' && receipt.submittable && receipt.files.length >= 2 && receipt.files.every((file) => file.verified))

function SubmissionRecordRow({ item, onReview, reviewing }: { item: SubmissionReceipt; onReview: (binding: NonNullable<SubmissionReceipt['boundVersion']>) => void; reviewing: boolean }): ReactNode {
 return <li className="wk-submission__record" aria-label={`投递记录 ${item.submissionId}`}>
  <div className="wk-submission__record-head">
   <time dateTime={item.occurredAt}>{item.occurredAt}</time>
   <span>渠道：{channelLabels[item.channel]}</span>
   <span>确认者：<code>{item.confirmer}</code></span>
   <span>请求编号 <code>{item.requestId}</code></span>
  </div>
  {item.boundVersion ? <p className="wk-submission__record-version">投递版本：版本 V{item.boundVersion.version}（材料 <code>{item.boundVersion.materialId}</code> · 导出 <code>{item.boundVersion.exportId}</code>）<button type="button" className="wk-submission__review" aria-label={`回看版本 V${item.boundVersion.version}`} disabled={reviewing} onClick={() => onReview(item.boundVersion!)}>回看版本 V{item.boundVersion.version}</button></p>
   : <p className="wk-submission__record-version wk-submission__record-version--unknown">投递版本：版本未确认（显式未知）——确认时未绑定材料版本，记录不指向任何版本。</p>}
  {item.note ? <p className="wk-submission__record-note">备注：{item.note}</p> : null}
 </li>
}

// From the application detail the user confirms what they did externally.
// The product never submits, mails, or fills anything itself; it records the
// claimed fact — the picked channel, the claimed time, and either the exact
// submittable export or the explicit unknown marker — and never infers a
// submission from a download. One application holds at most one record, so a
// repeat confirmation is surfaced as the typed conflict it is.
export function SubmissionPage({ client, scopeController, applicationId, materialId }: { client: WeKnoraClient; scopeController: ScopeController; applicationId: string; materialId?: string }): ReactNode {
 const scope = scopeController.current()
 const [records, setRecords] = useState<SubmissionReceipt[]>()
 const [readState, setReadState] = useState<ReadState>('loading')
 const [readMessage, setReadMessage] = useState('')
 const [reload, setReload] = useState(0)
 const [revision, setRevision] = useState<number | undefined>()
 const [revisionState, setRevisionState] = useState<'loading' | 'ready' | 'error'>('loading')
 const [exports, setExports] = useState<MaterialExportReceipt[]>()
 const [exportsError, setExportsError] = useState('')
 const [channel, setChannel] = useState<SubmissionChannel | ''>('')  // '' 占位由 runWrite 守卫排除，写入载荷收窄为冻结枚举（ocr2-078）
 const [versionChoice, setVersionChoice] = useState('')
 const [occurredAt, setOccurredAt] = useState('')
 const [note, setNote] = useState('')
 const [attempt, setAttempt] = useState<WriteAttempt>()
 const [writePhase, setWritePhase] = useState<WritePhase>('idle')
 const [message, setMessage] = useState('')
 const [revisionConflict, setRevisionConflict] = useState<number>()
 const [versionDetail, setVersionDetail] = useState<MaterialVersionView>()
 const [versionMessage, setVersionMessage] = useState('')
 const writeInFlight = useRef(false)
 const privateReadGeneration = useRef(0)

 const clearPrivate = useCallback((notice: string, nextState: 'forbidden' | 'scope-changed' = 'forbidden') => {
  privateReadGeneration.current += 1
  setRecords(undefined); setReadState(nextState); setReadMessage(notice)
  setRevision(undefined); setRevisionState('error'); setExports(undefined); setExportsError('')
  setAttempt(undefined); setWritePhase('idle'); setMessage('')
  setChannel(''); setVersionChoice(''); setOccurredAt(''); setNote(''); setVersionDetail(undefined); setVersionMessage('')
 }, [])
 useEffect(() => {
  const requestScope = scopeController.current()
  const clear = () => clearPrivate('空间已切换或登录已失效，已清除投递确认内容。', 'scope-changed')
  requestScope.signal?.addEventListener('abort', clear, { once: true })
  return () => requestScope.signal?.removeEventListener('abort', clear)
 }, [clearPrivate, scopeController, scope.scope.generation])

 useEffect(() => {
  let active = true
  const requestScope = scopeController.current()
  const readGeneration = privateReadGeneration.current
  setRecords(undefined); setReadState('loading'); setReadMessage('')
  if (!applicationId.trim()) { setReadState('error'); setReadMessage('缺少申请编号，无法读取投递记录。'); return () => { active = false } }
  const read = async (): Promise<void> => {
   try {
    const next = await client.career.applicationSubmissions(applicationId, requestScope.signal)
    if (!active || readGeneration !== privateReadGeneration.current || !scopeController.isCurrent(requestScope.scope)) return
    setRecords(next.submissions); setReadState('ready')
   } catch (cause) {
    if (!active || readGeneration !== privateReadGeneration.current || !scopeController.isCurrent(requestScope.scope)) return
    const parsed = errorDetails(cause)
    setRecords(undefined)
    if (parsed.code === 'forbidden') { setReadState('forbidden'); setReadMessage('当前空间不可访问此申请的投递确认。'); return }
    setReadState('error')
    setReadMessage(parsed.code === 'not_found' ? '未找到此申请（可能不属于当前空间）。可刷新重试。' : '投递记录暂时无法读取，可刷新重试。')
   }
  }
  void read()
  return () => { active = false }
 }, [applicationId, client, reload, scopeController, scope.scope.generation])

 // The submission CAS houses against the profile head revision, so the panel
 // reads the same revision the application flow reads.
 const readRevision = useCallback(async (): Promise<void> => {
  const requestScope = scopeController.current()
  const readGeneration = privateReadGeneration.current
  setRevisionState('loading')
  try {
   const view = await client.career.open(requestScope.signal)
   if (readGeneration !== privateReadGeneration.current || !scopeController.isCurrent(requestScope.scope)) return
   setRevision(view.revision); setRevisionState('ready')
  } catch {
   if (readGeneration !== privateReadGeneration.current || !scopeController.isCurrent(requestScope.scope)) return
   setRevision(undefined); setRevisionState('error')
  }
 }, [client, scopeController])
 useEffect(() => { void readRevision() }, [readRevision])

 useEffect(() => {
  let active = true
  const requestScope = scopeController.current()
  const readGeneration = privateReadGeneration.current
  setExports(undefined)
  setExportsError('')
  if (!materialId?.trim()) { if (active) setExports([]); return () => { active = false } }
  void client.career.materialExports(materialId, requestScope.signal).then((list) => {
   if (!active || readGeneration !== privateReadGeneration.current || !scopeController.isCurrent(requestScope.scope)) return
   const nextExports = deliverableExports(list.exports)
   setExports(nextExports)
   setVersionChoice((current) => current === UNKNOWN_VERSION_CHOICE || nextExports.some((receipt) => receipt.exportId === current) ? current : '')
   setExportsError('')
  }).catch((cause) => {
   if (!active || readGeneration !== privateReadGeneration.current || !scopeController.isCurrent(requestScope.scope)) return
   if (errorDetails(cause).code === 'forbidden') { clearPrivate('当前空间不可访问此材料的投递信息，已清除投递内容。'); return }
   setExports(undefined)
   setExportsError('可投递导出版本读取失败。读取恢复前，请勿选择“未知版本”并提交；可以重试读取。')
  })
  return () => { active = false }
 }, [clearPrivate, client, materialId, reload, scopeController, scope.scope.generation])

 const refresh = (): void => { privateReadGeneration.current += 1; setReload((value) => value + 1) }
 const resetCompose = (): void => { setChannel(''); setVersionChoice(''); setOccurredAt(''); setNote('') }
 const acceptReceipt = (next: SubmissionReceipt, expected: WriteAttempt): void => {
  if (next.requestId !== expected.requestId || next.applicationId !== applicationId) throw new ReceiptMismatchError('投递回执与本次请求不匹配')
  setAttempt(undefined); setWritePhase('idle'); setMessage('投递已记录。记录已呈现在投递时间线，可回看绑定的材料版本。'); resetCompose(); refresh()
 }
 const runWrite = async (fixed?: WriteAttempt): Promise<void> => {
  if (writeInFlight.current) return
  let current = fixed
  if (!current) {
   if (exports === undefined) return
   if (writePhase === 'busy' || writePhase === 'unknown') return
   if (revision === undefined || !channel || !versionChoice) return
   const selectedExport = exports.find((receipt) => receipt.exportId === versionChoice)
   if (versionChoice !== UNKNOWN_VERSION_CHOICE && !selectedExport) return
   const requestId = newRequestId()
   // OCR ocr2-078：直接构造 RecordSubmissionInput 类型化对象（编译期校验
   // 字段名），不再经 Record<string, unknown> + as 强转。
   const input: RecordSubmissionInput = versionChoice === UNKNOWN_VERSION_CHOICE
    ? { requestId, applicationId, channel, versionUnknown: true, expectedRevision: revision, ...(occurredAtFromInput(occurredAt) ? { occurredAt: occurredAtFromInput(occurredAt) } : {}), ...(note.trim() ? { note: note.trim() } : {}) }
    : { requestId, applicationId, channel, versionUnknown: false, materialId: selectedExport!.materialId, exportId: versionChoice, expectedRevision: revision, ...(occurredAtFromInput(occurredAt) ? { occurredAt: occurredAtFromInput(occurredAt) } : {}), ...(note.trim() ? { note: note.trim() } : {}) }
   current = { requestId, input }
  }
  writeInFlight.current = true
  const requestScope = scopeController.current()
  setAttempt(current); setWritePhase('busy'); setMessage('正在记录投递确认…'); setRevisionConflict(undefined)
  try {
   const next = await client.career.recordSubmission(current.input, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   acceptReceipt(next, current)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   // OCR ocr2-078：客户端同步校验拒绝时请求从未发出，写入确定未发生——
   // 直接置 error，不得判 unknown 走回执恢复。
   if (cause instanceof CareerValidationError) { setAttempt(undefined); setWritePhase('error'); setMessage(`投递确认未被接受：${cause.message}`); return }
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此申请的投递确认，已清除投递内容。'); return }
   if (parsed.code === 'submission_already_confirmed') {
    setAttempt(undefined); setWritePhase('error')
    setMessage('此申请已有投递记录：一个申请只有一条投递确认记录，重复确认不会产生第二条。')
    refresh()
    return
   }
   if (parsed.code === 'revision_conflict') {
    setAttempt(undefined); setWritePhase('error'); setRevisionConflict(parsed.currentRevision)
    setMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订后再次确认；新确认会使用新的请求编号。`)
    return
   }
   if (parsed.code === 'export_not_submittable') {
    setAttempt(undefined); setWritePhase('error')
    setMessage('所选导出已不可投递（可能已被撤销）。请刷新后重选版本，或选择显式未知版本。')
    refresh()
    return
   }
   if (['invalid_request', 'idempotency_conflict', 'not_found', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
    setAttempt(undefined); setWritePhase('error')
    setMessage(parsed.code === 'idempotency_conflict' ? '请求编号已对应其他内容，服务器拒绝了本次确认。请检查后重新确认。' : parsed.code === 'not_found' ? '申请或所选导出不存在（可能不属于当前空间）。请刷新后重试。' : `投递确认未被接受：${parsed.message}`)
    return
   }
   if (cause instanceof ReceiptMismatchError) { setAttempt(undefined); setWritePhase('error'); setMessage(`投递确认未完成：${cause.message}`); return }
   if (!isUncertainWrite(cause)) { setAttempt(undefined); setWritePhase('error'); setMessage(`投递确认未完成：${parsed.message}`); return }
   setWritePhase('unknown')
   setMessage(`暂时无法确认投递是否已记录（原请求编号 ${current.requestId}）。请先用原请求编号查询回执，或用同一编号重试；不会自动更换请求编号。`)
  } finally { writeInFlight.current = false }
 }
 const lookupReceipt = async (): Promise<void> => {
  if (!attempt || writePhase === 'busy') return
  const current = attempt
  const requestScope = scopeController.current()
  setWritePhase('busy'); setMessage('正在查询原投递回执…')
  try {
   const next = await client.career.submissionReceipt(current.requestId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   acceptReceipt(next, current)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   // OCR ocr2-079：查得的回执与原请求不匹配是确定性协议错误——置 error
   // 退出恢复流程，不得伪装瞬态置 unknown 困住用户。
   if (cause instanceof ReceiptMismatchError) { setAttempt(undefined); setWritePhase('error'); setMessage('查得的投递回执与原请求编号不匹配，已退出恢复流程。请用新的请求编号重新确认。'); return }
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此申请的投递确认，已清除投递内容。'); return }
   setWritePhase('unknown')
   setMessage(parsed.code === 'not_found' ? `尚未找到投递回执（原请求编号 ${current.requestId}）。可以继续查询，或使用原请求编号重试。` : '投递回执暂时无法读取。原请求编号已保留，可稍后重试查询。')
  }
 }
 const reviewVersion = useCallback(async (binding: NonNullable<SubmissionReceipt['boundVersion']>): Promise<void> => {
  const requestScope = scopeController.current()
  setVersionDetail(undefined); setVersionMessage('')
  try {
   const next = await client.career.materialVersion(binding.materialId, binding.version, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   if (next.version !== binding.version) throw new ReceiptMismatchError('版本回看与投递绑定的版本不匹配')
   setVersionDetail(next)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此材料，已清除投递确认内容。'); return }
   setVersionMessage(parsed.code === 'not_found' ? '绑定的材料版本不存在（可能不属于当前空间）。' : `版本回看暂时无法读取：${parsed.message}`)
  }
 }, [clearPrivate, client, scopeController])

 const composeBlocked = writePhase === 'busy' || writePhase === 'unknown'
 const confirmed = (records?.length ?? 0) > 0
 const selectedExportAvailable = versionChoice === UNKNOWN_VERSION_CHOICE || exports?.some((receipt) => receipt.exportId === versionChoice) === true
 const submitBlocked = composeBlocked || revision === undefined || exports === undefined || !channel || !versionChoice || !selectedExportAvailable
 return <section className="wk-submission" aria-labelledby="wk-submission-title">
  <h3 id="wk-submission-title">投递确认（本人确认）</h3>
  <p className="wk-submission__notice">投递由你本人在外部完成（发送邮件、在招聘网站提交等）。系统不代投、不发送邮件、不填写外部表单；点击下载或发布导出不会被视为投递，也不会被用来推断投递。请在完成外部投递后，由你本人在此确认结果。</p>
  {readState === 'forbidden' || readState === 'scope-changed' ? <p className="wk-submission__message wk-submission__message--error" role="alert">{readMessage}</p> : <>
   {readState === 'loading' ? <p className="wk-submission__state" role="status" aria-busy="true">正在读取投递记录…</p> : readState === 'error' ? <p className="wk-submission__message wk-submission__message--error" role="alert">{readMessage}</p> : <>
    <p className="wk-submission__revision" role="status">{revisionState === 'loading' ? '正在读取当前档案修订…' : revisionState === 'error' ? '暂时无法读取当前档案修订，可稍后重试；投递确认会被暂缓。' : revision !== undefined ? `当前档案修订 ${revision}（投递确认将按此修订提交）` : ''}</p>
    {confirmed ? <p className="wk-submission__confirmed" role="status">此申请已有投递记录：一个申请只有一条投递确认记录，再次确认将被拒绝并保留原记录。</p> : null}
    {records && records.length ? <ol className="wk-submission__records" aria-label="投递记录时间线">{records.map((item) => <SubmissionRecordRow key={item.submissionId} item={item} reviewing={writePhase === 'busy'} onReview={(binding) => { void reviewVersion(binding) }} />)}</ol> : <p className="wk-submission__hint">此申请还没有投递确认记录。</p>}
    <fieldset className="wk-submission__compose" aria-label="确认投递表单">
     <legend>记录投递确认</legend>
     <label className="wk-submission__label" htmlFor="wk-submission-channel">投递渠道（本人实际使用的渠道）</label>
     <select id="wk-submission-channel" aria-label="投递渠道" value={channel} disabled={composeBlocked} onChange={(event) => setChannel(event.target.value as SubmissionChannel | '')}>
      <option value="">请选择投递渠道</option>
      {channelOptions.map((option) => <option key={option} value={option}>{channelLabels[option]}</option>)}
     </select>
     <label className="wk-submission__label" htmlFor="wk-submission-version">投递版本（选择已验证可投递的材料版本，或显式未知）</label>
     <select id="wk-submission-version" aria-label="投递版本" value={versionChoice} disabled={composeBlocked} onChange={(event) => setVersionChoice(event.target.value)}>
      <option value="">请选择投递版本</option>
      {(exports ?? []).map((receipt) => <option key={receipt.exportId} value={receipt.exportId}>{`版本 V${receipt.version}（导出 ${receipt.exportId}）`}</option>)}
      <option value={UNKNOWN_VERSION_CHOICE} disabled={exports === undefined}>未知版本（显式未确认，不绑定材料版本）</option>
     </select>
     {exportsError ? <div className="wk-submission__message wk-submission__message--error" role="alert"><p>{exportsError}</p><button type="button" onClick={refresh}>重试读取导出版本</button></div> : exports === undefined ? <p className="wk-submission__hint" role="status" aria-busy="true">正在读取可投递版本…</p> : exports.length === 0 ? <p className="wk-submission__hint">尚无可投递版本：请先在材料区发布导出（双格式核验通过），或选择显式未知版本记录“未确认绑定版本”。</p> : null}
     <label className="wk-submission__label" htmlFor="wk-submission-occurred">声明投递时间（可选；留空按确认时刻记录）</label>
     <input id="wk-submission-occurred" type="datetime-local" aria-label="声明投递时间" value={occurredAt} disabled={composeBlocked} onChange={(event) => setOccurredAt(event.target.value)} />
     <label className="wk-submission__label" htmlFor="wk-submission-note">投递备注（可选）</label>
     <textarea id="wk-submission-note" aria-label="投递备注" value={note} disabled={composeBlocked} onChange={(event) => setNote(event.target.value)} />
     <div className="wk-submission__actions"><button type="button" className="wk-submission__submit" disabled={submitBlocked} onClick={() => void runWrite()}>确认投递</button><button type="button" disabled={composeBlocked} onClick={refresh}>刷新投递记录</button></div>
    </fieldset>
    {writePhase === 'unknown' && attempt ? <div className="wk-submission__actions" role="group" aria-label="恢复投递写入">
     <button type="button" onClick={() => void lookupReceipt()}>查询投递回执</button>
     <button type="button" onClick={() => void runWrite(attempt)}>用原请求编号重试</button>
    </div> : null}
    {message && writePhase !== 'idle' ? <p className={writePhase === 'error' ? 'wk-submission__message wk-submission__message--error' : 'wk-submission__message'} role={writePhase === 'error' ? 'alert' : 'status'} aria-live="polite">{message}</p> : null}
    {writePhase === 'error' && revisionConflict !== undefined ? <div className="wk-submission__actions"><button type="button" onClick={() => { setRevisionConflict(undefined); void readRevision() }}>重新读取档案修订</button></div> : null}
    {versionDetail ? <section className="wk-submission__version" aria-label="投递版本只读回看">
     <h4>版本 V{versionDetail.version}（只读）</h4>
     <p className="wk-submission__version-meta">材料 <code>{materialId ?? ''}</code> · 固定档案修订 {versionDetail.pinnedEvidence.profileRevision} · 事实基准修订 {versionDetail.factBasisRevision} · 确认请求 <code>{versionDetail.requestId}</code> · <time dateTime={versionDetail.createdAt}>{versionDetail.createdAt}</time></p>
     <div className="wk-submission__version-body">{versionDetail.body.sections.map((section, index) => <div className="wk-submission__version-section" key={`${section.heading}-${index}`}>
      <h5>{section.heading}</h5>
      <p>{section.content}</p>
      {section.claims.length ? <ul>{section.claims.map((claim) => <li key={claim.claimId} className={claim.needsReview ? 'wk-submission__claim--needs-review' : undefined}>{claim.text}{claim.factKey ? `（已链接确认事实：${claim.factKey}）` : '（缺失/待补充 needs_review，不得补造）'}</li>)}</ul> : null}
     </div>)}</div>
     <p className="wk-submission__hint">不可变版本不能修改；这是投递确认时绑定的版本，后续新版本不会改写此记录。</p>
    </section> : null}
    {versionMessage ? <p className="wk-submission__message wk-submission__message--error" role="alert">{versionMessage}</p> : null}
   </>}
  </>}
 </section>
}
