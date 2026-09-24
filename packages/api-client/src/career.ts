import type { CareerAction, CareerChangeSet, CareerDocumentSource, CareerReceipt, CareerUpload, CareerView, Evaluation, EvaluationReceipt, OpportunityEvidence, OpportunityImportInput, OpportunityReceipt } from '../../career-core/src/contracts.ts'
import { decodeCareerReceipt, decodeCareerSources, decodeCareerUpload, decodeEvaluation, decodeEvaluationReceipt, decodeOpportunityEvidence, decodeOpportunityReceipt } from '../../career-core/src/contracts.ts'
import type { ClientRequest } from './client.ts'

export type CareerRequest = (input: ClientRequest) => Promise<unknown>
export function createCareerApi(request: CareerRequest) {
 return {
  async open(signal?: AbortSignal): Promise<CareerView> { return await request({ method: 'GET', path: '/api/v1/career/open', ...(signal ? { signal } : {}) }) as CareerView },
  async list(signal?: AbortSignal): Promise<CareerView> { return await request({ method: 'GET', path: '/api/v1/career/list', ...(signal ? { signal } : {}) }) as CareerView },
  async changes(since: number, signal?: AbortSignal): Promise<CareerChangeSet> { return await request({ method: 'GET', path: `/api/v1/career/changes?since=${encodeURIComponent(String(since))}`, ...(signal ? { signal } : {}) }) as CareerChangeSet },
  async receipt(requestId: string, signal?: AbortSignal): Promise<CareerReceipt> { return decodeCareerReceipt(await request({ method: 'GET', path: `/api/v1/career/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) })) },
  async act(action: CareerAction, signal?: AbortSignal): Promise<CareerReceipt> { return decodeCareerReceipt(await request({ method: 'POST', path: '/api/v1/career/act', body: action, ...(signal ? { signal } : {}) })) },
  async sources(signal?: AbortSignal): Promise<CareerDocumentSource[]> { return decodeCareerSources(await request({ method: 'GET', path: '/api/v1/career/sources', ...(signal ? { signal } : {}) })) },
  async upload(file: Blob, fileName: string, requestId: string, expectedRevision: number, signal?: AbortSignal): Promise<CareerUpload> {
   if (!requestId.trim()) throw new TypeError('career upload requestId must not be empty')
   const body = new FormData()
   body.append('file', file, fileName)
   body.append('requestId', requestId)
   body.append('expectedRevision', String(expectedRevision))
   return decodeCareerUpload(await request({ method: 'POST', path: '/api/v1/career/sources/upload', body, ...(signal ? { signal } : {}) }))
  },
  async importOpportunity(input: OpportunityImportInput, signal?: AbortSignal): Promise<OpportunityReceipt> {
   if (!input.requestId.trim()) throw new TypeError('opportunity import requestId must not be empty')
   if (!input.rawText.trim()) throw new TypeError('opportunity import rawText must not be empty')
   const body: OpportunityImportInput = { requestId: input.requestId, rawText: input.rawText, ...(input.sourceLabel !== undefined ? { sourceLabel: input.sourceLabel } : {}), ...(input.sourceReference !== undefined ? { sourceReference: input.sourceReference } : {}) }
   return decodeOpportunityReceipt(await request({ method: 'POST', path: '/api/v1/career/opportunities/import', body, ...(signal ? { signal } : {}) }))
  },
  async opportunityReceipt(requestId: string, signal?: AbortSignal): Promise<OpportunityReceipt> {
   if (!requestId.trim()) throw new TypeError('opportunity receipt requestId must not be empty')
   return decodeOpportunityReceipt(await request({ method: 'GET', path: `/api/v1/career/opportunities/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) }))
  },
  async opportunityEvidence(opportunityId: string, snapshotId: string, signal?: AbortSignal): Promise<OpportunityEvidence> {
   if (!opportunityId.trim() || !snapshotId.trim()) throw new TypeError('opportunity evidence IDs must not be empty')
   return decodeOpportunityEvidence(await request({ method: 'GET', path: `/api/v1/career/opportunities/${encodeURIComponent(opportunityId)}?snapshotId=${encodeURIComponent(snapshotId)}`, ...(signal ? { signal } : {}) }))
  },
  async evaluateOpportunity(input: { requestId: string; opportunityId: string; snapshotId: string; profileRevision?: number }, signal?: AbortSignal): Promise<EvaluationReceipt> {
   if (!input.requestId.trim() || !input.opportunityId.trim() || !input.snapshotId.trim() || (input.profileRevision !== undefined && (!Number.isSafeInteger(input.profileRevision) || input.profileRevision < 0))) throw new TypeError('evaluation request identifiers and revision must be valid')
   return decodeEvaluationReceipt(await request({ method: 'POST', path: '/api/v1/career/evaluations', body: input, ...(signal ? { signal } : {}) }))
  },
  async evaluationReceipt(requestId: string, signal?: AbortSignal): Promise<EvaluationReceipt> {
   if (!requestId.trim()) throw new TypeError('evaluation receipt requestId must not be empty')
   return decodeEvaluationReceipt(await request({ method: 'GET', path: `/api/v1/career/evaluations/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) }))
  },
  async evaluation(evaluationId: string, signal?: AbortSignal): Promise<Evaluation> {
   if (!evaluationId.trim()) throw new TypeError('evaluation ID must not be empty')
   return decodeEvaluation(await request({ method: 'GET', path: `/api/v1/career/evaluations/${encodeURIComponent(evaluationId)}`, ...(signal ? { signal } : {}) }))
  },
 }
}
