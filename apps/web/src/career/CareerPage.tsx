import { useCallback, useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react'
import { Button, Card, Input, Select } from 'tdesign-react'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ScopeController } from '@weknora/domain/scope'
import { CareerDesk } from '../../../../packages/career-core/src/desk.ts'
import type { CareerAction, CareerSource, CareerView } from '../../../../packages/career-core/src/contracts.ts'

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
 const sync = useCallback(() => setView(desk.snapshot), [desk])
 const load = useCallback(async () => {
  setLoading(true); setError(undefined)
  try { const data = await desk.open(); if (data) setView(data) }
  catch (cause) { setError(message(cause)) }
  finally { setLoading(false) }
 }, [desk])
 useEffect(() => {
  const activeUser = scope.userId ?? userId
  setView(undefined); setUnknownAction(undefined); setReceiptNotice('')
  desk.activate(activeUser, scope.tenantId)
  void load()
  return () => { desk.clear() }
 }, [desk, scope.userId, userId, scope.tenantId, scope.generation, load])
 const doAction = useCallback(async (action: CareerAction) => {
  setBusy(true); setError(undefined); setReceiptNotice('')
  try {
   const receipt = await desk.mutate(action)
   if (receipt) { sync(); setUnknownAction(undefined); setReceiptNotice(`操作已记录，回执 ${receipt.requestId}，修订 ${receipt.revision}`); await desk.refresh(); sync() }
  } catch (cause) {
   const parsed = message(cause); setError(parsed)
   if (parsed.code === 'outcome_unknown') setUnknownAction(action)
   if (parsed.code === 'revision_conflict') { await desk.refresh().then(sync).catch(() => undefined) }
  } finally { setBusy(false) }
 }, [desk, sync])
 const submitProposal = (event: FormEvent) => {
  event.preventDefault(); if (!view || !form.value.trim()) return
  void doAction({ action: 'propose', key: form.key, value: form.value.trim(), source: { kind: 'user', label: '本人填写' }, requestId: makeId(), expectedRevision: view.revision })
 }
 const retryReceipt = async () => {
  if (!unknownAction) return
  setBusy(true)
  try { const receipt = await desk.reconcile(unknownAction.requestId); sync(); if (receipt) { setReceiptNotice(`已找回回执 ${receipt.requestId}，修订 ${receipt.revision}`); setUnknownAction(undefined) } else setReceiptNotice('暂未找到回执；保留原请求，请勿更换请求编号重试。') }
  catch (cause) { setError(message(cause)) }
  finally { setBusy(false) }
 }
 const sourceText = (source: CareerSource) => [source.label || (source.kind === 'user' ? '本人提供' : source.kind), source.referenceId].filter(Boolean).join(' · ')
 return <main className="wk-page wk-page--std" style={{ maxWidth: 1040, margin: '0 auto', padding: '24px 20px' }}>
  <header style={{ marginBottom: 20 }}><p style={{ color: '#666', margin: 0 }}>个人求职空间</p><h1 style={{ margin: '6px 0' }}>求职档案</h1><p>只有已确认的档案事实会成为后续求职判断的输入。空间由当前 WeKnora 身份与 Tenant 授权。</p></header>
  {loading ? <p role="status">正在打开个人求职空间…</p> : null}
  {error ? <Card bordered style={{ marginBottom: 16 }}><div role="alert"><strong>{error.code === 'forbidden' ? '当前空间不可访问' : error.code === 'revision_conflict' ? '档案已更新' : error.code === 'outcome_unknown' ? '提交结果暂时未知' : '暂时无法打开档案'}</strong><p>{error.text}{error.currentRevision !== undefined ? `（当前修订 ${error.currentRevision}）` : ''}</p></div><Button variant="outline" onClick={() => void load()}>重新读取</Button></Card> : null}
  {error?.code === 'forbidden' ? <Card bordered>请切换到本人拥有的单成员个人空间后重试。服务端会验证用户与空间归属。</Card> : null}
  {unknownAction ? <Card bordered style={{ marginBottom: 16 }}><div role="status"><strong>需要核对提交回执</strong><p>保留了本次输入。请先查询同一请求编号的持久回执，再决定下一步。</p><Button loading={busy} onClick={() => void retryReceipt()}>查询回执</Button></div></Card> : null}
  {receiptNotice ? <p role="status" style={{ color: '#087a55' }}>{receiptNotice}</p> : null}
  {view ? <>
   <Card bordered style={{ marginBottom: 16 }}><div style={{ display: 'flex', flexWrap: 'wrap', gap: 12, alignItems: 'center', justifyContent: 'space-between' }}><div><h2 style={{ margin: 0 }}>已确认档案</h2><p style={{ margin: '6px 0 0' }}>当前修订 {view.revision} · {view.facts.length} 条确认事实</p></div><Button variant="outline" onClick={() => void desk.refresh().then(sync).catch((e) => setError(message(e)))}>刷新</Button></div>
    {view.facts.length ? <dl style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 12 }}>{view.facts.map((fact) => <div key={`${fact.key}:${fact.revision}`} style={{ border: '1px solid #e7e7e7', borderRadius: 8, padding: 14 }}><dt style={{ color: '#666' }}>{fieldLabel[fact.key] || fact.key}</dt><dd style={{ margin: '6px 0', fontWeight: 600 }}>{fact.value}</dd><small>{sourceText(fact.source)} · {fact.confirmation.confirmedAt}</small></div>)}</dl> : <p>还没有确认事实。可以先提交档案提案，或直接确认本人提供的信息。</p>}
   </Card>
   <Card bordered style={{ marginBottom: 16 }}><h2 style={{ marginTop: 0 }}>待确认提案</h2>{view.proposals.filter((proposal) => proposal.status === 'pending').length ? <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>{view.proposals.filter((proposal) => proposal.status === 'pending').map((proposal) => <li key={proposal.id} style={{ display: 'flex', flexWrap: 'wrap', gap: 12, justifyContent: 'space-between', alignItems: 'center', borderTop: '1px solid #eee', padding: '12px 0' }}><div><strong>{fieldLabel[proposal.key] || proposal.key}：{proposal.value}</strong><div><small>待确认 · 来源：{sourceText(proposal.source)} · 提交于 {proposal.createdAt}</small></div></div><div style={{ display: 'flex', gap: 8 }}><Button size="small" loading={busy} onClick={() => void doAction({ action: 'confirm_proposal', proposalId: proposal.id, source: { kind: 'user', label: '本人确认' }, requestId: makeId(), expectedRevision: view.revision })}>确认</Button><Button size="small" variant="outline" disabled={busy} onClick={() => void doAction({ action: 'dismiss', proposalId: proposal.id, source: { kind: 'user', label: '本人忽略' }, requestId: makeId(), expectedRevision: view.revision })}>忽略</Button></div></li>)}</ul> : <p>没有待确认提案。</p>}</Card>
   <Card bordered><h2 style={{ marginTop: 0 }}>补充档案</h2><form onSubmit={submitProposal} style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(min(100%, 190px), 1fr))', gap: 12, alignItems: 'end' }}><label>档案字段<Select value={form.key} onChange={(value) => setForm((current) => ({ ...current, key: String(value) }))}>{fields.map((field) => <Select.Option key={field} value={field} label={fieldLabel[field]} />)}</Select></label><label>内容<Input value={form.value} placeholder="填写待确认内容" onChange={(value) => setForm((current) => ({ ...current, value: String(value) }))} /></label><Button type="submit" variant="outline" disabled={busy || !form.value.trim()}>保存为提案</Button><Button type="button" disabled={busy || !form.value.trim()} onClick={() => { if (view) void doAction({ action: 'confirm', key: form.key, value: form.value.trim(), source: { kind: 'user', label: '本人确认' }, requestId: makeId(), expectedRevision: view.revision }) }}>直接确认</Button></form></Card>
  </> : null}
 </main>
}
