export type CareerFact = { key: string; value: string; revision: number; confirmedAt: string }
export type CareerProposal = { key: string; value: string; createdAt: string }
export type CareerView = { revision: number; facts: CareerFact[]; proposals: CareerProposal[] }
export type CareerReceipt = { requestId: string; revision: number; fact: CareerFact }
export type CareerErrorCode = 'unauthorized' | 'revision_conflict' | 'idempotency_conflict' | 'invalid_request' | 'internal'
export type CareerError = { code: CareerErrorCode; message: string; currentRevision?: number }
export type CareerAction = { key: string; value: string; requestId: string; expectedRevision: number; confirmed: boolean }
export type CareerChangeSet = { revision: number; changes: CareerFact[] }
