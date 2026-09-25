import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { Button, Card, Input, Select } from 'tdesign-react'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ScopeController } from '@weknora/domain/scope'
import { CareerDesk } from '../../../../packages/career-core/src/desk.ts'
import type { CareerAction, CareerDocumentSource, CareerSource, CareerUpload, CareerView } from '../../../../packages/career-core/src/contracts.ts'
import { ExportDeletionPage } from './ExportDeletionPage.tsx'

const fields = ['毕业时间', '学历', '城市', '意向'] as const
const fieldLabel: Record<string, string> = { 毕业时间: '毕业时间', 学历: '最高学历', 城市: '意向城市', 意向: '求职意向' }
const makeId = (): string => typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`
function message(error: unknown): { code?: string; currentRevision?: number; text: string } {
 const value = error as { code?: string; currentRevision?: number; message?: string }
 return { code: value?.code, currentRevision: value?.currentRevision, text: value?.message || '请求未完成' }
}

export function CareerPage({ client, scopeController, userId }: { client: WeKnoraClient; scopeController: ScopeController; userId: string | null }): ReactNode {
 const scope = scopeController.current().scope
 const desk = useMemo(() => new CareerDesk({
  open: (signal) => client.career.open(signal), list: (signal) => client.career.list(signal),
  changes: (since, signal) => client.career.changes(since, signal), act: (action, signal) => client.career.act(action, signal), receipt: (id, signal) => client.career.receipt(id, signal),
 }), [client])
 const [view, setView] = useState<CareerView>()
 const [loading, setLoading] = useState(true)
 const [error, setError] = useState<{ code?: string; currentRevision?: number; text: string }>()
 const [busy, setBusy] = useState(false)
 const [form, setForm] = useState({ key: '毕业时间', value: '' })
 const [unknownAction, setUnknownAction] = useState<CareerAction>()
 const [receiptNotice, setReceiptNotice] = useState('')
 const [sources, setSources] = useState<CareerDocumentSource[]>([])
 const [selectedFile, setSelectedFile] = useState<File>()
 const [uploadUnknown, setUploadUnknown] = useState<{ file: File; requestId: string; expectedRevision: number }>()
 const [uploadBusy, setUploadBusy] = useState(false)
 const [uploadNotice, setUploadNotice] = useState('')
 const scopeEpoch = useRef(0)
 const fileInputRef = useRef<HTMLInputElement>(null)
 const mutationBlocked = Boolean(unknownAction || desk.pendingAction)
 const isCurrent = (epoch: number) => scopeEpoch.current === epoch
 const invalidateForbidden = (parsed?: { code?: string; currentRevision?: number; text: string }) => {
  scopeEpoch.current += 1
  desk.clear()
  setView(undefined); setUnknownAction(undefined); setSources([]); setSelectedFile(undefined); setUploadUnknown(undefined); setUploadNotice('')
  setReceiptNotice(''); setBusy(false); setUploadBusy(false); setLoading(false)
  if (parsed) setError(parsed)
 }
 const sync = useCallback(() => setView(desk.snapshot), [desk])
 const load = useCallback(async () => {
  const epoch = scopeEpoch.current
  setLoading(true); setError(undefined)
  desk.activate(scope.userId ?? userId, scope.tenantId)
  try {
   const data = await desk.open()
   if (!isCurrent(epoch)) return
   if (data) setView(data)
   const items = await client.career.sources()
   if (!isCurrent(epoch)) return
   setSources(items)
  } catch (cause) {
   if (!isCurrent(epoch)) return
   const parsed = message(cause)
   if (parsed.code === 'forbidden') invalidateForbidden(parsed)
   else setError(parsed)
  } finally { if (isCurrent(epoch)) setLoading(false) }
 }, [client, desk, scope.userId, userId, scope.tenantId])
 useEffect(() => {
  scopeEpoch.current += 1
  const activeUser = scope.userId ?? userId
  setView(undefined); setUnknownAction(undefined); setReceiptNotice('')
  setSources([]); setSelectedFile(undefined); setUploadUnknown(undefined); setUploadNotice(''); setUploadBusy(false)
  desk.activate(activeUser, scope.tenantId)
  void load()
  return () => { scopeEpoch.current += 1; desk.clear() }
 }, [desk, client, scope.userId, userId, scope.tenantId, scope.generation, load])
 const refreshSources = useCallback(async () => {
  const epoch = scopeEpoch.current
  try {
   const items = await client.career.sources()
   if (!isCurrent(epoch)) return []
   setSources(items)
   return items
  } catch (cause) {
   if (isCurrent(epoch) && message(cause).code === 'forbidden') invalidateForbidden(message(cause))
   throw cause
  }
 }, [client])
 const refreshSourceList = useCallback(async () => {
  const epoch = scopeEpoch.current
  setUploadBusy(true)
  try { await refreshSources(); if (isCurrent(epoch)) setUploadNotice('来源列表已刷新。') }
  catch (cause) { if (isCurrent(epoch) && message(cause).code !== 'forbidden') setError(message(cause)) }
  finally { if (isCurrent(epoch)) setUploadBusy(false) }
 }, [refreshSources])
 const sendUpload = useCallback(async (attempt: { file: File; requestId: string; expectedRevision: number }) => {
  const epoch = scopeEpoch.current
  setUploadBusy(true); setUploadNotice('上传中…'); setError(undefined)
  let result: CareerUpload
  try {
   result = await client.career.upload(attempt.file, attempt.file.name, attempt.requestId, attempt.expectedRevision)
  } catch (cause) {
   if (!isCurrent(epoch)) return
   const parsed = message(cause)
   if (parsed.code === 'forbidden') invalidateForbidden(parsed)
   else if (['invalid_request', 'idempotency_conflict', 'revision_conflict'].includes(parsed.code ?? '')) {
    setError(parsed)
    setUploadUnknown(undefined); setSelectedFile(undefined); if (fileInputRef.current) fileInputRef.current.value = ''
    if (parsed.code === 'revision_conflict') {
     setUploadNotice('档案已变化。请检查当前修订后重新选择文件并开始一次新的上传。')
     await desk.refresh().then(() => { if (isCurrent(epoch)) sync() }).catch((refreshError) => { if (isCurrent(epoch) && message(refreshError).code === 'forbidden') invalidateForbidden(message(refreshError)) })
     if (!isCurrent(epoch)) return
     await refreshSources().catch((refreshError) => { if (isCurrent(epoch) && message(refreshError).code !== 'forbidden') setError(message(refreshError)) })
    } else setUploadNotice('本次上传已被服务端拒绝。请重新选择简历后开始一次新的上传。')
   }
   else { setError(parsed); setUploadUnknown(attempt); setUploadNotice('上传结果暂时未知。已保留原文件和请求编号；先查询来源状态，再决定是否用相同内容重试。') }
   if (isCurrent(epoch)) setUploadBusy(false)
   return
  }
  if (!isCurrent(epoch)) return
  setSources((current) => [result.source, ...current.filter((source) => source.id !== result.source.id)])
  if (result.receipt) {
   setUploadNotice(`简历已处理，生成 ${result.receipt.proposals.length} 条待确认提案。`)
   setView((current) => current ? { ...current, revision: Math.max(current.revision, result.receipt!.revision), proposals: [...current.proposals.filter((proposal) => !result.receipt!.proposals.some((candidate) => candidate.id === proposal.id)), ...result.receipt!.proposals] } : current)
  }
  else if (result.source.status === 'processing') setUploadNotice('简历正在处理中。可刷新来源状态。')
  else if (result.source.status === 'failed') setUploadNotice(`处理失败：${result.source.errorMessage || '请重新上传，或手动补充档案。'}`)
  else setUploadNotice('来源状态已更新。')
  if (result.source.status === 'failed' || result.source.status === 'ready') { setUploadUnknown(undefined); setSelectedFile(undefined); if (fileInputRef.current) fileInputRef.current.value = '' }
  else setUploadUnknown(attempt)
  try { await refreshSources() }
  catch (cause) { if (!isCurrent(epoch)) return; const parsed = message(cause); if (parsed.code === 'forbidden') { invalidateForbidden(parsed); return }; setError(parsed) }
  if (!isCurrent(epoch)) return
  if (result.receipt) {
   try { await desk.refresh(); if (!isCurrent(epoch)) return; sync() }
   catch (cause) { if (!isCurrent(epoch)) return; const parsed = message(cause); if (parsed.code === 'forbidden') invalidateForbidden(parsed); else setError(parsed) }
  }
  if (isCurrent(epoch)) setUploadBusy(false)
 }, [client, desk, refreshSources, sync])
 const doAction = useCallback(async (action: CareerAction) => {
  if (desk.pendingAction || unknownAction) return
  const epoch = scopeEpoch.current
  setBusy(true); setError(undefined); setReceiptNotice('')
  try {
   const receipt = await desk.mutate(action)
   if (!isCurrent(epoch)) return
   if (receipt) { sync(); setUnknownAction(undefined); setReceiptNotice(`操作已记录，回执 ${receipt.requestId}，修订 ${receipt.revision}`); await desk.refresh(); if (!isCurrent(epoch)) return; sync() }
  } catch (cause) {
   if (!isCurrent(epoch)) return
   const parsed = message(cause)
   if (parsed.code === 'forbidden') { invalidateForbidden(parsed); return }
   setError(parsed)
   if (parsed.code === 'outcome_unknown') setUnknownAction(action)
   if (parsed.code === 'revision_conflict') { await desk.refresh().then(() => { if (isCurrent(epoch)) sync() }).catch((refreshError) => { if (isCurrent(epoch) && message(refreshError).code === 'forbidden') invalidateForbidden(message(refreshError)) }) }
  } finally { if (isCurrent(epoch)) setBusy(false) }
 }, [desk, sync, unknownAction])
 const submitProposal = (event: FormEvent) => {
  event.preventDefault(); if (!view || !form.value.trim()) return
  void doAction({ action: 'propose', key: form.key, value: form.value.trim(), source: { kind: 'user', label: '本人填写' }, requestId: makeId(), expectedRevision: view.revision })
 }
 const retryReceipt = async () => {
  if (!unknownAction) return
  const epoch = scopeEpoch.current
  setBusy(true)
  try { const receipt = await desk.reconcile(unknownAction.requestId); if (!isCurrent(epoch)) return; sync(); if (receipt) { setReceiptNotice(`已找回回执 ${receipt.requestId}，修订 ${receipt.revision}`); setUnknownAction(undefined) } else setReceiptNotice('暂未找到回执；保留原请求，请勿更换请求编号重试。') }
  catch (cause) { if (!isCurrent(epoch)) return; const parsed = message(cause); if (parsed.code === 'forbidden') { invalidateForbidden(parsed); return }; setError(parsed); if (parsed.code === 'not_found' && desk.safeToRetry) setReceiptNotice('未找到已提交回执。可以用完全相同的内容和请求编号安全重试。') }
  finally { if (isCurrent(epoch)) setBusy(false) }
 }
 const retrySameAction = async () => {
  if (!unknownAction || !desk.safeToRetry) return
  const epoch = scopeEpoch.current
  setBusy(true); setError(undefined)
  try { const receipt = await desk.retryUnknown(unknownAction); if (!isCurrent(epoch)) return; if (receipt) { sync(); setUnknownAction(undefined); setReceiptNotice(`已取得同一请求的回执 ${receipt.requestId}，修订 ${receipt.revision}`); await desk.refresh(); if (!isCurrent(epoch)) return; sync() } }
  catch (cause) { if (!isCurrent(epoch)) return; const parsed = message(cause); if (parsed.code === 'forbidden') { invalidateForbidden(parsed); return }; setError(parsed); if (parsed.code === 'outcome_unknown') setUnknownAction(desk.pendingAction ?? unknownAction); else if (!desk.pendingAction) setUnknownAction(undefined); if (parsed.code === 'revision_conflict') { await desk.refresh().then(() => { if (isCurrent(epoch)) sync() }).catch((refreshError) => { if (isCurrent(epoch) && message(refreshError).code === 'forbidden') invalidateForbidden(message(refreshError)) }) } }
  finally { if (isCurrent(epoch)) setBusy(false) }
 }
 const sourceText = (source: CareerSource) => [source.label || (source.kind === 'user' ? '本人提供' : source.kind), source.referenceId].filter(Boolean).join(' · ')
 return <main className="wk-page wk-page--std" style={{ maxWidth: 1040, margin: '0 auto', padding: '24px 20px' }}>
  <header style={{ marginBottom: 20 }}><p style={{ color: '#666', margin: 0 }}>个人求职空间</p><h1 style={{ margin: '6px 0' }}>求职档案</h1><p>只有已确认的档案事实会成为后续求职判断的输入。空间由当前 WeKnora 身份与 Tenant 授权。</p></header>
  {loading ? <p role="status">正在打开个人求职空间…</p> : null}
  {error ? <Card bordered style={{ marginBottom: 16 }}><div role="alert"><strong>{error.code === 'forbidden' ? '当前空间不可访问' : error.code === 'revision_conflict' ? '档案已更新' : error.code === 'outcome_unknown' ? '提交结果暂时未知' : '暂时无法打开档案'}</strong><p>{error.text}{error.currentRevision !== undefined ? `（当前修订 ${error.currentRevision}）` : ''}</p></div><Button variant="outline" onClick={() => void load()}>重新读取</Button></Card> : null}
  {error?.code === 'forbidden' ? <Card bordered>请切换到本人拥有的单成员个人空间后重试。服务端会验证用户与空间归属。</Card> : null}
  {unknownAction ? <Card bordered style={{ marginBottom: 16 }}><div role="status"><strong>需要核对提交回执</strong><p>保留了本次输入。请求编号 {unknownAction.requestId}。请先查询持久回执；查无回执后只能用同一请求编号安全重试。</p><Button loading={busy} onClick={() => void retryReceipt()}>查询回执</Button>{desk.safeToRetry ? <Button loading={busy} variant="outline" onClick={() => void retrySameAction()}>用原请求编号安全重试</Button> : null}</div></Card> : null}
  {receiptNotice ? <p role="status" style={{ color: '#087a55' }}>{receiptNotice}</p> : null}
  {error?.code === 'forbidden' ? null : <Card bordered style={{ marginBottom: 16 }}>
   <h2 style={{ marginTop: 0 }}>上传简历</h2>
   <p>系统只会生成待确认提案。上传或解析失败不会覆盖已确认档案。</p>
   <label htmlFor="career-resume-file">选择简历文件</label>{' '}
   <input ref={fileInputRef} id="career-resume-file" type="file" accept=".pdf,.doc,.docx,.txt" disabled={uploadBusy || Boolean(uploadUnknown)} onChange={(event) => { setSelectedFile(event.currentTarget.files?.[0]); setUploadNotice('') }} />
   <Button disabled={!selectedFile || uploadBusy || Boolean(uploadUnknown) || !view} loading={uploadBusy} onClick={() => { if (selectedFile && view) void sendUpload({ file: selectedFile, requestId: makeId(), expectedRevision: view.revision }) }}>开始上传</Button>
   {uploadNotice ? <p role="status" aria-live="polite">{uploadNotice}</p> : null}
   {uploadUnknown ? <div role="group" aria-label="恢复简历上传" style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
    <Button variant="outline" disabled={uploadBusy} onClick={async () => {
     const epoch = scopeEpoch.current
     setUploadBusy(true)
     try { await refreshSources(); if (!isCurrent(epoch)) return; setUploadNotice('来源列表不包含请求编号，无法确认本次上传结果。请用保留的原文件、请求编号和修订精确重试。') }
     catch (cause) { if (isCurrent(epoch)) { const parsed = message(cause); if (parsed.code !== 'forbidden') setError(parsed) } }
     finally { if (isCurrent(epoch)) setUploadBusy(false) }
    }}>查询来源状态</Button>
    <Button disabled={uploadBusy} onClick={() => void sendUpload(uploadUnknown)}>用原文件和请求编号重试</Button>
   </div> : null}
   {sources.length ? <section aria-label="简历来源版本" style={{ marginTop: 16 }}><div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, alignItems: 'center' }}><h3>来源版本</h3><Button size="small" variant="outline" disabled={uploadBusy} onClick={() => void refreshSourceList()}>刷新来源</Button></div><ul>{sources.map((source) => <li key={source.id} style={{ marginBottom: 12 }}>
    <strong>{source.fileName}</strong> · 修订 {source.revision} · {source.status === 'processing' ? '处理中' : source.status === 'ready' ? '已解析' : '失败'} · {source.createdAt}
    {source.status === 'failed' ? <p role="alert">{source.errorMessage || '解析失败'}；可以重新上传或手动补充，原有已确认事实仍保留。</p> : null}
    {source.missingCategories?.length ? <p>缺失类别：{source.missingCategories.join('、')}</p> : null}
    {source.reviewFlags?.length ? <p>需要核对：{source.reviewFlags.join('、')}</p> : null}
   </li>)}</ul></section> : <p>还没有简历来源记录。</p>}
  </Card>}
  {view ? <>
   <Card bordered style={{ marginBottom: 16 }}><div style={{ display: 'flex', flexWrap: 'wrap', gap: 12, alignItems: 'center', justifyContent: 'space-between' }}><div><h2 style={{ margin: 0 }}>已确认档案</h2><p style={{ margin: '6px 0 0' }}>当前修订 {view.revision} · {view.facts.length} 条确认事实</p></div><Button variant="outline" onClick={async () => { const epoch = scopeEpoch.current; try { await desk.refresh(); if (isCurrent(epoch)) sync() } catch (cause) { if (!isCurrent(epoch)) return; const parsed = message(cause); if (parsed.code === 'forbidden') invalidateForbidden(parsed); else setError(parsed) } }}>刷新</Button></div>
    {view.facts.length ? <dl style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 12 }}>{view.facts.map((fact) => <div key={`${fact.key}:${fact.revision}`} style={{ border: '1px solid #e7e7e7', borderRadius: 8, padding: 14 }}><dt style={{ color: '#666' }}>{fieldLabel[fact.key] || fact.key}</dt><dd style={{ margin: '6px 0', fontWeight: 600 }}>{fact.value}</dd><small>{sourceText(fact.source)} · {fact.confirmation.confirmedAt}</small></div>)}</dl> : <p>还没有确认事实。可以先提交档案提案，或直接确认本人提供的信息。</p>}
   </Card>
   <Card bordered style={{ marginBottom: 16 }}><h2 style={{ marginTop: 0 }}>待确认提案</h2>{view.proposals.filter((proposal) => proposal.status === 'pending').length ? <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>{view.proposals.filter((proposal) => proposal.status === 'pending').map((proposal) => <li key={proposal.id} style={{ display: 'flex', flexWrap: 'wrap', gap: 12, justifyContent: 'space-between', alignItems: 'center', borderTop: '1px solid #eee', padding: '12px 0' }}><div><strong>{fieldLabel[proposal.key] || proposal.key}：{proposal.value}</strong><div><small>待确认 · 来源：{sourceText(proposal.source)} · 提交于 {proposal.createdAt}</small></div>{proposal.evidence ? <blockquote>原文依据：{proposal.evidence}</blockquote> : null}</div><div style={{ display: 'flex', gap: 8 }}><Button size="small" loading={busy} disabled={busy || mutationBlocked} onClick={() => void doAction({ action: 'confirm_proposal', proposalId: proposal.id, source: { kind: 'user', label: '本人确认' }, requestId: makeId(), expectedRevision: view.revision })}>确认</Button><Button size="small" variant="outline" disabled={busy || mutationBlocked} onClick={() => void doAction({ action: 'dismiss', proposalId: proposal.id, source: { kind: 'user', label: '本人忽略' }, requestId: makeId(), expectedRevision: view.revision })}>忽略</Button></div></li>)}</ul> : <p>没有待确认提案。</p>}</Card>
   <Card bordered><h2 style={{ marginTop: 0 }}>补充档案</h2><form onSubmit={submitProposal} style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(min(100%, 190px), 1fr))', gap: 12, alignItems: 'end' }}><label>档案字段<Select value={form.key} onChange={(value) => setForm((current) => ({ ...current, key: String(value) }))}>{fields.map((field) => <Select.Option key={field} value={field} label={fieldLabel[field]} />)}</Select></label><label>内容<Input value={form.value} placeholder="填写待确认内容" onChange={(value) => setForm((current) => ({ ...current, value: String(value) }))} /></label><Button type="submit" variant="outline" disabled={busy || mutationBlocked || !form.value.trim()}>保存为提案</Button><Button type="button" disabled={busy || mutationBlocked || !form.value.trim()} onClick={() => { if (view) void doAction({ action: 'confirm', key: form.key, value: form.value.trim(), source: { kind: 'user', label: '本人确认' }, requestId: makeId(), expectedRevision: view.revision }) }}>直接确认</Button></form></Card>
  </> : null}
  {error?.code === 'forbidden' ? null : <ExportDeletionPage client={client} scopeController={scopeController} onCareerDeleted={() => { scopeEpoch.current += 1; desk.clear(); void load() }} />}
 </main>
}
