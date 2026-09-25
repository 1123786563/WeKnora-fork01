import type { CareerAction, CareerChangeSet, CareerDocumentSource, CareerReceipt, CareerUpload, CareerView, Evaluation, EvaluationReceipt, OpportunityEvidence, OpportunityImportInput, OpportunityReceipt, OpportunitySource, OpportunityStatus } from '../../career-core/src/contracts.ts'
import { decodeCareerReceipt, decodeCareerSources, decodeCareerUpload, decodeEvaluation, decodeEvaluationReceipt, decodeOpportunityReceipt } from '../../career-core/src/contracts.ts'
import type { ClientRequest } from './client.ts'

export type CareerRequest = (input: ClientRequest) => Promise<unknown>

// Frozen application contract owned by the career backend (T14). The client
// consumes the enums verbatim; decoders reject invented link states,
// evaluation statuses, or malformed pinned evidence before they reach the UI.
export type ApplicationLinkState = 'linking' | 'ready' | 'link_failed'
export type ApplicationEvaluationStatus = 'eligible' | 'ineligible' | 'unknown'
export type ApplicationEvidencePin = { opportunityId: string; snapshotId: string; evaluationId: string; profileRevision: number; evaluationStatus: ApplicationEvaluationStatus; batchIdentity: string }
export type ApplicationWarning = { evaluationId: string; evaluationStatus: 'ineligible'; hardRuleId?: string; reasonCode?: string }
export type ApplicationReceipt = { applicationId: string; requestId: string; linkState: ApplicationLinkState; taskId?: string; runId?: string; qualified: boolean; warning?: ApplicationWarning; pinnedEvidence: ApplicationEvidencePin }
export type CreateApplicationInput = { requestId: string; opportunityId: string; snapshotId: string; evaluationId: string; batchIdentity: string; continueDespiteHardFailure: boolean; expectedRevision: number }

// Frozen source-integrity enums owned by the career backend (T09). The client
// consumes them verbatim: decoders reject any value outside the frozen sets,
// so invented statuses or failure codes can never reach the UI.
export type OpportunitySourceStatus = 'complete' | 'partial' | 'login_required' | 'blocked' | 'not_found' | 'timed_out' | 'fetch_failed' | 'policy_unverified'
export type OpportunityCompleteness = 'complete' | 'incomplete' | 'unknown'
export type OpportunityFailureCode = 'login_required' | 'access_blocked' | 'not_found' | 'timeout' | 'source_unverified' | 'unsupported_content' | 'empty_content' | 'response_too_large' | 'network_error' | 'redirect_disallowed'
export type OpportunityURLImportInput = { requestId: string; url: string }
export type OpportunityURLImportReceipt = { kind: 'opportunity_url_imported'; requestId: string; opportunityId: string; observationId: string; snapshotId: string; status: OpportunityStatus; sourceStatus: OpportunitySourceStatus; completeness: OpportunityCompleteness; failureCode?: OpportunityFailureCode; submittedUrl: string; acquiredAt: string; needsUserJD: boolean }
export type OpportunityObservation = { observationId: string; snapshotId: string; source: OpportunitySource; sourceStatus?: OpportunitySourceStatus; completeness?: OpportunityCompleteness; failureCode?: OpportunityFailureCode; submittedUrl?: string; finalUrl?: string; adapterId?: string; adapterVersion?: string; observedHttpStatus?: number; needsUserJD: boolean; acquiredAt: string }
export type OpportunityObservationList = { observations: OpportunityObservation[] }

const opportunitySourceStatuses: OpportunitySourceStatus[] = ['complete', 'partial', 'login_required', 'blocked', 'not_found', 'timed_out', 'fetch_failed', 'policy_unverified']
const opportunityCompletenesses: OpportunityCompleteness[] = ['complete', 'incomplete', 'unknown']
const opportunityFailureCodes: OpportunityFailureCode[] = ['login_required', 'access_blocked', 'not_found', 'timeout', 'source_unverified', 'unsupported_content', 'empty_content', 'response_too_large', 'network_error', 'redirect_disallowed']
const rfc3339Timestamp = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(Z|[+-]\d{2}:\d{2})$/

function decodeRecord(value: unknown, message: string): Record<string, unknown> {
 if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new TypeError(message)
 return value as Record<string, unknown>
}
function validIdentifier(value: unknown): value is string { return typeof value === 'string' && value.trim().length > 0 }
function validTimestamp(value: unknown): value is string { return typeof value === 'string' && rfc3339Timestamp.test(value) && Number.isFinite(Date.parse(value)) }
function validOptionalString(value: unknown): boolean { return value === undefined || typeof value === 'string' }
function validOptionalHttpStatus(value: unknown): boolean { return value === undefined || (typeof value === 'number' && Number.isSafeInteger(value) && value >= 0) }

export function decodeOpportunityURLImportReceipt(value: unknown): OpportunityURLImportReceipt {
 const record = decodeRecord(value, 'invalid opportunity URL import receipt')
 if (record.kind !== 'opportunity_url_imported' || !validIdentifier(record.requestId) || !validIdentifier(record.opportunityId) || !validIdentifier(record.observationId) || !validIdentifier(record.snapshotId) || (record.status !== 'stored' && record.status !== 'needs_review') || !opportunitySourceStatuses.includes(record.sourceStatus as OpportunitySourceStatus) || !opportunityCompletenesses.includes(record.completeness as OpportunityCompleteness) || (record.failureCode !== undefined && !opportunityFailureCodes.includes(record.failureCode as OpportunityFailureCode)) || !validIdentifier(record.submittedUrl) || !validTimestamp(record.acquiredAt) || typeof record.needsUserJD !== 'boolean') throw new TypeError('invalid opportunity URL import receipt')
 return {
  kind: 'opportunity_url_imported', requestId: record.requestId, opportunityId: record.opportunityId, observationId: record.observationId, snapshotId: record.snapshotId, status: record.status as OpportunityStatus,
  sourceStatus: record.sourceStatus as OpportunitySourceStatus, completeness: record.completeness as OpportunityCompleteness, ...(record.failureCode !== undefined ? { failureCode: record.failureCode as OpportunityFailureCode } : {}),
  submittedUrl: record.submittedUrl, acquiredAt: record.acquiredAt, needsUserJD: record.needsUserJD,
 }
}

export function decodeOpportunityObservation(value: unknown): OpportunityObservation {
 const record = decodeRecord(value, 'invalid opportunity observation')
 const source = decodeRecord(record.source, 'invalid opportunity observation')
 const sourceLabel: unknown = source.label
 const sourceReferenceId: unknown = source.referenceId
 const submittedUrl: unknown = record.submittedUrl
 const finalUrl: unknown = record.finalUrl
 const adapterId: unknown = record.adapterId
 const adapterVersion: unknown = record.adapterVersion
 const observedHttpStatus: unknown = record.observedHttpStatus
 if (!validIdentifier(record.observationId) || !validIdentifier(record.snapshotId) || !validIdentifier(source.kind) || !validOptionalString(sourceLabel) || !validOptionalString(sourceReferenceId) || (record.sourceStatus !== undefined && !opportunitySourceStatuses.includes(record.sourceStatus as OpportunitySourceStatus)) || (record.completeness !== undefined && !opportunityCompletenesses.includes(record.completeness as OpportunityCompleteness)) || (record.failureCode !== undefined && !opportunityFailureCodes.includes(record.failureCode as OpportunityFailureCode)) || !validOptionalString(submittedUrl) || !validOptionalString(finalUrl) || !validOptionalString(adapterId) || !validOptionalString(adapterVersion) || !validOptionalHttpStatus(observedHttpStatus) || typeof record.needsUserJD !== 'boolean' || !validTimestamp(record.acquiredAt)) throw new TypeError('invalid opportunity observation')
 return {
  observationId: record.observationId, snapshotId: record.snapshotId,
  source: { kind: source.kind as string, ...(typeof sourceLabel === 'string' ? { label: sourceLabel } : {}), ...(typeof sourceReferenceId === 'string' ? { referenceId: sourceReferenceId } : {}) },
  ...(record.sourceStatus !== undefined ? { sourceStatus: record.sourceStatus as OpportunitySourceStatus } : {}),
  ...(record.completeness !== undefined ? { completeness: record.completeness as OpportunityCompleteness } : {}),
  ...(record.failureCode !== undefined ? { failureCode: record.failureCode as OpportunityFailureCode } : {}),
  ...(typeof submittedUrl === 'string' ? { submittedUrl } : {}),
  ...(typeof finalUrl === 'string' ? { finalUrl } : {}),
  ...(typeof adapterId === 'string' ? { adapterId } : {}),
  ...(typeof adapterVersion === 'string' ? { adapterVersion } : {}),
  ...(typeof observedHttpStatus === 'number' ? { observedHttpStatus } : {}),
  needsUserJD: record.needsUserJD, acquiredAt: record.acquiredAt,
 }
}

export function decodeOpportunityObservations(value: unknown): OpportunityObservationList {
 const record = decodeRecord(value, 'invalid opportunity observation list')
 if (!Array.isArray(record.observations)) throw new TypeError('invalid opportunity observation list')
 return { observations: record.observations.map(decodeOpportunityObservation) }
}

// T09 backend failure observations intentionally persist an empty-text
// snapshot (SHA-256 of zero bytes, needs_review) so the observation stays
// traceable. The career-core contract decoder keeps rejecting those pages, so
// the client layer owns this evidence-page decode: rawText may be the empty
// string, while every other field keeps the same strict validation.
function decodeEvidenceValue(value: unknown, message: string): OpportunityEvidence['extracted'][keyof OpportunityEvidence['extracted']] {
 if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new TypeError(message)
 const record = value as Record<string, unknown>
 if (record.state === 'unknown' && record.value === undefined) return { state: 'unknown' }
 if (record.state === 'known' && typeof record.value === 'string' && record.value.trim().length > 0) return { state: 'known', value: record.value }
 throw new TypeError(message)
}

export function decodeOpportunityEvidencePage(value: unknown): OpportunityEvidence {
 const record = decodeRecord(value, 'invalid opportunity evidence')
 const source = decodeRecord(record.source, 'invalid opportunity evidence')
 const rawText: unknown = record.rawText
 if (!validIdentifier(record.opportunityId) || !validIdentifier(record.observationId) || !validIdentifier(record.snapshotId)
  || typeof rawText !== 'string' || (rawText.length > 0 && rawText.trim().length === 0)
  || typeof record.rawSha256 !== 'string' || !/^[a-f0-9]{64}$/.test(record.rawSha256)
  || typeof record.extracted !== 'object' || record.extracted === null || Array.isArray(record.extracted)
  || !validIdentifier(source.kind) || (source.label !== undefined && typeof source.label !== 'string') || (source.referenceId !== undefined && typeof source.referenceId !== 'string')
  || !validTimestamp(record.acquiredAt) || (record.status !== 'stored' && record.status !== 'needs_review')) throw new TypeError('invalid opportunity evidence')
 const extractedRecord = record.extracted as Record<string, unknown>
 const extracted: OpportunityEvidence['extracted'] = {
  title: decodeEvidenceValue(extractedRecord.title, 'invalid opportunity evidence'),
  company: decodeEvidenceValue(extractedRecord.company, 'invalid opportunity evidence'),
  location: decodeEvidenceValue(extractedRecord.location, 'invalid opportunity evidence'),
  batch: decodeEvidenceValue(extractedRecord.batch, 'invalid opportunity evidence'),
  requirements: decodeEvidenceValue(extractedRecord.requirements, 'invalid opportunity evidence'),
 }
 return {
  opportunityId: record.opportunityId, observationId: record.observationId, snapshotId: record.snapshotId,
  rawText, rawSha256: record.rawSha256, extracted,
  source: { kind: source.kind, ...(typeof source.label === 'string' ? { label: source.label } : {}), ...(typeof source.referenceId === 'string' ? { referenceId: source.referenceId } : {}) },
  acquiredAt: record.acquiredAt, status: record.status as OpportunityStatus,
 }
}

const applicationLinkStates: ApplicationLinkState[] = ['linking', 'ready', 'link_failed']
const applicationEvaluationStatuses: ApplicationEvaluationStatus[] = ['eligible', 'ineligible', 'unknown']
const applicationWarningKeys = ['evaluationId', 'evaluationStatus', 'hardRuleId', 'reasonCode']

function decodeApplicationPin(value: unknown): ApplicationEvidencePin {
 const record = decodeRecord(value, 'invalid application pinned evidence')
 if (!validIdentifier(record.opportunityId) || !validIdentifier(record.snapshotId) || !validIdentifier(record.evaluationId)
  || !Number.isSafeInteger(record.profileRevision) || Number(record.profileRevision) < 0
  || !applicationEvaluationStatuses.includes(record.evaluationStatus as ApplicationEvaluationStatus)
  || !validIdentifier(record.batchIdentity)) throw new TypeError('invalid application pinned evidence')
 return { opportunityId: record.opportunityId, snapshotId: record.snapshotId, evaluationId: record.evaluationId, profileRevision: record.profileRevision as number, evaluationStatus: record.evaluationStatus as ApplicationEvaluationStatus, batchIdentity: record.batchIdentity }
}

export function decodeApplicationReceipt(value: unknown): ApplicationReceipt {
 const record = decodeRecord(value, 'invalid application receipt')
 if (!validIdentifier(record.applicationId) || !validIdentifier(record.requestId)
  || !applicationLinkStates.includes(record.linkState as ApplicationLinkState)
  || (record.taskId !== undefined && !validIdentifier(record.taskId))
  || (record.runId !== undefined && !validIdentifier(record.runId))
  || typeof record.qualified !== 'boolean') throw new TypeError('invalid application receipt')
 let warning: ApplicationWarning | undefined
 if (record.warning !== undefined) {
  const raw = decodeRecord(record.warning, 'invalid application warning')
  if (Object.keys(raw).some((key) => !applicationWarningKeys.includes(key))
   || !validIdentifier(raw.evaluationId) || raw.evaluationStatus !== 'ineligible'
   || (raw.hardRuleId !== undefined && !validIdentifier(raw.hardRuleId))
   || (raw.reasonCode !== undefined && !validIdentifier(raw.reasonCode))) throw new TypeError('invalid application warning')
  warning = { evaluationId: raw.evaluationId, evaluationStatus: 'ineligible', ...(raw.hardRuleId !== undefined ? { hardRuleId: raw.hardRuleId } : {}), ...(raw.reasonCode !== undefined ? { reasonCode: raw.reasonCode } : {}) }
 }
 return {
  applicationId: record.applicationId, requestId: record.requestId, linkState: record.linkState as ApplicationLinkState,
  ...(record.taskId !== undefined ? { taskId: record.taskId } : {}), ...(record.runId !== undefined ? { runId: record.runId } : {}),
  qualified: record.qualified, ...(warning ? { warning } : {}), pinnedEvidence: decodeApplicationPin(record.pinnedEvidence),
 }
}

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
  async importUrl(input: OpportunityURLImportInput, signal?: AbortSignal): Promise<OpportunityURLImportReceipt> {
   if (!input.requestId.trim()) throw new TypeError('URL import requestId must not be empty')
   if (!input.url.trim()) throw new TypeError('URL import url must not be empty')
   return decodeOpportunityURLImportReceipt(await request({ method: 'POST', path: '/api/v1/career/opportunities/import-url', body: { requestId: input.requestId, url: input.url }, ...(signal ? { signal } : {}) }))
  },
  async importOpportunity(input: OpportunityImportInput & { opportunityId?: string; priorObservationId?: string }, signal?: AbortSignal): Promise<OpportunityReceipt> {
   if (!input.requestId.trim()) throw new TypeError('opportunity import requestId must not be empty')
   if (!input.rawText.trim()) throw new TypeError('opportunity import rawText must not be empty')
   if ((input.opportunityId !== undefined) !== (input.priorObservationId !== undefined)) throw new TypeError('opportunity append requires opportunityId and priorObservationId together')
   const body: OpportunityImportInput & { opportunityId?: string; priorObservationId?: string } = { requestId: input.requestId, rawText: input.rawText, ...(input.sourceLabel !== undefined ? { sourceLabel: input.sourceLabel } : {}), ...(input.sourceReference !== undefined ? { sourceReference: input.sourceReference } : {}), ...(input.opportunityId !== undefined ? { opportunityId: input.opportunityId } : {}), ...(input.priorObservationId !== undefined ? { priorObservationId: input.priorObservationId } : {}) }
   return decodeOpportunityReceipt(await request({ method: 'POST', path: '/api/v1/career/opportunities/import', body, ...(signal ? { signal } : {}) }))
  },
  async opportunityReceipt(requestId: string, signal?: AbortSignal): Promise<OpportunityReceipt> {
   if (!requestId.trim()) throw new TypeError('opportunity receipt requestId must not be empty')
   return decodeOpportunityReceipt(await request({ method: 'GET', path: `/api/v1/career/opportunities/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) }))
  },
  async opportunityObservations(opportunityId: string, signal?: AbortSignal): Promise<OpportunityObservationList> {
   if (!opportunityId.trim()) throw new TypeError('opportunity observations opportunityId must not be empty')
   return decodeOpportunityObservations(await request({ method: 'GET', path: `/api/v1/career/opportunities/${encodeURIComponent(opportunityId)}/observations`, ...(signal ? { signal } : {}) }))
  },
  async opportunityEvidence(opportunityId: string, snapshotId: string, signal?: AbortSignal): Promise<OpportunityEvidence> {
   if (!opportunityId.trim() || !snapshotId.trim()) throw new TypeError('opportunity evidence IDs must not be empty')
   return decodeOpportunityEvidencePage(await request({ method: 'GET', path: `/api/v1/career/opportunities/${encodeURIComponent(opportunityId)}?snapshotId=${encodeURIComponent(snapshotId)}`, ...(signal ? { signal } : {}) }))
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
  async createApplication(input: CreateApplicationInput, signal?: AbortSignal): Promise<ApplicationReceipt> {
   if (!input.requestId.trim() || !input.opportunityId.trim() || !input.snapshotId.trim() || !input.evaluationId.trim() || !input.batchIdentity.trim() || !Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new TypeError('application request identifiers and revision must be valid')
   return decodeApplicationReceipt(await request({ method: 'POST', path: '/api/v1/career/applications', body: input, ...(signal ? { signal } : {}) }))
  },
  async applicationReceipt(requestId: string, signal?: AbortSignal): Promise<ApplicationReceipt> {
   if (!requestId.trim()) throw new TypeError('application receipt requestId must not be empty')
   return decodeApplicationReceipt(await request({ method: 'GET', path: `/api/v1/career/applications/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) }))
  },
  async application(applicationId: string, signal?: AbortSignal): Promise<ApplicationReceipt> {
   if (!applicationId.trim()) throw new TypeError('application ID must not be empty')
   return decodeApplicationReceipt(await request({ method: 'GET', path: `/api/v1/career/applications/${encodeURIComponent(applicationId)}`, ...(signal ? { signal } : {}) }))
  },
  async reconcileApplicationLink(requestId: string, signal?: AbortSignal): Promise<ApplicationReceipt> {
   if (!requestId.trim()) throw new TypeError('application reconcile requestId must not be empty')
   return decodeApplicationReceipt(await request({ method: 'POST', path: '/api/v1/career/applications/link/reconcile', body: { requestId }, ...(signal ? { signal } : {}) }))
  },
 }
}
