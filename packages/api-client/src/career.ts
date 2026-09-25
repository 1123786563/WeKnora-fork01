import type { CareerAction, CareerChangeSet, CareerDocumentSource, CareerFact, CareerProposal, CareerReceipt, CareerSource, CareerUpload, CareerView, Evaluation, EvaluationReceipt, OpportunityEvidence, OpportunityImportInput, OpportunityReceipt, OpportunitySource, OpportunityStatus } from '../../career-core/src/contracts.ts'
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

// Frozen recurring search rule enums owned by the career backend (T13,
// internal/modules/career/search_rule.go). A rule only ever runs when the
// user explicitly enables it; blocked runs (quota refused, no vetted
// sources) are durable visible statuses, never silent skips. The
// enable-time estimate is the backend's deterministic projection — the
// client displays it verbatim (basis text included) and never recomputes it.
export type RuleStatus = 'enabled' | 'paused' | 'disabled'
export type RuleRunStatus = 'completed' | 'failed' | 'blocked_no_quota' | 'no_vetted_sources'
export type RuleTodoStatus = 'open'
export type RuleCostEstimate = { triggersPerDay: number; sourcesPerTrigger: number; estimatedSearchesPerDay: number; basis: string }
export type SetRuleInput = { requestId: string; ruleId?: string; query: string; intervalMinutes: number; status: RuleStatus; expectedRevision: number }
export type SetRuleReceipt = { kind: 'rule_set'; requestId: string; ruleId: string; query: string; intervalMinutes: number; status: RuleStatus; revision: number; nextDueAt?: string; estimate: RuleCostEstimate }
export type RuleRunView = { kind: 'rule_run'; ruleId: string; period: number; requestId: string; status: RuleRunStatus; searchId?: string; failureCode?: string; note?: string; triggeredAt: string }
export type RuleTodoView = { todoId: string; ruleId: string; runId: string; searchId: string; sourceId?: string; link: string; status: RuleTodoStatus; createdAt: string }
export type RuleView = { ruleId: string; query: string; intervalMinutes: number; status: RuleStatus; revision: number; lastPeriod: number; nextDueAt?: string; estimate: RuleCostEstimate; runs: RuleRunView[]; todos: RuleTodoView[]; createdAt: string; updatedAt: string }

const ruleStatuses: RuleStatus[] = ['enabled', 'paused', 'disabled']
const ruleRunStatuses: RuleRunStatus[] = ['completed', 'failed', 'blocked_no_quota', 'no_vetted_sources']
const minRuleIntervalMinutes = 1
const maxRuleIntervalMinutes = 43200 // 30 days, mirroring the backend bounds

function decodeRuleEstimate(value: unknown): RuleCostEstimate {
 const record = decodeRecord(value, 'invalid rule estimate')
 if (!validEstimateNumber(record.triggersPerDay) || !validEstimateNumber(record.estimatedSearchesPerDay)
  || !Number.isSafeInteger(record.sourcesPerTrigger) || Number(record.sourcesPerTrigger) < 0
  || typeof record.basis !== 'string' || record.basis.trim().length === 0) throw new TypeError('invalid rule estimate')
 return { triggersPerDay: record.triggersPerDay as number, sourcesPerTrigger: record.sourcesPerTrigger as number, estimatedSearchesPerDay: record.estimatedSearchesPerDay as number, basis: record.basis }
}
function validEstimateNumber(value: unknown): value is number { return typeof value === 'number' && Number.isFinite(value) && value >= 0 }

export function decodeSetRuleReceipt(value: unknown): SetRuleReceipt {
 const record = decodeRecord(value, 'invalid rule receipt')
 if (record.kind !== 'rule_set' || !validIdentifier(record.requestId) || !validIdentifier(record.ruleId)
  || !validIdentifier(record.query)
  || !Number.isSafeInteger(record.intervalMinutes) || Number(record.intervalMinutes) < minRuleIntervalMinutes || Number(record.intervalMinutes) > maxRuleIntervalMinutes
  || !ruleStatuses.includes(record.status as RuleStatus)
  || !Number.isSafeInteger(record.revision) || Number(record.revision) < 1
  || (record.nextDueAt !== undefined && !validTimestamp(record.nextDueAt))) throw new TypeError('invalid rule receipt')
 return {
  kind: 'rule_set', requestId: record.requestId, ruleId: record.ruleId, query: record.query,
  intervalMinutes: record.intervalMinutes as number, status: record.status as RuleStatus, revision: record.revision as number,
  ...(record.nextDueAt !== undefined ? { nextDueAt: record.nextDueAt } : {}),
  estimate: decodeRuleEstimate(record.estimate),
 }
}

function decodeRuleRun(value: unknown): RuleRunView {
 const record = decodeRecord(value, 'invalid rule run')
 const failureCode: unknown = record.failureCode
 const note: unknown = record.note
 if (record.kind !== 'rule_run' || !validIdentifier(record.ruleId) || !validIdentifier(record.requestId)
  || !Number.isSafeInteger(record.period) || Number(record.period) < 1
  || !ruleRunStatuses.includes(record.status as RuleRunStatus)
  || (record.searchId !== undefined && !validIdentifier(record.searchId))
  || (failureCode !== undefined && !searchFailureCodes.includes(failureCode as SearchFailureCode))
  || (note !== undefined && (typeof note !== 'string' || note.trim().length === 0))
  || !validTimestamp(record.triggeredAt)) throw new TypeError('invalid rule run')
 return {
  kind: 'rule_run', ruleId: record.ruleId, period: record.period as number, requestId: record.requestId,
  status: record.status as RuleRunStatus,
  ...(record.searchId !== undefined ? { searchId: record.searchId } : {}),
  ...(failureCode !== undefined ? { failureCode: failureCode as SearchFailureCode } : {}),
  ...(note !== undefined ? { note: note as string } : {}),
  triggeredAt: record.triggeredAt,
 }
}

function decodeRuleTodo(value: unknown): RuleTodoView {
 const record = decodeRecord(value, 'invalid rule todo')
 if (!validIdentifier(record.todoId) || !validIdentifier(record.ruleId) || !validIdentifier(record.runId)
  || !validIdentifier(record.searchId) || (record.sourceId !== undefined && !validIdentifier(record.sourceId))
  || !validHttpUrl(record.link) || record.status !== 'open' || !validTimestamp(record.createdAt)) throw new TypeError('invalid rule todo')
 return {
  todoId: record.todoId, ruleId: record.ruleId, runId: record.runId, searchId: record.searchId,
  ...(record.sourceId !== undefined ? { sourceId: record.sourceId } : {}),
  link: record.link, status: 'open', createdAt: record.createdAt,
 }
}

export function decodeRuleView(value: unknown): RuleView {
 const record = decodeRecord(value, 'invalid rule view')
 if (!validIdentifier(record.ruleId) || !validIdentifier(record.query)
  || !Number.isSafeInteger(record.intervalMinutes) || Number(record.intervalMinutes) < minRuleIntervalMinutes || Number(record.intervalMinutes) > maxRuleIntervalMinutes
  || !ruleStatuses.includes(record.status as RuleStatus)
  || !Number.isSafeInteger(record.revision) || Number(record.revision) < 1
  || !Number.isSafeInteger(record.lastPeriod) || Number(record.lastPeriod) < 0
  || (record.nextDueAt !== undefined && !validTimestamp(record.nextDueAt))
  || !Array.isArray(record.runs) || !Array.isArray(record.todos)
  || !validTimestamp(record.createdAt) || !validTimestamp(record.updatedAt)) throw new TypeError('invalid rule view')
 return {
  ruleId: record.ruleId, query: record.query, intervalMinutes: record.intervalMinutes as number,
  status: record.status as RuleStatus, revision: record.revision as number, lastPeriod: record.lastPeriod as number,
  ...(record.nextDueAt !== undefined ? { nextDueAt: record.nextDueAt } : {}),
  estimate: decodeRuleEstimate(record.estimate),
  runs: record.runs.map(decodeRuleRun), todos: record.todos.map(decodeRuleTodo),
  createdAt: record.createdAt, updatedAt: record.updatedAt,
 }
}

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

// Frozen export contract owned by the career backend (T16,
// internal/modules/career/rendering.go). One publish renders a same-body
// PDF/DOCX pair of one immutable material version; both files record the
// same content digest and version binding; only the both-verified export is
// submittable; revocation kills already-issued download grants immediately.
// Decoders reject invented kinds, statuses, formats, or broken digests
// before they reach the UI, including a receipt that claims submittable
// while a format record says its verification failed.
export type MaterialExportKind = 'material_published' | 'material_export_revoked'
export type MaterialExportStatus = 'staged' | 'submittable' | 'failed' | 'revoked'
export type MaterialExportFormat = 'pdf' | 'docx'
export type MaterialExportFailureCode = 'export_verification_failed'
export type MaterialExportedFile = { format: MaterialExportFormat; materialId: string; version: number; contentDigest: string; objectKey?: string; fileDigest?: string; size?: number; verified: boolean; error?: string }
export type MaterialExportReceipt = { kind: MaterialExportKind; requestId: string; exportId: string; materialId: string; version: number; status: MaterialExportStatus; submittable: boolean; contentDigest: string; files: MaterialExportedFile[]; failureCode?: MaterialExportFailureCode; failureMessage?: string; createdAt: string; revokedAt?: string }
export type MaterialExportList = { materialId: string; exports: MaterialExportReceipt[] }
// A grant is a short-lived download authorization bound to the issuing
// owner, the export, and one format; `digest` is the SHA-256 of the stored
// file bytes (what a downloaded blob must hash to), not the body digest.
export type MaterialExportDownload = { exportId: string; materialId: string; version: number; format: MaterialExportFormat; digest: string; size: number; expiresAt: number; signature: string; url: string }
export type PublishMaterialInput = { requestId: string; materialId: string; version: number; expectedRevision: number }
export type RevokeMaterialExportInput = { requestId: string; materialId: string; exportId: string; expectedRevision: number }
export type MaterialExportFile = { format: MaterialExportFormat; digest: string; size: number; body: string | Blob | ArrayBuffer; contentType?: string }

const materialExportKinds: MaterialExportKind[] = ['material_published', 'material_export_revoked']
const materialExportStatuses: MaterialExportStatus[] = ['staged', 'submittable', 'failed', 'revoked']
const materialExportFormats: MaterialExportFormat[] = ['pdf', 'docx']
// MaxExportGrantTTL in rendering.go: a grant may live at most 15 minutes.
export const MAX_EXPORT_GRANT_TTL_SECONDS = 900
// The browser download is an authenticated redemption: the client rebuilds
// the path from the decoded grant fields and sends it through the binary
// transport (which carries Authorization), never following grant.url itself.
export type CareerBinaryRequest = (input: ClientRequest) => Promise<{ body: string | Blob | ArrayBuffer; contentType?: string; headers: Record<string, string> }>

const sha256Hex = /^[a-f0-9]{64}$/

function decodeMaterialExportedFile(value: unknown): MaterialExportedFile {
 const record = decodeRecord(value, 'invalid material export file')
 const objectKey: unknown = record.objectKey
 const fileDigest: unknown = record.fileDigest
 const size: unknown = record.size
 if (!materialExportFormats.includes(record.format as MaterialExportFormat) || !validIdentifier(record.materialId)
  || !validPositiveVersion(record.version)
  || typeof record.contentDigest !== 'string' || !sha256Hex.test(record.contentDigest)
  || (fileDigest !== undefined && (typeof fileDigest !== 'string' || !sha256Hex.test(fileDigest)))
  || (objectKey !== undefined && !validOptionalIdentifier(objectKey))
  || (size !== undefined && (typeof size !== 'number' || !Number.isSafeInteger(size) || size < 0))
  || typeof record.verified !== 'boolean' || !validOptionalString(record.error)) throw new TypeError('invalid material export file')
 return {
  format: record.format as MaterialExportFormat, materialId: record.materialId, version: record.version as number, contentDigest: record.contentDigest,
  ...(validIdentifier(objectKey) ? { objectKey: objectKey as string } : {}),
  ...(typeof fileDigest === 'string' ? { fileDigest } : {}),
  ...(typeof size === 'number' ? { size } : {}),
  verified: record.verified, ...(typeof record.error === 'string' ? { error: record.error } : {}),
 }
}

export function decodeMaterialExportReceipt(value: unknown): MaterialExportReceipt {
 const record = decodeRecord(value, 'invalid material export receipt')
 if (!materialExportKinds.includes(record.kind as MaterialExportKind) || !validIdentifier(record.requestId) || !validIdentifier(record.exportId) || !validIdentifier(record.materialId)
  || !validPositiveVersion(record.version)
  || !materialExportStatuses.includes(record.status as MaterialExportStatus)
  || typeof record.submittable !== 'boolean'
  || typeof record.contentDigest !== 'string' || !sha256Hex.test(record.contentDigest)
  || !Array.isArray(record.files)
  || (record.failureCode !== undefined && record.failureCode !== 'export_verification_failed')
  || !validOptionalString(record.failureMessage) || !validTimestamp(record.createdAt)
  || (record.revokedAt !== undefined && !validTimestamp(record.revokedAt))) throw new TypeError('invalid material export receipt')
 const files = record.files.map(decodeMaterialExportedFile)
 // Coherence frozen from the backend state machine: a publish is submittable
 // exactly when the status says so with both format records verified; a
 // revoked export is never submittable and always carries the audit stamp.
 const revoked = record.kind === 'material_export_revoked'
 if (revoked !== validTimestamp(record.revokedAt)) throw new TypeError('invalid material export receipt')
 if ((record.status === 'submittable') !== record.submittable) throw new TypeError('invalid material export receipt')
 if (revoked && (record.status !== 'revoked' || record.submittable)) throw new TypeError('invalid material export receipt')
 if (!revoked && record.status === 'revoked') throw new TypeError('invalid material export receipt')
 if (record.status === 'submittable' && !(files.length === 2 && files.every((file) => file.verified))) throw new TypeError('invalid material export receipt')
 // Both format records must carry the export's shared body digest; a file
 // bound to a different body digest is a tampered pair, not the same export.
 if (files.some((file) => file.contentDigest !== record.contentDigest)) throw new TypeError('invalid material export receipt')
 return {
  kind: record.kind as MaterialExportKind, requestId: record.requestId, exportId: record.exportId, materialId: record.materialId,
  version: record.version as number, status: record.status as MaterialExportStatus, submittable: record.submittable, contentDigest: record.contentDigest, files,
  ...(record.failureCode !== undefined ? { failureCode: record.failureCode as MaterialExportFailureCode } : {}),
  ...(typeof record.failureMessage === 'string' ? { failureMessage: record.failureMessage } : {}),
  createdAt: record.createdAt, ...(typeof record.revokedAt === 'string' ? { revokedAt: record.revokedAt } : {}),
 }
}

export function decodeMaterialExportList(value: unknown): MaterialExportList {
 const record = decodeRecord(value, 'invalid material export list')
 if (!validIdentifier(record.materialId) || !Array.isArray(record.exports)) throw new TypeError('invalid material export list')
 return { materialId: record.materialId, exports: record.exports.map(decodeMaterialExportReceipt) }
}

export function decodeMaterialExportDownload(value: unknown): MaterialExportDownload {
 const record = decodeRecord(value, 'invalid material export download grant')
 const size: unknown = record.size
 const expiresAt: unknown = record.expiresAt
 // The URL is only a server-echoed relative path; the client never fetches
 // it directly, so anything absolute (or off the career export seam) is an
 // invented payload and rejected before the UI sees the grant.
 if (!validIdentifier(record.exportId) || !validIdentifier(record.materialId) || !validPositiveVersion(record.version)
  || !materialExportFormats.includes(record.format as MaterialExportFormat)
  || typeof record.digest !== 'string' || !sha256Hex.test(record.digest)
  || !Number.isSafeInteger(size) || (size as number) < 0
  || !Number.isSafeInteger(expiresAt) || (expiresAt as number) <= 0
  || !validIdentifier(record.signature) || typeof record.url !== 'string' || !record.url.startsWith('/api/v1/career/materials/')) throw new TypeError('invalid material export download grant')
 return { exportId: record.exportId, materialId: record.materialId, version: record.version as number, format: record.format as MaterialExportFormat, digest: record.digest, size: size as number, expiresAt: expiresAt as number, signature: record.signature, url: record.url }
}

// Frozen progress contract owned by the career backend (T17,
// internal/modules/career/progress.go). The timeline is append-only: event
// types and the seven-stage projection are closed enums and a correction
// appends a referencing event instead of rewriting history. Decoders reject
// invented kinds, stages, event types, or provenance before they reach the UI.
export type ProgressStage = 'preparing' | 'pending_submission' | 'submitted' | 'assessment' | 'interview' | 'offer' | 'closed'
export type ProgressEventType = 'pending_submission' | 'submitted' | 'assessment' | 'interview' | 'offer' | 'resubmitted' | 'rejected' | 'withdrawn' | 'retracted'
export type ProgressKind = 'progress_appended' | 'progress_corrected'
export type ProgressSourceKind = 'manual' | 'user' | 'system_import'
export type ProgressSource = { kind: ProgressSourceKind; label?: string; referenceId?: string }
export type ProgressReceipt = { kind: ProgressKind; requestId: string; applicationId: string; eventId: string; seq: number; revision: number; correctsEventId?: string; eventType: ProgressEventType; stage: ProgressStage; note?: string; occurredAt: string; source: ProgressSource; confirmer: string; createdAt: string }
export type ProgressEventView = { eventId: string; seq: number; kind: ProgressKind; eventType: ProgressEventType; note?: string; occurredAt: string; source: ProgressSource; confirmer: string; correctsEventId?: string; corrected: boolean; requestId: string; createdAt: string }
export type ProgressView = { applicationId: string; revision: number; stage: ProgressStage; events: ProgressEventView[] }
// The server pins HTTP provenance to manual entry and overwrites the field
// (internal/modules/career/handler.go progressClientSource); the client never
// claims system_import, so the write inputs carry no source at all.
export type AppendProgressInput = { requestId: string; applicationId: string; eventType: ProgressEventType; note?: string; occurredAt?: string; expectedRevision: number }
export type CorrectProgressInput = AppendProgressInput & { correctsEventId: string }

const progressStages: ProgressStage[] = ['preparing', 'pending_submission', 'submitted', 'assessment', 'interview', 'offer', 'closed']
const progressEventTypes: ProgressEventType[] = ['pending_submission', 'submitted', 'assessment', 'interview', 'offer', 'resubmitted', 'rejected', 'withdrawn', 'retracted']
const progressKinds: ProgressKind[] = ['progress_appended', 'progress_corrected']
const progressSourceKinds: ProgressSourceKind[] = ['manual', 'user', 'system_import']

function validPositiveSeq(value: unknown): value is number { return typeof value === 'number' && Number.isSafeInteger(value) && value > 0 }

function decodeProgressSource(value: unknown): ProgressSource {
 const record = decodeRecord(value, 'invalid progress source')
 if (!progressSourceKinds.includes(record.kind as ProgressSourceKind) || !validOptionalString(record.label) || !validOptionalString(record.referenceId)) throw new TypeError('invalid progress source')
 return { kind: record.kind as ProgressSourceKind, ...(typeof record.label === 'string' ? { label: record.label } : {}), ...(typeof record.referenceId === 'string' ? { referenceId: record.referenceId } : {}) }
}

export function decodeProgressReceipt(value: unknown): ProgressReceipt {
 const record = decodeRecord(value, 'invalid progress receipt')
 if (!progressKinds.includes(record.kind as ProgressKind) || !validIdentifier(record.requestId) || !validIdentifier(record.applicationId)
  || !validIdentifier(record.eventId) || !validPositiveSeq(record.seq) || !validPositiveSeq(record.revision)
  || !progressEventTypes.includes(record.eventType as ProgressEventType)
  || !progressStages.includes(record.stage as ProgressStage)
  || !validOptionalIdentifier(record.correctsEventId) || !validOptionalString(record.note)
  || !validTimestamp(record.occurredAt) || !validIdentifier(record.confirmer) || !validTimestamp(record.createdAt)) throw new TypeError('invalid progress receipt')
 // A correction receipt always names its target; an append never does.
 if ((record.kind === 'progress_corrected') !== validIdentifier(record.correctsEventId)) throw new TypeError('invalid progress receipt')
 return {
  kind: record.kind as ProgressKind, requestId: record.requestId, applicationId: record.applicationId, eventId: record.eventId,
  seq: record.seq as number, revision: record.revision as number,
  ...(validIdentifier(record.correctsEventId) ? { correctsEventId: record.correctsEventId } : {}),
  eventType: record.eventType as ProgressEventType, stage: record.stage as ProgressStage,
  ...(typeof record.note === 'string' ? { note: record.note } : {}),
  occurredAt: record.occurredAt, source: decodeProgressSource(record.source), confirmer: record.confirmer, createdAt: record.createdAt,
 }
}

function decodeProgressEvent(value: unknown): ProgressEventView {
 const record = decodeRecord(value, 'invalid progress event')
 if (!validIdentifier(record.eventId) || !validPositiveSeq(record.seq) || !progressKinds.includes(record.kind as ProgressKind)
  || !progressEventTypes.includes(record.eventType as ProgressEventType)
  || !validOptionalString(record.note) || !validTimestamp(record.occurredAt) || !validIdentifier(record.confirmer)
  || !validOptionalIdentifier(record.correctsEventId) || typeof record.corrected !== 'boolean'
  || !validIdentifier(record.requestId) || !validTimestamp(record.createdAt)) throw new TypeError('invalid progress event')
 if ((record.kind === 'progress_corrected') !== validIdentifier(record.correctsEventId)) throw new TypeError('invalid progress event')
 return {
  eventId: record.eventId, seq: record.seq as number, kind: record.kind as ProgressKind,
  eventType: record.eventType as ProgressEventType,
  ...(typeof record.note === 'string' ? { note: record.note } : {}),
  occurredAt: record.occurredAt, source: decodeProgressSource(record.source), confirmer: record.confirmer,
  ...(validIdentifier(record.correctsEventId) ? { correctsEventId: record.correctsEventId } : {}),
  corrected: record.corrected, requestId: record.requestId, createdAt: record.createdAt,
 }
}

export function decodeProgressView(value: unknown): ProgressView {
 const record = decodeRecord(value, 'invalid progress view')
 if (!validIdentifier(record.applicationId) || !validRevision(record.revision)
  || !progressStages.includes(record.stage as ProgressStage) || !Array.isArray(record.events)) throw new TypeError('invalid progress view')
 return { applicationId: record.applicationId, revision: record.revision as number, stage: record.stage as ProgressStage, events: record.events.map(decodeProgressEvent) }
}

// Frozen submission contract owned by the career backend (T18,
// internal/modules/career/submission.go). One application holds at most one
// user-confirmed submission; the version reference is either an exact
// submittable export binding frozen at recording time or the explicit
// unknown marker. The client consumes the enums verbatim and never lets an
// invented channel, a binding that disagrees with its confirmation flag, or a
// two-row list for one application reach the UI.
export type SubmissionChannel = 'email' | 'web' | 'other'
export type SubmissionVersionBinding = { materialId: string; exportId: string; version: number; contentDigest: string }
export type SubmissionReceipt = { kind: 'submission_recorded'; requestId: string; applicationId: string; submissionId: string; channel: SubmissionChannel; occurredAt: string; versionConfirmed: boolean; boundVersion?: SubmissionVersionBinding; note?: string; confirmer: string; revision: number; createdAt: string }
export type RecordSubmissionInput = { requestId: string; applicationId: string; channel: SubmissionChannel; occurredAt?: string; materialId?: string; exportId?: string; versionUnknown: boolean; note?: string; expectedRevision: number }
export type SubmissionList = { submissions: SubmissionReceipt[] }

const submissionChannels: SubmissionChannel[] = ['email', 'web', 'other']
// maxSubmissionNoteBytes in internal/modules/career/submission.go.
const maxSubmissionNoteBytes = 4096

export function decodeSubmissionReceipt(value: unknown): SubmissionReceipt {
 const record = decodeRecord(value, 'invalid submission receipt')
 if (record.kind !== 'submission_recorded' || !validIdentifier(record.requestId) || !validIdentifier(record.applicationId)
  || !validIdentifier(record.submissionId) || !submissionChannels.includes(record.channel as SubmissionChannel)
  || !validTimestamp(record.occurredAt) || typeof record.versionConfirmed !== 'boolean'
  || !validOptionalString(record.note) || !validIdentifier(record.confirmer)
  || !validRevision(record.revision) || !validTimestamp(record.createdAt)) throw new TypeError('invalid submission receipt')
 // The confirmation flag and the binding are one fact: a confirmed version
 // always freezes the exact export reference; an explicit unknown never
 // carries one. Anything else is an invented payload.
 const bound = record.boundVersion === undefined ? undefined : (() => {
  const binding = decodeRecord(record.boundVersion, 'invalid submission version binding')
  if (!validIdentifier(binding.materialId) || !validIdentifier(binding.exportId)
   || !validPositiveVersion(binding.version)
   || typeof binding.contentDigest !== 'string' || !sha256Hex.test(binding.contentDigest)) throw new TypeError('invalid submission version binding')
  return { materialId: binding.materialId as string, exportId: binding.exportId as string, version: binding.version as number, contentDigest: binding.contentDigest as string }
 })()
 if ((bound !== undefined) !== record.versionConfirmed) throw new TypeError('invalid submission receipt')
 return {
  kind: 'submission_recorded', requestId: record.requestId, applicationId: record.applicationId, submissionId: record.submissionId,
  channel: record.channel as SubmissionChannel, occurredAt: record.occurredAt, versionConfirmed: record.versionConfirmed,
  ...(bound ? { boundVersion: bound } : {}), ...(typeof record.note === 'string' ? { note: record.note } : {}),
  confirmer: record.confirmer, revision: record.revision as number, createdAt: record.createdAt,
 }
}

export function decodeSubmissionList(value: unknown): SubmissionList {
 const record = decodeRecord(value, 'invalid submission list')
 if (!Array.isArray(record.submissions)) throw new TypeError('invalid submission list')
 const submissions = record.submissions.map(decodeSubmissionReceipt)
 // One application holds at most one submission (the frozen backend guard);
 // a list claiming more is an invented payload.
 if (submissions.length > 1) throw new TypeError('invalid submission list')
 return { submissions }
}

// Frozen preparation contract owned by the career backend (T19,
// internal/modules/career/preparation.go). The cover letter and the
// interview draft anchor to the version the user actually submitted, cite
// the frozen snapshot and the confirmed fact keys, and materialize as a
// material-domain draft that stays reviewable and revisable. A failed or
// in-flight generation answers its typed failure state with an empty body —
// never a blank success product. Decoders reject invented kinds, focuses,
// statuses or failure codes, and any payload whose product disagrees with
// its status, before they reach the UI.
export type PreparationFocus = 'cover_letter' | 'interview_prep'
export type PreparationKind = 'preparation_generated'
export type PreparationStatus = 'draft' | 'generating' | 'failed'
export type PreparationFailureCode = 'generation_failed' | 'claim_unconfirmed'
export type PreparationAnchor = { submissionId: string; materialId: string; exportId: string; version: number; contentDigest: string }
export type PreparationSnapshotRef = { opportunityId?: string; snapshotId?: string; snapshotSha256?: string }
export type PreparationSources = { submittedVersion: PreparationAnchor; snapshot: PreparationSnapshotRef; factKeys: string[]; profileRevision: number }
export type PreparationReceipt = { kind: PreparationKind; requestId: string; applicationId: string; preparationId: string; focus: PreparationFocus; status: PreparationStatus; anchor: PreparationAnchor; materialId?: string; body: MaterialBody; reviewRisks: MaterialReviewRisk[]; sources: PreparationSources; failureCode?: PreparationFailureCode; failureMessage?: string; revision: number; createdAt: string }
export type GeneratePreparationInput = { requestId: string; applicationId: string; focus: PreparationFocus; expectedRevision: number }
export type PreparationList = { preparations: PreparationReceipt[] }

const preparationFocuses: PreparationFocus[] = ['cover_letter', 'interview_prep']
const preparationStatuses: PreparationStatus[] = ['draft', 'generating', 'failed']
const preparationFailureCodes: PreparationFailureCode[] = ['generation_failed', 'claim_unconfirmed']

export function decodePreparationReceipt(value: unknown): PreparationReceipt {
 const record = decodeRecord(value, 'invalid preparation receipt')
 if (record.kind !== 'preparation_generated' || !preparationFocuses.includes(record.focus as PreparationFocus)
  || !preparationStatuses.includes(record.status as PreparationStatus)
  || !validIdentifier(record.requestId) || !validIdentifier(record.applicationId) || !validIdentifier(record.preparationId)
  || !validTimestamp(record.createdAt) || !validRevision(record.revision)) throw new TypeError('invalid preparation receipt')
 const anchorRecord = decodeRecord(record.anchor, 'invalid preparation anchor')
 if (!validIdentifier(anchorRecord.submissionId) || !validIdentifier(anchorRecord.materialId) || !validIdentifier(anchorRecord.exportId)
  || !validPositiveVersion(anchorRecord.version)
  || typeof anchorRecord.contentDigest !== 'string' || !sha256Hex.test(anchorRecord.contentDigest)) throw new TypeError('invalid preparation anchor')
 const anchor: PreparationAnchor = { submissionId: anchorRecord.submissionId as string, materialId: anchorRecord.materialId as string, exportId: anchorRecord.exportId as string, version: anchorRecord.version as number, contentDigest: anchorRecord.contentDigest as string }
 const sourcesRecord = decodeRecord(record.sources, 'invalid preparation sources')
 const submittedRecord = decodeRecord(sourcesRecord.submittedVersion, 'invalid preparation sources')
 if (!validIdentifier(submittedRecord.submissionId) || !validIdentifier(submittedRecord.materialId) || !validIdentifier(submittedRecord.exportId)
  || !validPositiveVersion(submittedRecord.version)
  || typeof submittedRecord.contentDigest !== 'string' || !sha256Hex.test(submittedRecord.contentDigest)
  || !validRevision(sourcesRecord.profileRevision)) throw new TypeError('invalid preparation sources')
 const snapshotRecord = decodeRecord(sourcesRecord.snapshot, 'invalid preparation sources')
 const draft = record.status === 'draft'
 // The frozen snapshot is fully cited on a materialized draft; a failure or
 // in-flight row carries only what the reservation persisted, and whatever
 // it does carry must still be a well-formed digest.
 if (draft && (!validIdentifier(snapshotRecord.opportunityId) || !validIdentifier(snapshotRecord.snapshotId)
  || typeof snapshotRecord.snapshotSha256 !== 'string' || !sha256Hex.test(snapshotRecord.snapshotSha256))) throw new TypeError('invalid preparation sources')
 if (!draft && (validIdentifier(snapshotRecord.opportunityId) || validIdentifier(snapshotRecord.snapshotId))
  && (typeof snapshotRecord.snapshotSha256 !== 'string' || !sha256Hex.test(snapshotRecord.snapshotSha256))) throw new TypeError('invalid preparation sources')
 // Fact keys are the confirmed citations of the draft; an interrupted row
 // has none yet. Anything that is not a clean identifier list is invented.
 const rawFactKeys: unknown = sourcesRecord.factKeys
 const factKeys = rawFactKeys === null || rawFactKeys === undefined ? [] : Array.isArray(rawFactKeys) ? rawFactKeys : undefined
 if (factKeys === undefined || factKeys.some((key) => !validIdentifier(key))) throw new TypeError('invalid preparation sources')
 const risks = decodeMaterialRisks(record.reviewRisks)
 // The product and its status are one fact: a draft carries its materialized
 // draft and a full body; a failed or in-flight row stays empty and typed.
 const hasMaterial = validIdentifier(record.materialId)
 if (draft !== hasMaterial) throw new TypeError('invalid preparation receipt')
 const failurePresent = record.failureCode !== undefined && record.failureCode !== null
 if (draft && failurePresent) throw new TypeError('invalid preparation receipt')
 if (record.status === 'failed' && !failurePresent) throw new TypeError('invalid preparation receipt')
 if (failurePresent && !preparationFailureCodes.includes(record.failureCode as PreparationFailureCode)) throw new TypeError('invalid preparation receipt')
 if (failurePresent && typeof record.failureMessage !== 'string') throw new TypeError('invalid preparation receipt')
 if (!draft && risks.length > 0) throw new TypeError('invalid preparation receipt')
 const bodyRecord = decodeRecord(record.body, 'invalid preparation receipt')
 let body: MaterialBody
 if (draft) {
  if (!Array.isArray(bodyRecord.sections) || bodyRecord.sections.length === 0) throw new TypeError('invalid preparation receipt')
  body = decodeMaterialBody(record.body)
 } else {
  if (bodyRecord.sections !== undefined && bodyRecord.sections !== null && (!Array.isArray(bodyRecord.sections) || bodyRecord.sections.length > 0)) throw new TypeError('invalid preparation receipt')
  body = { sections: [] }
 }
 return {
  kind: 'preparation_generated', requestId: record.requestId, applicationId: record.applicationId, preparationId: record.preparationId,
  focus: record.focus as PreparationFocus, status: record.status as PreparationStatus, anchor,
  ...(hasMaterial ? { materialId: record.materialId as string } : {}),
  body, reviewRisks: risks,
  sources: {
   submittedVersion: { submissionId: submittedRecord.submissionId as string, materialId: submittedRecord.materialId as string, exportId: submittedRecord.exportId as string, version: submittedRecord.version as number, contentDigest: submittedRecord.contentDigest as string },
   snapshot: {
    ...(validIdentifier(snapshotRecord.opportunityId) ? { opportunityId: snapshotRecord.opportunityId as string } : {}),
    ...(validIdentifier(snapshotRecord.snapshotId) ? { snapshotId: snapshotRecord.snapshotId as string } : {}),
    ...(typeof snapshotRecord.snapshotSha256 === 'string' ? { snapshotSha256: snapshotRecord.snapshotSha256 as string } : {}),
   },
   factKeys: factKeys as string[], profileRevision: sourcesRecord.profileRevision as number,
  },
  ...(failurePresent ? { failureCode: record.failureCode as PreparationFailureCode, failureMessage: record.failureMessage as string } : {}),
  revision: record.revision as number, createdAt: record.createdAt,
 }
}

export function decodePreparationList(value: unknown): PreparationList {
 const record = decodeRecord(value, 'invalid preparation list')
 if (!Array.isArray(record.preparations)) throw new TypeError('invalid preparation list')
 return { preparations: record.preparations.map(decodePreparationReceipt) }
}

// Frozen whole-space lifecycle contract owned by the career backend (T22,
// internal/modules/career/career_export.go). The export package travels
// inline (synchronous export) with a sha256 digest over the frozen archive;
// the deletion receipt only reports "deleted" after every step succeeded —
// partial failures keep a recoverable state under the same request ID and
// never claim complete deletion. The boundary view is the pre-deletion
// explanation: in-space sections the system can delete, external platform
// data it can never revoke, and the disclosed retention rows. Decoders
// reject invented kinds, statuses, step names or archives before they reach
// the UI.
export type CareerExportStatus = 'complete'
export type CareerDeletionStatus = 'deleting' | 'partial' | 'deleted'
export type CareerDeletionStepStatus = 'pending' | 'done' | 'failed'
export type CareerDeletionStepName = 'revoke_material_exports' | 'purge_career_data' | 'remove_workbench_tasks' | 'finalize'
export type CareerRetentionStatus = 'retained'
export type CareerRetentionItem = { holder: string; reason: string; status: CareerRetentionStatus }
export type CareerExternalBoundaryItem = { item: string; description: string; revocable: boolean }
export type CareerDeletionSection = { section: string; description: string; count: number }
export type CareerDeletionBoundaryView = { inSpace: CareerDeletionSection[]; external: CareerExternalBoundaryItem[]; retention: CareerRetentionItem[] }
export type CareerDeletionStep = { name: CareerDeletionStepName; status: CareerDeletionStepStatus; detail?: string }
export type CareerDeletionReceipt = { kind: 'career_deleted'; requestId: string; status: CareerDeletionStatus; steps: CareerDeletionStep[]; retention: CareerRetentionItem[]; revision: number; startedAt: string; completedAt?: string }
export type CareerExportInput = { requestId: string; expectedRevision: number }
export type CareerDeletionInput = { requestId: string; expectedRevision: number }
export type CareerExportSnapshot = { snapshotId: string; status: string; rawText: string; acquiredAt: string }
export type CareerExportOpportunity = { opportunityId: string; snapshots: CareerExportSnapshot[] }
export type CareerExportProgressEvent = { eventId: string; applicationId: string; seq: number; eventType: string; note?: string; occurredAt: string; correctsEventId?: string; source: CareerSource; confirmer: string }
export type CareerExportApplication = { applicationId: string; opportunityId: string; snapshotId: string; batchIdentity: string; taskId?: string; progressEvents: CareerExportProgressEvent[] }
export type CareerExportMaterialVersion = { version: number; requestId?: string; versionBody: string; createdAt: string }
export type CareerExportMaterial = { materialId: string; opportunityId: string; status: string; versions: CareerExportMaterialVersion[] }
export type CareerExportSubmission = { submissionId: string; applicationId: string; channel: string; occurredAt: string; versionConfirmed: boolean; materialId?: string; exportId?: string; version?: number; contentDigest?: string; note?: string; confirmer: string; createdAt: string }
export type CareerExportArchive = { profile: CareerView; factHistory: CareerFact[]; opportunities: CareerExportOpportunity[]; applications: CareerExportApplication[]; materials: CareerExportMaterial[]; submissions: CareerExportSubmission[] }
export type CareerExportReceipt = { kind: 'career_exported'; requestId: string; exportId: string; revision: number; status: CareerExportStatus; digest: string; archive: CareerExportArchive; createdAt: string }

const careerExportStatuses: CareerExportStatus[] = ['complete']
const careerDeletionStatuses: CareerDeletionStatus[] = ['deleting', 'partial', 'deleted']
const careerDeletionStepStatuses: CareerDeletionStepStatus[] = ['pending', 'done', 'failed']
const careerDeletionStepNames: CareerDeletionStepName[] = ['revoke_material_exports', 'purge_career_data', 'remove_workbench_tasks', 'finalize']
const careerRetentionStatuses: CareerRetentionStatus[] = ['retained']
const careerProposalStatuses = ['pending', 'confirmed', 'dismissed']

// The exported profile reuses the career-core fact/proposal shapes; the
// archive decoder re-validates them so a truncated or invented archive never
// reaches the UI even though open()/list() cast their views.
function decodeExportedSource(value: unknown): CareerSource {
 const record = decodeRecord(value, 'invalid exported source')
 if (typeof record.kind !== 'string' || !record.kind.trim() || !validOptionalString(record.label) || !validOptionalString(record.referenceId)) throw new TypeError('invalid exported source')
 return { kind: record.kind, ...(typeof record.label === 'string' ? { label: record.label } : {}), ...(typeof record.referenceId === 'string' ? { referenceId: record.referenceId } : {}) }
}

function decodeExportedConfirmation(value: unknown): { userId: string; confirmedAt: string } {
 const record = decodeRecord(value, 'invalid exported confirmation')
 if (typeof record.userId !== 'string' || !record.userId.trim() || !validTimestamp(record.confirmedAt)) throw new TypeError('invalid exported confirmation')
 return { userId: record.userId, confirmedAt: record.confirmedAt }
}

function decodeExportedFact(value: unknown): CareerFact {
 const record = decodeRecord(value, 'invalid exported fact')
 if (typeof record.key !== 'string' || !record.key.trim() || typeof record.value !== 'string' || !validRevision(record.revision) || !validTimestamp(record.confirmedAt)) throw new TypeError('invalid exported fact')
 return { key: record.key, value: record.value, revision: record.revision, source: decodeExportedSource(record.source), confirmation: decodeExportedConfirmation(record.confirmation), confirmedAt: record.confirmedAt }
}

function decodeExportedProposal(value: unknown): CareerProposal {
 const record = decodeRecord(value, 'invalid exported proposal')
 if (typeof record.id !== 'string' || !record.id.trim() || typeof record.key !== 'string' || !record.key.trim() || typeof record.value !== 'string'
  || !careerProposalStatuses.includes(record.status as CareerProposal['status']) || !validTimestamp(record.createdAt) || !validOptionalString(record.evidence)) throw new TypeError('invalid exported proposal')
 const confirmation = record.confirmation === undefined ? undefined : decodeExportedConfirmation(record.confirmation)
 const resolutionSource = record.resolutionSource === undefined ? undefined : decodeExportedSource(record.resolutionSource)
 return { id: record.id, key: record.key, value: record.value, ...(typeof record.evidence === 'string' ? { evidence: record.evidence } : {}), source: decodeExportedSource(record.source), status: record.status as CareerProposal['status'], ...(validRevision(record.revision) ? { revision: record.revision } : {}), ...(confirmation ? { confirmation } : {}), ...(resolutionSource ? { resolutionSource } : {}), createdAt: record.createdAt }
}

function decodeExportedProfile(value: unknown): CareerView {
 const record = decodeRecord(value, 'invalid exported profile')
 if (!Array.isArray(record.facts) || !Array.isArray(record.proposals) || !validRevision(record.revision)) throw new TypeError('invalid exported profile')
 return { revision: record.revision, facts: record.facts.map(decodeExportedFact), proposals: record.proposals.map(decodeExportedProposal) }
}

export function decodeCareerExportReceipt(value: unknown): CareerExportReceipt {
 const record = decodeRecord(value, 'invalid career export receipt')
 if (record.kind !== 'career_exported' || !validIdentifier(record.requestId) || !validIdentifier(record.exportId) || !validRevision(record.revision)
  || !careerExportStatuses.includes(record.status as CareerExportStatus) || typeof record.digest !== 'string' || !sha256Hex.test(record.digest) || !validTimestamp(record.createdAt)) throw new TypeError('invalid career export receipt')
 const archiveRecord = decodeRecord(record.archive, 'invalid career export archive')
 if (!Array.isArray(archiveRecord.factHistory) || !Array.isArray(archiveRecord.opportunities) || !Array.isArray(archiveRecord.applications) || !Array.isArray(archiveRecord.materials) || !Array.isArray(archiveRecord.submissions)) throw new TypeError('invalid career export archive')
 const opportunities = archiveRecord.opportunities.map((item) => {
  const row = decodeRecord(item, 'invalid exported opportunity')
  if (!validIdentifier(row.opportunityId) || !Array.isArray(row.snapshots)) throw new TypeError('invalid exported opportunity')
  return { opportunityId: row.opportunityId, snapshots: row.snapshots.map((snapshot) => {
   const snap = decodeRecord(snapshot, 'invalid exported snapshot')
   if (!validIdentifier(snap.snapshotId) || typeof snap.status !== 'string' || !snap.status.trim() || typeof snap.rawText !== 'string' || !validTimestamp(snap.acquiredAt)) throw new TypeError('invalid exported snapshot')
   return { snapshotId: snap.snapshotId, status: snap.status, rawText: snap.rawText, acquiredAt: snap.acquiredAt }
  }) }
 })
 const applications = archiveRecord.applications.map((item) => {
  const row = decodeRecord(item, 'invalid exported application')
  if (!validIdentifier(row.applicationId) || !validIdentifier(row.opportunityId) || !validIdentifier(row.snapshotId) || !validIdentifier(row.batchIdentity)
   || (row.taskId !== undefined && !validIdentifier(row.taskId)) || !Array.isArray(row.progressEvents)) throw new TypeError('invalid exported application')
  return { applicationId: row.applicationId, opportunityId: row.opportunityId, snapshotId: row.snapshotId, batchIdentity: row.batchIdentity, ...(row.taskId !== undefined ? { taskId: row.taskId } : {}), progressEvents: row.progressEvents.map((event) => {
   const entry = decodeRecord(event, 'invalid exported progress event')
   if (!validIdentifier(entry.eventId) || !validIdentifier(entry.applicationId) || !validPositiveVersion(entry.seq) || typeof entry.eventType !== 'string' || !entry.eventType.trim()
    || !validTimestamp(entry.occurredAt) || (entry.correctsEventId !== undefined && !validIdentifier(entry.correctsEventId)) || !validOptionalString(entry.note) || !validIdentifier(entry.confirmer)) throw new TypeError('invalid exported progress event')
   return { eventId: entry.eventId, applicationId: entry.applicationId, seq: entry.seq, eventType: entry.eventType, ...(typeof entry.note === 'string' ? { note: entry.note } : {}), occurredAt: entry.occurredAt, ...(entry.correctsEventId !== undefined ? { correctsEventId: entry.correctsEventId } : {}), source: decodeExportedSource(entry.source), confirmer: entry.confirmer }
  }) }
 })
 const materials = archiveRecord.materials.map((item) => {
  const row = decodeRecord(item, 'invalid exported material')
  if (!validIdentifier(row.materialId) || !validIdentifier(row.opportunityId) || typeof row.status !== 'string' || !row.status.trim() || !Array.isArray(row.versions)) throw new TypeError('invalid exported material')
  return { materialId: row.materialId, opportunityId: row.opportunityId, status: row.status, versions: row.versions.map((version) => {
   const entry = decodeRecord(version, 'invalid exported material version')
   if (!validPositiveVersion(entry.version) || (entry.requestId !== undefined && !validIdentifier(entry.requestId)) || typeof entry.versionBody !== 'string' || !validTimestamp(entry.createdAt)) throw new TypeError('invalid exported material version')
   return { version: entry.version, ...(entry.requestId !== undefined ? { requestId: entry.requestId } : {}), versionBody: entry.versionBody, createdAt: entry.createdAt }
  }) }
 })
 const submissions = archiveRecord.submissions.map((item) => {
  const row = decodeRecord(item, 'invalid exported submission')
  if (!validIdentifier(row.submissionId) || !validIdentifier(row.applicationId) || typeof row.channel !== 'string' || !row.channel.trim() || !validTimestamp(row.occurredAt) || typeof row.versionConfirmed !== 'boolean'
   || (row.materialId !== undefined && !validIdentifier(row.materialId)) || (row.exportId !== undefined && !validIdentifier(row.exportId)) || (row.version !== undefined && !validPositiveVersion(row.version))
   || (row.contentDigest !== undefined && (typeof row.contentDigest !== 'string' || !sha256Hex.test(row.contentDigest))) || !validOptionalString(row.note) || !validIdentifier(row.confirmer) || !validTimestamp(row.createdAt)) throw new TypeError('invalid exported submission')
  return { submissionId: row.submissionId, applicationId: row.applicationId, channel: row.channel, occurredAt: row.occurredAt, versionConfirmed: row.versionConfirmed, ...(row.materialId !== undefined ? { materialId: row.materialId } : {}), ...(row.exportId !== undefined ? { exportId: row.exportId } : {}), ...(row.version !== undefined ? { version: row.version } : {}), ...(row.contentDigest !== undefined ? { contentDigest: row.contentDigest } : {}), ...(typeof row.note === 'string' ? { note: row.note } : {}), confirmer: row.confirmer, createdAt: row.createdAt }
 })
 return { kind: 'career_exported', requestId: record.requestId, exportId: record.exportId, revision: record.revision as number, status: record.status as CareerExportStatus, digest: record.digest, archive: { profile: decodeExportedProfile(archiveRecord.profile), factHistory: archiveRecord.factHistory.map(decodeExportedFact), opportunities, applications, materials, submissions }, createdAt: record.createdAt }
}

function decodeRetentionItems(value: unknown): CareerRetentionItem[] {
 if (!Array.isArray(value)) throw new TypeError('invalid career retention disclosure')
 return value.map((item) => {
  const row = decodeRecord(item, 'invalid career retention item')
  if (!validIdentifier(row.holder) || typeof row.reason !== 'string' || !row.reason.trim() || !careerRetentionStatuses.includes(row.status as CareerRetentionStatus)) throw new TypeError('invalid career retention item')
  return { holder: row.holder, reason: row.reason, status: row.status as CareerRetentionStatus }
 })
}

export function decodeCareerDeletionBoundary(value: unknown): CareerDeletionBoundaryView {
 const record = decodeRecord(value, 'invalid career deletion boundary')
 if (!Array.isArray(record.inSpace) || !Array.isArray(record.external)) throw new TypeError('invalid career deletion boundary')
 const inSpace = record.inSpace.map((item) => {
  const row = decodeRecord(item, 'invalid career deletion section')
  if (!validIdentifier(row.section) || typeof row.description !== 'string' || !row.description.trim() || !Number.isSafeInteger(row.count) || (row.count as number) < 0) throw new TypeError('invalid career deletion section')
  return { section: row.section, description: row.description, count: row.count as number }
 })
 const external = record.external.map((item) => {
  const row = decodeRecord(item, 'invalid career external boundary item')
  if (!validIdentifier(row.item) || typeof row.description !== 'string' || !row.description.trim() || typeof row.revocable !== 'boolean') throw new TypeError('invalid career external boundary item')
  return { item: row.item, description: row.description, revocable: row.revocable }
 })
 return { inSpace, external, retention: decodeRetentionItems(record.retention) }
}

export function decodeCareerDeletionReceipt(value: unknown): CareerDeletionReceipt {
 const record = decodeRecord(value, 'invalid career deletion receipt')
 if (record.kind !== 'career_deleted' || !validIdentifier(record.requestId) || !careerDeletionStatuses.includes(record.status as CareerDeletionStatus)
  || !Array.isArray(record.steps) || !validRevision(record.revision) || !validTimestamp(record.startedAt)) throw new TypeError('invalid career deletion receipt')
 const steps = record.steps.map((item) => {
  const row = decodeRecord(item, 'invalid career deletion step')
  if (!careerDeletionStepNames.includes(row.name as CareerDeletionStepName) || !careerDeletionStepStatuses.includes(row.status as CareerDeletionStepStatus) || !validOptionalString(row.detail)) throw new TypeError('invalid career deletion step')
  return { name: row.name as CareerDeletionStepName, status: row.status as CareerDeletionStepStatus, ...(typeof row.detail === 'string' ? { detail: row.detail } : {}) }
 })
 const status = record.status as CareerDeletionStatus
 const failed = steps.filter((step) => step.status === 'failed').length
 const allDone = steps.length > 0 && steps.every((step) => step.status === 'done')
 // The truthful status contract: only every-step-done may claim "deleted"
 // (with a completion time), "partial" carries at least one failed step,
 // and "deleting" is still in flight. Anything else is an invented payload.
 if ((status === 'deleted') !== allDone) throw new TypeError('invalid career deletion receipt')
 if (status === 'partial' && failed === 0) throw new TypeError('invalid career deletion receipt')
 if (status !== 'deleted' && record.completedAt !== undefined) throw new TypeError('invalid career deletion receipt')
 if (status === 'deleted' && !validTimestamp(record.completedAt)) throw new TypeError('invalid career deletion receipt')
 return { kind: 'career_deleted', requestId: record.requestId, status, steps, retention: decodeRetentionItems(record.retention), revision: record.revision as number, startedAt: record.startedAt, ...(status === 'deleted' ? { completedAt: record.completedAt as string } : {}) }
}


export function createCareerApi(request: CareerRequest, binaryRequest?: CareerBinaryRequest) {
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
  async setRule(input: SetRuleInput, signal?: AbortSignal): Promise<SetRuleReceipt> {
   if (!input.requestId.trim() || !input.query.trim()) throw new TypeError('rule requestId and query must not be empty')
   if (!ruleStatuses.includes(input.status)) throw new TypeError('rule status must be enabled, paused, or disabled')
   if (!Number.isSafeInteger(input.intervalMinutes) || input.intervalMinutes < minRuleIntervalMinutes || input.intervalMinutes > maxRuleIntervalMinutes) throw new TypeError('rule interval must be an integer between 1 and 43200 minutes')
   if (!Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new TypeError('rule expected revision must be a non-negative integer')
   const body: Record<string, unknown> = { requestId: input.requestId, query: input.query, intervalMinutes: input.intervalMinutes, status: input.status, expectedRevision: input.expectedRevision, ...(input.ruleId?.trim() ? { ruleId: input.ruleId.trim() } : {}) }
   return decodeSetRuleReceipt(await request({ method: 'POST', path: '/api/v1/career/rules', body, ...(signal ? { signal } : {}) }))
  },
  async ruleReceipt(requestId: string, signal?: AbortSignal): Promise<SetRuleReceipt> {
   if (!requestId.trim()) throw new TypeError('rule receipt requestId must not be empty')
   return decodeSetRuleReceipt(await request({ method: 'GET', path: `/api/v1/career/rules/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) }))
  },
  async getRule(ruleId: string, signal?: AbortSignal): Promise<RuleView> {
   if (!ruleId.trim()) throw new TypeError('rule ID must not be empty')
   return decodeRuleView(await request({ method: 'GET', path: `/api/v1/career/rules/${encodeURIComponent(ruleId)}`, ...(signal ? { signal } : {}) }))
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
  async publishMaterial(input: PublishMaterialInput, signal?: AbortSignal): Promise<MaterialExportReceipt> {
   if (!input.requestId.trim() || !input.materialId.trim()) throw new TypeError('material export publish requestId and materialId must not be empty')
   if (!Number.isSafeInteger(input.version) || input.version <= 0) throw new TypeError('material export publish version must be a positive integer')
   if (!Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new TypeError('material export publish expected revision must be a non-negative integer')
   return decodeMaterialExportReceipt(await request({ method: 'POST', path: `/api/v1/career/materials/${encodeURIComponent(input.materialId)}/exports`, body: { requestId: input.requestId, version: input.version, expectedRevision: input.expectedRevision }, ...(signal ? { signal } : {}) }))
  },
  async materialExports(materialId: string, signal?: AbortSignal): Promise<MaterialExportList> {
   if (!materialId.trim()) throw new TypeError('material export materialId must not be empty')
   return decodeMaterialExportList(await request({ method: 'GET', path: `/api/v1/career/materials/${encodeURIComponent(materialId)}/exports`, ...(signal ? { signal } : {}) }))
  },
  async materialExportSignedURL(materialId: string, exportId: string, format: MaterialExportFormat, ttlSeconds: number, signal?: AbortSignal): Promise<MaterialExportDownload> {
   if (!materialId.trim() || !exportId.trim()) throw new TypeError('material export grant materialId and exportId must not be empty')
   if (!materialExportFormats.includes(format)) throw new TypeError('material export grant format must be pdf or docx')
   if (!Number.isSafeInteger(ttlSeconds) || ttlSeconds <= 0 || ttlSeconds > MAX_EXPORT_GRANT_TTL_SECONDS) throw new TypeError(`material export grant ttl must be an integer between 1 and ${MAX_EXPORT_GRANT_TTL_SECONDS}`)
   return decodeMaterialExportDownload(await request({ method: 'POST', path: `/api/v1/career/materials/${encodeURIComponent(materialId)}/exports/${encodeURIComponent(exportId)}/signed-url`, body: { format, ttlSeconds }, ...(signal ? { signal } : {}) }))
  },
  // Authenticated redemption: the path is rebuilt from the decoded grant
  // fields and sent through the binary transport so Authorization travels
  // with the download; grant.url is never fetched.
  async materialExportDownload(grant: MaterialExportDownload, signal?: AbortSignal): Promise<MaterialExportFile> {
   if (!binaryRequest) throw new Error('Binary transport is unavailable')
   if (!materialExportFormats.includes(grant.format)) throw new TypeError('material export download format must be pdf or docx')
   const response = await binaryRequest({ method: 'GET', path: `/api/v1/career/materials/${encodeURIComponent(grant.materialId)}/exports/${encodeURIComponent(grant.exportId)}/download?format=${grant.format}&expires=${grant.expiresAt}&signature=${encodeURIComponent(grant.signature)}`, ...(signal ? { signal } : {}) })
   return { format: grant.format, digest: grant.digest, size: grant.size, body: response.body, ...(response.contentType !== undefined ? { contentType: response.contentType } : {}) }
  },
  async revokeMaterialExport(input: RevokeMaterialExportInput, signal?: AbortSignal): Promise<MaterialExportReceipt> {
   if (!input.requestId.trim() || !input.materialId.trim() || !input.exportId.trim()) throw new TypeError('material export revoke requestId, materialId and exportId must not be empty')
   if (!Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new TypeError('material export revoke expected revision must be a non-negative integer')
   return decodeMaterialExportReceipt(await request({ method: 'DELETE', path: `/api/v1/career/materials/${encodeURIComponent(input.materialId)}/exports/${encodeURIComponent(input.exportId)}`, body: { requestId: input.requestId, expectedRevision: input.expectedRevision }, ...(signal ? { signal } : {}) }))
  },
  async appendProgress(input: AppendProgressInput, signal?: AbortSignal): Promise<ProgressReceipt> {
   if (!input.requestId.trim() || !input.applicationId.trim()) throw new TypeError('progress requestId and applicationId must not be empty')
   if (!progressEventTypes.includes(input.eventType)) throw new TypeError('progress eventType must come from the frozen event vocabulary')
   if (input.occurredAt !== undefined && !validTimestamp(input.occurredAt)) throw new TypeError('progress occurredAt must be an RFC3339 timestamp')
   if (!Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new TypeError('progress expected revision must be a non-negative integer')
   const body = { requestId: input.requestId, applicationId: input.applicationId, eventType: input.eventType, ...(input.note !== undefined ? { note: input.note } : {}), ...(input.occurredAt !== undefined ? { occurredAt: input.occurredAt } : {}), source: { kind: 'manual' }, expectedRevision: input.expectedRevision }
   return decodeProgressReceipt(await request({ method: 'POST', path: `/api/v1/career/applications/${encodeURIComponent(input.applicationId)}/progress`, body, ...(signal ? { signal } : {}) }))
  },
  async correctProgress(input: CorrectProgressInput, signal?: AbortSignal): Promise<ProgressReceipt> {
   if (!input.requestId.trim() || !input.applicationId.trim() || !input.correctsEventId.trim()) throw new TypeError('progress correct requestId, applicationId and correctsEventId must not be empty')
   if (!progressEventTypes.includes(input.eventType)) throw new TypeError('progress eventType must come from the frozen event vocabulary')
   if (input.occurredAt !== undefined && !validTimestamp(input.occurredAt)) throw new TypeError('progress occurredAt must be an RFC3339 timestamp')
   if (!Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new TypeError('progress expected revision must be a non-negative integer')
   const body = { requestId: input.requestId, applicationId: input.applicationId, correctsEventId: input.correctsEventId, eventType: input.eventType, ...(input.note !== undefined ? { note: input.note } : {}), ...(input.occurredAt !== undefined ? { occurredAt: input.occurredAt } : {}), source: { kind: 'manual' }, expectedRevision: input.expectedRevision }
   return decodeProgressReceipt(await request({ method: 'POST', path: `/api/v1/career/applications/${encodeURIComponent(input.applicationId)}/progress/correct`, body, ...(signal ? { signal } : {}) }))
  },
  async applicationProgress(applicationId: string, signal?: AbortSignal): Promise<ProgressView> {
   if (!applicationId.trim()) throw new TypeError('progress applicationId must not be empty')
   return decodeProgressView(await request({ method: 'GET', path: `/api/v1/career/applications/${encodeURIComponent(applicationId)}/progress`, ...(signal ? { signal } : {}) }))
  },
  async progressReceipt(requestId: string, signal?: AbortSignal): Promise<ProgressReceipt> {
   if (!requestId.trim()) throw new TypeError('progress receipt requestId must not be empty')
   return decodeProgressReceipt(await request({ method: 'GET', path: `/api/v1/career/progress/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) }))
  },
  // T18: the user confirms what they did externally; the product records the
  // claimed fact and never performs or infers any external action. The
  // explicit unknown marker is exclusive — no material/export reference may
  // accompany it — and a confirmed binding requires both identifiers.
  async recordSubmission(input: RecordSubmissionInput, signal?: AbortSignal): Promise<SubmissionReceipt> {
   if (!input.requestId.trim() || !input.applicationId.trim()) throw new TypeError('submission requestId and applicationId must not be empty')
   if (!submissionChannels.includes(input.channel)) throw new TypeError('submission channel must be email, web, or other')
   if (input.occurredAt !== undefined && !validTimestamp(input.occurredAt)) throw new TypeError('submission occurredAt must be an RFC3339 timestamp')
   if (input.note !== undefined && input.note.length > maxSubmissionNoteBytes) throw new TypeError('submission note must not exceed 4096 bytes')
   if (!Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new TypeError('submission expected revision must be a non-negative integer')
   if (input.versionUnknown && (input.materialId !== undefined || input.exportId !== undefined)) throw new TypeError('submission versionUnknown must not carry a material or export binding')
   if (!input.versionUnknown && (!input.materialId?.trim() || !input.exportId?.trim())) throw new TypeError('submission requires materialId and exportId unless versionUnknown is explicit')
   const body = { requestId: input.requestId, applicationId: input.applicationId, channel: input.channel,
    ...(input.occurredAt !== undefined ? { occurredAt: input.occurredAt } : {}),
    ...(input.materialId !== undefined ? { materialId: input.materialId } : {}),
    ...(input.exportId !== undefined ? { exportId: input.exportId } : {}),
    versionUnknown: input.versionUnknown, ...(input.note !== undefined ? { note: input.note } : {}), expectedRevision: input.expectedRevision }
   return decodeSubmissionReceipt(await request({ method: 'POST', path: `/api/v1/career/applications/${encodeURIComponent(input.applicationId)}/submissions`, body, ...(signal ? { signal } : {}) }))
  },
  async applicationSubmissions(applicationId: string, signal?: AbortSignal): Promise<SubmissionList> {
   if (!applicationId.trim()) throw new TypeError('submission applicationId must not be empty')
   return decodeSubmissionList(await request({ method: 'GET', path: `/api/v1/career/applications/${encodeURIComponent(applicationId)}/submissions`, ...(signal ? { signal } : {}) }))
  },
  async submissionReceipt(requestId: string, signal?: AbortSignal): Promise<SubmissionReceipt> {
   if (!requestId.trim()) throw new TypeError('submission receipt requestId must not be empty')
   return decodeSubmissionReceipt(await request({ method: 'GET', path: `/api/v1/career/submissions/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) }))
  },
  // T19 sourced preparations: generation anchors to the actually submitted
  // version (an unconfirmed submission answers the typed prompt state) and
  // the draft materializes as a material-domain body the user reviews,
  // revises and cites. Uncertain outcomes recover by replaying the same
  // request ID through the receipt endpoint.
  async generatePreparation(input: GeneratePreparationInput, signal?: AbortSignal): Promise<PreparationReceipt> {
   if (!input.requestId.trim() || !input.applicationId.trim()) throw new TypeError('preparation requestId and applicationId must not be empty')
   if (!preparationFocuses.includes(input.focus)) throw new TypeError('preparation focus must be cover_letter or interview_prep')
   if (!Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new TypeError('preparation expected revision must be a non-negative integer')
   return decodePreparationReceipt(await request({ method: 'POST', path: `/api/v1/career/applications/${encodeURIComponent(input.applicationId)}/preparations`, body: { requestId: input.requestId, applicationId: input.applicationId, focus: input.focus, expectedRevision: input.expectedRevision }, ...(signal ? { signal } : {}) }))
  },
  async applicationPreparations(applicationId: string, signal?: AbortSignal): Promise<PreparationList> {
   if (!applicationId.trim()) throw new TypeError('preparation applicationId must not be empty')
   return decodePreparationList(await request({ method: 'GET', path: `/api/v1/career/applications/${encodeURIComponent(applicationId)}/preparations`, ...(signal ? { signal } : {}) }))
  },
  async preparationReceipt(requestId: string, signal?: AbortSignal): Promise<PreparationReceipt> {
   if (!requestId.trim()) throw new TypeError('preparation receipt requestId must not be empty')
   return decodePreparationReceipt(await request({ method: 'GET', path: `/api/v1/career/preparations/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) }))
  },
  // T22 whole-space lifecycle: export packages, the pre-deletion boundary
  // explanation and complete deletion. Both writes carry a request ID and
  // the pinned profile revision; uncertain outcomes are recovered by
  // replaying the same request ID through the receipt endpoints.
  async exportCareer(input: CareerExportInput, signal?: AbortSignal): Promise<CareerExportReceipt> {
   if (!input.requestId.trim()) throw new TypeError('career export requestId must not be empty')
   if (!Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new TypeError('career export expected revision must be a non-negative integer')
   return decodeCareerExportReceipt(await request({ method: 'POST', path: '/api/v1/career/exports', body: { requestId: input.requestId, expectedRevision: input.expectedRevision }, ...(signal ? { signal } : {}) }))
  },
  async careerExportReceipt(requestId: string, signal?: AbortSignal): Promise<CareerExportReceipt> {
   if (!requestId.trim()) throw new TypeError('career export receipt requestId must not be empty')
   return decodeCareerExportReceipt(await request({ method: 'GET', path: `/api/v1/career/exports/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) }))
  },
  async careerDeletionBoundary(signal?: AbortSignal): Promise<CareerDeletionBoundaryView> {
   return decodeCareerDeletionBoundary(await request({ method: 'GET', path: '/api/v1/career/deletions/boundary', ...(signal ? { signal } : {}) }))
  },
  async deleteCareer(input: CareerDeletionInput, signal?: AbortSignal): Promise<CareerDeletionReceipt> {
   if (!input.requestId.trim()) throw new TypeError('career deletion requestId must not be empty')
   if (!Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new TypeError('career deletion expected revision must be a non-negative integer')
   return decodeCareerDeletionReceipt(await request({ method: 'POST', path: '/api/v1/career/deletions', body: { requestId: input.requestId, expectedRevision: input.expectedRevision }, ...(signal ? { signal } : {}) }))
  },
  async careerDeletionReceipt(requestId: string, signal?: AbortSignal): Promise<CareerDeletionReceipt> {
   if (!requestId.trim()) throw new TypeError('career deletion receipt requestId must not be empty')
   return decodeCareerDeletionReceipt(await request({ method: 'GET', path: `/api/v1/career/deletions/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) }))
  },
 }
}
