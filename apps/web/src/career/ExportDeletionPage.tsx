import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ScopeController } from '@weknora/domain/scope'
import type { CareerDeletionBoundaryView, CareerDeletionReceipt, CareerDeletionStatus, CareerDeletionStep, CareerDeletionStepName, CareerDeletionStepStatus, CareerExportReceipt } from '../../../../packages/api-client/src/career.ts'
import './export-deletion.css'
import { ReceiptMismatchError, errorDetails, isUncertainWrite as baseIsUncertainWrite, newRequestId } from './protocol.ts'
// ocr3-054/055：ReceiptMismatchError / errorDetails / newRequestId 统一改用
// protocol.ts 共享实现——本地副本与共享类同名但 instanceof 不互通；本页
// 端点特定的确定性失败码在基础契约之上叠加。
const endpointDefiniteCodes: readonly string[] = ['revision_conflict']
const isUncertainWrite = (cause: unknown): boolean => endpointDefiniteCodes.includes(errorDetails(cause).code ?? '') ? false : baseIsUncertainWrite(cause)


type Phase = 'idle' | 'busy' | 'unknown' | 'error'
type Attempt = { requestId: string; expectedRevision: number }
// Frozen backend enums rendered verbatim (internal/modules/career/
// career_export.go): the deletion step vocabulary and its three durable
// statuses are closed sets the UI never extends, and “已完全删除” is reserved
// for the status the backend only sets after every step succeeded.
const stepNameLabels: Record<CareerDeletionStepName, string> = {
 revoke_material_exports: '撤销材料导出与下载授权',
 purge_career_data: '清除空间内求职数据',
 remove_workbench_tasks: '移除 Workbench 申请任务投影',
 finalize: '落定删除',
}
const stepStatusLabels: Record<CareerDeletionStepStatus, string> = { pending: '待执行', done: '已完成', failed: '失败' }
const deletionStatusLabels: Record<CareerDeletionStatus, string> = { deleting: '删除进行中', partial: '部分失败（未完全删除，可恢复）', deleted: '已完全删除' }


const snapshotCount = (view: CareerDeletionBoundaryView | undefined, section: string): number => view?.inSpace.find((item) => item.section === section)?.count ?? 0

function DeletionStepRow({ step }: { step: CareerDeletionStep }): ReactNode {
 return <li className={step.status === 'failed' ? 'wk-lifecycle__step wk-lifecycle__step--failed' : 'wk-lifecycle__step'}>
  <span>{stepNameLabels[step.name]}：{stepStatusLabels[step.status]}</span>
  {step.status === 'failed' && step.detail ? <span className="wk-lifecycle__step-detail">{step.detail}</span> : null}
 </li>
}

// T22 whole-space lifecycle surface: one complete, digest-verifiable export
// of the Career space (profile, original job snapshots, application events,
// material versions, submission records), and one complete deletion that
// first explains the in-space vs external-platform boundary, keeps partial
// failures recoverable under the same request ID, and discloses exactly
// which rows are retained. The page never claims complete deletion while
// any step failed, and after a real deletion it re-reads the pre-deletion
// export receipt to prove the old grant is dead before clearing caches.
export function ExportDeletionPage({ client, scopeController, onCareerDeleted }: { client: WeKnoraClient; scopeController: ScopeController; onCareerDeleted?: () => void }): ReactNode {
 const scope = scopeController.current()
 const [revision, setRevision] = useState<number | undefined>()
 const [revisionState, setRevisionState] = useState<'loading' | 'ready' | 'error'>('loading')
 const [exported, setExported] = useState<CareerExportReceipt>()
 const [exportAttempt, setExportAttempt] = useState<Attempt>()
 const [exportPhase, setExportPhase] = useState<Phase>('idle')
 const [exportMessage, setExportMessage] = useState('')
 const [exportConflict, setExportConflict] = useState<number>()
 const [boundary, setBoundary] = useState<CareerDeletionBoundaryView>()
 const [boundaryState, setBoundaryState] = useState<'idle' | 'loading' | 'ready' | 'error' | 'forbidden' | 'scope-changed'>('idle')
 const [boundaryMessage, setBoundaryMessage] = useState('')
 const [scopeRecoveryLoading, setScopeRecoveryLoading] = useState(false)
 const [acknowledged, setAcknowledged] = useState(false)
 const [deletion, setDeletion] = useState<CareerDeletionReceipt>()
 const [deletionAttempt, setDeletionAttempt] = useState<Attempt>()
 const [deletionPhase, setDeletionPhase] = useState<Phase>('idle')
 const [deletionMessage, setDeletionMessage] = useState('')
 const [deletionConflict, setDeletionConflict] = useState<number>()
 const [verifyMessage, setVerifyMessage] = useState('')
 const lastExportRequest = useRef<string | undefined>(undefined)
 const revisionReadSequence = useRef(0)
 const exportInFlight = useRef(false)
 const deletionInFlight = useRef(false)
 const deletedAnnounced = useRef(false)

 const clearPrivate = useCallback((notice: string, nextState: 'forbidden' | 'scope-changed' = 'forbidden') => {
  revisionReadSequence.current++
  setRevision(undefined); setRevisionState('loading'); setVerifyMessage('')
  setExported(undefined); setExportAttempt(undefined); setExportPhase('idle'); setExportMessage(''); setExportConflict(undefined)
  setBoundary(undefined); setBoundaryState(nextState); setBoundaryMessage(notice); setAcknowledged(false)
  setDeletion(undefined); setDeletionAttempt(undefined); setDeletionPhase('idle'); setDeletionMessage(''); setDeletionConflict(undefined)
  setScopeRecoveryLoading(false)
  lastExportRequest.current = undefined; deletedAnnounced.current = false
 }, [])
 useEffect(() => {
  const requestScope = scopeController.current()
  const clear = () => clearPrivate('空间已切换或登录已失效，已清除导出与删除内容。', 'scope-changed')
  requestScope.signal?.addEventListener('abort', clear, { once: true })
  return () => requestScope.signal?.removeEventListener('abort', clear)
 }, [clearPrivate, scopeController, scope.scope.generation])

 const readRevision = useCallback(async (): Promise<void> => {
  const operation = ++revisionReadSequence.current
  const requestScope = scopeController.current()
  setRevisionState('loading')
  try {
   const view = await client.career.open(requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope) || operation !== revisionReadSequence.current) return
   setRevision(view.revision); setRevisionState('ready')
  } catch {
   if (!scopeController.isCurrent(requestScope.scope) || operation !== revisionReadSequence.current) return
   setRevision(undefined); setRevisionState('error')
  }
 }, [client, scopeController, scope.scope.generation])
 useEffect(() => { void readRevision() }, [readRevision])

 // After a truthful complete deletion the old export grant must be dead:
 // the page replays the pre-deletion export receipt (career_data_exports is
 // purged) and reports the unreadable outcome. A readable receipt is a
 // verification failure, never a silent pass.
 const verifyOldGrants = useCallback(async (exportRequestId: string | undefined): Promise<void> => {
  const requestScope = scopeController.current()
  if (!exportRequestId) { setVerifyMessage('本次会话未发起过导出：删除后验证需要先导出一次，才能核对旧授权已失效。'); return }
  try {
   await client.career.careerExportReceipt(exportRequestId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   setVerifyMessage(`删除后验证异常：导出回执 ${exportRequestId} 仍可读取，旧授权可能未失效。请刷新核对删除结果。`)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问导出与删除。'); return }
   setVerifyMessage(parsed.code === 'not_found' ? `删除后验证：导出回执 ${exportRequestId} 已不可读取（旧授权与旧入口已失效）。` : `删除后验证暂时无法完成：${parsed.message}`)
  }
 }, [clearPrivate, client, scopeController])

 const acceptExport = (next: CareerExportReceipt, expected: Attempt): void => {
  if (next.requestId !== expected.requestId) throw new ReceiptMismatchError('导出回执与本次请求不匹配')
  lastExportRequest.current = next.requestId
  setExportAttempt(undefined); setExportPhase('idle')
  setExported(next); setExportMessage('导出包已生成：内容与摘要如下，可下载留存。')
 }

 const runExport = async (fixed?: Attempt): Promise<void> => {
  if (exportInFlight.current) return
  let current = fixed
  if (!current) {
   if (exportPhase === 'busy' || exportPhase === 'unknown') return
   if (revision === undefined) return
   current = { requestId: newRequestId(), expectedRevision: revision }
  }
  exportInFlight.current = true
  const requestScope = scopeController.current()
  setExportAttempt(current); setExportPhase('busy'); setExportMessage('正在准备导出包（只读，不改变档案）…'); setExportConflict(undefined)
  try {
   const next = await client.career.exportCareer(current, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   acceptExport(next, current)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问导出与删除。'); return }
   if (parsed.code === 'revision_conflict') {
    setExportAttempt(undefined); setExportPhase('error'); setExportConflict(parsed.currentRevision)
    setExportMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订后再发起导出。`)
    return
   }
   if (['invalid_request', 'idempotency_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
    setExportAttempt(undefined); setExportPhase('error')
    setExportMessage(parsed.code === 'idempotency_conflict' ? '请求编号已对应其他内容，服务器拒绝了本次导出。请重新发起导出（将使用新请求编号）。' : `导出未被接受：${parsed.message}`)
    return
   }
   if (cause instanceof ReceiptMismatchError) { setExportAttempt(undefined); setExportPhase('error'); setExportMessage(`导出未完成：${cause.message}`); return }
   if (!isUncertainWrite(cause)) { setExportAttempt(undefined); setExportPhase('error'); setExportMessage(`导出未完成：${parsed.message}`); return }
   setExportPhase('unknown')
   setExportMessage(`暂时无法确认导出是否完成（原请求编号 ${current.requestId}）。请先用原请求编号查询回执，或用同一编号重试；不会自动更换请求编号。`)
  } finally { exportInFlight.current = false }
 }

 const lookupExportReceipt = async (): Promise<void> => {
  if (!exportAttempt || exportPhase === 'busy') return
  const current = exportAttempt
  const requestScope = scopeController.current()
  setExportPhase('busy'); setExportMessage('正在查询原导出回执…')
  try {
   const next = await client.career.careerExportReceipt(current.requestId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   acceptExport(next, current)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问导出与删除。'); return }
   if (cause instanceof ReceiptMismatchError) { setExportAttempt(undefined); setExportPhase('error'); setExportMessage(`导出未完成：${cause.message}`); return }
   setExportPhase('unknown')
   setExportMessage(parsed.code === 'not_found' ? `尚未找到导出回执（原请求编号 ${current.requestId}）。可以继续查询，或使用原请求编号重试。` : '导出回执暂时无法读取。原请求编号已保留，可稍后重试查询。')
  }
 }

 const downloadExport = (): void => {
  if (!exported) return
  const body = JSON.stringify(exported, null, 2)
  if (typeof URL.createObjectURL !== 'function') { setExportMessage('当前环境不支持文件下载。导出包内容已在上方完整呈现（含摘要），可复制留存。'); return }
  const blob = new Blob([body], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url; anchor.download = `career-export-${exported.exportId}.json`
  anchor.click()
  URL.revokeObjectURL(url)
  setExportMessage(`导出包已下载（career-export-${exported.exportId}.json）。摘要 ${exported.digest}。`)
 }

 const loadBoundary = async (): Promise<void> => {
  if (boundaryState === 'loading') return
  const requestScope = scopeController.current()
  setBoundaryState('loading'); setBoundaryMessage('正在读取删除边界清单…')
  try {
   const next = await client.career.careerDeletionBoundary(requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   setBoundary(next); setBoundaryState('ready'); setBoundaryMessage('')
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问导出与删除。'); return }
   setBoundary(undefined); setBoundaryState('error')
   setBoundaryMessage('删除边界清单暂时无法读取，可重试。未呈现边界前不能发起删除。')
  }
 }

 const recoverCurrentScope = async (): Promise<void> => {
  if (scopeRecoveryLoading) return
  const operation = ++revisionReadSequence.current
  const requestScope = scopeController.current()
  setScopeRecoveryLoading(true)
  setBoundaryMessage('正在重新读取当前空间的档案修订与删除边界…')
  try {
   const [view, nextBoundary] = await Promise.all([
    client.career.open(requestScope.signal),
    client.career.careerDeletionBoundary(requestScope.signal),
   ])
   if (!scopeController.isCurrent(requestScope.scope) || operation !== revisionReadSequence.current) return
   setRevision(view.revision); setRevisionState('ready')
   setBoundary(nextBoundary); setBoundaryState('ready'); setBoundaryMessage('')
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope) || operation !== revisionReadSequence.current) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问导出与删除。'); return }
   setRevision(undefined); setRevisionState('error')
   setBoundary(undefined); setBoundaryState('scope-changed')
   setBoundaryMessage('当前空间信息暂时无法完整读取。导出与删除仍不可用，可重试重新加载。')
  } finally {
   if (scopeController.isCurrent(requestScope.scope) && operation === revisionReadSequence.current) setScopeRecoveryLoading(false)
  }
 }

 const acceptDeletion = (next: CareerDeletionReceipt, expected: Attempt): void => {
  if (next.requestId !== expected.requestId) throw new ReceiptMismatchError('删除回执与本次请求不匹配')
  setDeletion(next)
  setAcknowledged(false)
  if (next.status === 'deleted') {
   setDeletionAttempt(undefined); setDeletionPhase('idle')
   setDeletionMessage(`空间已完全删除（完成于 ${next.completedAt ?? ''}）。保留范围已在下方披露。`)
   if (!deletedAnnounced.current) {
    deletedAnnounced.current = true
    onCareerDeleted?.()
    void verifyOldGrants(lastExportRequest.current)
   }
   return
  }
  // Partial keeps the attempt recoverable under the same request ID.
  setDeletionPhase('error')
  setDeletionMessage(next.status === 'partial' ? '删除部分失败：未完全删除。失败步骤已列出，状态与审计已保留，可用原请求编号重试恢复。' : '删除仍在进行中。可查询删除回执查看最新状态。')
 }

 const runDeletion = async (fixed?: Attempt): Promise<void> => {
  if (deletionInFlight.current) return
  let current = fixed
  if (!current) {
   if (deletionPhase === 'busy' || deletionPhase === 'unknown') return
   if (revision === undefined || !boundary || !acknowledged) return
   if (deletion?.status === 'deleted') return
   current = { requestId: newRequestId(), expectedRevision: revision }
  }
  deletionInFlight.current = true
  const requestScope = scopeController.current()
  setDeletionAttempt(current); setDeletionPhase('busy'); setDeletionMessage('正在执行完整删除…'); setDeletionConflict(undefined)
  try {
   const next = await client.career.deleteCareer(current, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   acceptDeletion(next, current)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问导出与删除。'); return }
   if (parsed.code === 'revision_conflict') {
    setDeletionAttempt(undefined); setDeletionPhase('error'); setDeletionConflict(parsed.currentRevision)
    setDeletionMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订，确认边界后再次发起；新删除会使用新的请求编号。`)
    return
   }
   if (['invalid_request', 'idempotency_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
    setDeletionAttempt(undefined); setDeletionPhase('error')
    setDeletionMessage(parsed.code === 'idempotency_conflict' ? '请求编号已对应其他内容，服务器拒绝了本次删除。请重新发起删除（将使用新请求编号）。' : `删除未被接受：${parsed.message}`)
    return
   }
   if (cause instanceof ReceiptMismatchError) {
    // Once the server has confirmed deletion is in progress, retain the trusted
    // request ID in a quarantined state. Only re-querying that ID can resolve it;
    // a receipt carrying a different ID is never accepted and cannot unlock a new delete.
    if (deletion?.status !== 'deleting') setDeletionAttempt(undefined)
    setDeletionPhase('error'); setDeletionMessage(`删除未完成：${cause.message}`); return
   }
   if (!isUncertainWrite(cause)) { setDeletionAttempt(undefined); setDeletionPhase('error'); setDeletionMessage(`删除未完成：${parsed.message}`); return }
   setDeletionPhase('unknown')
   setDeletionMessage(`暂时无法确认删除是否完成（原请求编号 ${current.requestId}）。请先用原请求编号查询回执，或用同一编号重试；不会自动更换请求编号。`)
  } finally { deletionInFlight.current = false }
 }

 const lookupDeletionReceipt = async (): Promise<void> => {
  if (!deletionAttempt || deletionPhase === 'busy') return
  const current = deletionAttempt
  const requestScope = scopeController.current()
  setDeletionPhase('busy'); setDeletionMessage('正在查询原删除回执…')
  try {
   const next = await client.career.careerDeletionReceipt(current.requestId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   acceptDeletion(next, current)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问导出与删除。'); return }
   if (cause instanceof ReceiptMismatchError) {
    if (deletion?.status !== 'deleting') setDeletionAttempt(undefined)
    setDeletionPhase('error'); setDeletionMessage(`删除未完成：${cause.message}`); return
   }
   setDeletionPhase('unknown')
   setDeletionMessage(parsed.code === 'not_found' ? `尚未找到删除回执（原请求编号 ${current.requestId}）。可以继续查询，或使用原请求编号重试。` : '删除回执暂时无法读取。原请求编号已保留，可稍后重试查询。')
  }
 }

 const exportBlocked = exportPhase === 'busy' || exportPhase === 'unknown'
 const deletionBlocked = deletionPhase === 'busy' || deletionPhase === 'unknown' || exportBlocked
 const startDeletionDisabled = deletionBlocked || revision === undefined || !boundary || !acknowledged || deletion?.status === 'deleting' || deletion?.status === 'deleted'
 const archive = exported?.archive
 const snapshotTotal = archive?.opportunities.reduce((total, item) => total + item.snapshots.length, 0) ?? 0
 const eventTotal = archive?.applications.reduce((total, item) => total + item.progressEvents.length, 0) ?? 0
 const versionTotal = archive?.materials.reduce((total, item) => total + item.versions.length, 0) ?? 0
 return <section className="wk-lifecycle" aria-labelledby="wk-lifecycle-title">
  <h3 id="wk-lifecycle-title">导出与完整删除</h3>
  <p className="wk-lifecycle__notice">导出会生成一份完整的不可变导出包（档案、原始岗位快照、申请事件、材料版本、投递记录），可下载留存。完整删除不可恢复：空间内的求职数据会被删除，外部平台的投递与已发出的副本不受本系统控制、无法撤回；删除前会先呈现边界清单。部分失败时保留可恢复状态与审计，绝不声称已完全删除。</p>
  {boundaryState === 'forbidden' || boundaryState === 'scope-changed' ? <>
   <p className="wk-lifecycle__message wk-lifecycle__message--error" role="alert">{boundaryMessage}</p>
   {boundaryState === 'scope-changed' ? <div className="wk-lifecycle__actions"><button type="button" disabled={scopeRecoveryLoading} onClick={() => void recoverCurrentScope()}>{scopeRecoveryLoading ? '正在重新加载…' : '重新加载当前空间'}</button></div> : null}
  </> : <>
   <p className="wk-lifecycle__revision" role="status">{revisionState === 'loading' ? '正在读取当前档案修订…' : revisionState === 'error' ? '暂时无法读取当前档案修订，可稍后重试；导出与删除会被暂缓。' : revision !== undefined ? `当前档案修订 ${revision}（导出与删除将按此修订提交）` : ''}</p>
   <fieldset className="wk-lifecycle__panel" aria-label="导出数据">
    <legend>导出数据</legend>
    <div className="wk-lifecycle__actions"><button type="button" className="wk-lifecycle__export" disabled={exportBlocked || revision === undefined} onClick={() => void runExport()}>发起导出</button></div>
    {exportPhase === 'unknown' && exportAttempt ? <div className="wk-lifecycle__actions" role="group" aria-label="恢复导出写入">
     <button type="button" onClick={() => void lookupExportReceipt()}>查询导出回执</button>
     <button type="button" onClick={() => void runExport(exportAttempt)}>用原请求编号重试</button>
    </div> : null}
    {exportMessage && exportPhase !== 'idle' ? <p className={exportPhase === 'error' ? 'wk-lifecycle__message wk-lifecycle__message--error' : 'wk-lifecycle__message'} role={exportPhase === 'error' ? 'alert' : 'status'} aria-live="polite">{exportMessage}</p> : null}
    {exportPhase === 'error' && exportConflict !== undefined ? <div className="wk-lifecycle__actions"><button type="button" onClick={() => { setExportConflict(undefined); void readRevision() }}>重新读取档案修订</button></div> : null}
    {archive && exported ? <section className="wk-lifecycle__package" aria-label="导出包内容">
     <h4>导出包（导出编号 <code>{exported.exportId}</code>）</h4>
     <p className="wk-lifecycle__package-meta">摘要 <code>{exported.digest}</code> · 固定档案修订 {exported.revision} · 生成于 <time dateTime={exported.createdAt}>{exported.createdAt}</time> · 状态：完成</p>
     <ul className="wk-lifecycle__inventory" aria-label="导出包包含性清单">
      <li>档案事实：{archive.profile.facts.length} 条（待处理提案 {archive.profile.proposals.length} 条）</li>
      <li>事实历史：{archive.factHistory.length} 条</li>
      <li>岗位与原始快照：{archive.opportunities.length} 个岗位 / {snapshotTotal} 份快照</li>
      <li>申请与进展事件：{archive.applications.length} 个申请 / {eventTotal} 条事件</li>
      <li>材料与版本：{archive.materials.length} 份材料 / {versionTotal} 个版本</li>
      <li>投递记录：{archive.submissions.length} 条</li>
     </ul>
     <div className="wk-lifecycle__actions"><button type="button" className="wk-lifecycle__download" onClick={downloadExport}>下载导出包</button></div>
    </section> : null}
   </fieldset>
   <fieldset className="wk-lifecycle__panel" aria-label="完整删除">
    <legend>完整删除</legend>
    <div className="wk-lifecycle__actions"><button type="button" disabled={boundaryState === 'loading'} onClick={() => void loadBoundary()}>{boundary ? '刷新删除边界' : '查看删除边界'}</button></div>
    {boundaryMessage && boundaryState === 'loading' ? <p className="wk-lifecycle__message" role="status" aria-live="polite">{boundaryMessage}</p> : null}
    {boundaryState === 'error' ? <p className="wk-lifecycle__message wk-lifecycle__message--error" role="alert">{boundaryMessage}</p> : null}
    {boundary ? <section className="wk-lifecycle__boundary" aria-label="删除边界清单">
     <h4>空间内将删除的数据</h4>
     <ul>{boundary.inSpace.map((item) => <li key={item.section}>{item.section}（{item.description}）：{item.count} 项</li>)}</ul>
     <h4>外部平台资料（不可撤回）</h4>
     <ul>{boundary.external.map((item) => <li key={item.item}>{item.item}：{item.description}{item.revocable ? '' : '（不可撤回）'}</li>)}</ul>
     <h4>保留范围与状态</h4>
     <ul>{boundary.retention.map((item) => <li key={item.holder}>{item.holder}：{item.reason}（{item.status}）</li>)}</ul>
     <label className="wk-lifecycle__ack"><input type="checkbox" checked={acknowledged} disabled={deletionBlocked} onChange={() => setAcknowledged((value) => !value)} />我已知悉外部平台资料不可撤回、保留范围如上，并理解完整删除不可恢复。</label>
    </section> : null}
    <div className="wk-lifecycle__actions"><button type="button" className="wk-lifecycle__delete" disabled={startDeletionDisabled} onClick={() => void runDeletion()}>发起完整删除</button></div>
    {(deletionPhase === 'unknown' || (deletion?.status === 'deleting' && deletionAttempt !== undefined)) && deletionAttempt ? <div className="wk-lifecycle__actions" role="group" aria-label="恢复删除写入">
     <button type="button" onClick={() => void lookupDeletionReceipt()}>查询删除回执</button>
     {deletionPhase === 'unknown' ? <button type="button" onClick={() => void runDeletion(deletionAttempt)}>用原请求编号重试</button> : null}
    </div> : null}
    {deletion?.status === 'partial' && deletionAttempt && deletionPhase !== 'busy' ? <div className="wk-lifecycle__actions"><button type="button" onClick={() => void runDeletion(deletionAttempt)}>用原请求编号重试删除</button></div> : null}
    {deletionMessage && deletionPhase !== 'idle' ? <p className={deletionPhase === 'error' ? 'wk-lifecycle__message wk-lifecycle__message--error' : 'wk-lifecycle__message'} role={deletionPhase === 'error' ? 'alert' : 'status'} aria-live="polite">{deletionMessage}</p> : null}
    {deletionPhase === 'error' && deletionConflict !== undefined ? <div className="wk-lifecycle__actions"><button type="button" onClick={() => { setDeletionConflict(undefined); void readRevision() }}>重新读取档案修订</button></div> : null}
    {deletion ? <section className={deletion.status === 'deleted' ? 'wk-lifecycle__receipt wk-lifecycle__receipt--deleted' : deletion.status === 'partial' ? 'wk-lifecycle__receipt wk-lifecycle__receipt--partial' : 'wk-lifecycle__receipt'} aria-label="删除结果">
     <h4>删除状态：{deletionStatusLabels[deletion.status]}</h4>
     {deletion.status === 'deleted' && deletion.completedAt ? <p className="wk-lifecycle__receipt-meta">完成于 <time dateTime={deletion.completedAt}>{deletion.completedAt}</time> · 起始于 <time dateTime={deletion.startedAt}>{deletion.startedAt}</time> · 空间修订 {deletion.revision}</p> : <p className="wk-lifecycle__receipt-meta">起始于 <time dateTime={deletion.startedAt}>{deletion.startedAt}</time> · 空间修订 {deletion.revision}</p>}
     <ol className="wk-lifecycle__steps" aria-label="删除步骤">{deletion.steps.map((step) => <DeletionStepRow key={step.name} step={step} />)}</ol>
     <h4>保留范围与状态</h4>
     <ul>{deletion.retention.map((item) => <li key={item.holder}>{item.holder}：{item.reason}（{item.status}）</li>)}</ul>
    </section> : null}
    {verifyMessage ? <p className="wk-lifecycle__message" role="status" aria-live="polite">{verifyMessage}</p> : null}
   </fieldset>
  </>}
 </section>
}
