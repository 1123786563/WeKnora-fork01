import { decodeCareerReceipt, type CareerAction, type CareerChangeSet, type CareerReceipt, type CareerView } from './contracts.ts'

export interface CareerRemote {
 open(signal?: AbortSignal): Promise<CareerView>
 list(signal?: AbortSignal): Promise<CareerView>
 changes(since: number, signal?: AbortSignal): Promise<CareerChangeSet>
 act(action: CareerAction, signal?: AbortSignal): Promise<CareerReceipt>
 receipt(requestId: string, signal?: AbortSignal): Promise<CareerReceipt>
}

export interface CareerScope { userId: string | null; tenantId: string | null }
export class CareerDesk {
 private epoch = 0
 private controller?: AbortController
 private currentScope?: CareerScope
 private currentView?: CareerView
 constructor(private readonly remote: CareerRemote) {}
 get snapshot(): CareerView | undefined { return this.currentView }
 activate(userId: string | null, tenantId: string | null): void {
  if (this.currentScope?.userId === userId && this.currentScope.tenantId === tenantId) return
  this.epoch += 1
  this.controller?.abort()
  this.controller = new AbortController()
  this.currentScope = { userId, tenantId }
  this.currentView = undefined
 }
 clear(): void { this.activate(null, null) }
 private capture(): { epoch: number; signal: AbortSignal } {
  if (!this.currentScope?.userId || !this.currentScope.tenantId) throw Object.assign(new Error('Career workspace requires an active user and tenant'), { code: 'forbidden' })
  if (!this.controller || this.controller.signal.aborted) this.controller = new AbortController()
  return { epoch: this.epoch, signal: this.controller.signal }
 }
 private current(epoch: number): boolean { return epoch === this.epoch }
 private async read<T>(operation: (signal: AbortSignal) => Promise<T>, commit: (value: T) => void): Promise<T | undefined> {
  const { epoch, signal } = this.capture()
  const value = await operation(signal)
  if (!this.current(epoch)) return undefined
  commit(value)
  return value
 }
 async open(): Promise<CareerView | undefined> { return this.read((signal) => this.remote.open(signal), (value) => { this.currentView = value }) }
 async refresh(): Promise<CareerView | undefined> { return this.read((signal) => this.remote.list(signal), (value) => { this.currentView = value }) }
 async syncChanges(): Promise<CareerChangeSet | undefined> {
  const revision = this.currentView?.revision ?? 0
  return this.read((signal) => this.remote.changes(revision, signal), (set) => {
   if (!this.currentView) return
   const facts = [...this.currentView.facts]
   const proposals = [...this.currentView.proposals]
   for (const change of set.changes) {
    if (change.fact) { const index = facts.findIndex((fact) => fact.key === change.fact!.key); if (index < 0) facts.push(change.fact); else facts[index] = change.fact }
    if (change.proposal) { const index = proposals.findIndex((proposal) => proposal.id === change.proposal!.id); if (index < 0) proposals.push(change.proposal); else proposals[index] = change.proposal }
   }
   this.currentView = { revision: set.revision, facts, proposals }
  })
 }
 async reconcile(requestId: string): Promise<CareerReceipt | undefined> {
  return this.read((signal) => this.remote.receipt(requestId, signal).then(decodeCareerReceipt), (receipt) => {
   if (!this.currentView) return
   this.currentView = applyReceipt(this.currentView, receipt)
  })
 }
 async mutate(action: CareerAction): Promise<CareerReceipt | undefined> {
  const { epoch, signal } = this.capture()
  try {
   const receipt = decodeCareerReceipt(await this.remote.act(action, signal))
   if (!this.current(epoch)) return undefined
   if (this.currentView) this.currentView = applyReceipt(this.currentView, receipt)
   return receipt
  } catch (error) {
   if (!this.current(epoch)) return undefined
   if (errorCode(error) !== 'outcome_unknown') throw error
   // A mutation outcome must be checked using its original request ID before
   // callers can decide whether to retry. Keep the exact action in the caller.
   try {
    return await this.reconcile(action.requestId)
   } catch (receiptError) {
    if (errorCode(receiptError) === 'not_found') throw error
    throw receiptError
   }
  }
 }
}
function errorCode(error: unknown): string | undefined { return typeof error === 'object' && error !== null && 'code' in error ? String((error as { code: unknown }).code) : undefined }
function applyReceipt(view: CareerView, receipt: CareerReceipt): CareerView {
 const facts = [...view.facts]
 const proposals = [...view.proposals]
 if (receipt.kind === 'confirmed') {
  const index = facts.findIndex((fact) => fact.key === receipt.fact.key)
  if (index < 0) facts.push(receipt.fact); else facts[index] = receipt.fact
  if (receipt.proposal) { const p = proposals.findIndex((item) => item.id === receipt.proposal!.id); if (p < 0) proposals.push(receipt.proposal); else proposals[p] = receipt.proposal }
 } else {
  const index = proposals.findIndex((item) => item.id === receipt.proposal.id)
  if (index < 0) proposals.push(receipt.proposal); else proposals[index] = receipt.proposal
 }
 return { revision: Math.max(view.revision, receipt.revision), facts, proposals }
}
