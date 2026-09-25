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

// Frozen one-shot search enums owned by the career backend (T11,
// internal/modules/career/search_once.go). A search is never a continuous
// rule: the client consumes the enums verbatim and decoders reject any
// invented status, qualification, uncertainty, or failure code before it
// reaches the UI. Receipt-level failures are the two bounded terminal codes;
// per-source coverage failures reuse the bounded source fetch codes plus the
// dedicated misconfiguration code.
export type SearchOnceInput = { requestId: string; query: string; expectedRevision: number }
export type SearchStatus = 'completed' | 'failed'
export type SearchQualification = 'needs_review' | 'qualified' | 'not_qualified'
export type SearchUncertainty = 'low_confidence'
export type SearchFailureCode = 'no_vetted_sources' | 'all_sources_unavailable'
export type SearchCoverageFailureCode = 'source_misconfigured' | 'login_required' | 'access_blocked' | 'not_found' | 'timeout' | 'source_unverified' | 'unsupported_content' | 'empty_content' | 'response_too_large' | 'network_error' | 'redirect_disallowed'
export type SearchSourceCoverage = { sourceId: string; label: string; accessMethods: string[]; cities: string[]; available: boolean; failureCode?: SearchCoverageFailureCode }
export type SearchCoverage = { sources: SearchSourceCoverage[] }
export type SearchResultRow = { resultId: string; sourceId: string; link: string; checkedAt: string; qualification: SearchQualification; uncertainty: SearchUncertainty }
export type SearchOnceReceipt = { kind: 'search_once'; requestId: string; searchId: string; status: SearchStatus; query: string; coverage: SearchCoverage; scopeNotes: string[]; failureCode?: SearchFailureCode; results: SearchResultRow[]; checkedAt: string }

const searchStatuses: SearchStatus[] = ['completed', 'failed']
const searchQualifications: SearchQualification[] = ['needs_review', 'qualified', 'not_qualified']
const searchUncertainties: SearchUncertainty[] = ['low_confidence']
const searchFailureCodes: SearchFailureCode[] = ['no_vetted_sources', 'all_sources_unavailable']
const searchCoverageFailureCodes: SearchCoverageFailureCode[] = ['source_misconfigured', 'login_required', 'access_blocked', 'not_found', 'timeout', 'source_unverified', 'unsupported_content', 'empty_content', 'response_too_large', 'network_error', 'redirect_disallowed']

// Backend rows only ever contain links that literally appeared in fetched
// source text as absolute http(s) URLs; the decoder holds the same line.
function validHttpUrl(value: unknown): value is string {
 if (typeof value !== 'string') return false
 return value.startsWith('http://') || value.startsWith('https://')
}

function decodeStringArray(value: unknown, message: string): string[] {
 if (!Array.isArray(value) || value.some((item) => typeof item !== 'string')) throw new TypeError(message)
 return value as string[]
}

function decodeSearchSourceCoverage(value: unknown): SearchSourceCoverage {
 const record = decodeRecord(value, 'invalid search source coverage')
 const failureCode: unknown = record.failureCode
 if (!validIdentifier(record.sourceId) || typeof record.label !== 'string'
  || (failureCode !== undefined && !searchCoverageFailureCodes.includes(failureCode as SearchCoverageFailureCode))
  || typeof record.available !== 'boolean') throw new TypeError('invalid search source coverage')
 return {
  sourceId: record.sourceId, label: record.label,
  accessMethods: decodeStringArray(record.accessMethods, 'invalid search source coverage'),
  cities: decodeStringArray(record.cities, 'invalid search source coverage'),
  available: record.available,
  ...(failureCode !== undefined ? { failureCode: failureCode as SearchCoverageFailureCode } : {}),
 }
}

export function decodeSearchOnceReceipt(value: unknown): SearchOnceReceipt {
 const record = decodeRecord(value, 'invalid search receipt')
 const coverage = decodeRecord(record.coverage, 'invalid search receipt')
 const failureCode: unknown = record.failureCode
 if (record.kind !== 'search_once' || !validIdentifier(record.requestId) || !validIdentifier(record.searchId)
  || !searchStatuses.includes(record.status as SearchStatus)
  || !validIdentifier(record.query) || !Array.isArray(coverage.sources)
  || (failureCode !== undefined && !searchFailureCodes.includes(failureCode as SearchFailureCode))
  || !validTimestamp(record.checkedAt)) throw new TypeError('invalid search receipt')
 const results = record.results
 if (!Array.isArray(results)) throw new TypeError('invalid search receipt')
 return {
  kind: 'search_once', requestId: record.requestId, searchId: record.searchId,
  status: record.status as SearchStatus, query: record.query,
  coverage: { sources: coverage.sources.map(decodeSearchSourceCoverage) },
  scopeNotes: decodeStringArray(record.scopeNotes, 'invalid search receipt'),
  ...(failureCode !== undefined ? { failureCode: failureCode as SearchFailureCode } : {}),
  results: results.map((row) => {
   const rowRecord = decodeRecord(row, 'invalid search result row')
   if (!validIdentifier(rowRecord.resultId) || !validIdentifier(rowRecord.sourceId)
    || !validHttpUrl(rowRecord.link) || !validTimestamp(rowRecord.checkedAt)
    || !searchQualifications.includes(rowRecord.qualification as SearchQualification)
    || !searchUncertainties.includes(rowRecord.uncertainty as SearchUncertainty)) throw new TypeError('invalid search result row')
   return { resultId: rowRecord.resultId, sourceId: rowRecord.sourceId, link: rowRecord.link, checkedAt: rowRecord.checkedAt, qualification: rowRecord.qualification as SearchQualification, uncertainty: rowRecord.uncertainty as SearchUncertainty }
  }),
  checkedAt: record.checkedAt,
 }
}

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

// Frozen material contract owned by the career backend (T15,
// internal/modules/career/material.go). One structured body feeds every
// client; confirmation makes an append-only immutable version and missing
// items stay explicit needs_review placeholders instead of fabricated
// values. Decoders reject invented kinds, statuses, risk codes, change
// kinds, or malformed pinned evidence before they reach the UI.
export type MaterialKind = 'material_edited' | 'material_confirmed'
export type MaterialStatus = 'draft' | 'failed' | 'confirmed'
export type MaterialRiskCode = 'missing_placeholder' | 'needs_review'
export type MaterialChangeKind = 'section_added' | 'section_removed' | 'section_changed' | 'claim_added' | 'claim_removed' | 'claim_changed'
export type MaterialClaim = { claimId: string; text: string; factKey?: string; needsReview: boolean; reviewNote?: string }
export type MaterialSection = { heading: string; content: string; claims: MaterialClaim[] }
export type MaterialBody = { sections: MaterialSection[] }
export type MaterialEvidencePin = { opportunityId: string; snapshotId: string; snapshotSha256: string; profileRevision: number }
export type MaterialReviewRisk = { code: MaterialRiskCode; message: string; claimId?: string }
export type MaterialReceipt = { kind: MaterialKind; requestId: string; materialId: string; status: MaterialStatus; version?: number; pinnedEvidence: MaterialEvidencePin; body: MaterialBody; reviewRisks: MaterialReviewRisk[]; failureCode?: string; failureMessage?: string }
export type EditMaterialInput = { requestId: string; materialId?: string; opportunityId?: string; snapshotId?: string; body: MaterialBody; expectedRevision: number }
export type ConfirmMaterialInput = { requestId: string; materialId: string; expectedRevision: number }
export type MaterialVersionSummary = { version: number; createdAt: string }
export type MaterialVersionList = { materialId: string; versions: MaterialVersionSummary[] }
export type MaterialVersionView = { version: number; pinnedEvidence: MaterialEvidencePin; factBasisRevision: number; body: MaterialBody; reviewRisks: MaterialReviewRisk[]; requestId: string; createdAt: string }
export type MaterialVersionChange = { kind: MaterialChangeKind; heading?: string; claimId?: string; baseline?: string; target?: string }
export type MaterialVersionComparison = { materialId: string; baseline: MaterialVersionView; target: MaterialVersionView; changes: MaterialVersionChange[] }
export type MaterialView = { materialId: string; status: MaterialStatus; pinnedEvidence: MaterialEvidencePin; body: MaterialBody; reviewRisks: MaterialReviewRisk[]; failureCode?: string; failureMessage?: string; versionCount: number; versions: MaterialVersionSummary[]; createdAt: string; updatedAt: string }

const materialKinds: MaterialKind[] = ['material_edited', 'material_confirmed']
const materialStatuses: MaterialStatus[] = ['draft', 'failed', 'confirmed']
const materialRiskCodes: MaterialRiskCode[] = ['missing_placeholder', 'needs_review']
const materialChangeKinds: MaterialChangeKind[] = ['section_added', 'section_removed', 'section_changed', 'claim_added', 'claim_removed', 'claim_changed']

function validOptionalIdentifier(value: unknown): boolean { return value === undefined || validIdentifier(value) }
function validPositiveVersion(value: unknown): value is number { return typeof value === 'number' && Number.isSafeInteger(value) && value > 0 }
function validRevision(value: unknown): value is number { return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0 }

function decodeMaterialPin(value: unknown): MaterialEvidencePin {
 const record = decodeRecord(value, 'invalid material pinned evidence')
 if (!validIdentifier(record.opportunityId) || !validIdentifier(record.snapshotId)
  || typeof record.snapshotSha256 !== 'string' || !/^[a-f0-9]{64}$/.test(record.snapshotSha256)
  || !validRevision(record.profileRevision)) throw new TypeError('invalid material pinned evidence')
 return { opportunityId: record.opportunityId, snapshotId: record.snapshotId, snapshotSha256: record.snapshotSha256, profileRevision: record.profileRevision as number }
}

function decodeMaterialClaim(value: unknown): MaterialClaim {
 const record = decodeRecord(value, 'invalid material claim')
 if (!validIdentifier(record.claimId) || typeof record.text !== 'string'
  || !validOptionalIdentifier(record.factKey) || typeof record.needsReview !== 'boolean'
  || !validOptionalString(record.reviewNote)) throw new TypeError('invalid material claim')
 return {
  claimId: record.claimId, text: record.text,
  ...(validIdentifier(record.factKey) ? { factKey: record.factKey } : {}),
  needsReview: record.needsReview,
  ...(typeof record.reviewNote === 'string' ? { reviewNote: record.reviewNote } : {}),
 }
}

function decodeMaterialBody(value: unknown): MaterialBody {
 const record = decodeRecord(value, 'invalid material body')
 if (!Array.isArray(record.sections) || record.sections.length === 0) throw new TypeError('invalid material body')
 return { sections: record.sections.map((section) => {
  const sectionRecord = decodeRecord(section, 'invalid material section')
  if (!validIdentifier(sectionRecord.heading) || typeof sectionRecord.content !== 'string' || !Array.isArray(sectionRecord.claims)) throw new TypeError('invalid material section')
  return { heading: sectionRecord.heading, content: sectionRecord.content, claims: sectionRecord.claims.map(decodeMaterialClaim) }
 }) }
}

function decodeMaterialRisks(value: unknown): MaterialReviewRisk[] {
 if (!Array.isArray(value)) throw new TypeError('invalid material review risks')
 return value.map((risk) => {
  const record = decodeRecord(risk, 'invalid material review risk')
  if (!materialRiskCodes.includes(record.code as MaterialRiskCode) || !validIdentifier(record.message) || !validOptionalIdentifier(record.claimId)) throw new TypeError('invalid material review risk')
  return { code: record.code as MaterialRiskCode, message: record.message, ...(validIdentifier(record.claimId) ? { claimId: record.claimId } : {}) }
 })
}

export function decodeMaterialReceipt(value: unknown): MaterialReceipt {
 const record = decodeRecord(value, 'invalid material receipt')
 if (!materialKinds.includes(record.kind as MaterialKind) || !validIdentifier(record.requestId) || !validIdentifier(record.materialId)
  || !materialStatuses.includes(record.status as MaterialStatus)
  || (record.version !== undefined && !validPositiveVersion(record.version))
  || !validOptionalString(record.failureCode) || !validOptionalString(record.failureMessage)) throw new TypeError('invalid material receipt')
 return {
  kind: record.kind as MaterialKind, requestId: record.requestId, materialId: record.materialId, status: record.status as MaterialStatus,
  ...(validPositiveVersion(record.version) ? { version: record.version } : {}),
  pinnedEvidence: decodeMaterialPin(record.pinnedEvidence), body: decodeMaterialBody(record.body), reviewRisks: decodeMaterialRisks(record.reviewRisks),
  ...(typeof record.failureCode === 'string' ? { failureCode: record.failureCode } : {}),
  ...(typeof record.failureMessage === 'string' ? { failureMessage: record.failureMessage } : {}),
 }
}

function decodeMaterialVersionSummary(value: unknown): MaterialVersionSummary {
 const record = decodeRecord(value, 'invalid material version summary')
 if (!validPositiveVersion(record.version) || !validTimestamp(record.createdAt)) throw new TypeError('invalid material version summary')
 return { version: record.version, createdAt: record.createdAt }
}

export function decodeMaterialView(value: unknown): MaterialView {
 const record = decodeRecord(value, 'invalid material view')
 if (!validIdentifier(record.materialId) || !materialStatuses.includes(record.status as MaterialStatus)
  || !validRevision(record.versionCount) || !Array.isArray(record.versions)
  || !validOptionalString(record.failureCode) || !validOptionalString(record.failureMessage)
  || !validTimestamp(record.createdAt) || !validTimestamp(record.updatedAt)) throw new TypeError('invalid material view')
 return {
  materialId: record.materialId, status: record.status as MaterialStatus,
  pinnedEvidence: decodeMaterialPin(record.pinnedEvidence), body: decodeMaterialBody(record.body), reviewRisks: decodeMaterialRisks(record.reviewRisks),
  ...(typeof record.failureCode === 'string' ? { failureCode: record.failureCode } : {}),
  ...(typeof record.failureMessage === 'string' ? { failureMessage: record.failureMessage } : {}),
  versionCount: record.versionCount as number, versions: record.versions.map(decodeMaterialVersionSummary),
  createdAt: record.createdAt, updatedAt: record.updatedAt,
 }
}

export function decodeMaterialVersionList(value: unknown): MaterialVersionList {
 const record = decodeRecord(value, 'invalid material version list')
 if (!validIdentifier(record.materialId) || !Array.isArray(record.versions)) throw new TypeError('invalid material version list')
 return { materialId: record.materialId, versions: record.versions.map(decodeMaterialVersionSummary) }
}

export function decodeMaterialVersionView(value: unknown): MaterialVersionView {
 const record = decodeRecord(value, 'invalid material version')
 if (!validPositiveVersion(record.version) || !validRevision(record.factBasisRevision)
  || !validIdentifier(record.requestId) || !validTimestamp(record.createdAt)) throw new TypeError('invalid material version')
 return {
  version: record.version, pinnedEvidence: decodeMaterialPin(record.pinnedEvidence), factBasisRevision: record.factBasisRevision as number,
  body: decodeMaterialBody(record.body), reviewRisks: decodeMaterialRisks(record.reviewRisks),
  requestId: record.requestId, createdAt: record.createdAt,
 }
}

function decodeMaterialChange(value: unknown): MaterialVersionChange {
 const record = decodeRecord(value, 'invalid material version change')
 if (!materialChangeKinds.includes(record.kind as MaterialChangeKind)
  || !validOptionalIdentifier(record.heading) || !validOptionalIdentifier(record.claimId)
  || !validOptionalString(record.baseline) || !validOptionalString(record.target)) throw new TypeError('invalid material version change')
 return {
  kind: record.kind as MaterialChangeKind,
  ...(validIdentifier(record.heading) ? { heading: record.heading } : {}),
  ...(validIdentifier(record.claimId) ? { claimId: record.claimId } : {}),
  ...(typeof record.baseline === 'string' ? { baseline: record.baseline } : {}),
  ...(typeof record.target === 'string' ? { target: record.target } : {}),
 }
}

export function decodeMaterialComparison(value: unknown): MaterialVersionComparison {
 const record = decodeRecord(value, 'invalid material comparison')
 if (!validIdentifier(record.materialId) || !Array.isArray(record.changes)) throw new TypeError('invalid material comparison')
 return { materialId: record.materialId, baseline: decodeMaterialVersionView(record.baseline), target: decodeMaterialVersionView(record.target), changes: record.changes.map(decodeMaterialChange) }
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
  async searchOnce(input: SearchOnceInput, signal?: AbortSignal): Promise<SearchOnceReceipt> {
   if (!input.requestId.trim() || !input.query.trim()) throw new TypeError('search requestId and query must not be empty')
   if (!Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new TypeError('search expected revision must be a non-negative integer')
   return decodeSearchOnceReceipt(await request({ method: 'POST', path: '/api/v1/career/searches', body: { requestId: input.requestId, query: input.query, expectedRevision: input.expectedRevision }, ...(signal ? { signal } : {}) }))
  },
  async searchReceipt(requestId: string, signal?: AbortSignal): Promise<SearchOnceReceipt> {
   if (!requestId.trim()) throw new TypeError('search receipt requestId must not be empty')
   return decodeSearchOnceReceipt(await request({ method: 'GET', path: `/api/v1/career/searches/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) }))
  },
  async search(searchId: string, signal?: AbortSignal): Promise<SearchOnceReceipt> {
   if (!searchId.trim()) throw new TypeError('search ID must not be empty')
   return decodeSearchOnceReceipt(await request({ method: 'GET', path: `/api/v1/career/searches/${encodeURIComponent(searchId)}`, ...(signal ? { signal } : {}) }))
  },
  async editMaterial(input: EditMaterialInput, signal?: AbortSignal): Promise<MaterialReceipt> {
   if (!input.requestId.trim()) throw new TypeError('material requestId must not be empty')
   if (!Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new TypeError('material expected revision must be a non-negative integer')
   if (!input.materialId?.trim() && (!input.opportunityId?.trim() || !input.snapshotId?.trim())) throw new TypeError('material creation requires opportunity and snapshot IDs')
   if (!Array.isArray(input.body?.sections) || input.body.sections.length === 0) throw new TypeError('material body must contain at least one section')
   const body: Record<string, unknown> = { requestId: input.requestId, ...(input.materialId?.trim() ? { materialId: input.materialId.trim() } : { opportunityId: input.opportunityId!.trim(), snapshotId: input.snapshotId!.trim() }), body: input.body, expectedRevision: input.expectedRevision }
   return decodeMaterialReceipt(await request({ method: 'POST', path: '/api/v1/career/materials', body, ...(signal ? { signal } : {}) }))
  },
  async confirmMaterial(input: ConfirmMaterialInput, signal?: AbortSignal): Promise<MaterialReceipt> {
   if (!input.requestId.trim() || !input.materialId.trim()) throw new TypeError('material confirm requestId and materialId must not be empty')
   if (!Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new TypeError('material confirm expected revision must be a non-negative integer')
   return decodeMaterialReceipt(await request({ method: 'POST', path: '/api/v1/career/materials/confirm', body: { requestId: input.requestId, materialId: input.materialId, expectedRevision: input.expectedRevision }, ...(signal ? { signal } : {}) }))
  },
  async materialReceipt(requestId: string, signal?: AbortSignal): Promise<MaterialReceipt> {
   if (!requestId.trim()) throw new TypeError('material receipt requestId must not be empty')
   return decodeMaterialReceipt(await request({ method: 'GET', path: `/api/v1/career/materials/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) }))
  },
  async material(materialId: string, signal?: AbortSignal): Promise<MaterialView> {
   if (!materialId.trim()) throw new TypeError('material ID must not be empty')
   return decodeMaterialView(await request({ method: 'GET', path: `/api/v1/career/materials/${encodeURIComponent(materialId)}`, ...(signal ? { signal } : {}) }))
  },
  async materialVersions(materialId: string, signal?: AbortSignal): Promise<MaterialVersionList> {
   if (!materialId.trim()) throw new TypeError('material ID must not be empty')
   return decodeMaterialVersionList(await request({ method: 'GET', path: `/api/v1/career/materials/${encodeURIComponent(materialId)}/versions`, ...(signal ? { signal } : {}) }))
  },
  async materialVersion(materialId: string, version: number, signal?: AbortSignal): Promise<MaterialVersionView> {
   if (!materialId.trim()) throw new TypeError('material ID must not be empty')
   if (!Number.isSafeInteger(version) || version <= 0) throw new TypeError('material version must be a positive integer')
   return decodeMaterialVersionView(await request({ method: 'GET', path: `/api/v1/career/materials/${encodeURIComponent(materialId)}/versions/${version}`, ...(signal ? { signal } : {}) }))
  },
  async compareMaterialVersions(materialId: string, baseline: number, target: number, signal?: AbortSignal): Promise<MaterialVersionComparison> {
   if (!materialId.trim()) throw new TypeError('material ID must not be empty')
   if (!Number.isSafeInteger(baseline) || baseline <= 0 || !Number.isSafeInteger(target) || target <= 0) throw new TypeError('material compare versions must be positive integers')
   return decodeMaterialComparison(await request({ method: 'GET', path: `/api/v1/career/materials/${encodeURIComponent(materialId)}/versions/${target}/compare?baseline=${baseline}`, ...(signal ? { signal } : {}) }))
  },
 }
}
