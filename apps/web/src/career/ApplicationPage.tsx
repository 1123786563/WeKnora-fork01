import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ScopeController } from '@weknora/domain/scope'
import type { EvaluationReceipt } from '../../../../packages/career-core/src/contracts.ts'
import type { ApplicationReceipt, CreateApplicationInput } from '../../../../packages/api-client/src/career.ts'
import { MaterialPage } from './MaterialPage.tsx'
import { PreparationPage } from './PreparationPage.tsx'
import { ProgressPage } from './ProgressPage.tsx'
import { SubmissionPage } from './SubmissionPage.tsx'
import { opportunityEvidencePath } from './OpportunityPage.tsx'
import './application.css'
import { ReceiptMismatchError, errorDetails, isUncertainWrite as baseIsUncertainWrite, newRequestId } from './protocol.ts'
// ocr3-054/055：ReceiptMismatchError / errorDetails / newRequestId 统一改用
// protocol.ts 共享实现——本地副本与共享类同名但 instanceof 不互通；本页
// 端点特定的确定性失败码在基础契约之上叠加。
const endpointDefiniteCodes: readonly string[] = ['revision_conflict', 'application_conflict', 'hard_ineligible_requires_continue']
const isUncertainWrite = (cause: unknown): boolean => endpointDefiniteCodes.includes(errorDetails(cause).code ?? '') ? false : baseIsUncertainWrite(cause)


type ApplicationPhase = 'idle' | 'busy' | 'created' | 'unknown' | 'error' | 'forbidden' | 'scope-changed'
type Attempt = { requestId: string; input: CreateApplicationInput }
// Frozen backend enums rendered verbatim: the application UI never invents a
// link state or evaluation status the career backend did not send.
const evaluationStatusLabel = (status: EvaluationReceipt['status']): string => status === 'ineligible' ? '不符合' : status === 'eligible' ? '符合已识别条件' : '待确认'
const linkStateLabels: Record<ApplicationReceipt['linkState'], string> = { linking: 'Task 关联中（未就绪，可恢复）', ready: 'Task 已就绪', link_failed: 'Task 关联失败' }
function applicationParamUrl(applicationId?: string): string | undefined {
 if (typeof window === 'undefined' || !window.location) return undefined
 const params = new URLSearchParams(window.location.search)
 if (applicationId) params.set('application', applicationId)
 else params.delete('application')
 const search = params.toString()
 return `${window.location.pathname}${search ? `?${search}` : ''}`
}

// From a job card (fixed opportunity snapshot) the user opens one independent
// application. The application pins the snapshot, the evaluation and the
// profile revision it was created against; the Workbench task link reconciles
// only with the original request ID and an unknown outcome never becomes
// "ready" in the UI.
export function ApplicationPage({ client, scopeController, opportunityId, snapshotId, evaluations, batchHint }: { client: WeKnoraClient; scopeController: ScopeController; opportunityId: string; snapshotId: string; evaluations: EvaluationReceipt[]; batchHint?: { state: 'known'; value: string } | { state: 'unknown' } }): ReactNode {
 const scope = scopeController.current()
 const [revision, setRevision] = useState<number | undefined>()
 const [revisionState, setRevisionState] = useState<'loading' | 'ready' | 'error'>('loading')
 const [selectedEvaluationId, setSelectedEvaluationId] = useState('')
 const [batchIdentity, setBatchIdentity] = useState('')
 const [acknowledged, setAcknowledged] = useState(false)
 const [attempt, setAttempt] = useState<Attempt>()
 const [receipt, setReceipt] = useState<ApplicationReceipt>()
 const [phase, setPhase] = useState<ApplicationPhase>('idle')
 const [message, setMessage] = useState('')
 const [revisionConflict, setRevisionConflict] = useState<number>()
 const [reconcileBusy, setReconcileBusy] = useState(false)
 // T17: the created application opens its own progress timeline on demand;
 // the entry stays closed by default so the creation flow is unchanged.
 const [progressOpen, setProgressOpen] = useState(false)
 // T18: the submission confirmation panel of this application; the material
 // editor reports its resolved material so the panel can list the submittable
 // exports of exactly that material.
 const [submissionOpen, setSubmissionOpen] = useState(false)
 // T19: the interview preparation panel of this application; it anchors to
 // the actually submitted version and stays closed by default so the
 // creation flow is unchanged.
 const [preparationOpen, setPreparationOpen] = useState(false)
 const [careerMaterialId, setCareerMaterialId] = useState<string>()
 const restored = useRef(false)
 // After a reload the page-level evaluation history is gone; a restored
 // receipt still pins the evaluation it was created against, so that stored
 // server state re-enters the selectable list for another batch.
 const [pinnedEvaluation, setPinnedEvaluation] = useState<EvaluationReceipt>()
 useEffect(() => {
  if (!receipt) return
  const pinned = receipt.pinnedEvidence
  if (pinned.opportunityId !== opportunityId || pinned.snapshotId !== snapshotId) return
  if (evaluations.some((evaluation) => evaluation.evaluationId === pinned.evaluationId) || pinnedEvaluation?.evaluationId === pinned.evaluationId) return
  setPinnedEvaluation({ kind: 'evaluation_created', requestId: receipt.requestId, evaluationId: pinned.evaluationId, opportunityId, snapshotId, profileRevision: pinned.profileRevision, status: pinned.evaluationStatus })
 }, [evaluations, opportunityId, pinnedEvaluation, receipt, snapshotId])
 const selectableEvaluations = evaluations.some((evaluation) => evaluation.evaluationId === pinnedEvaluation?.evaluationId) || !pinnedEvaluation ? evaluations : [...evaluations, pinnedEvaluation]
 const selected = selectableEvaluations.find((evaluation) => evaluation.evaluationId === selectedEvaluationId)
 const ineligible = selected?.status === 'ineligible'

 const clearPrivate = useCallback((notice: string, nextState: 'forbidden' | 'scope-changed' = 'forbidden') => {
  setAttempt(undefined); setReceipt(undefined); setPhase(nextState); setMessage(notice)
  setSelectedEvaluationId(''); setBatchIdentity(''); setAcknowledged(false); setRevisionConflict(undefined)
  const url = applicationParamUrl(undefined)
  if (url) window.history.replaceState({}, document.title, url)
 }, [])
 const readRevision = useCallback(async (): Promise<number | undefined> => {
  const requestScope = scopeController.current()
  setRevisionState('loading')
  try {
   const view = await client.career.open(requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return undefined
   setRevision(view.revision); setRevisionState('ready')
   return view.revision
  } catch {
   if (!scopeController.isCurrent(requestScope.scope)) return undefined
   setRevision(undefined); setRevisionState('error')
   return undefined
  }
 }, [client, scopeController])
 useEffect(() => {
  const requestScope = scopeController.current()
  const clear = () => clearPrivate('空间已切换或登录已失效，已清除申请内容。', 'scope-changed')
  requestScope.signal?.addEventListener('abort', clear, { once: true })
  return () => requestScope.signal?.removeEventListener('abort', clear)
 }, [clearPrivate, scopeController, scope.scope.generation])
 useEffect(() => { void readRevision() }, [readRevision])
 const prefilled = useRef(false)
 useEffect(() => {
  if (!prefilled.current && batchIdentity === '' && batchHint?.state === 'known') { prefilled.current = true; setBatchIdentity(batchHint.value) }
 }, [batchHint, batchIdentity])
 // A reload keeps the stored application readable straight from the URL: the
 // application ID is the durable pointer, creation is never replayed here.
 useEffect(() => {
  if (restored.current) return
  restored.current = true
  const applicationId = new URLSearchParams(window.location.search).get('application')?.trim()
  if (!applicationId) return
  const requestScope = scopeController.current()
  void client.career.application(applicationId, requestScope.signal).then((next) => {
   if (!scopeController.isCurrent(requestScope.scope)) return
   if (next.pinnedEvidence.opportunityId !== opportunityId || next.pinnedEvidence.snapshotId !== snapshotId) {
    const url = applicationParamUrl(undefined)
    if (url) window.history.replaceState({}, document.title, url)
    return
   }
   setReceipt(next); setPhase('created'); setMessage('')
  }).catch(() => {
   if (!scopeController.isCurrent(requestScope.scope)) return
   setPhase('error'); setMessage('申请暂时无法读取。可刷新重试；创建新申请不会复用此编号。')
  })
 }, [client, opportunityId, scopeController, snapshotId])

 const acceptReceipt = (next: ApplicationReceipt, expected: Attempt): void => {
  if (next.requestId !== expected.requestId
   || next.pinnedEvidence.opportunityId !== expected.input.opportunityId
   || next.pinnedEvidence.snapshotId !== expected.input.snapshotId
   || next.pinnedEvidence.evaluationId !== expected.input.evaluationId) throw new ReceiptMismatchError('申请回执与本次固定证据不匹配')
  setReceipt(next); setPhase('created'); setMessage('')
  const url = applicationParamUrl(next.applicationId)
  if (url) window.history.replaceState({}, document.title, url)
 }
 const submitInFlight = useRef(false)
 const submit = async (fixedAttempt?: Attempt): Promise<void> => {
  if (submitInFlight.current) return
  const currentAttempt: Attempt | undefined = fixedAttempt ?? ((): Attempt | undefined => {
   if (phase === 'busy' || phase === 'unknown' || phase === 'created') return undefined
   if (!selected || revision === undefined || !batchIdentity.trim()) return undefined
   if (ineligible && !acknowledged) return undefined
   const requestId = newRequestId()
   return { requestId, input: { requestId, opportunityId, snapshotId, evaluationId: selected.evaluationId, batchIdentity: batchIdentity.trim(), continueDespiteHardFailure: ineligible, expectedRevision: revision } }
  })()
  if (!currentAttempt) return
  submitInFlight.current = true
  const requestScope = scopeController.current()
  setAttempt(currentAttempt); setPhase('busy'); setMessage('正在创建申请…'); setRevisionConflict(undefined)
  try {
   const next = await client.career.createApplication(currentAttempt.input, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   acceptReceipt(next, currentAttempt)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此岗位的申请，已清除申请内容。'); return }
   if (parsed.code === 'revision_conflict') {
    setAttempt(undefined); setPhase('error'); setRevisionConflict(parsed.currentRevision)
    setMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订后再次提交；新提交会使用新的请求编号。`)
    return
   }
   if (parsed.code === 'application_conflict') {
    setAttempt(undefined); setPhase('error')
    setMessage('此岗位与该批次已存在申请：一个岗位和招聘批次只有一个申请与 Task。可为其他批次创建申请。')
    return
   }
   if (parsed.code === 'hard_ineligible_requires_continue') {
    setAttempt(undefined); setPhase('error')
    setMessage('硬性条件不符，需要先勾选显式继续才能提交申请。')
    return
   }
   if (['invalid_request', 'idempotency_conflict', 'not_found', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
    setAttempt(undefined); setPhase('error')
    setMessage(parsed.code === 'idempotency_conflict' ? '请求编号已对应其他内容，服务器拒绝了本次提交。请检查后重新提交。' : parsed.code === 'not_found' ? '所选评估或岗位快照不存在（可能不属于当前空间）。请重新评估后再申请。' : `申请未被接受：${parsed.message}`)
    return
   }
   if (cause instanceof ReceiptMismatchError) {
    setAttempt(undefined); setPhase('error')
    setMessage(`申请未完成：${cause.message}，已放弃本次结果。请重新提交。`)
    return
   }
   if (!isUncertainWrite(cause)) {
    setAttempt(undefined); setPhase('error')
    setMessage(`申请未完成：${parsed.message}`)
    return
   }
   setPhase('unknown')
   setMessage('暂时无法确认申请是否已创建。请先用原请求编号查询回执，或用同一编号重试；不会自动更换请求编号。')
  } finally { submitInFlight.current = false }
 }
 const lookupReceipt = async (): Promise<void> => {
  if (!attempt || phase === 'busy') return
  const currentAttempt = attempt
  const requestScope = scopeController.current()
  setPhase('busy'); setMessage('正在查询原申请回执…')
  try {
   const next = await client.career.applicationReceipt(currentAttempt.requestId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   acceptReceipt(next, currentAttempt)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此申请，已清除申请内容。'); return }
   if (cause instanceof ReceiptMismatchError) { setAttempt(undefined); setPhase('error'); setMessage(`申请回执查询未完成：${cause.message}，已放弃本次结果。请重新提交。`); return }
   setPhase('unknown')
   setMessage(parsed.code === 'not_found' ? '尚未找到申请回执。可以继续查询，或使用原请求编号重试同一份申请。' : '申请回执暂时无法读取。原请求编号已保留，可稍后重试查询。')
  }
 }
 const reconcile = async (): Promise<void> => {
  if (!receipt || reconcileBusy) return
  const requestId = receipt.requestId
  setReconcileBusy(true); setMessage('正在用原请求编号对账 Task 关联…')
  const requestScope = scopeController.current()
  try {
   const next = await client.career.reconcileApplicationLink(requestId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   if (next.requestId !== requestId) throw new ReceiptMismatchError('对账回执与原请求编号不匹配')
   setReceipt(next); setMessage(next.linkState === 'ready' ? '' : next.linkState === 'linking' ? 'Task 尚未建立：申请保留关联中状态，可稍后再对账。' : 'Task 关联仍为失败。申请与固定证据已保留，可再次对账。')
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此申请，已清除申请内容。'); return }
   setMessage('对账暂时无法完成。申请保留原状态，可稍后重试。')
  } finally { if (scopeController.isCurrent(requestScope.scope)) setReconcileBusy(false) }
 }
 const startAnotherBatch = (): void => {
  setAttempt(undefined); setReceipt(undefined); setPhase('idle'); setMessage(''); setRevisionConflict(undefined); setAcknowledged(false); setBatchIdentity('')
  const url = applicationParamUrl(undefined)
  if (url) window.history.replaceState({}, document.title, url)
 }
 const refreshRevision = async (): Promise<void> => { await readRevision() }
 const canSubmit = phase === 'idle' || phase === 'error'
  const submitBlocked = !canSubmit || !selected || revision === undefined || !batchIdentity.trim() || (ineligible && !acknowledged)
 const submitLabel = phase === 'busy' ? '正在创建申请…' : '创建申请'

 return <section className="wk-application" aria-labelledby="wk-application-title">
  <h2 id="wk-application-title">创建求职申请</h2>
  <p>一份申请固定所用岗位快照、档案修订与资格评估，并关联一个独立 Task。同一岗位与批次只有一个申请。</p>
  {phase === 'forbidden' || phase === 'scope-changed' ? <p className="wk-application__message wk-application__message--error" role="alert">{message}</p> : <>
   <p className="wk-application__revision" role="status">{revisionState === 'loading' ? '正在读取当前档案修订…' : revisionState === 'error' ? '暂时无法读取当前档案修订，可稍后重试；申请创建会被暂缓。' : revision !== undefined ? `当前档案修订 ${revision}（申请将按此修订固定）` : ''}</p>
   {selectableEvaluations.length === 0 ? <p className="wk-application__hint">尚无可用的资格评估：请先使用当前档案评估此岗位快照，再创建申请。</p> : <fieldset className="wk-application__evaluations"><legend>选择固定资格评估</legend>{selectableEvaluations.map((evaluation) => <label key={evaluation.evaluationId} className="wk-application__evaluation"><input type="radio" name={`wk-application-evaluation-${opportunityId}`} value={evaluation.evaluationId} checked={selectedEvaluationId === evaluation.evaluationId} onChange={() => { setSelectedEvaluationId(evaluation.evaluationId); setAcknowledged(false) }} /> {evaluation.evaluationId} · {evaluationStatusLabel(evaluation.status)} · 档案修订 {evaluation.profileRevision}</label>)}</fieldset>}
   {ineligible ? <div className="wk-application__hard-warning" role="alert"><strong>硬性条件不符（警示常驻）</strong><p>所选评估结论为不符合。默认阻断申请；勾选显式继续后可提交，但申请不计入合格申请指标。</p></div> : null}
   {ineligible ? <label className="wk-application__acknowledge"><input type="checkbox" aria-label="我已知晓硬性条件不符，仍要显式继续申请" checked={acknowledged} onChange={() => setAcknowledged(!acknowledged)} /> 我已知晓硬性条件不符，仍要显式继续申请</label> : null}
   <label className="wk-application__label" htmlFor="wk-application-batch">招聘批次标识（同一岗位不同批次可分别申请）</label>
   <input id="wk-application-batch" aria-label="招聘批次标识" value={batchIdentity} disabled={phase === 'busy' || phase === 'unknown' || phase === 'created'} onChange={(event) => setBatchIdentity(event.target.value)} placeholder="例如：2026 秋招 A 批" />
   {phase === 'unknown' && attempt ? <div className="wk-application__actions" role="group" aria-label="恢复申请创建"><button type="button" onClick={() => void lookupReceipt()}>查询申请回执</button><button type="button" onClick={() => void submit(attempt)}>用原请求编号重试</button></div> : phase === 'created' ? null : <div className="wk-application__actions"><button type="button" className="wk-application__submit" disabled={submitBlocked} onClick={() => void submit()}>{submitLabel}</button>{phase === 'error' && revisionConflict !== undefined ? <button type="button" onClick={() => void refreshRevision()}>重新读取档案修订</button> : null}</div>}
   {message && phase !== 'created' ? <p className={phase === 'error' ? 'wk-application__message wk-application__message--error' : 'wk-application__message'} role={phase === 'error' ? 'alert' : 'status'} aria-live="polite">{message}</p> : null}
   {receipt ? <div className={`wk-application__receipt wk-application__receipt--${receipt.linkState}`} role="status" aria-live="polite">
    <strong>申请已创建</strong>
    <p>申请编号 <code>{receipt.applicationId}</code> · 请求编号 <code>{receipt.requestId}</code></p>
    <p className={`wk-application__link wk-application__link--${receipt.linkState}`}>{linkStateLabels[receipt.linkState]}{receipt.linkState === 'ready' && receipt.taskId ? <>：Task <code>{receipt.taskId}</code>{receipt.runId ? <> · Run <code>{receipt.runId}</code></> : null}</> : null}</p>
    {receipt.linkState === 'linking' ? <div className="wk-application__actions"><button type="button" disabled={reconcileBusy} onClick={() => void reconcile()}>用原请求编号对账</button></div> : null}
    {receipt.linkState === 'link_failed' ? <div className="wk-application__actions"><button type="button" disabled={reconcileBusy} onClick={() => void reconcile()}>重试对账</button></div> : null}
    {receipt.linkState !== 'ready' && message && phase === 'created' ? <p className="wk-application__message">{message}</p> : null}
    <p className={receipt.qualified ? 'wk-application__qualified' : 'wk-application__qualified wk-application__qualified--warned'}>{receipt.qualified ? '合格申请' : '不合格申请（显式继续，不计合格申请指标）'}</p>
    {receipt.warning ? <div className="wk-application__hard-warning" role="alert"><strong>硬性条件警示（常驻）</strong><p>评估 <code>{receipt.warning.evaluationId}</code> 结论不符合{receipt.warning.reasonCode ? `（原因 ${receipt.warning.reasonCode}）` : ''}。此警示在申请存续期间保持可见。</p></div> : null}
    <dl className="wk-application__pinned" aria-label="本次申请固定的证据"><div><dt>岗位</dt><dd>{receipt.pinnedEvidence.opportunityId}</dd></div><div><dt>快照</dt><dd><a href={opportunityEvidencePath(receipt.pinnedEvidence.opportunityId, receipt.pinnedEvidence.snapshotId)}>{receipt.pinnedEvidence.snapshotId}</a>（固定不变，旧申请始终展示此旧快照）</dd></div><div><dt>评估</dt><dd>{receipt.pinnedEvidence.evaluationId}</dd></div><div><dt>档案修订</dt><dd>{receipt.pinnedEvidence.profileRevision}</dd></div><div><dt>评估结论</dt><dd>{evaluationStatusLabel(receipt.pinnedEvidence.evaluationStatus)}</dd></div><div><dt>批次</dt><dd>{receipt.pinnedEvidence.batchIdentity}</dd></div></dl>
    <div className="wk-application__actions"><button type="button" onClick={startAnotherBatch}>为其他批次创建新申请</button><button type="button" onClick={() => setProgressOpen(!progressOpen)}>{progressOpen ? '收起申请进展时间线' : '查看申请进展时间线'}</button><button type="button" onClick={() => setSubmissionOpen(!submissionOpen)}>{submissionOpen ? '收起投递确认与回看' : '投递确认与回看'}</button><button type="button" onClick={() => setPreparationOpen(!preparationOpen)}>{preparationOpen ? '收起面试准备与来源' : '面试准备与来源'}</button></div>
   </div> : null}
   {/* T15: the application context opens the material editor pinned to the
       same frozen opportunity snapshot and profile revision. */}
   {receipt ? <MaterialPage key={`material-${receipt.pinnedEvidence.opportunityId}-${receipt.pinnedEvidence.snapshotId}`} client={client} scopeController={scopeController} opportunityId={receipt.pinnedEvidence.opportunityId} snapshotId={receipt.pinnedEvidence.snapshotId} onMaterialId={setCareerMaterialId} /> : null}
   {/* T17: the timeline of this one application; keyed by application so
       another batch's application never inherits the previous events. */}
   {receipt && progressOpen ? <ProgressPage key={`progress-${receipt.applicationId}`} client={client} scopeController={scopeController} applicationId={receipt.applicationId} /> : null}
   {/* T18: the user-confirmed submission record of this application, with the
       bound material version reviewable; keyed by application so another
       batch's application never inherits the previous submission. */}
   {receipt && submissionOpen ? <SubmissionPage key={`submission-${receipt.applicationId}`} client={client} scopeController={scopeController} applicationId={receipt.applicationId} materialId={careerMaterialId} /> : null}
   {/* T19: the sourced interview preparation of this application; keyed by
       application so another batch's application never inherits the previous
       preparation state. */}
   {receipt && preparationOpen ? <PreparationPage key={`preparation-${receipt.applicationId}`} client={client} scopeController={scopeController} applicationId={receipt.applicationId} /> : null}
  </>}
 </section>
}
