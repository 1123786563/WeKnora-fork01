export type CareerSource = { kind: string; label?: string; referenceId?: string }
export type CareerConfirmation = { userId: string; confirmedAt: string }
export type CareerFact = { key: string; value: string; revision: number; source: CareerSource; confirmation: CareerConfirmation; confirmedAt: string }
export type CareerProposal = { id: string; key: string; value: string; source: CareerSource; status: 'pending' | 'confirmed' | 'dismissed'; revision?: number; confirmation?: CareerConfirmation; resolutionSource?: CareerSource; createdAt: string }
export type CareerView = { revision: number; facts: CareerFact[]; proposals: CareerProposal[] }
export type CareerReceipt =
  | { kind: 'proposed'; requestId: string; revision: number; proposal: CareerProposal }
  | { kind: 'confirmed'; requestId: string; revision: number; proposal?: CareerProposal; fact: CareerFact }
  | { kind: 'dismissed'; requestId: string; revision: number; proposal: CareerProposal }
export type CareerAction =
  | { action: 'propose'; key: string; value: string; source: CareerSource; requestId: string; expectedRevision: number }
  | { action: 'confirm'; key: string; value: string; source: CareerSource; requestId: string; expectedRevision: number }
  | { action: 'confirm_proposal'; proposalId: string; source: CareerSource; requestId: string; expectedRevision: number }
  | { action: 'dismiss'; proposalId: string; source: CareerSource; requestId: string; expectedRevision: number }
export type CareerChange = { revision: number; kind: 'proposed' | 'confirmed' | 'dismissed'; proposal?: CareerProposal; fact?: CareerFact }
export type CareerChangeSet = { revision: number; changes: CareerChange[] }
export type CareerErrorCode = 'forbidden' | 'revision_conflict' | 'idempotency_conflict' | 'invalid_request' | 'not_found' | 'proposal_resolved' | 'outcome_unknown' | 'internal'
export type CareerError =
  | { error: { code: 'outcome_unknown'; message: string; requestId: string } }
  | { error: { code: Exclude<CareerErrorCode, 'outcome_unknown'>; message: string; currentRevision?: number } }
function isRecord(value: unknown): value is Record<string, unknown> { return !!value && typeof value === 'object' }
function validSource(value: unknown): boolean { return isRecord(value) && typeof value.kind === 'string' && (value.label === undefined || typeof value.label === 'string') && (value.referenceId === undefined || typeof value.referenceId === 'string') }
function validProposal(value: unknown): boolean {
 if (!isRecord(value)) return false
 const validConfirmation = value.confirmation === undefined || (isRecord(value.confirmation) && typeof value.confirmation.userId === 'string' && typeof value.confirmation.confirmedAt === 'string')
 return typeof value.id === 'string' && typeof value.key === 'string' && typeof value.value === 'string' && validSource(value.source) && ['pending', 'confirmed', 'dismissed'].includes(String(value.status)) && typeof value.createdAt === 'string' && (value.revision === undefined || typeof value.revision === 'number') && validConfirmation && (value.resolutionSource === undefined || validSource(value.resolutionSource))
}
function validFact(value: unknown): boolean {
 if (!isRecord(value) || !isRecord(value.confirmation)) return false
 return typeof value.key === 'string' && typeof value.value === 'string' && typeof value.revision === 'number' && validSource(value.source) && typeof value.confirmation.userId === 'string' && typeof value.confirmation.confirmedAt === 'string' && typeof value.confirmedAt === 'string'
}
export function decodeCareerReceipt(value: unknown): CareerReceipt {
 if (!isRecord(value) || typeof value.requestId !== 'string' || typeof value.revision !== 'number') throw new TypeError('invalid career receipt')
 if (value.kind === 'proposed' && validProposal(value.proposal)) return value as CareerReceipt
 if (value.kind === 'confirmed' && validFact(value.fact) && (value.proposal === undefined || validProposal(value.proposal))) return value as CareerReceipt
 if (value.kind === 'dismissed' && validProposal(value.proposal)) return value as CareerReceipt
 throw new TypeError('invalid career receipt discriminator payload')
}
export function decodeCareerError(value: unknown): CareerError {
 if (!isRecord(value) || !isRecord(value.error)) throw new TypeError('invalid career error')
 const body = value.error
 const codes: CareerErrorCode[] = ['forbidden', 'revision_conflict', 'idempotency_conflict', 'invalid_request', 'not_found', 'proposal_resolved', 'outcome_unknown', 'internal']
 if (typeof body.message !== 'string' || !codes.includes(body.code as CareerErrorCode) || (body.currentRevision !== undefined && typeof body.currentRevision !== 'number') || (body.requestId !== undefined && typeof body.requestId !== 'string') || (body.code === 'outcome_unknown' && typeof body.requestId !== 'string')) throw new TypeError('invalid career error')
 return value as CareerError
}
