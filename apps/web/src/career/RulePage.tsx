import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { Button, Card } from 'tdesign-react'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ScopeController } from '@weknora/domain/scope'
import type { RuleRunView, RuleStatus, RuleTodoView, RuleView, SetRuleReceipt } from '../../../../packages/api-client/src/career.ts'
import { CareerUsagePanel, useCareerUsageEstimate } from './UsagePanel.tsx'
import './rule.css'

// T13 recurring search rule page. The rule is the only continuous search
// structure and it is entirely user-controlled: it never runs while
// disabled or paused, every write carries a request ID and the observed
// profile revision, and the enable-time estimate is the backend's frozen
// projection displayed verbatim (never recomputed here).
const makeId = (): string => typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`
type Attempt = { requestId: string; ruleId?: string; query: string; intervalMinutes: number; status: RuleStatus; expectedRevision: number }
type Phase = 'idle' | 'busy' | 'unknown'
type ViewPhase = 'loading' | 'ready' | 'forbidden' | 'error' | 'scope-changed'
type TypedError = { code?: string; currentRevision?: number; text: string }

const statusLabels: Record<RuleStatus, string> = { enabled: '已启用', paused: '已暂停', disabled: '未启用' }
const runStatusLabels: Record<RuleRunView['status'], string> = { completed: '已完成', failed: '未完成', blocked_no_quota: '额度不足（本次未执行搜索）', no_vetted_sources: '暂无已核验来源（本次未执行搜索）' }

function errorDetails(cause: unknown): TypedError {
 const error = cause as { code?: string; currentRevision?: number; message?: string }
 return { code: error?.code, currentRevision: error?.currentRevision, text: error?.message || '请求未完成' }
}
// Ambiguous transport failures leave the durable outcome unknown exactly
// like outcome_unknown: recover by receipt under the original request ID,
// never by silently minting a new rule write.
function isUncertainOutcome(cause: unknown): boolean {
 const error = errorDetails(cause)
 if (error.code === 'TIMEOUT' || error.code === 'outcome_unknown') return true
 if (['forbidden', 'invalid_request', 'idempotency_conflict', 'revision_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE', 'not_found', 'unauthorized'].includes(error.code ?? '')) return false
 const status = (cause as { status?: number }).status
 if (status !== undefined) return status >= 500 || status < 400
 return true
}
function formatCheckTime(timestamp: string): string {
 return `${timestamp.slice(0, 10)} ${timestamp.slice(11, 16)} UTC`
}
function formatEstimateNumber(value: number): string {
 return Number.isInteger(value) ? String(value) : value.toFixed(2)
}
function ruleIdKey(userId: string | null, tenantId: string | null): string {
 return `weknora:career:rule-id:${userId ?? ''}:${tenantId ?? ''}`
}
function attemptKey(userId: string | null, tenantId: string | null): string { return `weknora:career:rule-attempt:${userId ?? ''}:${tenantId ?? ''}` }
function readStoredRuleId(storage: Storage | undefined, key: string): string | undefined {
 if (!storage) return undefined
 try {
  const raw = storage.getItem(key)
  return raw && raw.trim() ? raw : undefined
 } catch { return undefined }
}
function decodeAttempt(value: unknown): Attempt | undefined {
 if (!value || typeof value !== 'object') return undefined
 const item = value as Record<string, unknown>
 if (typeof item.requestId !== 'string' || !item.requestId.trim() || (item.ruleId !== undefined && (typeof item.ruleId !== 'string' || !item.ruleId.trim()))
  || typeof item.query !== 'string' || !item.query.trim() || !Number.isSafeInteger(item.intervalMinutes) || Number(item.intervalMinutes) < 1 || Number(item.intervalMinutes) > 43200
  || !['enabled', 'paused', 'disabled'].includes(String(item.status)) || !Number.isSafeInteger(item.expectedRevision) || Number(item.expectedRevision) < 0
  || Object.keys(item).some((key) => !['requestId', 'ruleId', 'query', 'intervalMinutes', 'status', 'expectedRevision'].includes(key))) return undefined
 return value as Attempt
}

export function CareerRulePage({ client, scopeController }: { client: WeKnoraClient; scopeController: ScopeController }): ReactNode {
 const scope = scopeController.current().scope
 // T21: every rule trigger is one charged search_once, so the quota
 // estimate is shown before enabling; enabling requires a live estimate
 // (fail-closed, no enable-first-report-later path).
 const usage = useCareerUsageEstimate(client, scopeController)
 const [viewPhase, setViewPhase] = useState<ViewPhase>('loading')
 const [revision, setRevision] = useState<number>()
 const [viewError, setViewError] = useState<TypedError>()
 const [draft, setDraft] = useState({ query: '', interval: '1440', status: 'disabled' as RuleStatus })
 const [attempt, setAttempt] = useState<Attempt>()
 const [receipt, setReceipt] = useState<SetRuleReceipt>()
 const [ruleView, setRuleView] = useState<RuleView>()
 const [rules, setRules] = useState<Array<{ ruleId: string; query: string; intervalMinutes: number; status: RuleStatus; revision: number }>>([])
 const [selectedRuleId, setSelectedRuleId] = useState<string>()
 const [detailLoading, setDetailLoading] = useState(false)
 const [storedRuleUnreadable, setStoredRuleUnreadable] = useState<TypedError>()
 const [phase, setPhase] = useState<Phase>('idle')
 const [notice, setNotice] = useState('')
 const [error, setError] = useState<TypedError>()
 const loadedForScope = useRef<string | undefined>(undefined)
 const selectionGeneration = useRef(0)

 const clearForScopeChange = useCallback((message: string): void => {
  selectionGeneration.current += 1
  // Memory only. The rule-id key is already isolated per userId/tenantId, and
  // the abort listener fires while current() still reports the outgoing
  // identity (scope.advance aborts before installing the new scope), so a
  // removeItem here would delete the outgoing user's own stored reference and
  // make their next save silently mint a second rule. The stored reference is
  // only dropped on a server-confirmed not_found in load().
  setDraft({ query: '', interval: '1440', status: 'disabled' }); setAttempt(undefined); setReceipt(undefined); setRuleView(undefined); setRules([]); setSelectedRuleId(undefined); setDetailLoading(false); setStoredRuleUnreadable(undefined)
  setPhase('idle'); setNotice(message); setError(undefined)
  setRevision(undefined); setViewPhase('scope-changed'); setViewError(undefined)
 }, [scopeController])

 const load = useCallback(async (): Promise<void> => {
  const requestScope = scopeController.current()
  setViewPhase('loading'); setViewError(undefined); setStoredRuleUnreadable(undefined)
  try {
   const view = await client.career.open(requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   setRevision(view.revision)
   const allRules: Awaited<ReturnType<typeof client.career.listRules>>['rules'] = []
   let cursor: string | undefined
   const seenCursors = new Set<string>()
   do {
    const listed = await client.career.listRules(cursor, requestScope.signal)
    if (!scopeController.isCurrent(requestScope.scope)) return
    allRules.push(...listed.rules)
    cursor = listed.nextCursor ?? undefined
    if (cursor && seenCursors.has(cursor)) throw new TypeError('重复的规则列表游标')
    if (cursor) seenCursors.add(cursor)
   } while (cursor)
   setRules(allRules)
   const storage = typeof window === 'undefined' ? undefined : window.localStorage
   const storedRuleId = readStoredRuleId(storage, ruleIdKey(requestScope.scope.userId, requestScope.scope.tenantId))
   const selected = allRules.find((rule) => rule.ruleId === storedRuleId) ?? (allRules.length === 1 ? allRules[0] : undefined)
   setSelectedRuleId(selected?.ruleId)
   if (selected) {
    try { storage?.setItem(ruleIdKey(requestScope.scope.userId, requestScope.scope.tenantId), selected.ruleId) } catch { /* private mode */ }
    try {
     const stored = await client.career.getRule(selected.ruleId, requestScope.signal)
     if (!scopeController.isCurrent(requestScope.scope)) return
     setRuleView(stored)
     setDraft({ query: stored.query, interval: String(stored.intervalMinutes), status: stored.status })
    } catch (cause) {
     if (!scopeController.isCurrent(requestScope.scope)) return
     const parsed = errorDetails(cause)
     if (parsed.code === 'forbidden') { clearForScopeChange('当前空间不可访问，已清除持续找岗状态。'); return }
     if (parsed.code === 'not_found') {
      try { storage?.removeItem(ruleIdKey(requestScope.scope.userId, requestScope.scope.tenantId)) } catch { /* private mode */ }
      setNotice('这条规则在服务端已不可见，已清除本地引用。可重新创建一条规则。')
     } else {
      setStoredRuleUnreadable(parsed)
      setNotice(`读取已保存规则未成功：${parsed.text}。此时保存会新建一条规则；请先重新读取成功后再保存，避免出现双重规则。`)
     }
    }
   }
   const attemptStorageKey = attemptKey(requestScope.scope.userId, requestScope.scope.tenantId)
   let pending: Attempt | undefined
   let rawAttempt: string | null = null
   try { rawAttempt = storage?.getItem(attemptStorageKey) ?? null; if (rawAttempt) pending = decodeAttempt(JSON.parse(rawAttempt)) } catch { /* invalid persisted data fails closed below */ }
   if (rawAttempt && !pending) { setStoredRuleUnreadable({ text: '本地保存的未确认请求无法安全读取；为避免重复写入，保存已停用。' }); setViewPhase('ready'); return }
   if (pending) {
    setAttempt(pending); setPhase('unknown'); setNotice('正在恢复上次未确认的规则保存…')
    try {
     let recovered: SetRuleReceipt
     try { recovered = await client.career.ruleReceipt(pending.requestId, requestScope.signal) }
     catch (cause) { if (errorDetails(cause).code !== 'not_found') throw cause; recovered = await client.career.setRule(pending, requestScope.signal) }
     if (!scopeController.isCurrent(requestScope.scope)) return
     if (recovered.requestId === pending.requestId) {
      try { storage?.removeItem(attemptStorageKey) } catch { /* private mode */ }
      setReceipt(recovered); setRuleView(undefined); setSelectedRuleId(recovered.ruleId); setPhase('idle'); setAttempt(undefined)
      try { storage?.setItem(ruleIdKey(requestScope.scope.userId, requestScope.scope.tenantId), recovered.ruleId) } catch { /* private mode */ }
     }
    } catch (cause) { if (scopeController.isCurrent(requestScope.scope)) { setPhase('unknown'); setNotice(`上次保存结果仍未确认：${errorDetails(cause).text}。请先恢复原请求。`) } }
   }
   if (scopeController.isCurrent(requestScope.scope)) setViewPhase('ready')
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { setViewPhase('forbidden'); setViewError(parsed); return }
   setViewPhase('error'); setViewError(parsed)
  }
 }, [clearForScopeChange, client, scopeController])

 useEffect(() => {
  const scopeKey = `${scope.userId}:${scope.tenantId}:${scope.generation}`
  if (loadedForScope.current !== scopeKey) {
   loadedForScope.current = scopeKey
   void load()
  }
 }, [load, scope.userId, scope.tenantId, scope.generation])

 // Desk semantics: a space switch (or logout) aborts the live scope; cached
 // rule state must not leak across identities.
 useEffect(() => {
  const activeScope = scopeController.current()
  const clear = () => clearForScopeChange('空间已切换或登录已失效，已清除持续找岗状态。')
  activeScope.signal?.addEventListener('abort', clear, { once: true })
  return () => activeScope.signal?.removeEventListener('abort', clear)
 }, [clearForScopeChange, scopeController, scope.generation])

 const refreshRuns = useCallback(async (ruleId: string): Promise<void> => {
  const requestScope = scopeController.current()
  try {
   const stored = await client.career.getRule(ruleId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   setRuleView(stored)
  } catch { /* run history is best-effort after a write; the receipt above stays authoritative */ }
 }, [client, scopeController])

 const acceptReceipt = useCallback((next: SetRuleReceipt): void => {
  setReceipt(next); setPhase('idle'); setError(undefined); setNotice(''); setAttempt(undefined)
  const activeScope = scopeController.current().scope
  try { window.localStorage.removeItem(attemptKey(activeScope.userId, activeScope.tenantId)) } catch { /* private mode */ }
  setSelectedRuleId(next.ruleId)
  try { window.localStorage.setItem(ruleIdKey(activeScope.userId, activeScope.tenantId), next.ruleId) } catch { /* private mode */ }
  // A saved rule (especially an enabled one) changes what the next charged
  // run would consume; re-read the live estimate instead of showing a
  // stale balance.
  usage.reload()
  void refreshRuns(next.ruleId)
 }, [refreshRuns, usage.reload])

 const send = useCallback(async (next: Attempt): Promise<void> => {
  const requestScope = scopeController.current()
  setPhase('busy'); setError(undefined); setNotice('正在保存规则…')
  const storageScope = requestScope.scope
  try { window.localStorage.setItem(attemptKey(storageScope.userId, storageScope.tenantId), JSON.stringify(next)) }
  catch { setPhase('idle'); setError({ code: 'storage_unavailable', text: '浏览器无法保存恢复请求，本次规则写入未发送。' }); setNotice(''); return }
  try {
   const result = await client.career.setRule(next, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   if (result.requestId !== next.requestId) { setError({ code: 'invalid_response', text: '服务返回的请求编号与本次保存不匹配，已放弃本次结果。请重新保存。' }); setPhase('idle'); setAttempt(undefined); return }
   acceptReceipt(result)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearForScopeChange('当前空间不可访问，已清除持续找岗状态。'); return }
   if (parsed.code === 'revision_conflict') {
    setError(parsed); setPhase('idle'); setNotice('')
    // The caller's observed revision is stale; re-read it so the retry under
    // the original request ID pins the fresh view.
    try {
     const view = await client.career.open(requestScope.signal)
     if (scopeController.isCurrent(requestScope.scope)) setRevision(view.revision)
    } catch { /* the conflict panel keeps the server-reported currentRevision */ }
    return
   }
   if (['invalid_request', 'idempotency_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
    setError(parsed); setPhase('idle'); setAttempt(undefined); setNotice(parsed.code === 'invalid_request' ? '规则内容未被接受，请调整后重新保存。' : '本次请求与已保存的规则内容不一致，已放弃；请重新保存。')
    return
   }
   if (isUncertainOutcome(cause)) { setPhase('unknown'); setNotice(''); return }
   setError(parsed); setPhase('idle'); setNotice('')
  }
 }, [acceptReceipt, clearForScopeChange, client, scopeController])

 const intervalNumber = Number(draft.interval)
 const intervalValid = Number.isSafeInteger(intervalNumber) && intervalNumber >= 1 && intervalNumber <= 43200
 // T21: enabling is the charged path — every trigger is one search_once —
 // so it requires a live estimate. Paused/disabled configuration stays
 // open: those states can never trigger a charged run. An exhausted window
 // does not block the write itself; each trigger is then visibly blocked
 // (blocked_no_quota), which the estimate panel states up front.
 const enableRequiresEstimate = draft.status === 'enabled' && usage.state.phase !== 'ready'
 // An existing rule (created here or restored from the stored reference) is
 // updated under its rule ID; a fresh save without one creates a new rule.
 const existingRuleId = receipt?.ruleId ?? ruleView?.ruleId
 const submit = (event: FormEvent): void => {
  event.preventDefault()
  if (composeLocked || !draft.query.trim() || !intervalValid || revision === undefined || enableRequiresEstimate) return
  const next: Attempt = { requestId: makeId(), ...(existingRuleId ? { ruleId: existingRuleId } : {}), query: draft.query.trim(), intervalMinutes: intervalNumber, status: draft.status, expectedRevision: revision }
  setAttempt(next)
  void send(next)
 }
 const retrySameAttempt = (): void => { if (attempt) void send(attempt) }
 const retryWithCurrentRevision = (): void => {
  if (!attempt || revision === undefined) return
  const next: Attempt = { ...attempt, expectedRevision: revision }
  setAttempt(next)
  void send(next)
 }
 const queryReceipt = async (): Promise<void> => {
  if (!attempt) return
  const requestScope = scopeController.current()
  setPhase('busy')
  try {
   const stored = await client.career.ruleReceipt(attempt.requestId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   if (stored.requestId !== attempt.requestId) { setError({ code: 'invalid_response', text: '服务返回的请求编号与本次保存不匹配；已放弃本次结果，请使用新的请求编号重新保存。' }); setPhase('idle'); setAttempt(undefined); setNotice(''); return }
   acceptReceipt(stored)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearForScopeChange('当前空间不可访问，已清除持续找岗状态。'); return }
   setPhase('unknown')
   setNotice(parsed.code === 'not_found' ? '暂未找到回执；保留原内容与请求编号，可用同一请求编号重试。' : `查询回执未成功：${parsed.text}。保留原内容与请求编号。`)
  }
 }

 const busy = phase === 'busy'
 // An unknown write outcome recovers only under the original request ID, so
 // the compose form must stay locked exactly like busy — editing and
 // submitting a new attempt here would mint a fresh request ID and break
 // that recovery contract. A stored rule whose reference cannot be resolved
 // locks the form too (saving then would mint a second rule).
 const composeLocked = viewPhase !== 'ready' || detailLoading || phase === 'busy' || phase === 'unknown' || storedRuleUnreadable !== undefined || (rules.length > 1 && !selectedRuleId)
 // The latest write receipt is the authoritative live configuration; the
 // stored rule view backs it up on a fresh load and carries run history.
 const live = receipt ?? ruleView
 return <main className="wk-page wk-page--std wk-career-rule">
  <header className="wk-career-rule__header">
   <p className="wk-career-rule__eyebrow">个人求职空间</p>
   <h1>持续找岗规则</h1>
   <p>持续找岗规则由你显式开启与关闭：规则默认不开启，未开启或已暂停的规则不会触发任何搜索，也不会在后台运行；每次触发都经预算准入。修改规则后，下次运行计划按新条件与新频率重新排程。</p>
  </header>
  {viewPhase === 'loading' ? <p role="status">正在打开个人求职空间…</p> : null}
  {viewPhase === 'forbidden' ? <Card bordered><div role="alert"><strong>当前空间不可访问</strong><p>{viewError?.text}</p></div><p>请切换到本人拥有的单成员个人空间后重试。服务端会验证用户与空间归属。</p><Button variant="outline" onClick={() => void load()}>重新读取</Button></Card> : null}
  {viewPhase === 'error' ? <Card bordered><div role="alert"><strong>暂时无法打开求职空间</strong><p>{viewError?.text}</p></div><Button variant="outline" onClick={() => void load()}>重新读取</Button></Card> : null}
  {viewPhase === 'scope-changed' ? <Card bordered><p>{notice}</p><Button variant="outline" onClick={() => void load()}>重新读取</Button></Card> : null}
  {viewPhase === 'ready' ? <>
   {rules.length > 1 ? <Card bordered><label htmlFor="career-rule-select">选择要查看或修改的规则</label><select id="career-rule-select" value={selectedRuleId ?? ''} onChange={(event) => {
    const ruleId = event.currentTarget.value
    const generation = ++selectionGeneration.current
    setSelectedRuleId(ruleId || undefined); setRuleView(undefined); setReceipt(undefined); setStoredRuleUnreadable(undefined)
    setDetailLoading(Boolean(ruleId))
    const requestScope = scopeController.current()
    try { if (ruleId) window.localStorage.setItem(ruleIdKey(requestScope.scope.userId, requestScope.scope.tenantId), ruleId); else window.localStorage.removeItem(ruleIdKey(requestScope.scope.userId, requestScope.scope.tenantId)) } catch { /* private mode */ }
    if (ruleId) void client.career.getRule(ruleId, requestScope.signal).then((next) => {
     if (scopeController.isCurrent(requestScope.scope) && selectionGeneration.current === generation) { setRuleView(next); setDraft({ query: next.query, interval: String(next.intervalMinutes), status: next.status }); setDetailLoading(false) }
    }).catch((cause) => {
     if (scopeController.isCurrent(requestScope.scope) && selectionGeneration.current === generation) { setStoredRuleUnreadable(errorDetails(cause)); setDetailLoading(false) }
    })
   }}><option value="">请选择规则</option>{rules.map((rule) => <option key={rule.ruleId} value={rule.ruleId}>{rule.query}（{statusLabels[rule.status]}）</option>)}</select></Card> : null}
   {storedRuleUnreadable ? <Card bordered><div role="alert"><strong>已保存规则暂时无法读取</strong><p>本地记录的规则编号无法读取（{storedRuleUnreadable.text}）。在重新读取成功前，保存会新建一条规则、可能造成重复的启用规则，因此保存已暂时停用。</p></div><Button variant="outline" onClick={() => void load()}>重新读取</Button></Card> : null}
   <CareerUsagePanel usage={usage.state} onRetry={usage.reload} />
   <Card bordered className="wk-career-rule__compose">
    <h2>规则内容</h2>
    <form onSubmit={submit}>
     <label htmlFor="career-rule-query">找岗条件（一句话描述要持续找的岗位）</label>
     <textarea id="career-rule-query" aria-label="找岗条件" value={draft.query} placeholder="例如：上海 前端开发 实习" disabled={composeLocked} onChange={(event) => { const value = event.currentTarget.value; setDraft((current) => ({ ...current, query: value })) }} rows={3} />
     <label htmlFor="career-rule-interval">触发间隔（分钟，1–43200，即最长 30 天）</label>
     <input id="career-rule-interval" aria-label="触发间隔（分钟）" type="number" min={1} max={43200} step={1} value={draft.interval} disabled={composeLocked} onChange={(event) => { const value = event.currentTarget.value; setDraft((current) => ({ ...current, interval: value })) }} />
     <fieldset className="wk-career-rule__status-field">
      <legend>规则状态（默认不开启）</legend>
      <label><input type="radio" name="career-rule-status" value="disabled" checked={draft.status === 'disabled'} disabled={composeLocked} onChange={() => setDraft((current) => ({ ...current, status: 'disabled' }))} />停用（不运行）</label>
      <label><input type="radio" name="career-rule-status" value="paused" checked={draft.status === 'paused'} disabled={composeLocked} onChange={() => setDraft((current) => ({ ...current, status: 'paused' }))} />暂停（取消下一次触发，恢复后顺延）</label>
      <label><input type="radio" name="career-rule-status" value="enabled" checked={draft.status === 'enabled'} disabled={composeLocked} onChange={() => setDraft((current) => ({ ...current, status: 'enabled' }))} />启用（按间隔自动触发）</label>
     </fieldset>
     <div className="wk-career-rule__actions">
      <Button type="submit" disabled={composeLocked || !draft.query.trim() || !intervalValid || revision === undefined || enableRequiresEstimate} loading={busy}>保存规则</Button>
     </div>
     <p className="wk-career-rule__form-note">保存会创建新规则或更新现有规则；创建时默认为停用，不会开始运行。启用、暂停、停用都由你在本页显式操作。{enableRequiresEstimate ? '启用需要先取得可用的额度预估；预估恢复前不能启用（不会先执行后补报）。' : ''}</p>
    </form>
    {notice && phase !== 'unknown' ? <p aria-live="polite" className="wk-career-rule__notice">{notice}</p> : null}
   </Card>
   {phase === 'unknown' && attempt ? <Card bordered><div role="status"><strong>保存结果暂时未知</strong><p>保留了本次规则内容。请求编号 {attempt.requestId}。请先查询持久回执；查无回执后只能用同一请求编号重试，重试不会写入第二条规则。</p><div className="wk-career-rule__actions"><Button disabled={busy} loading={busy} onClick={() => void queryReceipt()}>查询回执</Button><Button variant="outline" disabled={busy} onClick={retrySameAttempt}>用原请求编号重试</Button></div>{notice ? <p>{notice}</p> : null}</div></Card> : null}
   {error ? <Card bordered><div role="alert"><strong>{error.code === 'revision_conflict' ? '档案已更新' : '规则保存未成功'}</strong><p>{error.text}{error.currentRevision !== undefined ? `（当前修订 ${error.currentRevision}）` : ''}</p></div>
    {error.code === 'revision_conflict' ? <div className="wk-career-rule__actions"><Button disabled={busy} onClick={retryWithCurrentRevision}>按当前修订重试（原请求编号）</Button></div> : null}
   </Card> : null}
   {live ? <Card bordered className="wk-career-rule__live">
    <div className="wk-career-rule__live-head">
     <h2>当前规则</h2>
     <p>规则编号 {live.ruleId} · 修订 {live.revision}</p>
    </div>
    <section aria-label="规则状态">
     <h3>条件与频率</h3>
     <p className="wk-career-rule__statusline">「{live.query}」 · 每 {live.intervalMinutes} 分钟触发一次 · {statusLabels[live.status]}</p>
     <p className="wk-career-rule__scope-note">规则只在启用状态下触发；暂停与停用都不会执行搜索。触发时经预算准入，预算不足会形成可见的执行记录，不会被静默跳过；额度耗尽时仍可读取既有档案和申请。</p>
    </section>
    <section aria-label="下次运行计划">
     <h3>下次运行计划</h3>
     {live.status === 'enabled' && live.nextDueAt ? <p className="wk-career-rule__plan wk-career-rule__plan--scheduled">下次运行（计划）：{formatCheckTime(live.nextDueAt)}。修改规则后该计划按新频率重新排程。</p>
      : live.status === 'paused' ? <p className="wk-career-rule__plan wk-career-rule__plan--paused">已暂停：下一次触发已取消，当前没有排程。恢复启用后将按恢复时刻重新排程（顺延，不追补暂停期间的周期）。</p>
      : <p className="wk-career-rule__plan wk-career-rule__plan--off">规则未启用：不会运行，也不会在后台执行任何搜索。启用后才会排出下次运行计划。</p>}
    </section>
    <section aria-label="预计消耗">
     <h3>预计消耗（启用前确认）</h3>
     <p>每天预计触发 {formatEstimateNumber(live.estimate.triggersPerDay)} 次 · 每次触发检索 {live.estimate.sourcesPerTrigger} 个已核验来源 · 每天预计消耗 {formatEstimateNumber(live.estimate.estimatedSearchesPerDay)} 次搜索。</p>
     <p className="wk-career-rule__basis">估算口径（后端原文）：{live.estimate.basis}</p>
     <p className="wk-career-rule__scope-note">以上为后端按规则参数给出的确定性估算，前端如实展示、不重算；它不是额度余额。</p>
    </section>
   </Card> : null}
   {ruleView ? <Card bordered>
    <section aria-label="执行历史">
     <h3>执行历史</h3>
     {ruleView.runs.length ? <ul className="wk-career-rule__runs">
      {ruleView.runs.map((run: RuleRunView) => <li key={`${run.ruleId}:${run.period}`}>
       <div className="wk-career-rule__run-main">
        <span>第 {run.period} 次</span>
        <span>触发于 {formatCheckTime(run.triggeredAt)}</span>
        <span className={`wk-career-rule__run-status wk-career-rule__run-status--${run.status === 'blocked_no_quota' || run.status === 'no_vetted_sources' ? 'blocked' : run.status}`}>{runStatusLabels[run.status]}</span>
        {run.searchId ? <span>搜索编号 {run.searchId}</span> : null}
        {run.failureCode ? <span>失败代码 {run.failureCode}</span> : null}
       </div>
       {run.note ? <p className="wk-career-rule__run-note">{run.note}</p> : null}
      </li>)}
     </ul> : <p>还没有执行记录。规则启用并到达触发时间后，每次触发（包括被预算或来源拦下的触发）都会在这里留下一条可见记录。</p>}
    </section>
    <section aria-label="发现待办">
     <h3>发现待办</h3>
     {ruleView.todos.length ? <ul className="wk-career-rule__todos">
      {ruleView.todos.map((todo: RuleTodoView) => <li key={todo.todoId}>
       <a href={todo.link} target="_blank" rel="noreferrer noopener">岗位链接</a>
       <span>发现于 {formatCheckTime(todo.createdAt)}</span>
       {todo.sourceId ? <span>来源 {todo.sourceId}</span> : <span>来源未登记</span>}
      </li>)}
     </ul> : <p>暂无发现待办。</p>}
     <p className="wk-career-rule__scope-note">同一岗位链接只生成一条待办（服务端按链接去重，跨周期与跨规则不重复）；待办只作为出处展示，点击在浏览器打开。</p>
    </section>
   </Card> : null}
  </> : null}
 </main>
}
