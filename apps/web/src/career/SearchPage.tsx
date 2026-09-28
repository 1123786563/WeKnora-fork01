import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { Button, Card } from 'tdesign-react'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ScopeController } from '@weknora/domain/scope'
import type { SearchFailureCode, SearchOnceReceipt, SearchQualification, SearchUncertainty, SearchResultRow } from '../../../../packages/api-client/src/career.ts'
import type { OpportunityURLImportReceipt } from '../../../../packages/api-client/src/career.ts'
import { opportunityEvidencePath } from './OpportunityPage.tsx'
import { CareerCoveragePanel } from './reconciliation.tsx'
import { CareerUsagePanel, useCareerUsageEstimate, usageAllowsChargedRun } from './UsagePanel.tsx'
import './search.css'

// T11 one-shot search page. Every observable enum is the frozen backend
// vocabulary; nothing here creates or configures a continuous search rule
// (that is T13's separate, explicit surface).
const makeId = (): string => typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`
type Attempt = { requestId: string; query: string; expectedRevision: number }
type Phase = 'idle' | 'busy' | 'unknown' | 'terminal'
type ViewPhase = 'loading' | 'ready' | 'forbidden' | 'error' | 'scope-changed'
type HistoryEntry = { searchId: string; requestId: string; query: string; status: 'completed' | 'failed'; checkedAt: string }
type TypedError = { code?: string; currentRevision?: number; text: string }

const qualificationLabels: Record<SearchQualification, string> = { needs_review: '待人工判断', qualified: '符合', not_qualified: '不符合' }
const uncertaintyLabels: Record<SearchUncertainty, string> = { low_confidence: '低置信度' }
const failureLabels: Record<SearchFailureCode, string> = { no_vetted_sources: '暂无已核验来源，本次未抓取任何数据', all_sources_unavailable: '本次所有已核验来源都不可用' }

function errorDetails(cause: unknown): TypedError {
 const error = cause as { code?: string; currentRevision?: number; message?: string }
 return { code: error?.code, currentRevision: error?.currentRevision, text: error?.message || '请求未完成' }
}
// Ambiguous transport failures (gateway timeout, dropped connection) leave the
// durable outcome unknown exactly like outcome_unknown: recover by receipt
// under the original request ID, never by silently minting a new search.
function isUncertainOutcome(cause: unknown): boolean {
 const error = errorDetails(cause)
 if (error.code === 'TIMEOUT' || error.code === 'outcome_unknown') return true
 if (['forbidden', 'invalid_request', 'idempotency_conflict', 'revision_conflict', 'search_quota_refused', 'request_too_large', 'PAYLOAD_TOO_LARGE', 'not_found', 'unauthorized'].includes(error.code ?? '')) return false
 const status = (cause as { status?: number }).status
 if (status !== undefined) return status >= 500 || status < 400
 return true
}
function formatCheckTime(timestamp: string): string {
 return `${timestamp.slice(0, 10)} ${timestamp.slice(11, 16)} UTC`
}
function historyKey(userId: string | null, tenantId: string | null): string {
 return `weknora:career:search-history:${userId ?? ''}:${tenantId ?? ''}`
}
function readHistory(storage: Storage | undefined, key: string): HistoryEntry[] {
 if (!storage) return []
 try {
  const raw = storage.getItem(key)
  if (!raw) return []
  const parsed: unknown = JSON.parse(raw)
  if (!Array.isArray(parsed)) return []
  const entries = parsed.filter((item): item is HistoryEntry => {
   const record = item as Record<string, unknown>
   return typeof record.searchId === 'string' && record.searchId.trim().length > 0
    && typeof record.query === 'string' && (record.status === 'completed' || record.status === 'failed')
    && typeof record.checkedAt === 'string'
  })
  return entries.slice(0, 8)
 } catch { return [] }
}
function writeHistory(storage: Storage | undefined, key: string, entries: HistoryEntry[]): void {
 if (!storage) return
 try { storage.setItem(key, JSON.stringify(entries.slice(0, 8))) } catch { /* private mode: history stays in memory only */ }
}

export function CareerSearchPage({ client, scopeController }: { client: WeKnoraClient; scopeController: ScopeController }): ReactNode {
 const scope = scopeController.current().scope
 // T21: the pre-execution usage estimate is a live read on the page; a
 // charged run may only start from a live, admitting estimate.
 const usage = useCareerUsageEstimate(client, scopeController)
 const [viewPhase, setViewPhase] = useState<ViewPhase>('loading')
 const [revision, setRevision] = useState<number>()
 const [viewError, setViewError] = useState<TypedError>()
 const [draft, setDraft] = useState('')
 const [attempt, setAttempt] = useState<Attempt>()
 const [receipt, setReceipt] = useState<SearchOnceReceipt>()
 const [phase, setPhase] = useState<Phase>('idle')
 const [notice, setNotice] = useState('')
 const [error, setError] = useState<TypedError>()
 const [historyEntries, setHistoryEntries] = useState<HistoryEntry[]>([])
 const [imports, setImports] = useState<Record<string, { requestId?: string; receipt?: OpportunityURLImportReceipt; error?: string; uncertain?: boolean; busy?: boolean }>>({})
 const historyAutoLoaded = useRef(false)
 const pendingImports = useRef(new Set<string>())

 const clearForScopeChange = useCallback((message: string): void => {
  setDraft(''); setAttempt(undefined); setReceipt(undefined); setPhase('idle'); setNotice(message); setError(undefined)
  setHistoryEntries([]); setImports({}); pendingImports.current.clear()
  setRevision(undefined); setViewPhase('scope-changed'); setViewError(undefined)
 }, [])

 const load = useCallback(async (): Promise<void> => {
  const requestScope = scopeController.current()
  setViewPhase('loading'); setViewError(undefined)
  try {
   const view = await client.career.open(requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   setRevision(view.revision)
   setViewPhase('ready')
   const storage = typeof window === 'undefined' ? undefined : window.localStorage
   const entries = readHistory(storage, historyKey(requestScope.scope.userId, requestScope.scope.tenantId))
   setHistoryEntries(entries)
   if (!historyAutoLoaded.current && entries[0]) {
    historyAutoLoaded.current = true
    try {
     const stored = await client.career.search(entries[0].searchId, requestScope.signal)
     if (!scopeController.isCurrent(requestScope.scope)) return
     setReceipt(stored)
    } catch {
     // The stored search is no longer readable under this scope; drop it
     // from the visible history instead of claiming a result we cannot show.
     if (scopeController.isCurrent(requestScope.scope)) {
      const remaining = entries.slice(1)
      setHistoryEntries(remaining)
      writeHistory(storage, historyKey(requestScope.scope.userId, requestScope.scope.tenantId), remaining)
     }
    }
   }
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { setViewPhase('forbidden'); setViewError(parsed); return }
   setViewPhase('error'); setViewError(parsed)
  }
 }, [client, scopeController])

 useEffect(() => {
  historyAutoLoaded.current = false
  void load()
 }, [load, scope.userId, scope.tenantId, scope.generation])

 // Desk semantics: a space switch (or logout) aborts the live scope; cached
 // search state must not leak across identities.
 useEffect(() => {
  const activeScope = scopeController.current()
  const clear = () => clearForScopeChange('空间已切换或登录已失效，已清除本次找岗状态。')
  activeScope.signal?.addEventListener('abort', clear, { once: true })
  return () => activeScope.signal?.removeEventListener('abort', clear)
 }, [clearForScopeChange, scopeController, scope.generation])

 const acceptReceipt = useCallback((next: SearchOnceReceipt, storageKey: string): void => {
  setReceipt(next); setPhase('terminal'); setError(undefined); setNotice('')
  // A terminal charged run moved the ledger; the estimate panel must never
  // show a stale balance afterwards.
  usage.reload()
  setHistoryEntries((current) => {
   const entry: HistoryEntry = { searchId: next.searchId, requestId: next.requestId, query: next.query, status: next.status, checkedAt: next.checkedAt }
   const merged = [entry, ...current.filter((item) => item.searchId !== next.searchId)].slice(0, 8)
   writeHistory(typeof window === 'undefined' ? undefined : window.localStorage, storageKey, merged)
   return merged
  })
 }, [usage.reload])

 const send = useCallback(async (next: Attempt): Promise<void> => {
  const requestScope = scopeController.current()
  const storageKey = historyKey(requestScope.scope.userId, requestScope.scope.tenantId)
  setPhase('busy'); setError(undefined); setNotice('正在执行一次性找岗…')
  try {
   const result = await client.career.searchOnce(next, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   if (result.requestId !== next.requestId) { setError({ code: 'invalid_response', text: '服务返回的请求编号与本次找岗不匹配，已放弃本次结果。请开始一次新的找岗。' }); setPhase('idle'); setAttempt(undefined); return }
   acceptReceipt(result, storageKey)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearForScopeChange('当前空间不可访问，已清除本次找岗状态。'); return }
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
   if (parsed.code === 'search_quota_refused') { setError(parsed); setPhase('idle'); setNotice(''); usage.reload(); return }
   if (['invalid_request', 'idempotency_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
    setError(parsed); setPhase('idle'); setAttempt(undefined); setNotice(parsed.code === 'invalid_request' ? '指令未被接受，请调整后重新发起一次找岗。' : '本次请求与已保存的找岗内容不一致，已放弃；请开始一次新的找岗。')
    return
   }
   if (isUncertainOutcome(cause)) { setPhase('unknown'); setNotice(''); return }
   setError(parsed); setPhase('idle'); setNotice('')
  }
  }, [acceptReceipt, clearForScopeChange, client, scopeController, usage.reload])

 const submit = (event: FormEvent): void => {
  event.preventDefault()
  if (attempt || !draft.trim() || revision === undefined) return
  const next: Attempt = { requestId: makeId(), query: draft.trim(), expectedRevision: revision }
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
  const storageKey = historyKey(requestScope.scope.userId, requestScope.scope.tenantId)
  setPhase('busy')
  try {
   const stored = await client.career.searchReceipt(attempt.requestId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   if (stored.requestId !== attempt.requestId) { setError({ code: 'invalid_response', text: '服务返回的请求编号与本次找岗不匹配，已放弃本次结果。请开始一次新的找岗。' }); setPhase('idle'); setAttempt(undefined); setNotice(''); return }
   acceptReceipt(stored, storageKey)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearForScopeChange('当前空间不可访问，已清除本次找岗状态。'); return }
   setPhase('unknown')
   setNotice(parsed.code === 'not_found' ? '暂未找到回执；保留原指令与请求编号，可用同一请求编号重试。' : `查询回执未成功：${parsed.text}。保留原指令与请求编号。`)
  }
 }
 const startNewSearch = (): void => {
  setDraft(''); setAttempt(undefined); setReceipt(undefined); setPhase('idle'); setNotice(''); setError(undefined)
 }
 const openHistoryEntry = async (entry: HistoryEntry): Promise<void> => {
  const requestScope = scopeController.current()
  setNotice(`正在读取历史找岗 ${entry.searchId}…`)
  try {
   const stored = await client.career.search(entry.searchId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   setReceipt(stored); setNotice('')
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearForScopeChange('当前空间不可访问，已清除本次找岗状态。'); return }
   if (parsed.code === 'not_found') {
    setHistoryEntries((current) => {
     const remaining = current.filter((item) => item.searchId !== entry.searchId)
     writeHistory(typeof window === 'undefined' ? undefined : window.localStorage, historyKey(requestScope.scope.userId, requestScope.scope.tenantId), remaining)
     return remaining
    })
    setNotice('这条历史找岗在服务端已不可读，已从列表移除。')
    return
   }
   setNotice(`读取历史找岗未成功：${parsed.text}`)
  }
 }
 const importResult = async (row: SearchResultRow): Promise<void> => {
  if (pendingImports.current.has(row.resultId)) return
  // The backend claims URL imports by (tenant, user, request_id): one result
  // row keeps one request ID across retries, so an uncertain failure can
  // never mint a second evidence row for the same link.
  const requestId = imports[row.resultId]?.requestId ?? makeId()
  pendingImports.current.add(row.resultId)
  const requestScope = scopeController.current()
  setImports((current) => ({ ...current, [row.resultId]: { ...current[row.resultId], requestId, busy: true, error: undefined, uncertain: undefined } }))
  try {
   const imported = await client.career.importUrl({ requestId, url: row.link }, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   setImports((current) => ({ ...current, [row.resultId]: { ...current[row.resultId], busy: false, receipt: imported } }))
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearForScopeChange('当前空间不可访问，已清除本次找岗状态。'); return }
   if (isUncertainOutcome(cause)) {
    // Unknown durable outcome: keep the request number so the retry under it
    // can only ever reach the same evidence row, never a duplicate.
    setImports((current) => ({ ...current, [row.resultId]: { ...current[row.resultId], busy: false, uncertain: true } }))
    return
   }
   setImports((current) => ({ ...current, [row.resultId]: { ...current[row.resultId], busy: false, error: parsed.text } }))
  } finally {
   pendingImports.current.delete(row.resultId)
  }
 }

 const busy = phase === 'busy'
 return <main className="wk-page wk-page--std wk-career-search">
  <header className="wk-career-search__header">
   <p className="wk-career-search__eyebrow">个人求职空间</p>
   <h1>一次性找岗</h1>
   <p>输入一句找岗指令，系统执行一次搜索并保存结果。每次找岗都是一次性搜索，不会创建持续找岗规则，也没有任何订阅或自动执行。</p>
  </header>
  {viewPhase === 'loading' ? <p role="status">正在打开个人求职空间…</p> : null}
  {viewPhase === 'forbidden' ? <Card bordered><div role="alert"><strong>当前空间不可访问</strong><p>{viewError?.text}</p></div><p>请切换到本人拥有的单成员个人空间后重试。服务端会验证用户与空间归属。</p><Button variant="outline" onClick={() => void load()}>重新读取</Button></Card> : null}
  {viewPhase === 'error' ? <Card bordered><div role="alert"><strong>暂时无法打开求职空间</strong><p>{viewError?.text}</p></div><Button variant="outline" onClick={() => void load()}>重新读取</Button></Card> : null}
  {viewPhase === 'scope-changed' ? <Card bordered><p>{notice}</p><Button variant="outline" onClick={() => void load()}>重新读取</Button></Card> : null}
  {viewPhase === 'ready' ? <>
   <CareerUsagePanel usage={usage.state} onRetry={usage.reload} />
   <Card bordered className="wk-career-search__compose">
    <h2>找岗指令</h2>
    <form onSubmit={submit}>
     <label htmlFor="career-search-query">用一句话描述要找的岗位</label>
     <textarea id="career-search-query" aria-label="找岗指令" value={draft} placeholder="例如：上海 前端开发 实习" disabled={busy || attempt !== undefined} onChange={(event) => setDraft(event.currentTarget.value)} rows={3} />
     <div className="wk-career-search__actions">
      <Button type="submit" disabled={busy || attempt !== undefined || !draft.trim() || revision === undefined || !usageAllowsChargedRun(usage.state)} loading={busy}>{attempt && phase === 'terminal' ? '已找岗（一次性）' : '找岗（一次性）'}</Button>
      {attempt && phase === 'terminal' ? <Button variant="outline" onClick={startNewSearch}>开始新的一次找岗</Button> : null}
     </div>
    </form>
    {notice ? <p aria-live="polite" className="wk-career-search__notice">{notice}</p> : null}
   </Card>
   {phase === 'unknown' && attempt ? <Card bordered><div role="status"><strong>找岗结果暂时未知</strong><p>保留了本次指令。请求编号 {attempt.requestId}。请先查询持久回执；查无回执后只能用同一请求编号重试，重试不会创建新的搜索。</p><div className="wk-career-search__actions"><Button disabled={busy} loading={busy} onClick={() => void queryReceipt()}>查询回执</Button><Button variant="outline" disabled={busy} onClick={retrySameAttempt}>用原请求编号重试</Button></div></div></Card> : null}
   {error ? <Card bordered><div role="alert"><strong>{error.code === 'revision_conflict' ? '档案已更新' : error.code === 'search_quota_refused' ? '找岗额度受限' : '找岗未成功'}</strong><p>{error.text}{error.currentRevision !== undefined ? `（当前修订 ${error.currentRevision}）` : ''}</p></div>
    {error.code === 'revision_conflict' ? <div className="wk-career-search__actions"><Button disabled={busy} onClick={retryWithCurrentRevision}>按当前修订重试（原请求编号）</Button><Button variant="outline" disabled={busy} onClick={startNewSearch}>开始新的一次找岗</Button></div> : null}
    {error.code === 'search_quota_refused' ? <div className="wk-career-search__actions"><Button disabled={busy} onClick={retrySameAttempt}>稍后用原请求编号重试</Button><Button variant="outline" disabled={busy} onClick={startNewSearch}>开始新的一次找岗</Button></div> : null}
   </Card> : null}
   {receipt ? <Card bordered className="wk-career-search__receipt">
    <div className="wk-career-search__receipt-head">
     <h2>找岗结果</h2>
     <p>「{receipt.query}」 · {receipt.status === 'completed' ? '已完成' : '未完成'} · 检查于 {formatCheckTime(receipt.checkedAt)} · 搜索编号 {receipt.searchId}</p>
    </div>
    {receipt.status === 'failed' && receipt.failureCode ? <div role="alert"><strong>找岗未完成：{failureLabels[receipt.failureCode]}</strong><p>本次没有抓取可用的来源，也没有生成演示结果。可用同一请求编号重试恢复，不会创建新的搜索。</p></div> : null}
    <section aria-label="来源覆盖">
     <h3>来源覆盖与范围说明</h3>
     {receipt.coverage.sources.length ? <ul className="wk-career-search__coverage">
      {receipt.coverage.sources.map((source) => <li key={source.sourceId}><strong>{source.label}</strong> · {source.available ? '本次可用' : `本次不可用（${source.failureCode ?? '未提供原因'}）`}{source.cities.length ? ` · 城市：${source.cities.join('、')}` : ' · 未登记城市'}{source.accessMethods.length ? ` · 访问方式：${source.accessMethods.join('、')}` : ''}</li>)}
     </ul> : <p>暂无已核验来源：当前没有通过核验的找岗来源，本次没有抓取任何数据。</p>}
     {receipt.scopeNotes.length ? <ul className="wk-career-search__scope-notes">{receipt.scopeNotes.map((note, index) => <li key={index}>{note}</li>)}</ul> : null}
     <p className="wk-career-search__coverage-note">覆盖清单只列出实际核验过的来源；没有覆盖到的范围不会出现在结果里，也没有生成演示数据。</p>
    </section>
    <section aria-label="找岗结果">
     {receipt.results.length ? <ul className="wk-career-search__results">
      {receipt.results.map((row) => <li key={row.resultId}>
       <div className="wk-career-search__row-main">
        <a href={row.link} target="_blank" rel="noreferrer noopener">原始链接</a>
        <span>检查时间 {formatCheckTime(row.checkedAt)}</span>
        <span>资格状态 {qualificationLabels[row.qualification]}</span>
        <span>不确定性 {uncertaintyLabels[row.uncertainty]}</span>
       </div>
       <div className="wk-career-search__row-actions">
        <Button size="small" variant="outline" disabled={imports[row.resultId]?.busy} onClick={() => void importResult(row)}>{imports[row.resultId]?.uncertain ? '用原请求编号重试导入' : '导入为岗位证据'}</Button>
        {imports[row.resultId]?.receipt ? <a href={opportunityEvidencePath(imports[row.resultId]!.receipt!.opportunityId, imports[row.resultId]!.receipt!.snapshotId)}>查看岗位证据</a> : null}
        {imports[row.resultId]?.uncertain ? <span role="status">导入结果暂时未知：可稍后重试，将复用原请求编号，不会生成第二条岗位证据。</span> : null}
        {imports[row.resultId]?.error ? <span role="alert">导入未成功：{imports[row.resultId]?.error}</span> : null}
       </div>
       <p className="wk-career-search__row-note">原始链接只作为出处展示，点击在浏览器打开；导入后才会作为岗位证据保存。</p>
      </li>)}
     </ul> : <p>本次没有结果。</p>}
    </section>
   </Card> : null}
   {historyEntries.length ? <Card bordered><section aria-label="历史找岗">
    <h2>历史找岗</h2>
    <ul className="wk-career-search__history">
     {historyEntries.map((entry) => <li key={entry.searchId}><Button size="small" variant="outline" onClick={() => void openHistoryEntry(entry)}>{entry.query}</Button><small>{entry.status === 'completed' ? '已完成' : '未完成'} · {formatCheckTime(entry.checkedAt)} · {entry.searchId}</small></li>)}
    </ul>
    <p>历史按当前身份与空间保存；点击用原搜索编号重新读取，不会发起新的搜索。</p>
   </section></Card> : null}
   <CareerCoveragePanel client={client} scopeController={scopeController} />
  </> : null}
 </main>
}
