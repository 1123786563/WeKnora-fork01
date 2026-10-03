import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ScopeController } from '@weknora/domain/scope'
import type { ReminderReceipt, ReminderView, SetReminderInput } from '../../../../packages/api-client/src/career.ts'
import type { CareerAction, CareerReceipt, CareerView } from '../../../../packages/career-core/src/contracts.ts'
import { opportunityEvidencePath } from './OpportunityPage.tsx'
import './inbox.css'
import { ReceiptMismatchError, errorDetails, isUncertainWrite as baseIsUncertainWrite, newRequestId } from './protocol.ts'
// ocr3-054/055：ReceiptMismatchError / errorDetails / newRequestId 统一改用
// protocol.ts 共享实现——本地副本与共享类同名但 instanceof 不互通；本页
// 端点特定的确定性失败码在基础契约之上叠加。
const endpointDefiniteCodes: readonly string[] = ['revision_conflict']
const isUncertainWrite = (cause: unknown): boolean => endpointDefiniteCodes.includes(errorDetails(cause).code ?? '') ? false : baseIsUncertainWrite(cause)

// CAREER-OCR H10: the unknown write attempt must survive a refresh/route
// change — the UI promises "原请求编号已保留". Persisted per scope; restored
// only when the scope matches, cleared once the receipt settles the attempt.
const inboxWriteKey = (userId: string | null, tenantId: string | null): string => `weknora:career:inbox-write:${userId ?? ''}:${tenantId ?? ''}`
const inboxSessionStorage = (): Storage | undefined => (typeof window === 'undefined' ? undefined : window.sessionStorage)
function persistInboxWrite(scope: { userId: string | null; tenantId: string | null }, attempt: WriteAttempt): void {
 try { inboxSessionStorage()?.setItem(inboxWriteKey(scope.userId, scope.tenantId), JSON.stringify(attempt)) } catch { /* private mode */ }
}
function clearInboxWrite(scope: { userId: string | null; tenantId: string | null }): void {
 try { inboxSessionStorage()?.removeItem(inboxWriteKey(scope.userId, scope.tenantId)) } catch { /* private mode */ }
}
function readInboxWrite(scope: { userId: string | null; tenantId: string | null }): WriteAttempt | undefined {
 try {
  const raw = inboxSessionStorage()?.getItem(inboxWriteKey(scope.userId, scope.tenantId))
  if (!raw) return undefined
  const parsed = JSON.parse(raw) as WriteAttempt
  if ((parsed.kind !== 'reminder' && parsed.kind !== 'subscription') || typeof parsed.requestId !== 'string') return undefined
  return parsed
 } catch { return undefined }
}


type ReadState = 'loading' | 'ready' | 'error' | 'forbidden' | 'scope-changed'
type WritePhase = 'idle' | 'busy' | 'unknown' | 'error'
type WriteAttempt = { kind: 'reminder'; requestId: string; input: SetReminderInput } | { kind: 'subscription'; requestId: string; value: 'subscribed' | 'unsubscribed'; expectedRevision: number }
type SubscriptionAttempt = Extract<WriteAttempt, { kind: 'subscription' }>
type ApplicationRef = { snapshotId: string; opportunityId: string }
// Frozen backend enums rendered verbatim (internal/modules/career/
// reminder.go): the todo body is the frozen privacy literal and the push
// reasons are the closed set; the UI never invents a source kind, a notice
// or a push outcome the career backend did not send.
const sourceKindLabels: Record<ReminderView['sourceKind'], string> = { progress_event: '来源：求职进展事件', discovery: '来源：持续找岗发现' }
const PUSH_FACT_KEY = 'notifications.push'
const PUSH_UNSUBSCRIBED = 'unsubscribed'

// A receipt that does not answer the request it claims is a definite
// protocol error, unlike a network TypeError, which leaves the write outcome
// genuinely unknown and must route into receipt recovery.
function pushOutcomeNote(receipt: ReminderReceipt): string {
 // The push report is response-only: neither outcome touches the todo.
 if (!receipt.push) return ''
 if (!receipt.push.attempted) return '当前已退订推送：未发送提醒，待办以站内为准。'
 return receipt.push.delivered ? '推送提醒已发送（推送只是提醒，以站内为准）。' : '推送投递失败——待办已保存，以站内为准；推送失败不改变站内事实。'
}
function applicationDetailPath(ref: ApplicationRef, applicationId: string): string {
 return `${opportunityEvidencePath(ref.opportunityId, ref.snapshotId)}&application=${encodeURIComponent(applicationId)}`
}

function InboxTodoRow({ item, applicationRef }: { item: ReminderView; applicationRef: ApplicationRef | 'failed' | undefined }): ReactNode {
 const applicationId = item.applicationId
 return <li className="wk-inbox__todo" aria-label={`站内待办 ${item.reminderId}`}>
  <p className="wk-inbox__notice">{item.notice}</p>
  <div className="wk-inbox__todo-meta">
   <span>{sourceKindLabels[item.sourceKind]}</span>
   <time dateTime={item.createdAt}>{item.createdAt}</time>
  </div>
  {applicationId ? (applicationRef === 'failed'
   ? <p className="wk-inbox__todo-note" role="note">权威申请暂时无法定位（可能已不可见），可刷新重试；待办本身不受影响。</p>
   : applicationRef
    ? <a className="wk-inbox__todo-link" href={applicationDetailPath(applicationRef, applicationId)}>进入权威申请详情</a>
    : <p className="wk-inbox__todo-note" role="status">正在定位权威申请…</p>)
   : <a className="wk-inbox__todo-link" href="/platform/career/search">前往持续找岗查看发现</a>}
 </li>
}

// T20 in-station todo inbox: the durable todo row is the authoritative
// reminder fact, so the list is the reading surface every push points back
// to. One source event holds exactly one todo (a re-trigger answers a
// deduplicated receipt and never renders a second row), every todo body is
// the frozen privacy literal — company, job and interview detail never
// appear — and the push subscription is a confirmed profile fact: opting out
// stops the pushes while the todos stay readable here. Writes carry a
// request ID and the pinned profile revision; an unknown outcome recovers
// through the receipt of the original request ID, never a new one.
export function InboxPage({ client, scopeController, deletionGeneration = 0 }: { client: WeKnoraClient; scopeController: ScopeController; deletionGeneration?: number }): ReactNode {
 const scope = scopeController.current()
 const [todos, setTodos] = useState<ReminderView[]>()
 const [applicationRefs, setApplicationRefs] = useState<Record<string, ApplicationRef | 'failed'>>()
 const [readState, setReadState] = useState<ReadState>('loading')
 const [readMessage, setReadMessage] = useState('')
 const [view, setView] = useState<CareerView>()
 const [reload, setReload] = useState(0)
 const [sourceKind, setSourceKind] = useState<'progress_event' | 'discovery'>('progress_event')
 const [sourceId, setSourceId] = useState('')
 const [attempt, setAttempt] = useState<WriteAttempt>()
 const [writePhase, setWritePhase] = useState<WritePhase>('idle')
 const [message, setMessage] = useState('')
 const [notice, setNotice] = useState('')
 const writeInFlight = useRef(false)
 const deletionEpoch = useRef(deletionGeneration)

 const clearPrivate = useCallback((notice: string, nextState: 'forbidden' | 'scope-changed' = 'forbidden') => {
  setTodos(undefined); setApplicationRefs(undefined); setReadState(nextState); setReadMessage(notice)
  setView(undefined); setAttempt(undefined); setWritePhase('idle'); setMessage(''); setSourceId(''); setNotice('')
 }, [])
 useEffect(() => {
  const requestScope = scopeController.current()
  const clear = () => clearPrivate('空间已切换或登录已失效，已清除收件箱内容。', 'scope-changed')
  requestScope.signal?.addEventListener('abort', clear, { once: true })
  return () => requestScope.signal?.removeEventListener('abort', clear)
 }, [clearPrivate, scopeController, scope.scope.generation])

 useEffect(() => {
  if (deletionEpoch.current === deletionGeneration) return
  deletionEpoch.current = deletionGeneration
  clearPrivate('个人求职空间已删除，已清除收件箱内容。', 'scope-changed')
 }, [clearPrivate, deletionGeneration])

 const readView = useCallback(async (): Promise<CareerView | undefined> => {
  const requestScope = scopeController.current()
  try {
   const next = await client.career.open(requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return undefined
   setView(next)
   return next
  } catch {
   if (!scopeController.isCurrent(requestScope.scope)) return undefined
   return undefined
  }
 }, [client, scopeController])

 // CAREER-OCR H10: restore an unknown attempt persisted before the
 // refresh/route change (per scope; nothing to restore after settlement).
 useEffect(() => {
  const requestScope = scopeController.current()
  const stored = readInboxWrite(requestScope.scope)
  if (!stored) return
  setAttempt(stored); setWritePhase('unknown')
  setMessage(`有一次结果未知的写入（原请求编号 ${stored.requestId}）。请先用原请求编号查询回执，或用同一编号重试；不会自动更换请求编号。`)
 }, [scopeController, scope.scope.generation])

 useEffect(() => {
  let active = true
  const requestScope = scopeController.current()
  setTodos(undefined); setApplicationRefs(undefined); setReadState('loading'); setReadMessage('')
  const read = async (): Promise<void> => {
   try {
    const next = await client.career.reminders(requestScope.signal)
    if (!active || !scopeController.isCurrent(requestScope.scope) || deletionEpoch.current !== deletionGeneration) return
    // Progress-event todos deep-link into the authoritative application
    // detail; the pinned snapshot is resolved through the application
    // receipt so the link opens exactly the application the todo binds to.
    const refs: Record<string, ApplicationRef | 'failed'> = {}
    for (const item of next.reminders) {
     const applicationId = item.applicationId
     if (!applicationId || applicationId in refs) continue
     try {
      const receipt = await client.career.application(applicationId, requestScope.signal)
      if (!scopeController.isCurrent(requestScope.scope) || deletionEpoch.current !== deletionGeneration) return
      refs[applicationId] = { snapshotId: receipt.pinnedEvidence.snapshotId, opportunityId: receipt.pinnedEvidence.opportunityId }
     } catch {
      if (!scopeController.isCurrent(requestScope.scope) || deletionEpoch.current !== deletionGeneration) return
      refs[applicationId] = 'failed'
     }
    }
    if (!active || deletionEpoch.current !== deletionGeneration) return
    setTodos(next.reminders); setApplicationRefs(refs); setReadState('ready')
   } catch (cause) {
    if (!active || !scopeController.isCurrent(requestScope.scope) || deletionEpoch.current !== deletionGeneration) return
    setTodos(undefined)
    const parsed = errorDetails(cause)
    if (parsed.code === 'forbidden') { setReadState('forbidden'); setReadMessage('当前空间不可访问收件箱。请切换到本人拥有的单成员个人空间后重试。'); return }
    setReadState('error')
    setReadMessage('收件箱暂时无法读取，可刷新重试。')
   }
  }
  void read()
  void readView()
  return () => { active = false }
 }, [client, reload, readView, scopeController, scope.scope.generation])

 const refresh = (): void => setReload((value) => value + 1)
 const unsubscribed = view?.facts.some((fact) => fact.key === PUSH_FACT_KEY && fact.value === PUSH_UNSUBSCRIBED) ?? false

 const acceptReminderReceipt = (next: ReminderReceipt, expectedRequestId: string): void => {
  if (next.requestId !== expectedRequestId) throw new ReceiptMismatchError('待办回执与本次请求不匹配')
  setAttempt(undefined); setWritePhase('idle'); setSourceId(''); setMessage(''); clearInboxWrite(scopeController.current().scope)
  setNotice([next.deduplicated ? '该来源已有待办：同一来源事件只保留一条，未新增第二条。' : '待办已登记。', pushOutcomeNote(next)].filter(Boolean).join(' '))
  refresh()
 }
 const acceptSubscriptionReceipt = (next: CareerReceipt, expected: SubscriptionAttempt): void => {
  if (next.requestId !== expected.requestId) throw new ReceiptMismatchError('订阅回执与本次请求不匹配')
  setAttempt(undefined); setWritePhase('idle'); setMessage(''); clearInboxWrite(scopeController.current().scope)
  setNotice(expected.value === PUSH_UNSUBSCRIBED ? '推送提醒已退订：不再发送推送，已存在的站内待办仍可读取。' : '已重新订阅推送提醒。')
  refresh()
 }
 const runWrite = async (fixed?: WriteAttempt): Promise<void> => {
  if (writeInFlight.current) return
  let current = fixed
  if (!current) {
   if (writePhase === 'busy' || writePhase === 'unknown') return
   const revision = view?.revision
   if (revision === undefined || !sourceId.trim()) return
   const requestId = newRequestId()
   current = { kind: 'reminder', requestId, input: { requestId, sourceKind, sourceId: sourceId.trim(), expectedRevision: revision } }
  }
  writeInFlight.current = true
  const requestScope = scopeController.current()
  setAttempt(current); setWritePhase('busy'); setNotice(''); setMessage(current.kind === 'reminder' ? '正在登记待办…' : '正在更新推送订阅…')
  try {
   if (current.kind === 'reminder') {
    const next = await client.career.setReminder(current.input, requestScope.signal)
    if (!scopeController.isCurrent(requestScope.scope)) return
    acceptReminderReceipt(next, current.requestId)
   } else {
    const action: CareerAction = { action: 'confirm', key: PUSH_FACT_KEY, value: current.value, source: { kind: 'user', label: '本人确认' }, requestId: current.requestId, expectedRevision: current.expectedRevision }
    const next = await client.career.act(action, requestScope.signal)
    if (!scopeController.isCurrent(requestScope.scope)) return
    acceptSubscriptionReceipt(next, current)
   }
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问收件箱，已清除内容。'); return }
   if (parsed.code === 'revision_conflict') {
    setAttempt(undefined); setWritePhase('error'); clearInboxWrite(requestScope.scope)
    setMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请刷新后重新提交；新提交会使用新的请求编号。`)
    await readView()
    return
   }
   if (['invalid_request', 'idempotency_conflict', 'not_found', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
    setAttempt(undefined); setWritePhase('error'); clearInboxWrite(requestScope.scope)
    setMessage(parsed.code === 'not_found' ? (current.kind === 'reminder' ? '来源事件不存在（可能不属于当前空间）。请核对来源编号后重试。' : '订阅状态暂时无法读取。请刷新后重试。') : parsed.code === 'idempotency_conflict' ? '请求编号已对应其他内容，服务器拒绝了本次提交。请重新提交。' : `操作未被接受：${parsed.message}`)
    return
   }
   if (cause instanceof ReceiptMismatchError) { setAttempt(undefined); setWritePhase('error'); setMessage(`操作未完成：${cause.message}`); return }
   if (!isUncertainWrite(cause)) { setAttempt(undefined); setWritePhase('error'); setMessage(`操作未完成：${parsed.message}`); clearInboxWrite(requestScope.scope); return }
   setWritePhase('unknown')
   persistInboxWrite(requestScope.scope, current)
   setMessage(`暂时无法确认操作是否已保存（原请求编号 ${current.requestId}）。请先用原请求编号查询回执，或用同一编号重试；不会自动更换请求编号。`)
  } finally { writeInFlight.current = false }
 }
 const lookupReceipt = async (): Promise<void> => {
  if (!attempt || writePhase === 'busy') return
  const current = attempt
  const requestScope = scopeController.current()
  setWritePhase('busy'); setMessage('正在查询原回执…')
  try {
   if (current.kind === 'reminder') {
    const next = await client.career.reminderReceipt(current.requestId, requestScope.signal)
    if (!scopeController.isCurrent(requestScope.scope)) return
    acceptReminderReceipt(next, current.requestId)
   } else {
    const next = await client.career.receipt(current.requestId, requestScope.signal)
    if (!scopeController.isCurrent(requestScope.scope)) return
    if (next.kind !== 'confirmed' || next.fact.key !== PUSH_FACT_KEY) throw new ReceiptMismatchError('回执不是本次订阅结果')
    acceptSubscriptionReceipt(next, current)
   }
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问收件箱，已清除内容。'); return }
   if (cause instanceof ReceiptMismatchError) { setAttempt(undefined); setWritePhase('error'); setMessage(`操作未完成：${cause.message}`); clearInboxWrite(requestScope.scope); return }
   setWritePhase('unknown')
   setMessage(parsed.code === 'not_found' ? `尚未找到回执（原请求编号 ${current.requestId}）。可以继续查询，或使用原请求编号重试。` : '回执暂时无法读取。原请求编号已保留，可稍后重试查询。')
  }
 }

 const composeBlocked = writePhase === 'busy' || writePhase === 'unknown'
 return <section className="wk-inbox" aria-labelledby="wk-inbox-title">
  <h3 id="wk-inbox-title">站内待办收件箱</h3>
  <p className="wk-inbox__intro">推送只是提醒，站内待办才是事实源；通知正文不含公司、岗位或面试详情。</p>
  {readState === 'forbidden' || readState === 'scope-changed' ? <div>
   <p className="wk-inbox__message wk-inbox__message--error" role="alert">{readMessage}</p>
   <div className="wk-inbox__actions"><button type="button" onClick={refresh}>重新读取</button></div>
  </div> : <>
   {notice ? <p className="wk-inbox__notice-ok" role="status" aria-live="polite">{notice}</p> : null}
   {readState === 'loading' ? <p className="wk-inbox__state" role="status" aria-busy="true">正在读取站内待办…</p> : readState === 'error' ? <p className="wk-inbox__message wk-inbox__message--error" role="alert">{readMessage} <button type="button" onClick={refresh}>重新读取</button></p> : <>
    <div className="wk-inbox__push" aria-label="推送订阅">
     <p aria-label="推送提醒状态">{unsubscribed ? '已退订推送提醒（站内待办不受影响，仍可读取）' : '已订阅推送提醒'}</p>
     <button type="button" disabled={composeBlocked || view === undefined} onClick={() => { if (view) void runWrite({ kind: 'subscription', requestId: newRequestId(), value: unsubscribed ? 'subscribed' : PUSH_UNSUBSCRIBED, expectedRevision: view.revision }) }}>{unsubscribed ? '重新订阅推送提醒' : '退订推送提醒'}</button>
    </div>
    <div className="wk-inbox__actions"><button type="button" onClick={refresh}>刷新待办</button></div>
    {todos && todos.length ? <ol className="wk-inbox__todos" aria-label="站内待办列表">
     {todos.map((item) => <InboxTodoRow key={item.reminderId} item={item} applicationRef={item.applicationId ? applicationRefs?.[item.applicationId] : undefined} />)}
    </ol> : <p className="wk-inbox__empty">没有站内待办。</p>}
    <fieldset className="wk-inbox__compose"><legend>登记待办</legend>
     <label className="wk-inbox__label" htmlFor="wk-inbox-source-kind">来源类型</label>
     <select id="wk-inbox-source-kind" aria-label="来源类型" value={sourceKind} disabled={composeBlocked} onChange={(event) => setSourceKind(event.target.value === 'discovery' ? 'discovery' : 'progress_event')}>
      <option value="progress_event">求职进展事件</option>
      <option value="discovery">持续找岗发现</option>
     </select>
     <label className="wk-inbox__label" htmlFor="wk-inbox-source-id">来源编号</label>
     <input id="wk-inbox-source-id" aria-label="来源编号" value={sourceId} disabled={composeBlocked} placeholder="进展事件或发现的编号" onChange={(event) => setSourceId(event.currentTarget.value)} />
     <div className="wk-inbox__actions"><button type="button" className="wk-inbox__submit" disabled={composeBlocked || view === undefined || !sourceId.trim()} onClick={() => void runWrite()}>登记待办</button></div>
    </fieldset>
   </>}
   {writePhase === 'unknown' && attempt ? <div className="wk-inbox__actions" role="group" aria-label="恢复待办写入">
    <button type="button" onClick={() => void lookupReceipt()}>查询待办回执</button>
    <button type="button" onClick={() => void runWrite(attempt)}>用原请求编号重试</button>
   </div> : null}
   {message && writePhase !== 'idle' ? <p className={writePhase === 'error' ? 'wk-inbox__message wk-inbox__message--error' : 'wk-inbox__message'} role={writePhase === 'error' ? 'alert' : 'status'} aria-live="polite">{message}</p> : null}
  </>}
 </section>
}
