import { decodeCareerReceipt, type CareerAction, type CareerChangeSet, type CareerReceipt, type CareerView } from './contracts.ts'

export interface CareerRemote {
 open(signal?: AbortSignal): Promise<CareerView>
 list(signal?: AbortSignal): Promise<CareerView>
 changes(since: number, signal?: AbortSignal): Promise<CareerChangeSet>
 act(action: CareerAction, signal?: AbortSignal): Promise<CareerReceipt>
 receipt(requestId: string, signal?: AbortSignal): Promise<CareerReceipt>
}

export interface CareerScope { userId: string | null; tenantId: string | null }
type OutcomeUnknown = Error & { code: 'outcome_unknown'; requestId: string; safeToRetry: boolean }
function outcomeUnknown(action: CareerAction, cause: unknown, safeToRetry: boolean): OutcomeUnknown {
 const error = new Error('Career action outcome is unknown; reconcile the original request before retrying', { cause }) as OutcomeUnknown
 error.code = 'outcome_unknown'; error.requestId = action.requestId; error.safeToRetry = safeToRetry
 return error
}
function errorCode(error: unknown): string | undefined { return typeof error === 'object' && error !== null && 'code' in error ? String((error as { code: unknown }).code) : undefined }
function isAmbiguousOutcome(error: unknown): boolean {
 const code = errorCode(error)
 if (code === 'outcome_unknown' || code === 'TIMEOUT' || code === 'NETWORK_ERROR' || code === 'CANCELLED') return true
 if (code && ['forbidden', 'revision_conflict', 'idempotency_conflict', 'invalid_request', 'not_found', 'proposal_resolved'].includes(code)) return false
 const status = typeof error === 'object' && error !== null && 'status' in error ? Number((error as { status: unknown }).status) : undefined
 return status === undefined || status >= 500
}
function sameAction(left: CareerAction, right: CareerAction): boolean { return JSON.stringify(left) === JSON.stringify(right) }

export class CareerDesk {
 private epoch = 0
 private controller?: AbortController
 private currentScope?: CareerScope
 private currentView?: CareerView
 private unresolved?: CareerAction
 private receiptMissing = false
 constructor(private readonly remote: CareerRemote) {}
 get snapshot(): CareerView | undefined { return this.currentView }
 get pendingAction(): CareerAction | undefined { return this.unresolved }
 get safeToRetry(): boolean { return this.receiptMissing }
 activate(userId: string | null, tenantId: string | null): void {
  if (this.currentScope?.userId === userId && this.currentScope.tenantId === tenantId) return
  this.epoch += 1
  this.controller?.abort()
  this.controller = new AbortController()
  this.currentScope = { userId, tenantId }
  this.currentView = undefined
  this.unresolved = undefined
  this.receiptMissing = false
 }
 clear(): void { this.activate(null, null) }
 private clearPrivateState(): void { this.currentView = undefined; this.unresolved = undefined; this.receiptMissing = false }
 private capture(): { epoch: number; signal: AbortSignal } {
  if (!this.currentScope?.userId || !this.currentScope.tenantId) throw Object.assign(new Error('Career workspace requires an active user and tenant'), { code: 'forbidden' })
  if (!this.controller || this.controller.signal.aborted) this.controller = new AbortController()
  return { epoch: this.epoch, signal: this.controller.signal }
 }
 private current(epoch: number): boolean { return epoch === this.epoch }
 private async read<T>(operation: (signal: AbortSignal) => Promise<T>, commit: (value: T) => void, result: () => T): Promise<T | undefined> {
  const { epoch, signal } = this.capture()
  try {
   const value = await operation(signal)
   if (!this.current(epoch)) return undefined
   commit(value)
   return result()
  } catch (error) {
   if (this.current(epoch) && errorCode(error) === 'forbidden') this.clearPrivateState()
   throw error
  }
 }
 async open(): Promise<CareerView | undefined> { return this.read((signal) => this.remote.open(signal), (candidate) => this.applyView(candidate), () => this.currentView!) }
 async refresh(): Promise<CareerView | undefined> { return this.read((signal) => this.remote.list(signal), (candidate) => this.applyView(candidate), () => this.currentView!) }
 async syncChanges(): Promise<CareerChangeSet | undefined> {
  let response: CareerChangeSet | undefined
  const since = this.currentView?.revision ?? 0
  return this.read((signal) => this.remote.changes(since, signal), (set) => {
   response = set
   if (!this.currentView || set.revision < this.currentView.revision) return
   const facts = [...this.currentView.facts]
   const proposals = [...this.currentView.proposals]
   for (const change of set.changes) {
    if (change.fact) mergeFact(facts, change.fact)
    if (change.proposal) mergeProposal(proposals, change.proposal)
   }
   this.currentView = { revision: Math.max(this.currentView.revision, set.revision), facts, proposals }
  }, () => response!)
 }
 async reconcile(requestId: string): Promise<CareerReceipt | undefined> {
  let reconciled: CareerReceipt | undefined
  try { return await this.read((signal) => this.remote.receipt(requestId, signal).then(decodeCareerReceipt), (receipt) => {
   reconciled = receipt
   if (this.currentView) this.currentView = applyReceipt(this.currentView, receipt)
   if (this.unresolved?.requestId === requestId) { this.unresolved = undefined; this.receiptMissing = false }
  }, () => reconciled!) } catch (error) { if (errorCode(error) === 'not_found' && this.unresolved?.requestId === requestId) this.receiptMissing = true; throw error }
 }
 async mutate(action: CareerAction): Promise<CareerReceipt | undefined> {
  if (this.unresolved) throw Object.assign(new Error('Resolve the pending Career action before starting another mutation'), { code: 'unresolved_action', requestId: this.unresolved.requestId })
  return this.send(action)
 }
 async retryUnknown(action: CareerAction): Promise<CareerReceipt | undefined> {
  if (!this.unresolved || !sameAction(this.unresolved, action)) throw Object.assign(new Error('Retry must use the exact pending action and request ID'), { code: 'retry_payload_mismatch' })
  if (!this.receiptMissing) {
   try {
    const existing = await this.reconcile(action.requestId)
    if (existing) return existing
   } catch (error) {
    if (errorCode(error) !== 'not_found') throw outcomeUnknown(action, error, false)
   }
  }
  try { return await this.send(action) } catch (error) {
   if (['revision_conflict', 'invalid_request', 'proposal_resolved', 'not_found'].includes(errorCode(error) ?? '')) { this.unresolved = undefined; this.receiptMissing = false }
   throw error
  }
 }
 private async send(action: CareerAction): Promise<CareerReceipt | undefined> {
  const { epoch, signal } = this.capture()
  try {
   const receipt = decodeCareerReceipt(await this.remote.act(action, signal))
   if (!this.current(epoch)) return undefined
   if (this.currentView) this.currentView = applyReceipt(this.currentView, receipt)
   if (this.unresolved?.requestId === action.requestId) { this.unresolved = undefined; this.receiptMissing = false }
   return receipt
  } catch (error) {
   if (!this.current(epoch)) return undefined
   if (errorCode(error) === 'forbidden') this.clearPrivateState()
   if (!isAmbiguousOutcome(error)) throw error
   this.unresolved = action
   this.receiptMissing = false
   try {
    const receipt = decodeCareerReceipt(await this.remote.receipt(action.requestId, signal))
    if (!this.current(epoch)) return undefined
    if (this.currentView) this.currentView = applyReceipt(this.currentView, receipt)
    this.unresolved = undefined; this.receiptMissing = false
    return receipt
   } catch (receiptError) {
    if (!this.current(epoch)) return undefined
    const missing = errorCode(receiptError) === 'not_found'
    this.receiptMissing = missing
    throw outcomeUnknown(action, receiptError, missing)
   }
  }
 }
 private applyView(candidate: CareerView): void {
  if (!this.currentView || candidate.revision >= this.currentView.revision) this.currentView = candidate
 }
}
function mergeFact(facts: CareerView['facts'], incoming: CareerView['facts'][number]): void {
 const index = facts.findIndex((fact) => fact.key === incoming.key)
 if (index < 0) facts.push(incoming)
 else if (incoming.revision >= facts[index]!.revision) facts[index] = incoming
}
function mergeProposal(proposals: CareerView['proposals'], incoming: CareerView['proposals'][number]): void {
 const index = proposals.findIndex((proposal) => proposal.id === incoming.id)
 if (index < 0) { proposals.push(incoming); return }
 const current = proposals[index]!
 if (current.status !== 'pending' && incoming.status === 'pending') return
 if (current.status === 'pending' || incoming.status !== 'pending' || (incoming.revision ?? 0) >= (current.revision ?? 0)) proposals[index] = incoming
}
function applyReceipt(view: CareerView, receipt: CareerReceipt): CareerView {
 const facts = [...view.facts]
 const proposals = [...view.proposals]
 if (receipt.kind === 'confirmed') {
  mergeFact(facts, receipt.fact)
  if (receipt.proposal) mergeProposal(proposals, receipt.proposal)
 } else mergeProposal(proposals, receipt.proposal)
 return { revision: Math.max(view.revision, receipt.revision), facts, proposals }
}
