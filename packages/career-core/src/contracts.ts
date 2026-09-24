export type CareerSource = { kind: string; label?: string; referenceId?: string }
export type CareerConfirmation = { userId: string; confirmedAt: string }
export type CareerFact = { key: string; value: string; revision: number; source: CareerSource; confirmation: CareerConfirmation; confirmedAt: string }
export type CareerProposal = { id: string; key: string; value: string; evidence?: string; source: CareerSource; status: 'pending' | 'confirmed' | 'dismissed'; revision?: number; confirmation?: CareerConfirmation; resolutionSource?: CareerSource; createdAt: string }
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
 return typeof value.id === 'string' && typeof value.key === 'string' && typeof value.value === 'string' && (value.evidence === undefined || typeof value.evidence === 'string') && validSource(value.source) && ['pending', 'confirmed', 'dismissed'].includes(String(value.status)) && typeof value.createdAt === 'string' && (value.revision === undefined || typeof value.revision === 'number') && validConfirmation && (value.resolutionSource === undefined || validSource(value.resolutionSource))
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

export type CareerDocumentSource = {
 id: string; revision: number; fileName: string; mimeType: string; size: number; digest: string; status: 'processing' | 'ready' | 'failed';
 errorCategory?: string; errorMessage?: string; missingCategories?: string[]; reviewFlags?: string[]; createdAt: string; completedAt?: string
}
export type CareerIntakeReceipt = { kind: 'intake_completed'; requestId: string; revision: number; proposals: CareerProposal[] }
export type CareerUpload = { source: CareerDocumentSource; receipt?: CareerIntakeReceipt }

export function decodeCareerSource(value: unknown): CareerDocumentSource {
 if (!isRecord(value) || typeof value.id !== 'string' || typeof value.revision !== 'number' || typeof value.fileName !== 'string' || typeof value.mimeType !== 'string' || typeof value.size !== 'number' || typeof value.digest !== 'string' || !['processing', 'ready', 'failed'].includes(String(value.status)) || typeof value.createdAt !== 'string' || (value.errorCategory !== undefined && typeof value.errorCategory !== 'string') || (value.errorMessage !== undefined && typeof value.errorMessage !== 'string') || (value.completedAt !== undefined && typeof value.completedAt !== 'string') || (value.missingCategories !== undefined && (!Array.isArray(value.missingCategories) || !value.missingCategories.every((item) => typeof item === 'string'))) || (value.reviewFlags !== undefined && (!Array.isArray(value.reviewFlags) || !value.reviewFlags.every((item) => typeof item === 'string')))) throw new TypeError('invalid career source')
 return value as CareerDocumentSource
}
export function decodeCareerUpload(value: unknown): CareerUpload {
 if (!isRecord(value) || !('source' in value)) throw new TypeError('invalid career upload')
 const source = decodeCareerSource(value.source)
 let receipt: CareerIntakeReceipt | undefined
 if (value.receipt !== undefined) {
  const raw = value.receipt
  if (!isRecord(raw) || raw.kind !== 'intake_completed' || typeof raw.requestId !== 'string' || typeof raw.revision !== 'number' || !Array.isArray(raw.proposals) || !raw.proposals.every(validProposal)) throw new TypeError('invalid career intake receipt')
  receipt = raw as CareerIntakeReceipt
 }
 return { source, ...(receipt ? { receipt } : {}) }
}
export function decodeCareerSources(value: unknown): CareerDocumentSource[] {
 if (!isRecord(value) || !Array.isArray(value.sources)) throw new TypeError('invalid career sources')
 return value.sources.map(decodeCareerSource)
}

export type OpportunityStatus = 'stored' | 'needs_review'
export type OpportunityExtractedValue = { state: 'known'; value: string } | { state: 'unknown' }
export type OpportunityFields = { title: OpportunityExtractedValue; company: OpportunityExtractedValue; location: OpportunityExtractedValue; batch: OpportunityExtractedValue; requirements: OpportunityExtractedValue }
export type OpportunitySource = { kind: string; label?: string; referenceId?: string }
export type OpportunityImportInput = { requestId: string; rawText: string; sourceLabel?: string; sourceReference?: string }
export type OpportunityReceipt = { kind: 'opportunity_imported'; requestId: string; opportunityId: string; observationId: string; snapshotId: string; status: OpportunityStatus; acquiredAt: string }
export type OpportunityEvidence = { opportunityId: string; observationId: string; snapshotId: string; rawText: string; rawSha256: string; extracted: OpportunityFields; source: OpportunitySource; acquiredAt: string; status: OpportunityStatus }

const opportunityStatuses: OpportunityStatus[] = ['stored', 'needs_review']
function validIdentifier(value: unknown): value is string { return typeof value === 'string' && value.trim().length > 0 }
function validTimestamp(value: unknown): value is string {
 if (typeof value !== 'string') return false
 const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(Z|([+-])(\d{2}):(\d{2}))$/.exec(value)
 if (!match) return false
 const [, yearText, monthText, dayText, hourText, minuteText, secondText, , , offsetHourText, offsetMinuteText] = match
 const year = Number(yearText), month = Number(monthText), day = Number(dayText)
 const hour = Number(hourText), minute = Number(minuteText), second = Number(secondText)
 const offsetHour = offsetHourText === undefined ? 0 : Number(offsetHourText)
 const offsetMinute = offsetMinuteText === undefined ? 0 : Number(offsetMinuteText)
 const leapYear = year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0)
 const monthDays = [31, leapYear ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]
 return month >= 1 && month <= 12 && day >= 1 && day <= (monthDays[month - 1] ?? 0) && hour <= 23 && minute <= 59 && second <= 59 && offsetHour <= 23 && offsetMinute <= 59 && Number.isFinite(Date.parse(value))
}
function decodeOpportunityValue(value: unknown): OpportunityExtractedValue {
 if (!isRecord(value)) throw new TypeError('invalid opportunity extracted value')
 if (value.state === 'unknown' && value.value === undefined) return { state: 'unknown' }
 if (value.state === 'known' && typeof value.value === 'string' && value.value.trim().length > 0) return { state: 'known', value: value.value }
 throw new TypeError('invalid opportunity extracted value state')
}
export function decodeOpportunityReceipt(value: unknown): OpportunityReceipt {
 if (!isRecord(value) || value.kind !== 'opportunity_imported' || !validIdentifier(value.requestId) || !validIdentifier(value.opportunityId) || !validIdentifier(value.observationId) || !validIdentifier(value.snapshotId) || !opportunityStatuses.includes(value.status as OpportunityStatus) || !validTimestamp(value.acquiredAt)) throw new TypeError('invalid opportunity receipt')
 return { kind: 'opportunity_imported', requestId: value.requestId, opportunityId: value.opportunityId, observationId: value.observationId, snapshotId: value.snapshotId, status: value.status as OpportunityStatus, acquiredAt: value.acquiredAt }
}
export function decodeOpportunityEvidence(value: unknown): OpportunityEvidence {
 if (!isRecord(value) || !validIdentifier(value.opportunityId) || !validIdentifier(value.observationId) || !validIdentifier(value.snapshotId) || typeof value.rawText !== 'string' || value.rawText.trim().length === 0 || typeof value.rawSha256 !== 'string' || !/^[a-f0-9]{64}$/.test(value.rawSha256) || !isRecord(value.extracted) || !isRecord(value.source) || !validIdentifier(value.source.kind) || (value.source.label !== undefined && typeof value.source.label !== 'string') || (value.source.referenceId !== undefined && typeof value.source.referenceId !== 'string') || !validTimestamp(value.acquiredAt) || !opportunityStatuses.includes(value.status as OpportunityStatus)) throw new TypeError('invalid opportunity evidence')
 const extracted: OpportunityFields = {
  title: decodeOpportunityValue(value.extracted.title),
  company: decodeOpportunityValue(value.extracted.company),
  location: decodeOpportunityValue(value.extracted.location),
  batch: decodeOpportunityValue(value.extracted.batch),
  requirements: decodeOpportunityValue(value.extracted.requirements),
 }
 return { opportunityId: value.opportunityId, observationId: value.observationId, snapshotId: value.snapshotId, rawText: value.rawText, rawSha256: value.rawSha256, extracted, source: { kind: value.source.kind, ...(value.source.label !== undefined ? { label: value.source.label } : {}), ...(value.source.referenceId !== undefined ? { referenceId: value.source.referenceId } : {}) }, acquiredAt: value.acquiredAt, status: value.status as OpportunityStatus }
}

export type EvaluationStatus = 'eligible' | 'ineligible' | 'unknown'
export type EvaluationReceipt = { kind: 'evaluation_created'; requestId: string; evaluationId: string; opportunityId: string; snapshotId: string; profileRevision: number; status: EvaluationStatus }
export type EvaluationFactEvidence = { factKey: string; value: string; revision: number; factRevision: number; source: CareerSource; confirmation: CareerConfirmation; confirmedAt: string }
export type EvaluationJobEvidence = { snapshotId: string; observationId: string; acquiredAt: string; rawSha256: string; spanStart: number; spanEnd: number; quotedText: string }
export type EvaluationHardRule = { ruleId: string; criterion: string; outcome: EvaluationStatus; reasonCode: string; jobEvidence?: EvaluationJobEvidence; profileEvidence?: EvaluationFactEvidence }
export type EvaluationSoftMatch = { kind: 'skill' | 'project' | 'intent'; value: string; jobEvidence: EvaluationJobEvidence; profileEvidence: EvaluationFactEvidence }
export type Evaluation = EvaluationReceipt & { createdAt: string; rulesetVersion: string; snapshot: { opportunityId: string; observationId: string; snapshotId: string; rawText: string; rawSha256: string; source: OpportunitySource; acquiredAt: string }; hard: { overall: EvaluationStatus; rules: EvaluationHardRule[] }; soft: { matches: EvaluationSoftMatch[] }; facts: EvaluationFactEvidence[] }

const evaluationStatuses: EvaluationStatus[] = ['eligible', 'ineligible', 'unknown']
function onlyKeys(value: Record<string, unknown>, keys: string[]): boolean { return Object.keys(value).every((key) => keys.includes(key)) }
function validEvaluationFact(value: unknown): value is EvaluationFactEvidence {
 if (!isRecord(value) || !onlyKeys(value, ['factKey', 'value', 'revision', 'factRevision', 'source', 'confirmation', 'confirmedAt']) || !validIdentifier(value.factKey) || typeof value.value !== 'string' || !Number.isSafeInteger(value.revision) || Number(value.revision) < 0 || !Number.isSafeInteger(value.factRevision) || Number(value.factRevision) < 0 || !isRecord(value.source) || !onlyKeys(value.source, ['kind', 'label', 'referenceId']) || !validSource(value.source) || !isRecord(value.confirmation) || !onlyKeys(value.confirmation, ['userId', 'confirmedAt']) || !validIdentifier(value.confirmation.userId) || !validTimestamp(value.confirmation.confirmedAt) || !validTimestamp(value.confirmedAt)) return false
 return true
}
function validEvaluationJob(value: unknown): value is EvaluationJobEvidence {
 return isRecord(value) && onlyKeys(value, ['snapshotId', 'observationId', 'acquiredAt', 'rawSha256', 'spanStart', 'spanEnd', 'quotedText']) && validIdentifier(value.snapshotId) && validIdentifier(value.observationId) && validTimestamp(value.acquiredAt) && typeof value.rawSha256 === 'string' && /^[a-f0-9]{64}$/.test(value.rawSha256) && Number.isSafeInteger(value.spanStart) && Number(value.spanStart) >= 0 && Number.isSafeInteger(value.spanEnd) && Number(value.spanEnd) > Number(value.spanStart) && typeof value.quotedText === 'string' && value.quotedText.length > 0
}
function sameEvaluationFact(left: EvaluationFactEvidence, right: EvaluationFactEvidence): boolean {
 return left.factKey === right.factKey && left.value === right.value && left.revision === right.revision && left.factRevision === right.factRevision && left.confirmedAt === right.confirmedAt && left.source.kind === right.source.kind && left.source.label === right.source.label && left.source.referenceId === right.source.referenceId && left.confirmation.userId === right.confirmation.userId && left.confirmation.confirmedAt === right.confirmation.confirmedAt
}
function validEvaluationReceipt(value: unknown): value is Evaluation & EvaluationReceipt {
 return isRecord(value) && onlyKeys(value, ['kind', 'requestId', 'evaluationId', 'opportunityId', 'snapshotId', 'profileRevision', 'status', 'createdAt', 'rulesetVersion', 'snapshot', 'hard', 'soft', 'facts']) && value.kind === 'evaluation_created' && validIdentifier(value.requestId) && validIdentifier(value.evaluationId) && validIdentifier(value.opportunityId) && validIdentifier(value.snapshotId) && Number.isSafeInteger(value.profileRevision) && Number(value.profileRevision) >= 0 && evaluationStatuses.includes(value.status as EvaluationStatus)
}
export function decodeEvaluationReceipt(value: unknown): EvaluationReceipt {
 if (!validEvaluationReceipt(value)) throw new TypeError('invalid career evaluation receipt')
 return { kind: 'evaluation_created', requestId: value.requestId, evaluationId: value.evaluationId, opportunityId: value.opportunityId, snapshotId: value.snapshotId, profileRevision: value.profileRevision as number, status: value.status as EvaluationStatus }
}
export function decodeEvaluation(value: unknown): Evaluation {
 if (!validEvaluationReceipt(value) || !isRecord(value) || !onlyKeys(value, ['kind', 'requestId', 'evaluationId', 'opportunityId', 'snapshotId', 'profileRevision', 'status', 'createdAt', 'rulesetVersion', 'snapshot', 'hard', 'soft', 'facts']) || !validTimestamp(value.createdAt) || !validIdentifier(value.rulesetVersion) || !isRecord(value.snapshot) || !onlyKeys(value.snapshot, ['opportunityId', 'observationId', 'snapshotId', 'rawText', 'rawSha256', 'source', 'acquiredAt']) || value.snapshot.opportunityId !== value.opportunityId || value.snapshot.snapshotId !== value.snapshotId || !validIdentifier(value.snapshot.observationId) || typeof value.snapshot.rawText !== 'string' || !value.snapshot.rawText.trim() || typeof value.snapshot.rawSha256 !== 'string' || !/^[a-f0-9]{64}$/.test(value.snapshot.rawSha256) || !isRecord(value.snapshot.source) || !validIdentifier(value.snapshot.source.kind) || (value.snapshot.source.label !== undefined && typeof value.snapshot.source.label !== 'string') || (value.snapshot.source.referenceId !== undefined && typeof value.snapshot.source.referenceId !== 'string') || !validTimestamp(value.snapshot.acquiredAt) || !isRecord(value.hard) || !onlyKeys(value.hard, ['overall', 'rules']) || !evaluationStatuses.includes(value.hard.overall as EvaluationStatus) || value.hard.overall !== value.status || !Array.isArray(value.hard.rules) || !isRecord(value.soft) || !onlyKeys(value.soft, ['matches']) || !Array.isArray(value.soft.matches) || !Array.isArray(value.facts) || !value.facts.every(validEvaluationFact)) throw new TypeError('invalid career evaluation')
 const rawText = value.snapshot.rawText
 const bytes = new TextEncoder().encode(rawText)
 const validEvidence = (evidence: unknown): evidence is EvaluationJobEvidence => {
  if (!validEvaluationJob(evidence) || evidence.snapshotId !== value.snapshot.snapshotId || evidence.observationId !== value.snapshot.observationId || evidence.acquiredAt !== value.snapshot.acquiredAt || evidence.rawSha256 !== value.snapshot.rawSha256 || evidence.spanEnd > bytes.length) return false
  return new TextDecoder().decode(bytes.slice(evidence.spanStart, evidence.spanEnd)) === evidence.quotedText
 }
 const validRule = (rule: unknown): rule is EvaluationHardRule => isRecord(rule) && onlyKeys(rule, ['ruleId', 'criterion', 'outcome', 'reasonCode', 'jobEvidence', 'profileEvidence']) && validIdentifier(rule.ruleId) && typeof rule.criterion === 'string' && evaluationStatuses.includes(rule.outcome as EvaluationStatus) && validIdentifier(rule.reasonCode) && (rule.jobEvidence === undefined || validEvidence(rule.jobEvidence)) && (rule.profileEvidence === undefined || (validEvaluationFact(rule.profileEvidence) && rule.profileEvidence.revision === value.profileRevision))
 const validMatch = (match: unknown): match is EvaluationSoftMatch => isRecord(match) && onlyKeys(match, ['kind', 'value', 'jobEvidence', 'profileEvidence']) && ['skill', 'project', 'intent'].includes(String(match.kind)) && typeof match.value === 'string' && validEvidence(match.jobEvidence) && validEvaluationFact(match.profileEvidence) && match.profileEvidence.revision === value.profileRevision
 if (!value.hard.rules.length || !value.hard.rules.every(validRule) || !value.soft.matches.every(validMatch) || !value.facts.every((fact) => fact.revision === value.profileRevision)) throw new TypeError('invalid career evaluation evidence')
 const citesPinnedFact = (fact: EvaluationFactEvidence | undefined): boolean => fact === undefined || value.facts.some((pinned) => sameEvaluationFact(fact, pinned))
 for (const rule of value.hard.rules as unknown as EvaluationHardRule[]) {
  if ((rule.outcome === 'eligible' || rule.outcome === 'ineligible') && (!rule.jobEvidence || !rule.profileEvidence)) throw new TypeError('conclusive career evaluation rule requires both evidence citations')
  if (!citesPinnedFact(rule.profileEvidence)) throw new TypeError('career evaluation rule cites a fact outside the pinned manifest')
 }
 for (const match of value.soft.matches as unknown as EvaluationSoftMatch[]) {
  if (!citesPinnedFact(match.profileEvidence)) throw new TypeError('career evaluation match cites a fact outside the pinned manifest')
 }
 const precedence: Record<EvaluationStatus, number> = { eligible: 0, unknown: 1, ineligible: 2 }
 const derivedOverall = (value.hard.rules as unknown as EvaluationHardRule[]).reduce<EvaluationStatus>((overall, rule) => precedence[rule.outcome] > precedence[overall] ? rule.outcome : overall, 'eligible')
 if (derivedOverall !== value.hard.overall) throw new TypeError('career evaluation overall status does not match its hard rules')
 return value as unknown as Evaluation
}
