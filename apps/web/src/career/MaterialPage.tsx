import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ScopeController } from '@weknora/domain/scope'
import type { CareerView } from '../../../../packages/career-core/src/contracts.ts'
import type { ConfirmMaterialInput, EditMaterialInput, MaterialBody, MaterialClaim, MaterialReceipt, MaterialVersionChange, MaterialVersionComparison, MaterialVersionView, MaterialView } from '../../../../packages/api-client/src/career.ts'
import './material.css'

type MaterialPhase = 'idle' | 'busy' | 'unknown' | 'error' | 'forbidden' | 'scope-changed'
type EditableClaim = { claimId: string; text: string; factKey: string; needsReview: boolean; reviewNote: string }
type EditableSection = { heading: string; content: string; claims: EditableClaim[] }
type MaterialAttempt = { kind: 'edit' | 'confirm'; requestId: string; input: EditMaterialInput | ConfirmMaterialInput }
const newRequestId = (): string => typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`

function errorDetails(cause: unknown): { code?: string; requestId?: string; currentRevision?: number; status?: number; message: string } {
 const error = cause as { code?: string; requestId?: string; currentRevision?: number; status?: number; message?: string }
 return { code: error?.code, requestId: error?.requestId, currentRevision: error?.currentRevision, status: error?.status, message: error?.message || '请求未完成' }
}
function isUncertainWrite(cause: unknown): boolean {
 const error = errorDetails(cause)
 if (error.code === 'TIMEOUT' || error.code === 'outcome_unknown') return true
 if (['forbidden', 'invalid_request', 'idempotency_conflict', 'revision_conflict', 'material_claim_unconfirmed', 'request_too_large', 'PAYLOAD_TOO_LARGE', 'not_found', 'unauthorized'].includes(error.code ?? '')) return false
 if (error.status !== undefined) return error.status >= 500 || error.status < 400
 return true
}
function materialParamUrl(materialId?: string): string | undefined {
 if (typeof window === 'undefined' || !window.location) return undefined
 const params = new URLSearchParams(window.location.search)
 if (materialId) params.set('material', materialId)
 else params.delete('material')
 const search = params.toString()
 return `${window.location.pathname}${search ? `?${search}` : ''}`
}
const editableFromBody = (body: MaterialBody): EditableSection[] => body.sections.map((section) => ({ heading: section.heading, content: section.content, claims: section.claims.map((claim: MaterialClaim) => ({ claimId: claim.claimId, text: claim.text, factKey: claim.factKey ?? '', needsReview: claim.needsReview, reviewNote: claim.reviewNote ?? '' })) }))
const bodyFromEditable = (sections: EditableSection[]): MaterialBody => ({ sections: sections.map((section) => ({ heading: section.heading.trim(), content: section.content, claims: section.claims.map((claim) => ({ claimId: claim.claimId, text: claim.text, ...(claim.factKey.trim() ? { factKey: claim.factKey.trim() } : {}), needsReview: claim.needsReview, ...(claim.reviewNote.trim() ? { reviewNote: claim.reviewNote } : {}) })) })) })
const changeText = (change: MaterialVersionChange): string => {
 const heading = change.heading ? `「${change.heading}」` : ''
 switch (change.kind) {
  case 'section_added': return `新增章节${heading}：${change.target ?? ''}`
  case 'section_removed': return `删除章节${heading}：${change.baseline ?? ''}`
  case 'section_changed': return `章节${heading}变更：${change.baseline ?? ''} → ${change.target ?? ''}`
  case 'claim_added': return `新增主张 ${change.claimId ?? ''}（${change.heading ?? ''}）：${change.target ?? ''}`
  case 'claim_removed': return `删除主张 ${change.claimId ?? ''}（${change.heading ?? ''}）：${change.baseline ?? ''}`
  case 'claim_changed': return `主张 ${change.claimId ?? ''}（${change.heading ?? ''}）变更：${change.baseline ?? ''} → ${change.target ?? ''}`
 }
}

function BodyReadonly({ body }: { body: MaterialBody }): ReactNode {
 return <div className="wk-material__body-readonly">{body.sections.map((section, index) => <div className="wk-material__body-section" key={`${section.heading}-${index}`}>
  <h5>{section.heading}</h5>
  <p>{section.content}</p>
  {section.claims.length ? <ul>{section.claims.map((claim) => <li key={claim.claimId} className={claim.needsReview ? 'wk-material__claim--needs-review' : undefined}>{claim.text}{claim.factKey ? `（已链接确认事实：${claim.factKey}）` : '（缺失/待补充 needs_review，不得补造）'}</li>)}</ul> : null}
 </div>)}</div>
}

// From the application context (one fixed opportunity snapshot) the user
// edits one structured material. Creation freezes the opportunity snapshot
// and profile revision; missing items stay explicit needs_review
// placeholders and are never fabricated; confirming the draft appends the
// next immutable version while every earlier version stays readable,
// comparable, and read-only.
export function MaterialPage({ client, scopeController, opportunityId, snapshotId }: { client: WeKnoraClient; scopeController: ScopeController; opportunityId: string; snapshotId: string }): ReactNode {
 const scope = scopeController.current()
 const [revision, setRevision] = useState<number | undefined>()
 const [revisionState, setRevisionState] = useState<'loading' | 'ready' | 'error'>('loading')
 const [materialId, setMaterialId] = useState<string>()
 const [view, setView] = useState<MaterialView>()
 const [savedBody, setSavedBody] = useState<MaterialBody>()
 const [sections, setSections] = useState<EditableSection[]>([])
 const [attempt, setAttempt] = useState<MaterialAttempt>()
 const [phase, setPhase] = useState<MaterialPhase>('idle')
 const [message, setMessage] = useState('')
 const [revisionConflict, setRevisionConflict] = useState<number>()
 const [comparison, setComparison] = useState<MaterialVersionComparison>()
 const [versionDetail, setVersionDetail] = useState<MaterialVersionView>()
 const claimCounter = useRef(0)
 const restored = useRef(false)
 const dirty = savedBody === undefined || JSON.stringify(bodyFromEditable(sections)) !== JSON.stringify(savedBody)
 // Review round 1 F1: a restored server draft can already contain claims in
 // the local claim-N pattern. New placeholder IDs must continue past the
 // restored maximum (and never collide with any existing claim ID at all) so
 // React keys stay unique and the backend's per-body seenClaims check never
 // rejects the next save with invalid_request.
 const syncClaimCounter = (body: MaterialBody): void => {
  for (const section of body.sections) for (const claim of section.claims) {
   const match = claim.claimId.match(/^claim-(\d+)$/)
   if (match) claimCounter.current = Math.max(claimCounter.current, Number(match[1]))
  }
 }
 const nextClaimId = (existing: EditableSection[]): string => {
  do { claimCounter.current += 1 } while (existing.some((section) => section.claims.some((claim) => claim.claimId === `claim-${claimCounter.current}`)))
  return `claim-${claimCounter.current}`
 }

 const clearPrivate = useCallback((notice: string, nextState: 'forbidden' | 'scope-changed' = 'forbidden') => {
  setAttempt(undefined); setView(undefined); setSavedBody(undefined); setSections([]); setPhase(nextState); setMessage(notice)
  setMaterialId(undefined); setRevisionConflict(undefined); setComparison(undefined); setVersionDetail(undefined)
  const url = materialParamUrl(undefined)
  if (url) window.history.replaceState({}, document.title, url)
 }, [])
 const readRevision = useCallback(async (): Promise<number | undefined> => {
  const requestScope = scopeController.current()
  setRevisionState('loading')
  try {
   const next: CareerView = await client.career.open(requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return undefined
   setRevision(next.revision); setRevisionState('ready')
   return next.revision
  } catch {
   if (!scopeController.isCurrent(requestScope.scope)) return undefined
   setRevision(undefined); setRevisionState('error')
   return undefined
  }
 }, [client, scopeController])
 useEffect(() => {
  const requestScope = scopeController.current()
  const clear = () => clearPrivate('空间已切换或登录已失效，已清除材料编辑内容。', 'scope-changed')
  requestScope.signal?.addEventListener('abort', clear, { once: true })
  return () => requestScope.signal?.removeEventListener('abort', clear)
 }, [clearPrivate, scopeController, scope.scope.generation])
 useEffect(() => { void readRevision() }, [readRevision])

 const acceptView = useCallback((next: MaterialView): void => {
  setView(next); setMaterialId(next.materialId); setSavedBody(next.body); setSections(editableFromBody(next.body))
  syncClaimCounter(next.body)
 }, [])
 // A reload keeps the stored material readable straight from the URL: the
 // material ID is the durable pointer, nothing is replayed here.
 useEffect(() => {
  if (restored.current) return
  restored.current = true
  const stored = new URLSearchParams(window.location.search).get('material')?.trim()
  if (!stored) return
  const requestScope = scopeController.current()
  void client.career.material(stored, requestScope.signal).then((next) => {
   if (!scopeController.isCurrent(requestScope.scope)) return
   if (next.pinnedEvidence.opportunityId !== opportunityId || next.pinnedEvidence.snapshotId !== snapshotId) {
    const url = materialParamUrl(undefined)
    if (url) window.history.replaceState({}, document.title, url)
    return
   }
   acceptView(next); setPhase('idle'); setMessage('')
  }).catch(() => {
   if (!scopeController.isCurrent(requestScope.scope)) return
   clearPrivate('当前空间不可访问此材料，已清除编辑内容。')
  })
 }, [acceptView, clearPrivate, client, opportunityId, scopeController, snapshotId])

 const reloadView = useCallback(async (id: string): Promise<void> => {
  const requestScope = scopeController.current()
  const next = await client.career.material(id, requestScope.signal)
  if (!scopeController.isCurrent(requestScope.scope)) return
  setView(next)
 }, [client, scopeController])

 const writeInFlight = useRef(false)
 const saveDraft = useCallback(async (fixedAttempt?: MaterialAttempt): Promise<void> => {
  if (writeInFlight.current) return
  const currentAttempt: MaterialAttempt | undefined = fixedAttempt ?? ((): MaterialAttempt | undefined => {
   if (phase === 'busy' || phase === 'unknown' || revision === undefined || sections.length === 0) return undefined
   const requestId = newRequestId()
   const body = bodyFromEditable(sections)
   return { kind: 'edit', requestId, input: { requestId, ...(materialId ? { materialId } : { opportunityId, snapshotId }), body, expectedRevision: revision } }
  })()
  if (!currentAttempt) return
  writeInFlight.current = true
  const requestScope = scopeController.current()
  setAttempt(currentAttempt); setPhase('busy'); setMessage('正在保存草稿…'); setRevisionConflict(undefined)
  try {
   const receipt: MaterialReceipt = await client.career.editMaterial(currentAttempt.input as EditMaterialInput, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   if (receipt.requestId !== currentAttempt.requestId || (currentAttempt.input.materialId !== undefined && receipt.materialId !== currentAttempt.input.materialId)) throw new TypeError('材料回执与本次请求不匹配')
   setMaterialId(receipt.materialId); setSavedBody(receipt.body); syncClaimCounter(receipt.body)
   const url = materialParamUrl(receipt.materialId)
   if (url) window.history.replaceState({}, document.title, url)
   await reloadView(receipt.materialId)
   if (!scopeController.isCurrent(requestScope.scope)) return
   setPhase('idle'); setMessage(`草稿已保存（材料编号 ${receipt.materialId}）`)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此材料，已清除编辑内容。'); return }
   if (parsed.code === 'revision_conflict') {
    setAttempt(undefined); setPhase('error'); setRevisionConflict(parsed.currentRevision)
    setMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订后再次保存；新提交会使用新的请求编号。`)
    return
   }
   if (parsed.code === 'material_claim_unconfirmed') {
    setAttempt(undefined); setPhase('error')
    setMessage(`草稿保留未发布：引用了未确认事实（${parsed.message}）。请改为缺失占位，或先在档案中确认该事实。`)
    if (materialId) await reloadView(materialId)
    return
   }
   if (['invalid_request', 'idempotency_conflict', 'not_found', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
    setAttempt(undefined); setPhase('error')
    setMessage(parsed.code === 'idempotency_conflict' ? '请求编号已对应其他内容，服务器拒绝了本次提交。请检查后重新保存。' : parsed.code === 'not_found' ? '材料不存在（可能不属于当前空间），本次保存未完成。' : `材料未被接受：${parsed.message}`)
    return
   }
   if (!isUncertainWrite(cause)) {
    setAttempt(undefined); setPhase('error')
    setMessage(`草稿保存未完成：${parsed.message}`)
    return
   }
   setPhase('unknown')
   setMessage('暂时无法确认材料写入是否完成。请先用原请求编号查询回执，或用同一编号重试；不会自动更换请求编号。')
  } finally { writeInFlight.current = false }
 }, [clearPrivate, materialId, opportunityId, phase, reloadView, revision, scopeController, sections, snapshotId])

 const confirmDraft = useCallback(async (fixedAttempt?: MaterialAttempt): Promise<void> => {
  if (writeInFlight.current) return
  const currentAttempt: MaterialAttempt | undefined = fixedAttempt ?? ((): MaterialAttempt | undefined => {
   if (phase === 'busy' || phase === 'unknown' || revision === undefined || !materialId || dirty) return undefined
   const requestId = newRequestId()
   return { kind: 'confirm', requestId, input: { requestId, materialId, expectedRevision: revision } }
  })()
  if (!currentAttempt) return
  writeInFlight.current = true
  const requestScope = scopeController.current()
  setAttempt(currentAttempt); setPhase('busy'); setMessage('正在确认发布不可变版本…'); setRevisionConflict(undefined)
  try {
   const receipt: MaterialReceipt = await client.career.confirmMaterial(currentAttempt.input as ConfirmMaterialInput, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   if (receipt.requestId !== currentAttempt.requestId || receipt.materialId !== (currentAttempt.input as ConfirmMaterialInput).materialId || receipt.kind !== 'material_confirmed') throw new TypeError('材料回执与本次请求不匹配')
   await reloadView(receipt.materialId)
   if (!scopeController.isCurrent(requestScope.scope)) return
   setPhase('idle'); setMessage(`已发布不可变版本 V${receipt.version ?? ''}`)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此材料，已清除编辑内容。'); return }
   if (parsed.code === 'revision_conflict') {
    setAttempt(undefined); setPhase('error'); setRevisionConflict(parsed.currentRevision)
    setMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订后再次确认；新提交会使用新的请求编号。`)
    return
   }
   if (parsed.code === 'material_claim_unconfirmed') {
    setAttempt(undefined); setPhase('error')
    setMessage(`草稿保留未发布：引用了未确认事实（${parsed.message}）。请改为缺失占位，或先在档案中确认该事实。`)
    if (materialId) await reloadView(materialId)
    return
   }
   if (['invalid_request', 'idempotency_conflict', 'not_found', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
    setAttempt(undefined); setPhase('error')
    setMessage(parsed.code === 'idempotency_conflict' ? '请求编号已对应其他内容，服务器拒绝了本次确认。请检查后重新确认。' : parsed.code === 'not_found' ? '材料不存在（可能不属于当前空间），本次确认未完成。' : `确认未被接受：${parsed.message}`)
    return
   }
   if (!isUncertainWrite(cause)) {
    setAttempt(undefined); setPhase('error')
    setMessage(`确认未完成：${parsed.message}`)
    return
   }
   setPhase('unknown')
   setMessage('暂时无法确认版本是否已发布。请先用原请求编号查询回执，或用同一编号重试；不会自动更换请求编号。')
  } finally { writeInFlight.current = false }
 }, [clearPrivate, dirty, materialId, phase, reloadView, revision, scopeController])

 const lookupReceipt = useCallback(async (): Promise<void> => {
  if (!attempt || phase === 'busy') return
  const currentAttempt = attempt
  const requestScope = scopeController.current()
  setPhase('busy'); setMessage('正在查询原材料回执…')
  try {
   const receipt: MaterialReceipt = await client.career.materialReceipt(currentAttempt.requestId, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   if (receipt.requestId !== currentAttempt.requestId) throw new TypeError('材料回执与原请求编号不匹配')
   setMaterialId(receipt.materialId); setSavedBody(receipt.body); syncClaimCounter(receipt.body)
   const url = materialParamUrl(receipt.materialId)
   if (url) window.history.replaceState({}, document.title, url)
   await reloadView(receipt.materialId)
   if (!scopeController.isCurrent(requestScope.scope)) return
   setPhase('idle')
   setMessage(receipt.kind === 'material_confirmed' ? `已发布不可变版本 V${receipt.version ?? ''}` : `草稿已保存（材料编号 ${receipt.materialId}）`)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此材料，已清除编辑内容。'); return }
   setPhase('unknown')
   setMessage(parsed.code === 'not_found' ? '尚未找到材料回执。可以继续查询，或使用原请求编号重试。' : '材料回执暂时无法读取。原请求编号已保留，可稍后重试查询。')
  }
 }, [attempt, clearPrivate, client, phase, reloadView, scopeController])

 const openVersion = useCallback(async (version: number): Promise<void> => {
  if (!materialId || phase === 'busy') return
  const requestScope = scopeController.current()
  setVersionDetail(undefined); setMessage('')
  try {
   const next = await client.career.materialVersion(materialId, version, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   setVersionDetail(next)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此材料，已清除编辑内容。'); return }
   setPhase('error'); setMessage(parsed.code === 'not_found' ? '该版本不存在（可能不属于当前空间）。' : `版本暂时无法读取：${parsed.message}`)
  }
 }, [clearPrivate, client, materialId, phase, scopeController])

 const openComparison = useCallback(async (baseline: number, target: number): Promise<void> => {
  if (!materialId || phase === 'busy') return
  const requestScope = scopeController.current()
  setComparison(undefined); setMessage('')
  try {
   const next = await client.career.compareMaterialVersions(materialId, baseline, target, requestScope.signal)
   if (!scopeController.isCurrent(requestScope.scope)) return
   setComparison(next)
  } catch (cause) {
   if (!scopeController.isCurrent(requestScope.scope)) return
   const parsed = errorDetails(cause)
   if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此材料，已清除编辑内容。'); return }
   setPhase('error'); setMessage(parsed.code === 'not_found' ? '比较的版本不存在（可能不属于当前空间）。' : `版本比较暂时无法读取：${parsed.message}`)
  }
 }, [clearPrivate, client, materialId, phase, scopeController])

 const locked = phase === 'busy' || phase === 'unknown'
 const canSubmit = phase === 'idle' || phase === 'error'
 const saveBlocked = !canSubmit || revision === undefined || sections.length === 0
 const confirmBlocked = !canSubmit || revision === undefined || !materialId || view === undefined || dirty
 const retryAttempt = (): void => {
  if (!attempt) return
  if (attempt.kind === 'edit') void saveDraft(attempt)
  else void confirmDraft(attempt)
 }
 const updateSection = (index: number, patch: Partial<EditableSection>): void => setSections((current) => current.map((section, position) => position === index ? { ...section, ...patch } : section))
 const updateClaim = (sectionIndex: number, claimIndex: number, patch: Partial<EditableClaim>): void => setSections((current) => current.map((section, position) => position === sectionIndex ? { ...section, claims: section.claims.map((claim, index) => index === claimIndex ? { ...claim, ...patch } : claim) } : section))

 return <section className="wk-material" aria-labelledby="wk-material-title">
  <h2 id="wk-material-title">求职材料编辑</h2>
  <p>材料由同一结构化正文生成；确认正文后形成不可变版本，旧版本不会被覆盖。缺失信息以占位标注，不会由系统补造。</p>
  {phase === 'forbidden' || phase === 'scope-changed' ? <p className="wk-material__message wk-material__message--error" role="alert">{message}</p> : <>
   <p className="wk-material__revision" role="status">{revisionState === 'loading' ? '正在读取当前档案修订…' : revisionState === 'error' ? '暂时无法读取当前档案修订，可稍后重试；材料保存会被暂缓。' : revision !== undefined ? `当前档案修订 ${revision}（材料将按此修订固定）` : ''}</p>
   {view ? <dl className="wk-material__pinned" aria-label="材料固定的证据"><div><dt>岗位</dt><dd><code>{view.pinnedEvidence.opportunityId}</code></dd></div><div><dt>快照</dt><dd><code>{view.pinnedEvidence.snapshotId}</code></dd></div><div><dt>快照摘要</dt><dd><code>{view.pinnedEvidence.snapshotSha256}</code></dd></div><div><dt>档案修订</dt><dd>{view.pinnedEvidence.profileRevision}</dd></div></dl> : null}
   {view?.failureMessage ? <div className="wk-material__failure" role="alert"><strong>审阅失败（{view.failureCode}）：{view.failureMessage}</strong><p>草稿已保留，未发布任何版本。缺失信息请改为占位标注，或先在档案中确认该事实。</p></div> : null}
   <fieldset className="wk-material__editor" aria-label="结构化正文编辑">
    <legend>结构化正文</legend>
    {sections.length === 0 ? <p className="wk-material__hint">还没有章节。点击“新增章节”开始编辑正文；缺失的实习、证书或数字请用占位主张标注，不要补造。</p> : null}
    {sections.map((section, sectionIndex) => <fieldset className="wk-material__section" key={sectionIndex}>
     <legend>章节 {sectionIndex + 1}</legend>
     <label className="wk-material__label">标题<input aria-label={`章节 ${sectionIndex + 1} 标题`} value={section.heading} disabled={locked} onChange={(event) => updateSection(sectionIndex, { heading: event.target.value })} /></label>
     <label className="wk-material__label">正文<textarea aria-label={`章节 ${sectionIndex + 1} 正文`} rows={3} value={section.content} disabled={locked} onChange={(event) => updateSection(sectionIndex, { content: event.target.value })} /></label>
     {section.claims.map((claim, claimIndex) => <div className={`wk-material__claim${claim.needsReview ? ' wk-material__claim--needs-review' : ''}`} key={claim.claimId}>
      <p className="wk-material__claim-id">主张 <code>{claim.claimId}</code></p>
      <label className="wk-material__label">主张内容<textarea aria-label={`章节 ${sectionIndex + 1} 主张 ${claimIndex + 1} 内容`} rows={2} value={claim.text} disabled={locked} onChange={(event) => updateClaim(sectionIndex, claimIndex, { text: event.target.value })} /></label>
      <label className="wk-material__label">事实编号（已确认档案事实的键；留空即缺失占位）<input aria-label={`章节 ${sectionIndex + 1} 主张 ${claimIndex + 1} 事实编号`} value={claim.factKey} disabled={locked} placeholder="例如：学历" onChange={(event) => updateClaim(sectionIndex, claimIndex, { factKey: event.target.value })} /></label>
      <label className="wk-material__acknowledge"><input type="checkbox" aria-label={`章节 ${sectionIndex + 1} 主张 ${claimIndex + 1} 标记待审阅`} checked={claim.needsReview} disabled={locked} onChange={() => updateClaim(sectionIndex, claimIndex, { needsReview: !claim.needsReview })} /> 待审阅（needs_review）</label>
      <p className="wk-material__claim-state">{claim.factKey.trim() ? `已链接确认事实：${claim.factKey}` : '缺失/待补充（needs_review）：不得补造'}</p>
      {claim.reviewNote ? <p className="wk-material__claim-note">审阅备注：{claim.reviewNote}</p> : null}
      <button type="button" aria-label={`章节 ${sectionIndex + 1} 移除主张 ${claimIndex + 1}`} disabled={locked} onClick={() => updateSection(sectionIndex, { claims: section.claims.filter((_, index) => index !== claimIndex) })}>移除主张</button>
     </div>)}
     <div className="wk-material__actions">
      <button type="button" aria-label={`章节 ${sectionIndex + 1} 添加缺失占位主张`} disabled={locked} onClick={() => updateSection(sectionIndex, { claims: [...section.claims, { claimId: nextClaimId(sections), text: '', factKey: '', needsReview: true, reviewNote: '' }] })}>添加缺失占位主张</button>
      <button type="button" aria-label={`移除章节 ${sectionIndex + 1}`} disabled={locked} onClick={() => setSections((current) => current.filter((_, index) => index !== sectionIndex))}>移除章节</button>
     </div>
    </fieldset>)}
    <div className="wk-material__actions"><button type="button" aria-label="新增章节" disabled={locked} onClick={() => setSections((current) => [...current, { heading: '', content: '', claims: [] }])}>新增章节</button></div>
   </fieldset>
   {view && view.reviewRisks.length ? <section className="wk-material__risks" aria-label="审阅风险清单"><h3>审阅风险（{view.reviewRisks.length} 项）</h3><ul>{view.reviewRisks.map((risk, index) => <li key={`${risk.claimId ?? ''}-${index}`}><code>{risk.code}</code>：{risk.message}{risk.claimId ? <>（主张 <code>{risk.claimId}</code>）</> : null}</li>)}</ul></section> : null}
   {phase === 'unknown' && attempt ? <div className="wk-material__actions" role="group" aria-label="恢复材料写入"><button type="button" onClick={() => void lookupReceipt()}>查询材料回执</button><button type="button" onClick={retryAttempt}>用原请求编号重试</button></div> : <div className="wk-material__actions">
    <button type="button" className="wk-material__save" disabled={saveBlocked} onClick={() => void saveDraft()}>保存草稿</button>
    <button type="button" className="wk-material__confirm" disabled={confirmBlocked} title={dirty && materialId ? '正文有未保存修改：请先保存草稿再确认发布' : undefined} onClick={() => void confirmDraft()}>确认发布不可变版本</button>
    {phase === 'error' && revisionConflict !== undefined ? <button type="button" onClick={() => void readRevision()}>重新读取档案修订</button> : null}
   </div>}
   {message ? <p className={phase === 'error' ? 'wk-material__message wk-material__message--error' : 'wk-material__message'} role={phase === 'error' ? 'alert' : 'status'} aria-live="polite">{message}</p> : null}
   {view && view.versions.length ? <section className="wk-material__versions" aria-label="不可变版本列表">
    <h3>不可变版本（{view.versions.length}）</h3>
    <ul>{view.versions.map((version) => <li key={version.version}><strong>V{version.version}</strong> · <time dateTime={version.createdAt}>{version.createdAt}</time>
     <button type="button" aria-label={`只读查看 V${version.version}`} disabled={locked} onClick={() => void openVersion(version.version)}>只读查看</button>
     {version.version >= 2 ? <button type="button" aria-label={`比较 V${version.version - 1} 与 V${version.version}`} disabled={locked} onClick={() => void openComparison(version.version - 1, version.version)}>{`与 V${version.version - 1} 比较`}</button> : null}
    </li>)}</ul>
   </section> : null}
   {comparison ? <section className="wk-material__compare" aria-label="版本并排比较">
    <h3>版本 V{comparison.baseline.version} 与 V{comparison.target.version} 并排比较</h3>
    <div className="wk-material__compare-panes">
     <div className="wk-material__compare-pane"><h4>版本 V{comparison.baseline.version}（只读）</h4><BodyReadonly body={comparison.baseline.body} /></div>
     <div className="wk-material__compare-pane"><h4>版本 V{comparison.target.version}（只读）</h4><BodyReadonly body={comparison.target.body} /></div>
    </div>
    <h4>差异{comparison.changes.length ? `（${comparison.changes.length} 项）` : ''}</h4>
    <ul className="wk-material__changes" aria-label="版本差异">{comparison.changes.length === 0 ? <li>两个版本正文一致，无差异。</li> : comparison.changes.map((change, index) => <li key={index}>{changeText(change)}</li>)}</ul>
   </section> : null}
   {versionDetail ? <section className="wk-material__version-readonly" aria-label="版本只读回看">
    <h3>版本 V{versionDetail.version}（只读，不可修改）</h3>
    <p className="wk-material__version-meta">固定档案修订 {versionDetail.pinnedEvidence.profileRevision} · 事实基准修订 {versionDetail.factBasisRevision} · 确认请求 <code>{versionDetail.requestId}</code> · <time dateTime={versionDetail.createdAt}>{versionDetail.createdAt}</time></p>
    <BodyReadonly body={versionDetail.body} />
    <p className="wk-material__hint">不可变版本不能修改；最新草稿仍在编辑区。</p>
   </section> : null}
  </>}
 </section>
}
