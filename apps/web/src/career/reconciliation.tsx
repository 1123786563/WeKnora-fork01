import { useEffect, useState, type ReactNode } from 'react'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ScopeController } from '@weknora/domain/scope'
import type { OpportunityEvidence } from '../../../../packages/career-core/src/contracts.ts'
import type { CareerCoverageView, OpportunityAnnotation, OpportunityObservation, OpportunitySourceStatus, OpportunityStatusView, ReconcileIdentityEvidence, ReconcileReceipt } from '../../../../packages/api-client/src/career.ts'
import { opportunityEvidencePath } from './OpportunityPage.tsx'
import { ReceiptMismatchError, errorDetails, isUncertainWrite, newRequestId } from './protocol.ts'
import './reconciliation.css'

// T12 Web surface: the frozen reconciliation contract rendered honestly.
// Sufficient identity evidence merges; an uncertain pair stays side by side;
// every original link, check time, and immutable snapshot stays openable, and
// a failed recheck keeps the last successful observation with an explicit
// stale window instead of discarding anything.

const annotationLabels: Record<OpportunityAnnotation, string> = { expired: '已过期', delisted: '已下架', requirements_changed: '要求已变化' }
const sourceStatusLabels: Record<OpportunitySourceStatus, string> = { complete: '来源完整', partial: '内容不完整', login_required: '需要登录', blocked: '访问受限', not_found: '页面不存在', timed_out: '抓取超时', fetch_failed: '抓取失败', policy_unverified: '来源未核验' }
const diffFields: Array<[string, keyof OpportunityEvidence['extracted']]> = [['职位名称', 'title'], ['公司', 'company'], ['地点', 'location'], ['招聘批次', 'batch'], ['要求', 'requirements']]
function observationLink(observation: OpportunityObservation): string {
 return observation.source.referenceId ?? observation.submittedUrl ?? observation.finalUrl ?? ''
}
function fieldValue(value: OpportunityEvidence['extracted'][keyof OpportunityEvidence['extracted']]): string {
 return value.state === 'known' ? value.value : '未知'
}
function evidenceValue(value: string | undefined): string {
 return value && value.trim() ? value : '未提供'
}
// The backend emits the Go zero timestamp when no healthy check exists yet;
// the panel says so instead of rendering year 0001.
const missingTimestamp = (value: string): boolean => value.startsWith('0001-01-01')

function EvidenceColumn({ title, evidence }: { title: string; evidence: ReconcileIdentityEvidence }): ReactNode {
 return <div className="wk-reconciliation__evidence-column"><h4>{title}</h4><dl>
  <div><dt>岗位编号</dt><dd><code>{evidenceValue(evidence.jobCode)}</code></dd></div>
  <div><dt>职位名称</dt><dd>{evidenceValue(evidence.title)}</dd></div>
  <div><dt>公司</dt><dd>{evidenceValue(evidence.company)}</dd></div>
  <div><dt>地点</dt><dd>{evidenceValue(evidence.location)}</dd></div>
  <div><dt>招聘批次</dt><dd>{evidenceValue(evidence.batch)}</dd></div>
  {evidence.sourceRef ? <div><dt>来源引用</dt><dd><code>{evidence.sourceRef}</code></dd></div> : null}
 </dl></div>
}

// CAREER-OCR H11（与 InboxPage H10 同族）：unknown 对账的 attempt 必须跨刷新/
// 路由切换存续——恢复文案承诺「原请求编号已保留」。按 scope 隔离持久化，
// 验收成功或确定性失败即清除。
type ReconcileAttempt = { requestId: string; targetId: string; candidateId: string }
const reconcileAttemptKey = (userId: string | null, tenantId: string | null): string => `weknora:career:reconcile-attempt:${userId ?? ''}:${tenantId ?? ''}`
const reconcileSessionStorage = (): Storage | undefined => (typeof window === 'undefined' ? undefined : window.sessionStorage)
function persistReconcileAttempt(scope: { userId: string | null; tenantId: string | null }, attempt: ReconcileAttempt): void {
 try { reconcileSessionStorage()?.setItem(reconcileAttemptKey(scope.userId, scope.tenantId), JSON.stringify(attempt)) } catch { /* private mode */ }
}
function clearReconcileAttempt(scope: { userId: string | null; tenantId: string | null }): void {
 try { reconcileSessionStorage()?.removeItem(reconcileAttemptKey(scope.userId, scope.tenantId)) } catch { /* private mode */ }
}
function readReconcileAttempt(scope: { userId: string | null; tenantId: string | null }): ReconcileAttempt | undefined {
 try {
  const raw = reconcileSessionStorage()?.getItem(reconcileAttemptKey(scope.userId, scope.tenantId))
  if (!raw) return undefined
  const parsed = JSON.parse(raw) as ReconcileAttempt
  if (typeof parsed.requestId !== 'string' || typeof parsed.targetId !== 'string' || typeof parsed.candidateId !== 'string') return undefined
  return parsed
 } catch { return undefined }
}

export function OpportunityStatusPanel({ client, scopeController, opportunityId }: { client: WeKnoraClient; scopeController: ScopeController; opportunityId: string }): ReactNode {
 const scope = scopeController.current()
 const [phase, setPhase] = useState<'loading' | 'ready' | 'error' | 'forbidden' | 'scope-changed'>('loading')
 const [status, setStatus] = useState<OpportunityStatusView>()
 const [history, setHistory] = useState<ReconcileReceipt[]>()
 const [reload, setReload] = useState(0)
 const [diffOld, setDiffOld] = useState(0)
 const [diffNew, setDiffNew] = useState<number>()
 const [oldEvidence, setOldEvidence] = useState<OpportunityEvidence>()
 const [newEvidence, setNewEvidence] = useState<OpportunityEvidence>()
 const [diffPhase, setDiffPhase] = useState<'idle' | 'loading' | 'ready' | 'error'>('idle')
 const [candidateDraft, setCandidateDraft] = useState('')
 const [reconcilePhase, setReconcilePhase] = useState<'idle' | 'busy' | 'unknown' | 'saved' | 'error'>('idle')
 const [reconcileMessage, setReconcileMessage] = useState('')
 const [receipt, setReceipt] = useState<ReconcileReceipt>()
 const [attempt, setAttempt] = useState<ReconcileAttempt>()

 // CAREER-OCR H11: restore an unknown attempt persisted before the refresh/
 // route change (per scope; nothing to restore after settlement).
 useEffect(() => {
  const requestScope = scopeController.current()
  const stored = readReconcileAttempt(requestScope.scope)
  if (!stored || stored.targetId !== opportunityId) return
  setAttempt(stored); setReconcilePhase('unknown')
  setReconcileMessage(`有一次结果未知的对账（原请求编号 ${stored.requestId}）。请先查询原请求回执，或用同一编号重试。`)
 }, [scopeController, scope.scope.generation, opportunityId])

 useEffect(() => {
  const requestScope = scopeController.current()
  let active = true
  // A scope switch must clear the whole reconcile surface, not just the read
  // state: the receipt carries batch identity evidence of the outgoing space,
  // and a kept attempt would replay the old reconcile request against the new
  // space under its original request ID.
  const clearForScopeChange = () => {
   active = false
   setStatus(undefined); setHistory(undefined); setPhase('scope-changed')
   setReceipt(undefined); setAttempt(undefined)
   setCandidateDraft(''); setReconcilePhase('idle'); setReconcileMessage('')
   setDiffPhase('idle'); setOldEvidence(undefined); setNewEvidence(undefined)
  }
  setStatus(undefined); setHistory(undefined)
  if (!opportunityId.trim()) { setPhase('error'); return () => { active = false } }
  setPhase('loading')
  requestScope.signal?.addEventListener('abort', clearForScopeChange, { once: true })
  const loadPanel = async (): Promise<void> => {
   const [nextStatus, nextHistory] = await Promise.all([
    client.career.opportunityStatus(opportunityId, requestScope.signal),
    client.career.opportunityReconciliations(opportunityId, requestScope.signal),
   ])
   if (!active || !scopeController.isCurrent(requestScope.scope)) return
   // ocr3-063：回执 opportunityId 不匹配是确定性协议错误，不能与作用域守卫
   // 合并短路——此前直接 return 让面板永久停留 loading（无错误文案无重试）。
   if (nextStatus.opportunityId !== opportunityId) { setStatus(undefined); setHistory(undefined); setPhase('error'); return }
   setStatus(nextStatus); setHistory(nextHistory.reconciliations); setPhase('ready')
   setDiffOld(0); setDiffNew(nextStatus.observations.length > 1 ? nextStatus.observations.length - 1 : undefined)
  }
  void loadPanel().catch((cause: unknown) => {
   if (!active || !scopeController.isCurrent(requestScope.scope)) return
   setStatus(undefined); setHistory(undefined)
   setPhase(errorDetails(cause).code === 'forbidden' ? 'forbidden' : 'error')
  })
  return () => { active = false; requestScope.signal?.removeEventListener('abort', clearForScopeChange) }
 }, [client, opportunityId, reload, scopeController, scope.scope.generation])

 const observations = status?.observations ?? []
 const oldObservation = observations[diffOld]
 const newObservation = diffNew === undefined ? undefined : observations[diffNew]
 useEffect(() => {
  let active = true
  setOldEvidence(undefined); setNewEvidence(undefined)
  if (!oldObservation || !newObservation || oldObservation.snapshotId === newObservation.snapshotId) { setDiffPhase('idle'); return () => { active = false } }
  const requestScope = scopeController.current()
  setDiffPhase('loading')
  const fetchDiff = async (): Promise<void> => {
   // Both sides of the comparison come from the immutable snapshot pages; a
   // failed read leaves the pair explicitly unreadable without hiding either
   // snapshot's permanent link.
   const [oldPage, newPage] = await Promise.all([
    client.career.opportunityEvidence(opportunityId, oldObservation.snapshotId, requestScope.signal),
    client.career.opportunityEvidence(opportunityId, newObservation.snapshotId, requestScope.signal),
   ])
   if (!active || !scopeController.isCurrent(requestScope.scope)) return
   const oldMatches = oldPage.opportunityId === opportunityId && oldPage.snapshotId === oldObservation.snapshotId
   const newMatches = newPage.opportunityId === opportunityId && newPage.snapshotId === newObservation.snapshotId
   if (!oldMatches || !newMatches) { setDiffPhase('error'); return }
   setOldEvidence(oldPage); setNewEvidence(newPage); setDiffPhase('ready')
  }
  void fetchDiff().catch(() => { if (active && scopeController.isCurrent(requestScope.scope)) setDiffPhase('error') })
  return () => { active = false }
 }, [client, opportunityId, newObservation?.snapshotId, oldObservation?.snapshotId, scopeController])

 const runReconcile = async (currentAttempt: { requestId: string; targetId: string; candidateId: string }): Promise<void> => {
  const requestScope = scopeController.current()
  setReconcilePhase('busy'); setReconcileMessage('正在按固定身份证据判定…')
  try {
   const next = await client.career.reconcileOpportunities(currentAttempt, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   if (next.requestId !== currentAttempt.requestId) throw new ReceiptMismatchError('对账回执与本次请求编号不匹配')
   setReceipt(next); setReconcilePhase('saved'); setReconcileMessage(''); clearReconcileAttempt(requestScope.scope); setReload((value) => value + 1)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   // OCR ocr2-082：回执不匹配是确定性协议错误，直接置 error，不得判
   // unknown 路由进回执恢复（「确定性协议错误不得路由进回执恢复」红线）。
   if (cause instanceof ReceiptMismatchError) { setAttempt(undefined); clearReconcileAttempt(requestScope.scope); setReconcilePhase('error'); setReconcileMessage('对账回执与本次请求编号不匹配，本次判定已中止。请更换新的请求编号重试。'); return }
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { setReceipt(undefined); setAttempt(undefined); clearReconcileAttempt(requestScope.scope); setReconcilePhase('error'); setReconcileMessage('当前空间不可访问此对账，已清除结果。'); return }
   if (parsed.code === 'idempotency_conflict') { setReconcilePhase('error'); setReconcileMessage('请求编号已对应其他对账内容，服务器拒绝了本次判定。请更换新的请求编号重试。'); return }
   // The backend maps ErrOpportunityNotFound into the unified `not_found`
   // code (internal/modules/career/handler.go writeError); there is no
   // `opportunity_not_found` wire code.
   if (parsed.code === 'invalid_request') { setReconcilePhase('error'); setReconcileMessage(`对账请求未被接受：${parsed.message}`); return }
   if (parsed.code === 'not_found') { setReconcilePhase('error'); setReconcileMessage('对账的岗位记录不存在（可能不属于当前空间）。请检查目标与候选记录编号。'); return }
   if (isUncertainWrite(cause)) { setReconcilePhase('unknown'); persistReconcileAttempt(requestScope.scope, currentAttempt); setReconcileMessage('暂时无法确认对账结果。请先查询原请求回执，再决定是否使用同一编号重试。'); return }
   setReconcilePhase('error'); setReconcileMessage('对账未完成，请检查记录编号后重试。')
  }
 }
 const beginReconcile = async (): Promise<void> => {
  if (reconcilePhase === 'busy' || reconcilePhase === 'unknown' || !candidateDraft.trim()) return
  const nextAttempt = { requestId: newRequestId(), targetId: opportunityId, candidateId: candidateDraft.trim() }
  setAttempt(nextAttempt)
  await runReconcile(nextAttempt)
 }
 const retryReconcile = async (): Promise<void> => {
  if (!attempt || reconcilePhase === 'busy') return
  await runReconcile(attempt)
 }
 const lookupReconcileReceipt = async (): Promise<void> => {
  if (!attempt || reconcilePhase === 'busy') return
  const requestScope = scopeController.current()
  setReconcilePhase('busy'); setReconcileMessage('正在查询原对账回执…')
  try {
   const next = await client.career.reconciliationReceipt(attempt.requestId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   if (next.requestId !== attempt.requestId) throw new ReceiptMismatchError('对账回执与本次请求编号不匹配')
   setReceipt(next); setReconcilePhase('saved'); setReconcileMessage(''); clearReconcileAttempt(requestScope.scope); setReload((value) => value + 1)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   // OCR ocr2-082：查询路径同款——回执不匹配置 error 退出恢复，不再自成
   // 「unknown → 查询 → 又 mismatch → unknown」循环。
   if (cause instanceof ReceiptMismatchError) { setAttempt(undefined); clearReconcileAttempt(requestScope.scope); setReconcilePhase('error'); setReconcileMessage('查得的对账回执与原请求编号不匹配，已退出恢复流程。请更换新的请求编号重试。'); return }
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { setReceipt(undefined); setAttempt(undefined); clearReconcileAttempt(requestScope.scope); setReconcilePhase('error'); setReconcileMessage('当前空间不可访问此对账，已清除结果。'); return }
   setReconcilePhase('unknown')
   persistReconcileAttempt(requestScope.scope, attempt)
   setReconcileMessage(parsed.code === 'not_found' ? '尚未找到对账回执。可继续查询，或使用同一请求编号重试。' : '对账回执暂时无法读取。原请求编号已保留。')
  }
 }

 const retry = (): void => setReload((value) => value + 1)
 if (phase === 'loading') return <section className="wk-reconciliation" aria-busy="true"><h2>岗位状态与对账</h2><p role="status">正在读取岗位状态与对账历史…</p></section>
 if (phase !== 'ready' || !status) return <section className="wk-reconciliation"><h2>岗位状态与对账</h2><p role={phase === 'error' ? 'alert' : 'status'}>{phase === 'error' ? '岗位状态暂时无法读取。' : phase === 'forbidden' ? '当前空间不可访问此岗位。' : '空间已切换，已清除岗位状态。'}</p>{phase === 'error' ? <button type="button" onClick={retry}>重试</button> : null}</section>

 return <section className="wk-reconciliation" aria-labelledby="wk-reconciliation-title">
  <h2 id="wk-reconciliation-title">岗位状态与对账</h2>
  {status.mergedInto ? <p className="wk-reconciliation__merged" role="status">此岗位记录已并入 <code>{status.mergedInto}</code>；其历史观察与快照随合并迁移，原有固定链接仍然可以打开。</p> : null}
  {status.annotations.length ? <p className="wk-reconciliation__annotations" role="status">状态标注：{status.annotations.map((annotation) => annotationLabels[annotation]).join(' · ')}</p> : null}
   {status.stale ? <p className="wk-reconciliation__stale" role="status">最近一次来源检查失败：{missingTimestamp(status.lastHealthyAt) ? '尚无成功观察记录，以下内容来自失败的检查，数据可能已陈旧。' : <>以下为最后成功观察，数据可能已陈旧。最后成功检查于 <time dateTime={status.lastHealthyAt}>{status.lastHealthyAt}</time>。</>}</p> : null}
  <section className="wk-reconciliation__history" aria-label="来源观察历史">
   <h3>来源观察历史（不可变）</h3>
   {observations.length ? <ul>{observations.map((observation) => {
    const link = observationLink(observation)
    return <li key={observation.observationId}>
     <p>来源：{observation.source.label || observation.source.kind}{observation.sourceStatus !== undefined ? `（${sourceStatusLabels[observation.sourceStatus]}）` : '（手工粘贴）'}</p>
     <p>原始链接：{link ? <code>{link}</code> : '未记录'}</p>
     <p>检查时间：<time dateTime={observation.acquiredAt}>{observation.acquiredAt}</time></p>
     <p><a href={opportunityEvidencePath(opportunityId, observation.snapshotId)}>打开固定快照</a></p>
    </li>
   })}</ul> : <p>暂无来源观察记录。</p>}
   <p className="wk-reconciliation__history-note">历史观察不可变：每次检查产生新快照，原始链接和检查时间永久保留。</p>
  </section>
  {observations.length >= 2 ? <section className="wk-reconciliation__diff" aria-label="变化前后对比">
   <h3>变化前后对比</h3>
   <div className="wk-reconciliation__diff-controls">
    <label>旧快照<select aria-label="旧快照" value={diffOld} onChange={(event) => setDiffOld(Number(event.currentTarget.value))}>{observations.map((observation, index) => <option key={observation.observationId} value={index}>{observation.acquiredAt} · {observation.snapshotId}</option>)}</select></label>
    <label>新快照<select aria-label="新快照" value={diffNew ?? observations.length - 1} onChange={(event) => setDiffNew(Number(event.currentTarget.value))}>{observations.map((observation, index) => <option key={observation.observationId} value={index}>{observation.acquiredAt} · {observation.snapshotId}</option>)}</select></label>
   </div>
   {diffPhase === 'ready' && oldEvidence && newEvidence && oldObservation && newObservation ? <div>
    <table className="wk-reconciliation__diff-table">
     <thead><tr><th scope="col">字段</th><th scope="col">旧值</th><th scope="col">新值</th><th scope="col">变化</th></tr></thead>
     <tbody>{diffFields.map(([label, key]) => {
      const previous = fieldValue(oldEvidence.extracted[key])
      const next = fieldValue(newEvidence.extracted[key])
      return <tr key={key}><th scope="row">{label}</th><td>{previous}</td><td>{next}</td><td><span className={`wk-reconciliation__diff-marker${previous === next ? '' : ' wk-reconciliation__diff-marker--changed'}`}>{previous === next ? '未变化' : '已变化'}</span></td></tr>
     })}</tbody>
    </table>
    <p><a href={opportunityEvidencePath(opportunityId, oldObservation.snapshotId)}>打开旧快照</a> · <a href={opportunityEvidencePath(opportunityId, newObservation.snapshotId)}>打开新快照</a></p>
    <div className="wk-reconciliation__diff-raw">
     <div><h4>旧原文（摘要 <code>{oldEvidence.rawSha256.slice(0, 12)}</code>）</h4><pre>{oldEvidence.rawText}</pre></div>
     <div><h4>新原文（摘要 <code>{newEvidence.rawSha256.slice(0, 12)}</code>）</h4><pre>{newEvidence.rawText}</pre></div>
    </div>
    {oldEvidence.rawSha256 === newEvidence.rawSha256 ? <p>两个快照的原文摘要一致。</p> : <p>原文内容已变化：以上为两次检查保存的完整原文，均永久可访问。</p>}
   </div> : diffPhase === 'error' ? <p role="status">快照对比暂时无法读取，可稍后重试；两个快照本身仍可分别打开。</p> : <p role="status">正在读取两个固定快照…</p>}
  </section> : null}
  <section className="wk-reconciliation__reconcile" aria-label="岗位对账">
   <h3>对账判定（仅有充分证据才合并）</h3>
   <p>本岗位记录 <code>{opportunityId}</code> 作为判定目标。仅当两条记录的岗位编号、公司、地点与批次全部一致时才合并；不确定重复并列保留，不会静默合并。</p>
   <label className="wk-reconciliation__candidate">候选记录编号<input aria-label="候选记录编号" value={candidateDraft} disabled={reconcilePhase === 'busy' || reconcilePhase === 'unknown'} onChange={(event) => setCandidateDraft(event.target.value)} placeholder="另一条岗位记录编号" autoComplete="off" /></label>
   <div className="wk-reconciliation__actions">
    {reconcilePhase === 'unknown' ? <><button type="button" onClick={() => void lookupReconcileReceipt()}>查询原请求回执</button><button type="button" onClick={() => void retryReconcile()}>使用原请求编号重试</button></> : <button type="button" disabled={reconcilePhase === 'busy' || !candidateDraft.trim()} onClick={() => void beginReconcile()}>{reconcilePhase === 'busy' ? '正在判定…' : '对账判定'}</button>}
   </div>
   {reconcileMessage ? <p className={reconcilePhase === 'error' ? 'wk-reconciliation__message wk-reconciliation__message--error' : 'wk-reconciliation__message'} role={reconcilePhase === 'error' ? 'alert' : 'status'} aria-live="polite">{reconcileMessage}{reconcilePhase === 'unknown' && attempt ? <>（请求编号 <code>{attempt.requestId}</code>）</> : null}</p> : null}
   {receipt ? <div className="wk-reconciliation__receipt" role="status" aria-live="polite">
    <strong>{receipt.decision === 'merged' ? '已合并：候选记录已并入本岗位' : '不确定重复：两条记录并列保留'}</strong>
    {receipt.decision === 'merged' ? <p>合并只迁移观察与快照的归属，历史观察与快照全部保留，未删除任何记录。</p> : <p>身份证据不足以判定为同一岗位，两条记录并列展示；不会静默合并。</p>}
    {receipt.suspectedDuplicate ? <p>疑似同一岗位（职位名称与公司一致），仅供参考。</p> : null}
    {receipt.conflictingBatches?.length ? <p className="wk-reconciliation__conflicts">冲突批次披露：{receipt.conflictingBatches.join('、')}——这些批次在合并前的两条记录上各有申请，相关申请保留在原记录上（诚实历史）。</p> : null}
    <div className="wk-reconciliation__evidence">
     <EvidenceColumn title="判定目标" evidence={receipt.evidence.target} />
     <EvidenceColumn title="候选记录" evidence={receipt.evidence.candidate} />
    </div>
    <p>判定时间：<time dateTime={receipt.createdAt}>{receipt.createdAt}</time> · 请求编号 <code>{receipt.requestId}</code></p>
   </div> : null}
  </section>
  {history?.length ? <section className="wk-reconciliation__decisions" aria-label="对账历史">
   <h3>对账历史</h3>
   <ul>{history.map((decision) => <li key={decision.requestId}>
    <p><time dateTime={decision.createdAt}>{decision.createdAt}</time> · {decision.decision === 'merged' ? '已合并' : '不确定重复（并列保留）'} · 目标 <code>{decision.targetId}</code> ← 候选 <code>{decision.candidateId}</code>{decision.suspectedDuplicate ? ' · 疑似同一岗位' : ''}</p>
    {decision.conflictingBatches?.length ? <p>冲突批次：{decision.conflictingBatches.join('、')}</p> : null}
   </li>)}</ul>
  </section> : null}
 </section>
}

export function CareerCoveragePanel({ client, scopeController }: { client: WeKnoraClient; scopeController: ScopeController }): ReactNode {
 const scope = scopeController.current()
 const [phase, setPhase] = useState<'loading' | 'ready' | 'error' | 'forbidden' | 'scope-changed'>('loading')
 const [coverage, setCoverage] = useState<CareerCoverageView>()
 const [reload, setReload] = useState(0)
 useEffect(() => {
  const requestScope = scopeController.current()
  let active = true
  const clearForScopeChange = () => { active = false; setCoverage(undefined); setPhase('scope-changed') }
  setCoverage(undefined); setPhase('loading')
  requestScope.signal?.addEventListener('abort', clearForScopeChange, { once: true })
  const loadCoverage = async (): Promise<void> => {
   const next = await client.career.careerCoverage(requestScope.signal)
   if (!active || !scopeController.isCurrent(requestScope.scope)) return
   setCoverage(next); setPhase('ready')
  }
  void loadCoverage().catch((cause: unknown) => {
   if (!active || !scopeController.isCurrent(requestScope.scope)) return
   setCoverage(undefined)
   setPhase(errorDetails(cause).code === 'forbidden' ? 'forbidden' : 'error')
  })
  return () => { active = false; requestScope.signal?.removeEventListener('abort', clearForScopeChange) }
 }, [client, reload, scopeController, scope.scope.generation])
 const retry = (): void => setReload((value) => value + 1)
 return <section className="wk-coverage" aria-labelledby="wk-coverage-title">
  <h2 id="wk-coverage-title">来源覆盖说明</h2>
  {phase !== 'ready' || !coverage ? <p role={phase === 'error' ? 'alert' : 'status'}>{phase === 'loading' ? '正在读取来源覆盖…' : phase === 'error' ? '来源覆盖暂时无法读取。' : phase === 'forbidden' ? '当前空间不可读取来源覆盖。' : '空间已切换，已清除来源覆盖。'}{phase === 'error' ? <button type="button" onClick={retry}>重试</button> : null}</p> : <>
   <section aria-label="已接入来源"><h3>已接入（已核验）来源</h3>
    {coverage.configuredSources.length ? <ul>{coverage.configuredSources.map((source) => <li key={source.sourceId}><strong>{source.label}</strong> · {source.available ? '可用' : '不可用'}{source.cities.length ? ` · 城市：${source.cities.join('、')}` : ' · 未登记城市'}{source.accessMethods.length ? ` · 访问方式：${source.accessMethods.join('、')}` : ''}</li>)}</ul> : <p>暂无已核验来源（生产环境从空清单开始，不会虚构来源）。</p>}
   </section>
   <section aria-label="实际观察来源"><h3>实际观察来源</h3>
    {coverage.observedSources.length ? <ul>{coverage.observedSources.map((source) => <li key={source.sourceKind}>{source.label || source.sourceKind} · {source.observations} 次观察 · 最后检查 <time dateTime={source.lastCheckedAt}>{source.lastCheckedAt}</time></li>)}</ul> : <p>暂无观察记录。</p>}
   </section>
   <section aria-label="实际覆盖城市"><h3>实际覆盖城市</h3>
    {coverage.observedCities.length ? <p>{coverage.observedCities.join('、')}</p> : <p>暂无覆盖城市记录（如实呈现，不虚构范围）。</p>}
   </section>
  </>}
 </section>
}
