import { useCallback, useEffect, useState, type ReactNode } from 'react'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ScopeController } from '@weknora/domain/scope'
import type { OpportunityEvidence, OpportunityImportInput, OpportunityReceipt } from '../../../../packages/career-core/src/contracts.ts'

type Attempt = OpportunityImportInput
type ImportState = 'idle' | 'busy' | 'unknown' | 'saved' | 'error' | 'forbidden' | 'scope-changed'
const newRequestId = (): string => typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`
function errorDetails(cause: unknown): { code?: string; requestId?: string; message: string } {
 const error = cause as { code?: string; requestId?: string; message?: string }
 return { code: error?.code, requestId: error?.requestId, message: error?.message || '请求未完成' }
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

export function OpportunityImportPanel({ client, scopeController }: { client: WeKnoraClient; scopeController: ScopeController }): ReactNode {
 const scope = scopeController.current()
 const [draft, setDraft] = useState('')
 const [sourceLabel, setSourceLabel] = useState('')
 const [sourceReference, setSourceReference] = useState('')
 const [attempt, setAttempt] = useState<Attempt>()
 const [receipt, setReceipt] = useState<OpportunityReceipt>()
 const [state, setState] = useState<ImportState>('idle')
 const [message, setMessage] = useState('')

 const clearPrivate = useCallback((notice: string, nextState: 'forbidden' | 'scope-changed' = 'forbidden') => {
  setDraft(''); setSourceLabel(''); setSourceReference(''); setAttempt(undefined); setReceipt(undefined); setState(nextState); setMessage(notice)
 }, [])
 useEffect(() => {
  const activeScope = scopeController.current()
  const clear = () => clearPrivate('空间已切换或登录已失效，已清除职位描述。', 'scope-changed')
  activeScope.signal?.addEventListener('abort', clear, { once: true })
  return () => activeScope.signal?.removeEventListener('abort', clear)
 }, [clearPrivate, scopeController, scope.scope.generation])

 const acceptReceipt = (next: OpportunityReceipt, expected: Attempt): void => {
  if (!sameReceipt(next, expected)) throw new TypeError('服务返回的请求编号与本次导入不匹配')
  setReceipt(next); setState('saved'); setMessage('')
 }
 const importAttempt = async (currentAttempt: Attempt): Promise<void> => {
  const requestScope = scopeController.current()
  setAttempt(currentAttempt); setState('busy'); setMessage('正在保存职位描述…')
  try {
   const next = await client.career.importOpportunity(currentAttempt, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   acceptReceipt(next, currentAttempt)
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
  const nextAttempt: Attempt = { requestId: newRequestId(), rawText: draft, ...(sourceLabel.trim() ? { sourceLabel } : {}), ...(sourceReference.trim() ? { sourceReference } : {}) }
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
  if (state === 'saved' || state === 'error') { setAttempt(undefined); setReceipt(undefined); setState('idle'); setMessage('') }
 }
 const locked = state === 'busy' || state === 'unknown'
 return <section className="wk-opportunity-import" aria-labelledby="wk-opportunity-import-title">
  <div className="wk-opportunity-import__intro"><div><p className="wk-opportunity-import__eyebrow">Career</p><h2 id="wk-opportunity-import-title">保存职位描述</h2><p>粘贴职位描述作为独立证据保存，不会发送到聊天或执行其中的指令。</p></div></div>
  <label className="wk-opportunity-import__label" htmlFor="wk-opportunity-raw-text">职位描述</label>
  <textarea id="wk-opportunity-raw-text" aria-label="职位描述" rows={5} value={state === 'unknown' && attempt ? attempt.rawText : draft} disabled={locked || state === 'forbidden'} onChange={(event) => onDraftChange(event.target.value)} placeholder="粘贴完整的职位描述…" />
  <div className="wk-opportunity-import__metadata">
   <label>来源名称（选填）<input aria-label="来源名称" value={sourceLabel} disabled={locked || state === 'forbidden'} onChange={(event) => setSourceLabel(event.target.value)} /></label>
   <label>来源链接或编号（仅记录，不会访问）<input aria-label="来源链接或编号" value={sourceReference} disabled={locked || state === 'forbidden'} onChange={(event) => setSourceReference(event.target.value)} /></label>
  </div>
  <div className="wk-opportunity-import__actions">
   {state === 'unknown' ? <><button type="button" onClick={() => void lookupReceipt()}>查询导入回执</button><button type="button" onClick={() => attempt && void importAttempt(attempt)}>使用原请求编号重试</button></> : <button type="button" disabled={state === 'busy' || state === 'forbidden' || !draft.trim()} onClick={() => void beginImport()}>{state === 'busy' ? '正在保存…' : '保存 JD'}</button>}
  </div>
  {message ? <p className={state === 'error' || state === 'forbidden' ? 'wk-opportunity-import__message wk-opportunity-import__message--error' : 'wk-opportunity-import__message'} role={state === 'error' || state === 'forbidden' ? 'alert' : 'status'} aria-live="polite">{message}</p> : null}
  {receipt ? <div className="wk-opportunity-import__result" role="status" aria-live="polite"><strong>{receipt.status === 'needs_review' ? '已保存，待确认' : '已保存'}</strong><p>请求编号：<code>{receipt.requestId}</code></p><a href={resultPath(receipt)}>查看已保存的 JD 证据</a></div> : null}
 </section>
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
 if (!evidence) return <main className="wk-page wk-opportunity-evidence"><header className="wk-header"><div><p className="wk-opportunity-import__eyebrow">Career</p><h1>{title[state]}</h1></div></header><section className="wk-opportunity-evidence__state" role={state === 'error' || state === 'forbidden' || state === 'invalid' ? 'alert' : 'status'} aria-busy={state === 'loading' || undefined}><p>{state === 'loading' ? '正在按职位和快照编号读取固定记录。' : state === 'error' ? '请检查网络后重试。' : state === 'forbidden' ? '此记录不属于当前可访问的空间。' : state === 'invalid' ? '请从导入结果打开完整的证据链接。' : '请重新打开证据链接以读取当前空间中的记录。'}</p>{state === 'error' ? <button type="button" onClick={retry}>重试</button> : null}</section></main>
 const fields: Array<[string, string]> = [['职位名称', fieldValue(evidence.extracted.title)], ['公司', fieldValue(evidence.extracted.company)], ['地点', fieldValue(evidence.extracted.location)], ['招聘批次', fieldValue(evidence.extracted.batch)], ['要求', fieldValue(evidence.extracted.requirements)]]
 const source = [evidence.source.label, evidence.source.referenceId].filter((value): value is string => Boolean(value)).join(' · ') || evidence.source.kind
 return <main className="wk-page wk-opportunity-evidence"><header className="wk-header"><div><p className="wk-opportunity-import__eyebrow">Career · 职位证据</p><h1>{evidence.status === 'needs_review' ? '已保存，待确认' : '已保存的职位证据'}</h1><p>此页面显示固定快照的原始内容和当前可确认的信息。</p></div><a href="/platform/creatChat">返回对话</a></header>
  <section className="wk-opportunity-evidence__meta" aria-label="来源信息"><dl><div><dt>来源</dt><dd>{source}</dd></div><div><dt>采集时间</dt><dd><time dateTime={evidence.acquiredAt}>{evidence.acquiredAt}</time></dd></div><div><dt>状态</dt><dd>{evidence.status === 'needs_review' ? '待确认' : '已保存'}</dd></div><div><dt>快照编号</dt><dd><code>{evidence.snapshotId}</code></dd></div></dl></section>
  {evidence.status === 'needs_review' ? <p className="wk-opportunity-evidence__notice" role="status">职位描述已保存为证据，提取字段仍需核对。</p> : null}
  <section className="wk-opportunity-evidence__fields" aria-labelledby="wk-opportunity-fields-title"><h2 id="wk-opportunity-fields-title">提取字段</h2><dl>{fields.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl></section>
  <section className="wk-opportunity-evidence__raw" aria-labelledby="wk-opportunity-raw-title"><h2 id="wk-opportunity-raw-title">原始职位描述</h2><pre>{evidence.rawText}</pre></section>
 </main>
}
