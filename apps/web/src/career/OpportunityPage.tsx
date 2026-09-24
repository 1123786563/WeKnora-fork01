import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ScopeController } from '@weknora/domain/scope'
import type { Evaluation, EvaluationReceipt, OpportunityEvidence, OpportunityImportInput, OpportunityReceipt } from '../../../../packages/career-core/src/contracts.ts'
import type { OpportunityCompleteness, OpportunityFailureCode, OpportunityObservation, OpportunitySourceStatus, OpportunityURLImportReceipt } from '../../../../packages/api-client/src/career.ts'

type Attempt = OpportunityImportInput & { opportunityId?: string; priorObservationId?: string }
type URLAttempt = { requestId: string; url: string; attemptedAt: string }
type URLImportState = 'idle' | 'busy' | 'unknown' | 'observed' | 'error'
type ImportState = 'idle' | 'busy' | 'unknown' | 'saved' | 'error' | 'forbidden' | 'scope-changed'
const newRequestId = (): string => typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`
// Frozen backend enums rendered verbatim: no frontend-invented status, completeness, or failure code ever reaches the user.
const sourceStatusLabels: Record<OpportunitySourceStatus, string> = { complete: '来源完整', partial: '内容不完整', login_required: '需要登录', blocked: '访问受限', not_found: '页面不存在', timed_out: '抓取超时', fetch_failed: '抓取失败', policy_unverified: '来源未核验' }
const completenessLabels: Record<OpportunityCompleteness, string> = { complete: '完整', incomplete: '不完整', unknown: '未知' }
const failureReasons: Record<OpportunityFailureCode, string> = { login_required: '目标站点要求登录', access_blocked: '目标站点拒绝访问', not_found: '目标页面不存在', timeout: '抓取超时', source_unverified: '该来源尚未通过核验', unsupported_content: '不支持的内容类型', empty_content: '页面没有可用正文', response_too_large: '响应超过大小上限', network_error: '网络错误', redirect_disallowed: '重定向不在允许范围内' }
function errorDetails(cause: unknown): { code?: string; requestId?: string; status?: number; message: string } {
 const error = cause as { code?: string; requestId?: string; status?: number; message?: string }
 return { code: error?.code, requestId: error?.requestId, status: error?.status, message: error?.message || '请求未完成' }
}
function isUncertainWrite(cause: unknown): boolean {
 const error = errorDetails(cause)
 if (error.code === 'TIMEOUT' || error.code === 'outcome_unknown') return true
 if (['forbidden', 'invalid_request', 'idempotency_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE', 'not_found', 'unauthorized'].includes(error.code ?? '')) return false
 if (error.status !== undefined) return error.status >= 500 || error.status < 400
 return true
}
export function opportunityEvidencePath(opportunityId: string, snapshotId: string): string {
 return `/platform/career/opportunities/${encodeURIComponent(opportunityId)}?snapshotId=${encodeURIComponent(snapshotId)}`
}
function sameReceipt(receipt: OpportunityReceipt, attempt: Attempt): boolean {
 return receipt.requestId === attempt.requestId
}
function resultPath(receipt: OpportunityReceipt): string {
 return opportunityEvidencePath(receipt.opportunityId, receipt.snapshotId)
}
export function evaluationPath(evaluationId: string): string { return `/platform/career/evaluations/${encodeURIComponent(evaluationId)}` }

export function OpportunityImportPanel({ client, scopeController }: { client: WeKnoraClient; scopeController: ScopeController }): ReactNode {
 const scope = scopeController.current()
 const [draft, setDraft] = useState('')
 const [sourceLabel, setSourceLabel] = useState('')
 const [sourceReference, setSourceReference] = useState('')
 const [attempt, setAttempt] = useState<Attempt>()
 const [receipt, setReceipt] = useState<OpportunityReceipt>()
 const [state, setState] = useState<ImportState>('idle')
 const [message, setMessage] = useState('')
 const [urlDraft, setUrlDraft] = useState('')
 const [urlAttempt, setUrlAttempt] = useState<URLAttempt>()
 const [urlReceipt, setUrlReceipt] = useState<OpportunityURLImportReceipt>()
 const [urlState, setUrlState] = useState<URLImportState>('idle')
 const [urlMessage, setUrlMessage] = useState('')
 const [observations, setObservations] = useState<OpportunityObservation[]>()
 const [evaluationState, setEvaluationState] = useState<'idle' | 'busy' | 'unknown' | 'saved' | 'error'>('idle')
 const [evaluationRequestId, setEvaluationRequestId] = useState('')
 const [evaluationReceipt, setEvaluationReceipt] = useState<EvaluationReceipt>()
 const [evaluationReceipts, setEvaluationReceipts] = useState<EvaluationReceipt[]>([])
 const [evaluationMessage, setEvaluationMessage] = useState('')
 const draftGeneration = useRef(0)
 const currentReceipt = useRef<OpportunityReceipt | undefined>(undefined)
 const evaluationInFlight = useRef<object | undefined>(undefined)

 const evaluationIsCurrent = (generation: number, fixedReceipt: OpportunityReceipt): boolean =>
  draftGeneration.current === generation && currentReceipt.current?.opportunityId === fixedReceipt.opportunityId && currentReceipt.current?.snapshotId === fixedReceipt.snapshotId

 const clearURL = useCallback((): void => {
  setUrlDraft(''); setUrlAttempt(undefined); setUrlReceipt(undefined); setUrlState('idle'); setUrlMessage(''); setObservations(undefined)
 }, [])
 const clearPrivate = useCallback((notice: string, nextState: 'forbidden' | 'scope-changed' = 'forbidden') => {
  draftGeneration.current += 1; currentReceipt.current = undefined; evaluationInFlight.current = undefined
  setDraft(''); setSourceLabel(''); setSourceReference(''); setAttempt(undefined); setReceipt(undefined); setState(nextState); setMessage(notice)
  setEvaluationReceipt(undefined); setEvaluationReceipts([]); setEvaluationRequestId(''); setEvaluationState('idle'); setEvaluationMessage('')
  clearURL()
 }, [clearURL])
 useEffect(() => {
  const activeScope = scopeController.current()
  const clear = () => clearPrivate('空间已切换或登录已失效，已清除职位描述。', 'scope-changed')
  activeScope.signal?.addEventListener('abort', clear, { once: true })
  return () => activeScope.signal?.removeEventListener('abort', clear)
 }, [clearPrivate, scopeController, scope.scope.generation])

 const acceptReceipt = (next: OpportunityReceipt, expected: Attempt): void => {
  if (!sameReceipt(next, expected)) throw new TypeError('服务返回的请求编号与本次导入不匹配')
  currentReceipt.current = next
  setReceipt(next); setState('saved'); setMessage('')
 }
 const refreshObservations = async (opportunityId: string): Promise<void> => {
  const requestScope = scopeController.current()
  try {
   const list = await client.career.opportunityObservations(opportunityId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   setObservations(list.observations)
  } catch {
   // The observation history is supplementary trace; the fixed evidence links stay authoritative.
   setObservations(undefined)
  }
 }
 const importURLAttempt = async (currentAttempt: URLAttempt): Promise<void> => {
  const requestScope = scopeController.current()
  setUrlAttempt(currentAttempt); setUrlState('busy'); setUrlMessage('正在导入链接…')
  try {
   const next = await client.career.importUrl({ requestId: currentAttempt.requestId, url: currentAttempt.url }, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   if (next.requestId !== currentAttempt.requestId) throw new TypeError('服务返回的请求编号与本次链接导入不匹配')
   setUrlReceipt(next); setUrlState('observed'); setUrlMessage('')
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问，已清除链接和职位描述。'); return }
   if (['invalid_request', 'idempotency_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
    setUrlState('error')
    setUrlAttempt(undefined)
    setUrlMessage(parsed.code === 'PAYLOAD_TOO_LARGE' || parsed.code === 'request_too_large' ? '链接请求超过服务端允许的大小，请缩短后重新导入。' : `链接未被接受：${parsed.message}`)
    return
   }
   setUrlState('unknown')
   setUrlMessage('暂时无法确认链接导入结果。请使用原请求编号重试导入以恢复。')
  }
 }
 const beginURLImport = async (): Promise<void> => {
  if (urlState === 'busy' || urlState === 'unknown' || !urlDraft.trim()) return
  await importURLAttempt({ requestId: newRequestId(), url: urlDraft.trim(), attemptedAt: new Date().toISOString() })
 }
 const retryURLImport = async (): Promise<void> => {
  if (!urlAttempt || urlState === 'busy') return
  await importURLAttempt(urlAttempt)
 }
 const onURLDraftChange = (value: string): void => {
  setUrlDraft(value)
  if (urlState === 'error') { setUrlAttempt(undefined); setUrlReceipt(undefined); setUrlState('idle'); setUrlMessage('') }
 }
 const importAttempt = async (currentAttempt: Attempt): Promise<void> => {
  const requestScope = scopeController.current()
  setAttempt(currentAttempt); setState('busy'); setMessage('正在保存职位描述…')
  try {
   const next = await client.career.importOpportunity(currentAttempt, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   acceptReceipt(next, currentAttempt)
   // A paste after a URL observation appends to the same opportunity; refresh
   // the immutable observation history so the original URL trace stays visible.
   if (currentAttempt.opportunityId) await refreshObservations(currentAttempt.opportunityId)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问，已清除职位描述。'); return }
   if (['invalid_request', 'idempotency_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
    setState('error')
    setAttempt(undefined)
    setMessage(parsed.code === 'idempotency_conflict' ? '请求编号已对应其他内容，服务器拒绝了本次提交。请检查内容后使用新的请求重新保存。' : parsed.code === 'PAYLOAD_TOO_LARGE' || parsed.code === 'request_too_large' ? '职位描述超过服务端允许的大小，请缩短后重新保存。' : `职位描述未被接受：${parsed.message}`)
    return
   }
   setState('unknown')
   setMessage(parsed.code === 'outcome_unknown' ? '服务器暂时无法确认是否已保存。请查询原请求回执，或使用同一编号安全重试。' : '网络未能确认保存结果。请查询原请求回执，或使用同一编号安全重试。')
  }
 }
 const beginImport = async (): Promise<void> => {
  if (state === 'busy' || state === 'unknown' || !draft.trim()) return
  // After a URL observation the paste joins the same opportunity as a new
  // immutable snapshot, referencing the prior URL observation by ID.
  const appendContext = urlState === 'observed' && urlReceipt ? { opportunityId: urlReceipt.opportunityId, priorObservationId: urlReceipt.observationId } : {}
  const nextAttempt: Attempt = { requestId: newRequestId(), rawText: draft, ...(sourceLabel.trim() ? { sourceLabel } : {}), ...(sourceReference.trim() ? { sourceReference } : {}), ...appendContext }
  await importAttempt(nextAttempt)
 }
 const lookupReceipt = async (): Promise<void> => {
  if (!attempt || state === 'busy') return
  const currentAttempt = attempt
  const requestScope = scopeController.current()
  setState('busy'); setMessage('正在查询原请求回执…')
  try {
   const next = await client.career.opportunityReceipt(currentAttempt.requestId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   acceptReceipt(next, currentAttempt)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问，已清除职位描述。'); return }
   setState('unknown')
   setMessage(parsed.code === 'not_found' ? '尚未找到回执。可以继续查询，或使用原请求编号重试同一份内容。' : '回执暂时无法读取。原请求编号和内容已保留，可稍后重试查询。')
  }
 }
 const onDraftChange = (value: string): void => {
  setDraft(value)
  if (state === 'error') { setAttempt(undefined); setReceipt(undefined); setState('idle'); setMessage('') }
 }
 const beginNewDraft = (): void => {
  draftGeneration.current += 1; currentReceipt.current = undefined; evaluationInFlight.current = undefined
  setDraft(''); setSourceLabel(''); setSourceReference(''); setAttempt(undefined); setReceipt(undefined); setState('idle'); setMessage(''); setEvaluationReceipt(undefined); setEvaluationReceipts([]); setEvaluationRequestId(''); setEvaluationState('idle'); setEvaluationMessage('')
  clearURL()
 }
 const runEvaluation = async (requestId: string): Promise<void> => {
  if (!receipt || evaluationInFlight.current) return
  const fixedReceipt = receipt
  const generation = draftGeneration.current
  const flight = {}
  evaluationInFlight.current = flight
  const requestScope = scopeController.current()
  setEvaluationRequestId(requestId); setEvaluationState('busy'); setEvaluationMessage('正在按此职位快照和当前已确认档案评估…')
  try {
   const next = await client.career.evaluateOpportunity({ requestId, opportunityId: fixedReceipt.opportunityId, snapshotId: fixedReceipt.snapshotId }, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope) || !evaluationIsCurrent(generation, fixedReceipt)) return
   if (next.requestId !== requestId || next.opportunityId !== fixedReceipt.opportunityId || next.snapshotId !== fixedReceipt.snapshotId) throw new TypeError('评估回执与固定职位快照不匹配')
   setEvaluationReceipt(next); setEvaluationReceipts((previous) => evaluationIsCurrent(generation, fixedReceipt) ? [...previous.filter((item) => item.evaluationId !== next.evaluationId), next] : previous); setEvaluationState('saved'); setEvaluationMessage('评估已保存，可随时重新打开此固定版本。')
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope) || !evaluationIsCurrent(generation, fixedReceipt)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此评估。'); return }
   else if (isUncertainWrite(cause)) { setEvaluationState('unknown'); setEvaluationMessage('暂时无法确认评估是否已保存。请先查询原请求回执，再决定是否使用同一编号重试。') }
   else { setEvaluationState('error'); setEvaluationMessage('评估未完成。请检查档案和职位快照后重试。') }
  } finally { if (evaluationInFlight.current === flight) evaluationInFlight.current = undefined }
 }
 const lookupEvaluationReceipt = async (): Promise<void> => {
  if (!evaluationRequestId || !receipt || evaluationInFlight.current) return
  const fixedReceipt = receipt
  const requestId = evaluationRequestId
  const generation = draftGeneration.current
  const flight = {}
  evaluationInFlight.current = flight
  const requestScope = scopeController.current()
  setEvaluationState('busy'); setEvaluationMessage('正在查询原评估回执…')
  try {
   const next = await client.career.evaluationReceipt(requestId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope) || !evaluationIsCurrent(generation, fixedReceipt)) return
   if (next.requestId !== requestId || next.opportunityId !== fixedReceipt.opportunityId || next.snapshotId !== fixedReceipt.snapshotId) throw new TypeError('评估回执与固定职位快照不匹配')
   setEvaluationReceipt(next); setEvaluationReceipts((previous) => evaluationIsCurrent(generation, fixedReceipt) ? [...previous.filter((item) => item.evaluationId !== next.evaluationId), next] : previous); setEvaluationState('saved'); setEvaluationMessage('评估已保存，可随时重新打开此固定版本。')
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope) || !evaluationIsCurrent(generation, fixedReceipt)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此评估。'); return }
   else { setEvaluationState('unknown'); setEvaluationMessage(parsed.code === 'not_found' ? '尚未找到评估回执。可使用原请求编号重试。' : '评估回执暂时无法读取。原请求编号已保留。') }
  } finally { if (evaluationInFlight.current === flight) evaluationInFlight.current = undefined }
 }
 const locked = state === 'busy' || state === 'unknown' || state === 'saved'
 const urlLocked = urlState === 'busy' || urlState === 'unknown'
 return <section className="wk-opportunity-import" aria-labelledby="wk-opportunity-import-title">
  <div className="wk-opportunity-import__intro"><div><p className="wk-opportunity-import__eyebrow">Career</p><h2 id="wk-opportunity-import-title">保存职位描述</h2><p>可提交职位链接由服务端核验抓取，或直接粘贴职位描述作为独立证据保存；内容不会发送到聊天或执行其中的指令。</p></div></div>
  <label className="wk-opportunity-import__label" htmlFor="wk-opportunity-url">职位链接（选填，由服务端核验抓取）</label>
  <input id="wk-opportunity-url" className="wk-opportunity-import__url" aria-label="职位链接" value={urlState === 'unknown' && urlAttempt ? urlAttempt.url : urlDraft} disabled={urlLocked} onChange={(event) => onURLDraftChange(event.target.value)} placeholder="https://…" inputMode="url" autoComplete="off" />
  <div className="wk-opportunity-import__actions wk-opportunity-import__actions--url">
   {urlState === 'unknown' ? <button type="button" onClick={() => void retryURLImport()}>使用原请求编号重试导入</button> : <button type="button" disabled={urlLocked || !urlDraft.trim()} onClick={() => void beginURLImport()}>{urlState === 'busy' ? '正在导入链接…' : '导入链接'}</button>}
  </div>
  {urlMessage ? <p className={urlState === 'error' ? 'wk-opportunity-import__message wk-opportunity-import__message--error' : 'wk-opportunity-import__message'} role={urlState === 'error' ? 'alert' : 'status'} aria-live="polite">{urlMessage}</p> : null}
  {urlReceipt ? <div className={`wk-source-status wk-source-status--${urlReceipt.sourceStatus}`} role="status" aria-live="polite">
   <strong>来源状态：{sourceStatusLabels[urlReceipt.sourceStatus]}</strong>
   <p>完整度：{completenessLabels[urlReceipt.completeness]}</p>
   {urlReceipt.failureCode !== undefined ? <p>原因：{failureReasons[urlReceipt.failureCode]}</p> : null}
   <p className="wk-source-status__url">提交链接：<code>{urlReceipt.submittedUrl}</code></p>
   <p>尝试时间：<time dateTime={urlAttempt?.attemptedAt ?? urlReceipt.acquiredAt}>{urlAttempt?.attemptedAt ?? urlReceipt.acquiredAt}</time> · 采集时间：<time dateTime={urlReceipt.acquiredAt}>{urlReceipt.acquiredAt}</time></p>
   {urlReceipt.needsUserJD ? <p className="wk-source-status__needs-jd">需用户补充 JD</p> : null}
   <p><a href={opportunityEvidencePath(urlReceipt.opportunityId, urlReceipt.snapshotId)}>查看来源观察证据</a></p>
  </div> : null}
  {observations && urlReceipt ? <section className="wk-source-history" aria-label="来源观察历史"><h3>来源观察历史</h3><ul>{observations.map((observation) => <li key={observation.observationId}><a href={opportunityEvidencePath(urlReceipt.opportunityId, observation.snapshotId)}>{observation.source.kind === 'url' ? `来源观察（${observation.sourceStatus !== undefined ? sourceStatusLabels[observation.sourceStatus] : '链接'}）` : '来源观察（手工粘贴）'}</a> <time dateTime={observation.acquiredAt}>{observation.acquiredAt}</time></li>)}</ul><p>历史观察不可变：补充粘贴产生新快照，原链接观察保持原样。</p></section> : null}
  <label className="wk-opportunity-import__label" htmlFor="wk-opportunity-raw-text">职位描述</label>
  <textarea id="wk-opportunity-raw-text" aria-label="职位描述" rows={5} value={state === 'unknown' && attempt ? attempt.rawText : draft} disabled={locked || state === 'forbidden'} onChange={(event) => onDraftChange(event.target.value)} placeholder={urlReceipt?.needsUserJD ? '来源不完整，请粘贴完整职位描述补充…' : '粘贴完整的职位描述…'} />
  <div className="wk-opportunity-import__metadata">
   <label>来源名称（选填）<input aria-label="来源名称" value={sourceLabel} disabled={locked || state === 'forbidden'} onChange={(event) => setSourceLabel(event.target.value)} /></label>
   <label>来源链接或编号（仅记录，不会访问）<input aria-label="来源链接或编号" value={sourceReference} disabled={locked || state === 'forbidden'} onChange={(event) => setSourceReference(event.target.value)} /></label>
  </div>
  <div className="wk-opportunity-import__actions">
   {state === 'unknown' ? <><button type="button" onClick={() => void lookupReceipt()}>查询导入回执</button><button type="button" onClick={() => attempt && void importAttempt(attempt)}>使用原请求编号重试</button></> : state === 'saved' ? <><button type="button" disabled>已保存</button><button type="button" onClick={beginNewDraft}>开始新草稿</button></> : <button type="button" disabled={state === 'busy' || state === 'forbidden' || !draft.trim()} onClick={() => void beginImport()}>{state === 'busy' ? '正在保存…' : '保存 JD'}</button>}
  </div>
  {message ? <p className={state === 'error' || state === 'forbidden' ? 'wk-opportunity-import__message wk-opportunity-import__message--error' : 'wk-opportunity-import__message'} role={state === 'error' || state === 'forbidden' ? 'alert' : 'status'} aria-live="polite">{message}</p> : null}
  {receipt ? <div className="wk-opportunity-import__result" aria-live="polite"><strong>{receipt.status === 'needs_review' ? '已保存，待确认' : '已保存'}</strong><p>请求编号：<code>{receipt.requestId}</code></p>{evaluationReceipt ? <div className={`wk-evaluation-status wk-evaluation-status--${evaluationReceipt.status}`} role={evaluationReceipt.status === 'ineligible' ? 'alert' : 'status'}><strong>资格判断：{statusLabel(evaluationReceipt.status)}</strong>{evaluationReceipt.status === 'ineligible' ? <p>存在明确的硬性条件冲突。请先查看原因和证据。</p> : null}</div> : null}<p><a href={resultPath(receipt)}>查看已保存的 JD 证据</a></p>{evaluationReceipts.length ? <ul aria-label="已保存的评估">{evaluationReceipts.map((item) => <li key={item.evaluationId}><a href={evaluationPath(item.evaluationId)}>查看评估结果（档案修订 {item.profileRevision}）</a></li>)}</ul> : null}<div className="wk-opportunity-import__actions"><button type="button" disabled={evaluationState === 'busy'} onClick={() => evaluationState === 'unknown' ? void lookupEvaluationReceipt() : void runEvaluation(evaluationState === 'saved' || evaluationState === 'error' ? newRequestId() : evaluationRequestId || newRequestId())}>{evaluationState === 'busy' ? '正在评估…' : evaluationState === 'unknown' ? '查询评估回执' : evaluationState === 'error' ? '重试评估' : evaluationState === 'saved' ? '重新评估当前档案' : '评估此 JD'}</button>{evaluationState === 'unknown' ? <button type="button" onClick={() => void runEvaluation(evaluationRequestId)}>使用原请求编号重试</button> : null}</div>{evaluationMessage ? <p role={evaluationState === 'error' ? 'alert' : 'status'} aria-live="polite">{evaluationMessage}</p> : null}</div> : null}
 </section>
}

const statusLabel = (status: Evaluation['status']): string => status === 'ineligible' ? '不符合' : status === 'eligible' ? '符合已识别条件' : '待确认'
const unknownReason = (reason: string): string => ['graduation_year_missing', 'confirmed_graduation_year_missing'].includes(reason) ? '缺少已确认的毕业届别资料。' : ['graduation_year_ambiguous', 'graduation_fact_ambiguous'].includes(reason) ? '档案中的毕业届别信息存在冲突，需要确认。' : reason === 'graduation_requirement_invalid' ? '职位描述中的毕业届别条件无法可靠解析，需要人工核对。' : '此项招聘条件尚未能从职位描述或档案中确认。'
const factAnchor = (fact: Evaluation['facts'][number]): string => `fact-${fact.factKey.replace(/[^a-zA-Z0-9_-]/g, '-')}-${fact.factRevision}`

export function EvaluationAction({ client, scopeController, opportunityId, snapshotId, initialEvaluation }: { client: WeKnoraClient; scopeController: ScopeController; opportunityId: string; snapshotId: string; initialEvaluation?: Evaluation }): ReactNode {
 const [state, setState] = useState<'idle' | 'busy' | 'unknown' | 'error' | 'forbidden' | 'scope-changed'>('idle')
 const [requestId, setRequestId] = useState('')
 const [message, setMessage] = useState('')
 const [latest, setLatest] = useState<EvaluationReceipt>()
 const [history, setHistory] = useState<EvaluationReceipt[]>(initialEvaluation ? [initialEvaluation] : [])
 const active = useRef(false)
 const scope = scopeController.current()
 useEffect(() => {
  const requestScope = scopeController.current()
  const clear = () => { setLatest(undefined); setHistory([]); setState('scope-changed'); setMessage('空间已切换或登录已失效，已清除评估结果。') }
  requestScope.signal?.addEventListener('abort', clear, { once: true })
  return () => requestScope.signal?.removeEventListener('abort', clear)
 }, [scopeController, scope.scope.generation])
 const accept = (next: EvaluationReceipt, expectedRequestId: string): void => {
  if (next.requestId !== expectedRequestId || next.opportunityId !== opportunityId || next.snapshotId !== snapshotId) throw new TypeError('评估回执与固定职位快照不匹配')
  setLatest(next); setHistory((previous) => [...previous.filter((item) => item.evaluationId !== next.evaluationId), next]); setState('idle'); setMessage('评估已保存。旧评估仍保留原档案版本。')
 }
 const evaluate = async (id = newRequestId()): Promise<void> => {
  if (active.current || state === 'forbidden' || state === 'scope-changed') return
  active.current = true; setRequestId(id); setState('busy'); setMessage('正在使用当前已确认档案评估此固定职位快照…')
  const requestScope = scopeController.current()
  try {
   const next = await client.career.evaluateOpportunity({ requestId: id, opportunityId, snapshotId }, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   accept(next, id)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { setLatest(undefined); setHistory([]); setState('forbidden'); setMessage('当前空间不可访问此职位，已清除评估结果。') }
   else if (isUncertainWrite(cause)) { setState('unknown'); setMessage('暂时无法确认评估是否已保存。请先查询原请求回执，再决定是否使用同一编号重试。') }
   else { setState('error'); setMessage('评估未完成。请检查当前档案后重试。') }
  } finally { active.current = false }
 }
 const findReceipt = async (): Promise<void> => {
  if (!requestId || active.current) return
  active.current = true; setState('busy'); setMessage('正在查询原评估回执…')
  const requestScope = scopeController.current()
  try {
   const next = await client.career.evaluationReceipt(requestId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   accept(next, requestId)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { setLatest(undefined); setHistory([]); setState('forbidden'); setMessage('当前空间不可访问此职位，已清除评估结果。') }
   else { setState('unknown'); setMessage(parsed.code === 'not_found' ? '尚未找到回执，可以继续使用原请求编号重试。' : '回执暂时无法读取，可以稍后重试查询。') }
  } finally { active.current = false }
 }
 const ineligible = latest?.status === 'ineligible'
 return <section className="wk-evaluation-action" aria-labelledby="wk-evaluation-action-title">
  <h2 id="wk-evaluation-action-title">使用当前档案评估</h2>
  <p>评估会引用固定快照 <code>{snapshotId}</code> 和当前已确认档案。已有评估会保留。</p>
  {latest ? <div className={`wk-evaluation-status wk-evaluation-status--${latest.status}`} role={ineligible ? 'alert' : 'status'}><strong>当前评估资格：{statusLabel(latest.status)}</strong>{ineligible ? <p>存在明确的硬性条件冲突。请查看评估中的具体依据。</p> : null}<p><a href={evaluationPath(latest.evaluationId)}>查看新评估结果</a> · 档案修订 {latest.profileRevision}</p></div> : null}
  {history.length > 0 ? <ul aria-label="历史评估">{history.map((item) => <li key={item.evaluationId}><a href={evaluationPath(item.evaluationId)}>查看评估结果（档案修订 {item.profileRevision}，{statusLabel(item.status)}）</a></li>)}</ul> : null}
  {state === 'forbidden' || state === 'scope-changed' ? <p role="alert">{message}</p> : null}
  {state === 'busy' || state === 'unknown' || state === 'error' || state === 'idle' ? <div className="wk-opportunity-import__actions"><button type="button" disabled={state === 'busy'} onClick={() => state === 'unknown' ? void findReceipt() : void evaluate()}>{state === 'busy' ? '正在评估…' : state === 'unknown' ? '查询评估回执' : state === 'error' ? '重试评估' : latest ? '重新评估当前档案' : '使用当前档案重新评估'}</button>{state === 'unknown' ? <button type="button" onClick={() => void evaluate(requestId)}>使用原请求编号重试</button> : null}</div> : null}
  {message && state !== 'forbidden' && state !== 'scope-changed' ? <p role={state === 'error' ? 'alert' : 'status'} aria-live="polite">{message}</p> : null}
 </section>
}

export function EvaluationDetailPage({ client, scopeController, evaluationId }: { client: WeKnoraClient; scopeController: ScopeController; evaluationId: string }): ReactNode {
 const scope = scopeController.current()
 const [evaluation, setEvaluation] = useState<Evaluation>()
 const [state, setState] = useState<'loading' | 'ready' | 'error' | 'forbidden' | 'invalid' | 'scope-changed'>('loading')
 const [reload, setReload] = useState(0)
 const scopeChanged = useRef(false)
 useEffect(() => {
  const requestScope = scopeController.current(); let active = true
  if (scopeChanged.current) { scopeChanged.current = false; return () => { active = false } }
  const clear = () => { active = false; scopeChanged.current = true; setEvaluation(undefined); setState('scope-changed') }
  setEvaluation(undefined)
  if (!evaluationId.trim()) { setState('invalid'); return () => { active = false } }
  setState('loading'); requestScope.signal?.addEventListener('abort', clear, { once: true })
  void client.career.evaluation(evaluationId, requestScope.signal).then((next) => {
   if (active && scopeController.isCurrent(requestScope.scope) && next.evaluationId === evaluationId) { setEvaluation(next); setState('ready') }
   else if (active && scopeController.isCurrent(requestScope.scope)) setState('error')
  }).catch((cause: unknown) => { if (!active || !scopeController.isCurrent(requestScope.scope)) return; setEvaluation(undefined); setState(errorDetails(cause).code === 'forbidden' ? 'forbidden' : 'error') })
  return () => { active = false; requestScope.signal?.removeEventListener('abort', clear) }
 }, [client, evaluationId, reload, scopeController, scope.scope.generation])
 const retry = (): void => setReload((value) => value + 1)
 const currentEvaluation = evaluation?.evaluationId === evaluationId ? evaluation : undefined
 const visibleState = evaluation && !currentEvaluation ? 'loading' : state
 if (!currentEvaluation) {
  const title = visibleState === 'loading' ? '正在读取固定评估…' : visibleState === 'forbidden' ? '当前空间不可访问' : visibleState === 'invalid' ? '评估链接无效' : visibleState === 'scope-changed' ? '空间已切换，已清除评估内容' : '无法读取评估结果'
  return <main className="wk-page wk-opportunity-evidence"><header className="wk-header"><div><p className="wk-opportunity-import__eyebrow">Career · 固定评估</p><h1>{title}</h1></div></header><section className="wk-opportunity-evidence__state" role={visibleState === 'error' || visibleState === 'forbidden' || visibleState === 'invalid' ? 'alert' : 'status'} aria-busy={visibleState === 'loading' || undefined}><p>{visibleState === 'loading' ? '正在按固定评估编号读取历史结果。' : visibleState === 'forbidden' ? '此评估不属于当前可访问的空间。' : visibleState === 'invalid' ? '评估编号缺失，请从评估结果链接进入。' : visibleState === 'scope-changed' ? '请在当前空间重新打开评估链接。' : '请检查网络或服务响应后重试。'}</p>{visibleState === 'error' ? <button type="button" onClick={retry}>重试</button> : null}</section></main>
 }
 const snapshotPath = opportunityEvidencePath(currentEvaluation.opportunityId, currentEvaluation.snapshotId)
 const facts = new Map(currentEvaluation.facts.map((fact) => [`${fact.factKey}:${fact.factRevision}`, fact]))
 return <main className="wk-page wk-opportunity-evidence wk-evaluation-detail">
  <header className="wk-header"><div><p className="wk-opportunity-import__eyebrow">Career · 固定评估</p><h1>岗位评估：{statusLabel(currentEvaluation.hard.overall)}</h1><p>硬性资格判断优先显示；后续档案或 JD 变化不会改写此评估。</p></div><a href={snapshotPath}>查看岗位快照</a></header>
  <section className={`wk-evaluation-detail__verdict wk-evaluation-detail__verdict--${currentEvaluation.hard.overall}`} aria-labelledby="wk-evaluation-verdict-title"><h2 id="wk-evaluation-verdict-title">资格判断：{statusLabel(currentEvaluation.hard.overall)}</h2><p>档案修订 {currentEvaluation.profileRevision} · 规则版本 {currentEvaluation.rulesetVersion}</p><p><time dateTime={currentEvaluation.createdAt}>{currentEvaluation.createdAt}</time> · 快照 <code>{currentEvaluation.snapshotId}</code></p></section>
  <EvaluationAction key={JSON.stringify([currentEvaluation.opportunityId, currentEvaluation.snapshotId, currentEvaluation.evaluationId])} client={client} scopeController={scopeController} opportunityId={currentEvaluation.opportunityId} snapshotId={currentEvaluation.snapshotId} initialEvaluation={currentEvaluation} />
  <section className="wk-evaluation-detail__hard" aria-labelledby="wk-evaluation-hard-title"><h2 id="wk-evaluation-hard-title">硬性资格判断</h2><ol>{currentEvaluation.hard.rules.map((rule, index) => {
   const fact = rule.profileEvidence ? facts.get(`${rule.profileEvidence.factKey}:${rule.profileEvidence.factRevision}`) : undefined
   return <li key={`${rule.ruleId}-${index}`} className={`wk-evaluation-detail__rule wk-evaluation-detail__rule--${rule.outcome}`}><h3>{rule.criterion}：{statusLabel(rule.outcome)}</h3><p>{rule.outcome === 'unknown' ? unknownReason(rule.reasonCode) : rule.reasonCode === 'graduation_year_mismatch' ? '已确认的毕业届别与岗位明确要求不符。' : rule.criterion}</p>{rule.jobEvidence ? <p>职位依据：<a href={`#job-evidence-${index}`}>“{rule.jobEvidence.quotedText}”</a>（JD 字符位置 {rule.jobEvidence.spanStart}–{rule.jobEvidence.spanEnd}）</p> : <p>职位依据：尚未找到可确认的条件，请查看固定 JD。</p>}{fact ? <p>档案依据：<a href={`#${factAnchor(fact)}`}>{fact.factKey} = {fact.value}（档案修订 {fact.revision}，事实版本 {fact.factRevision}）</a></p> : <p>档案依据：没有可用的已确认事实。</p>}</li>
  })}</ol></section>
  <section className="wk-evaluation-detail__soft" aria-labelledby="wk-evaluation-soft-title"><h2 id="wk-evaluation-soft-title">技能、项目与意向匹配</h2>{currentEvaluation.soft.matches.length ? <ul>{currentEvaluation.soft.matches.map((match, index) => { const fact = facts.get(`${match.profileEvidence.factKey}:${match.profileEvidence.factRevision}`)!; return <li key={`${match.kind}-${index}`}><h3>{match.kind === 'skill' ? '技能' : match.kind === 'project' ? '项目' : '意向'}：{match.value}</h3><p>职位依据：<a href={`#soft-evidence-${index}`}>“{match.jobEvidence.quotedText}”</a>（JD 字符位置 {match.jobEvidence.spanStart}–{match.jobEvidence.spanEnd}）</p><p>已确认档案依据：<a href={`#${factAnchor(fact)}`}>{fact.factKey} = {fact.value}（事实版本 {fact.factRevision}）</a></p></li> })}</ul> : <p>当前没有可引用的软匹配证据。</p>}</section>
  <section className="wk-evaluation-detail__snapshot" aria-labelledby="wk-evaluation-jd-title"><h2 id="wk-evaluation-jd-title">固定职位描述</h2><p>岗位快照：<a href={snapshotPath}><code>{currentEvaluation.snapshot.snapshotId}</code></a></p><div className="wk-evaluation-detail__citations" aria-label="职位描述引用"><h3>被引用的职位原文</h3>{currentEvaluation.hard.rules.flatMap((rule, index) => rule.jobEvidence ? [<blockquote id={`job-evidence-${index}`} key={`hard-${index}`}>{rule.jobEvidence.quotedText}<small>硬性条件 · 字符位置 {rule.jobEvidence.spanStart}–{rule.jobEvidence.spanEnd}</small></blockquote>] : []).concat(currentEvaluation.soft.matches.map((match, index) => <blockquote id={`soft-evidence-${index}`} key={`soft-${index}`}>{match.jobEvidence.quotedText}<small>匹配依据 · 字符位置 {match.jobEvidence.spanStart}–{match.jobEvidence.spanEnd}</small></blockquote>))}</div><pre>{currentEvaluation.snapshot.rawText}</pre></section>
  <section className="wk-evaluation-detail__facts" aria-labelledby="wk-evaluation-facts-title"><h2 id="wk-evaluation-facts-title">本次评估引用的已确认档案版本</h2><ul>{currentEvaluation.facts.map((fact) => <li id={factAnchor(fact)} key={factAnchor(fact)}><strong>{fact.factKey}</strong>: {fact.value} · 档案修订 {fact.revision} · 事实版本 {fact.factRevision} · 确认于 <time dateTime={fact.confirmedAt}>{fact.confirmedAt}</time></li>)}</ul></section>
 </main>
}

function fieldValue(value: OpportunityEvidence['extracted'][keyof OpportunityEvidence['extracted']]): string {
 return value.state === 'known' ? value.value : '未知'
}
export function OpportunityEvidencePage({ client, scopeController, opportunityId, snapshotId }: { client: WeKnoraClient; scopeController: ScopeController; opportunityId: string; snapshotId: string }): ReactNode {
 const scope = scopeController.current()
 const [evidence, setEvidence] = useState<OpportunityEvidence>()
 const [state, setState] = useState<'loading' | 'ready' | 'error' | 'forbidden' | 'invalid' | 'scope-changed'>('loading')
 const [reload, setReload] = useState(0)
 useEffect(() => {
  const requestScope = scopeController.current()
  let active = true
  const clearForScopeChange = () => { active = false; setEvidence(undefined); setState('scope-changed') }
  setEvidence(undefined)
  if (!opportunityId.trim() || !snapshotId.trim()) { setState('invalid'); return () => { active = false } }
  setState('loading')
  requestScope.signal?.addEventListener('abort', clearForScopeChange, { once: true })
  void client.career.opportunityEvidence(opportunityId, snapshotId, requestScope.signal).then((next) => {
   if (active && scopeController.isCurrent(requestScope.scope) && next.opportunityId === opportunityId && next.snapshotId === snapshotId) { setEvidence(next); setState('ready') }
   else if (active && scopeController.isCurrent(requestScope.scope)) setState('error')
  }).catch((cause: unknown) => {
   if (!active || !scopeController.isCurrent(requestScope.scope)) return
   setEvidence(undefined); setState(errorDetails(cause).code === 'forbidden' ? 'forbidden' : 'error')
  })
  return () => { active = false; requestScope.signal?.removeEventListener('abort', clearForScopeChange) }
 }, [client, opportunityId, reload, scopeController, scope.scope.generation, snapshotId])
 const retry = (): void => setReload((value) => value + 1)
 const title: Record<typeof state, string> = { loading: '正在读取已保存的职位证据…', ready: '职位证据', error: '无法读取职位证据', forbidden: '当前空间不可访问', invalid: '链接缺少有效的快照编号', 'scope-changed': '空间已切换，已清除职位描述' }
 const currentEvidence = evidence?.opportunityId === opportunityId && evidence.snapshotId === snapshotId ? evidence : undefined
 const visibleState = evidence && !currentEvidence ? 'loading' : state
 if (!currentEvidence) return <main className="wk-page wk-opportunity-evidence"><header className="wk-header"><div><p className="wk-opportunity-import__eyebrow">Career</p><h1>{title[visibleState]}</h1></div></header><section className="wk-opportunity-evidence__state" role={visibleState === 'error' || visibleState === 'forbidden' || visibleState === 'invalid' ? 'alert' : 'status'} aria-busy={visibleState === 'loading' || undefined}><p>{visibleState === 'loading' ? '正在按职位和快照编号读取固定记录。' : visibleState === 'error' ? '请检查网络后重试。' : visibleState === 'forbidden' ? '此记录不属于当前可访问的空间。' : visibleState === 'invalid' ? '请从导入结果打开完整的证据链接。' : '请重新打开证据链接以读取当前空间中的记录。'}</p>{visibleState === 'error' ? <button type="button" onClick={retry}>重试</button> : null}</section></main>
 const fields: Array<[string, string]> = [['职位名称', fieldValue(currentEvidence.extracted.title)], ['公司', fieldValue(currentEvidence.extracted.company)], ['地点', fieldValue(currentEvidence.extracted.location)], ['招聘批次', fieldValue(currentEvidence.extracted.batch)], ['要求', fieldValue(currentEvidence.extracted.requirements)]]
 const source = [currentEvidence.source.label, currentEvidence.source.referenceId].filter((value): value is string => Boolean(value)).join(' · ') || currentEvidence.source.kind
 return <main className="wk-page wk-opportunity-evidence"><header className="wk-header"><div><p className="wk-opportunity-import__eyebrow">Career · 职位证据</p><h1>{currentEvidence.status === 'needs_review' ? '已保存，待确认' : '已保存的职位证据'}</h1><p>此页面显示固定快照的原始内容和当前可确认的信息。</p></div><a href="/platform/creatChat">返回对话</a></header>
  <section className="wk-opportunity-evidence__meta" aria-label="来源信息"><dl><div><dt>来源</dt><dd>{source}</dd></div><div><dt>采集时间</dt><dd><time dateTime={currentEvidence.acquiredAt}>{currentEvidence.acquiredAt}</time></dd></div><div><dt>状态</dt><dd>{currentEvidence.status === 'needs_review' ? '待确认' : '已保存'}</dd></div><div><dt>快照编号</dt><dd><code>{currentEvidence.snapshotId}</code></dd></div></dl></section>
  <EvaluationAction key={JSON.stringify([currentEvidence.opportunityId, currentEvidence.snapshotId])} client={client} scopeController={scopeController} opportunityId={currentEvidence.opportunityId} snapshotId={currentEvidence.snapshotId} />
  {currentEvidence.status === 'needs_review' ? <p className="wk-opportunity-evidence__notice" role="status">职位描述已保存为证据，提取字段仍需核对。</p> : null}
  <section className="wk-opportunity-evidence__fields" aria-labelledby="wk-opportunity-fields-title"><h2 id="wk-opportunity-fields-title">提取字段</h2><dl>{fields.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl></section>
  <section className="wk-opportunity-evidence__raw" aria-labelledby="wk-opportunity-raw-title"><h2 id="wk-opportunity-raw-title">原始职位描述</h2><pre>{currentEvidence.rawText}</pre></section>
 </main>
}
