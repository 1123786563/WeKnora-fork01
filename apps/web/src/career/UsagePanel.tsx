import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { Button, Card } from 'tdesign-react'
import type { WeKnoraClient } from '@weknora/api-client'
import type { ScopeController } from '@weknora/domain/scope'
import type { UsageEstimateView } from '../../../../packages/api-client/src/career.ts'
import './usage.css'

// T21 pre-execution usage estimate panel. Exactly one operation is charged —
// a search_once run, including every period a recurring rule triggers. The
// estimate endpoint is a free read-only projection; this panel displays the
// backend's frozen numbers and conditions verbatim and never recomputes a
// balance. When the estimate cannot be obtained the state is fail-closed:
// the typed reason is shown and no execute-first path is offered. An
// exhausted window blocks the next charged run while every stored archive,
// application, evaluation and search stays readable.
export type CareerUsageState =
 | { phase: 'loading' }
 | { phase: 'ready'; estimate: UsageEstimateView }
 | { phase: 'unavailable'; code?: string; reason: string }
 | { phase: 'forbidden'; reason: string }

function usageErrorDetails(cause: unknown): { code?: string; text: string } {
 const error = cause as { code?: string; message?: string }
 return { code: error?.code, text: error?.message || '请求未完成' }
}

// usageAllowsChargedRun is the single admission gate consumed by the charged
// entries (SearchPage submit, RulePage enable): a charged run may only start
// from a live, admitting estimate. Loading, unreadable, forbidden and
// exhausted estimates all fail closed — there is never an execute-first
// path that reports the cost afterwards.
export function usageAllowsChargedRun(usage: CareerUsageState): boolean {
 return usage.phase === 'ready' && usage.estimate.wouldAdmit
}

export function useCareerUsageEstimate(client: WeKnoraClient, scopeController: ScopeController): { state: CareerUsageState; reload: () => void } {
 const scope = scopeController.current().scope
 const [state, setState] = useState<CareerUsageState>({ phase: 'loading' })
 const [attempt, setAttempt] = useState(0)
 const reload = useCallback((): void => { setAttempt((current) => current + 1) }, [])
 useEffect(() => {
  const requestScope = scopeController.current()
  let cancelled = false
  setState({ phase: 'loading' })
  client.career.usageEstimate('search_once', requestScope.signal).then(
   (estimate) => {
    if (cancelled || !scopeController.isCurrent(requestScope.scope)) return
    setState({ phase: 'ready', estimate })
   },
   (cause) => {
    if (cancelled || !scopeController.isCurrent(requestScope.scope)) return
    const parsed = usageErrorDetails(cause)
    // The estimate is a free read, so forbidden means the space itself is
    // not usable; everything else (typed admission_unavailable, transport)
    // is the fail-closed unavailable state with its reason shown verbatim.
    if (parsed.code === 'forbidden') { setState({ phase: 'forbidden', reason: parsed.text }); return }
    setState({ phase: 'unavailable', code: parsed.code, reason: parsed.text })
   },
  )
  return () => { cancelled = true }
 }, [client, scopeController, scope.userId, scope.tenantId, scope.generation, attempt])
 return { state, reload }
}

function formatPeriodDay(timestamp: string): string {
 return timestamp.slice(0, 10)
}

export function CareerUsagePanel({ usage, onRetry }: { usage: CareerUsageState; onRetry: () => void }): ReactNode {
 if (usage.phase === 'loading') {
  return <section aria-label="额度预估" className="wk-career-usage-wrap"><Card><p role="status" className="wk-career-usage__status">正在获取额度预估…</p></Card></section>
 }
 if (usage.phase === 'forbidden') {
  return <section aria-label="额度预估" className="wk-career-usage-wrap"><Card><div role="alert"><strong>当前空间不可访问</strong><p>{usage.reason}</p></div><div className="wk-career-usage__actions"><Button variant="outline" onClick={onRetry}>重新获取预估</Button></div></Card></section>
 }
 if (usage.phase === 'unavailable') {
  return <section aria-label="额度预估" className="wk-career-usage-wrap"><Card><div role="alert"><strong>额度预估暂不可用</strong><p>{usage.reason}</p><p>在预估恢复前不会发起收费找岗（不会先执行后补报）。可稍后重新获取预估；期间既有档案、申请、评估与搜索记录仍可完整读取。</p></div><div className="wk-career-usage__actions"><Button variant="outline" onClick={onRetry}>重新获取预估</Button></div></Card></section>
 }
 const estimate = usage.estimate
 return <section aria-label="额度预估" className="wk-career-usage-wrap"><Card className="wk-career-usage">
  <h2>额度预估（执行前）</h2>
  <p className="wk-career-usage__cost">下一次找岗将消耗 {estimate.costUnits} 个额度单位（操作 {estimate.operation === 'search_once' ? '一次性找岗' : estimate.operation}；持续找岗规则的每次触发按同一口径计）。</p>
  {estimate.wouldAdmit ? null : <div role="alert" className="wk-career-usage__overage"><strong>本期额度已耗尽</strong><p>新的收费找岗已被阻止；既有档案、申请、评估与搜索记录仍可完整读取。额度在新的计费窗口自动恢复。</p></div>}
  <p className="wk-career-usage__balance">本期剩余 {estimate.remainingUnits} / {estimate.limitUnits} 个额度单位（已预占 {estimate.reservedUnits} · 已结算 {estimate.settledUnits}） · 计费窗口 {formatPeriodDay(estimate.periodStart)} 至 {formatPeriodDay(estimate.periodEnd)} UTC</p>
  <section aria-label="触发条件" className="wk-career-usage__conditions">
   <h3>触发条件（后端原文）</h3>
   <ul>{estimate.conditions.map((condition, index) => <li key={index}>{condition}</li>)}</ul>
  </section>
  <p className="wk-career-usage__note">以上为后端只读预估，前端如实展示、不重算；余额与判定以服务端额度台账为准。</p>
 </Card></section>
}
