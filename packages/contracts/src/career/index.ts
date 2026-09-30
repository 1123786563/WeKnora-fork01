import { ContractError } from '../contract-error.ts';
export * from './types.ts';
export * from './desk.ts';
export * from './profile.ts';
export * from './opportunity.ts';
export * from './application.ts';
export * from './search.ts';
export * from './privacy.ts';
import type { CareerApplication, CareerDeleteReceipt, CareerEvaluation, CareerExportReceipt, CareerJobSnapshot, CareerMaterialVersion, CareerOpportunity, CareerProfile, CareerProfileFact, CareerReminder, CareerSearchReceipt, CareerSearchRequest, CareerSubmission, CareerTimelineEvent } from './types.ts';

type Row = Record<string, unknown>;
function object(value: unknown, path: string): Row {
  if (value === null || typeof value !== 'object' || Array.isArray(value) || Object.getPrototypeOf(value) !== Object.prototype) throw new ContractError(path, 'expected plain object');
  return value as Row;
}
function keys(row: Row, allowed: readonly string[], path: string): void {
  for (const key of Object.keys(row)) if (!allowed.includes(key)) throw new ContractError(`${path}.${key}`, 'unknown or untrusted field');
}
function text(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new ContractError(path, 'expected non-empty string');
  return value;
}
function positiveRevision(value: unknown, path: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 1) throw new ContractError(path, 'expected positive safe revision');
  return value;
}
function enumValue<T extends string>(value: unknown, values: readonly T[], path: string): T {
  if (typeof value !== 'string' || !values.includes(value as T)) throw new ContractError(path, `expected one of ${values.join(', ')}`);
  return value as T;
}
function instant(value: unknown, path: string): string {
  const result = text(value, path);
  if (!/^\d{4}-\d\d-\d\dT.*Z$/.test(result) || Number.isNaN(Date.parse(result))) throw new ContractError(path, 'expected ISO-8601 UTC timestamp');
  return result;
}
function parseJsonValue(value: unknown, path: string): import('./types.ts').CareerJsonValue {
  if (value === null || typeof value === 'string' || typeof value === 'boolean') return value;
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  if (Array.isArray(value)) return value.map((entry, index) => parseJsonValue(entry, `${path}[${index}]`));
  const row = object(value, path); authority(row, path);
  return Object.fromEntries(Object.entries(row).map(([key, entry]) => [key, parseJsonValue(entry, `${path}.${key}`)]));
}
function authority(row: Row, path: string): void {
  for (const key of Object.keys(row)) if (/^(tenant|owner|actor)(_|[A-Z]|$)/i.test(key)) throw new ContractError(`${path}.${key}`, 'authority fields are not client data');
}
function digest(value: unknown, path: string): string {
  const result = text(value, path);
  if (!/^sha256:[a-f0-9]{64}$/.test(result)) throw new ContractError(path, 'expected sha256 digest');
  return result;
}
function parseProvenance(value: unknown, path: string): CareerProfileFact['provenance'] {
  const row = object(value, path); authority(row, path); keys(row, ['kind', 'reference'], path);
  return { kind: enumValue(row.kind, ['user', 'resume', 'interview', 'source'] as const, `${path}.kind`), ...(row.reference === undefined ? {} : { reference: text(row.reference, `${path}.reference`) }) };
}
export function parseCareerProfile(value: unknown): CareerProfile {
  const row = object(value, 'profile'); authority(row, 'profile'); keys(row, ['id', 'revision', 'facts'], 'profile');
  if (!Array.isArray(row.facts)) throw new ContractError('profile.facts', 'expected array');
  const facts = row.facts.map((entry, index): CareerProfileFact => {
    const path = `profile.facts[${index}]`; const fact = object(entry, path); authority(fact, path); keys(fact, ['id', 'value', 'confirmed', 'provenance', 'confirmation'], path);
    if (typeof fact.confirmed !== 'boolean') throw new ContractError(`${path}.confirmed`, 'expected boolean');
    let confirmation: CareerProfileFact['confirmation'];
    if (fact.confirmed) {
      const row = object(fact.confirmation, `${path}.confirmation`); keys(row, ['confirmedBy', 'at'], `${path}.confirmation`);
      confirmation = { confirmedBy: enumValue(row.confirmedBy, ['user'] as const, `${path}.confirmation.confirmedBy`), at: instant(row.at, `${path}.confirmation.at`) };
    } else if (fact.confirmation !== undefined) throw new ContractError(`${path}.confirmation`, 'unconfirmed fact cannot carry a confirmation');
    return { id: text(fact.id, `${path}.id`), value: parseJsonValue(fact.value, `${path}.value`), confirmed: fact.confirmed, provenance: parseProvenance(fact.provenance, `${path}.provenance`), ...(confirmation ? { confirmation } : {}) };
  });
  return { id: text(row.id, 'profile.id'), revision: positiveRevision(row.revision, 'profile.revision'), facts };
}
export function parseCareerJobSnapshot(value: unknown): CareerJobSnapshot {
  const row = object(value, 'snapshot'); authority(row, 'snapshot'); keys(row, ['revision', 'digest', 'observedAt', 'sourceUrl', 'content', 'completeness', 'failureReason'], 'snapshot');
  const sourceUrl = text(row.sourceUrl, 'snapshot.sourceUrl');
  try { const url = new URL(sourceUrl); if (url.protocol !== 'https:' || !url.hostname) throw new Error(); } catch { throw new ContractError('snapshot.sourceUrl', 'expected an HTTPS source URL'); }
  if (typeof row.content !== 'string') throw new ContractError('snapshot.content', 'expected string');
  const completeness = enumValue(row.completeness, ['complete', 'partial', 'unavailable'] as const, 'snapshot.completeness');
  const failureReason = row.failureReason === undefined ? undefined : text(row.failureReason, 'snapshot.failureReason');
  if (completeness === 'unavailable' && !failureReason) throw new ContractError('snapshot.failureReason', 'unavailable source requires failure reason');
  return { revision: positiveRevision(row.revision, 'snapshot.revision'), digest: digest(row.digest, 'snapshot.digest'), observedAt: instant(row.observedAt, 'snapshot.observedAt'), sourceUrl, content: row.content, completeness, ...(failureReason ? { failureReason } : {}) };
}
export function parseCareerOpportunity(value: unknown): CareerOpportunity {
  const row = object(value, 'opportunity'); authority(row, 'opportunity'); keys(row, ['id', 'snapshot'], 'opportunity');
  return { id: text(row.id, 'opportunity.id'), snapshot: parseCareerJobSnapshot(row.snapshot) };
}
export function parseCareerSearchRequest(value: unknown): CareerSearchRequest {
  const row = object(value, 'search'); authority(row, 'search'); keys(row, ['query', 'requestId'], 'search');
  return { query: text(row.query, 'search.query'), requestId: text(row.requestId, 'search.requestId') };
}
export function parseCareerSearchReceipt(value: unknown): CareerSearchReceipt {
  const row = object(value, 'searchReceipt'); authority(row, 'searchReceipt'); keys(row, ['requestId', 'revision', 'status', 'opportunityIds'], 'searchReceipt');
  if (!Array.isArray(row.opportunityIds)) throw new ContractError('searchReceipt.opportunityIds', 'expected array');
  return { requestId: text(row.requestId, 'searchReceipt.requestId'), revision: positiveRevision(row.revision, 'searchReceipt.revision'), status: enumValue(row.status, ['completed', 'pending', 'unknown'] as const, 'searchReceipt.status'), opportunityIds: row.opportunityIds.map((id, i) => text(id, `searchReceipt.opportunityIds[${i}]`)) };
}
export function parseCareerEvaluation(value: unknown): CareerEvaluation {
  const row = object(value, 'evaluation'); authority(row, 'evaluation'); keys(row, ['status', 'revision', 'profileRevision', 'opportunityRevision', 'modelVersion', 'evidence', 'explanation'], 'evaluation');
  const status = enumValue(row.status, ['eligible', 'ineligible', 'unknown'] as const, 'evaluation.status');
  if (!Array.isArray(row.evidence)) throw new ContractError('evaluation.evidence', 'expected array');
  const evidence = row.evidence.map((entry, index) => {
    const path = `evaluation.evidence[${index}]`; const item = object(entry, path); authority(item, path); keys(item, ['claim', 'result', 'source', 'factId', 'rationale'], path);
    return { claim: text(item.claim, `${path}.claim`), result: enumValue(item.result, ['supports', 'conflicts', 'missing'] as const, `${path}.result`), source: enumValue(item.source, ['confirmed_profile', 'job_requirement'] as const, `${path}.source`), ...(item.factId === undefined ? {} : { factId: text(item.factId, `${path}.factId`) }), ...(item.rationale === undefined ? {} : { rationale: text(item.rationale, `${path}.rationale`) }) };
  });
  return { status, revision: positiveRevision(row.revision, 'evaluation.revision'), profileRevision: positiveRevision(row.profileRevision, 'evaluation.profileRevision'), opportunityRevision: positiveRevision(row.opportunityRevision, 'evaluation.opportunityRevision'), modelVersion: text(row.modelVersion, 'evaluation.modelVersion'), evidence, ...(row.explanation === undefined ? {} : { explanation: text(row.explanation, 'evaluation.explanation') }) };
}
export function parseCareerApplication(value: unknown): CareerApplication {
  const row = object(value, 'application'); authority(row, 'application'); keys(row, ['id', 'revision', 'opportunitySnapshot', 'stage'], 'application');
  const ref = object(row.opportunitySnapshot, 'application.opportunitySnapshot'); authority(ref, 'application.opportunitySnapshot'); keys(ref, ['opportunityId', 'revision', 'digest'], 'application.opportunitySnapshot');
  return { id: text(row.id, 'application.id'), revision: positiveRevision(row.revision, 'application.revision'), opportunitySnapshot: { opportunityId: text(ref.opportunityId, 'application.opportunitySnapshot.opportunityId'), revision: positiveRevision(ref.revision, 'application.opportunitySnapshot.revision'), digest: digest(ref.digest, 'application.opportunitySnapshot.digest') }, stage: enumValue(row.stage, ['preparing', 'ready_to_submit', 'submitted', 'assessment', 'interview', 'offer', 'closed'] as const, 'application.stage') };
}
export function parseCareerMaterialVersion(value: unknown): CareerMaterialVersion {
  const row = object(value, 'material'); authority(row, 'material'); keys(row, ['id', 'version', 'digest', 'bodyDigest', 'pdfDigest', 'docxDigest', 'publishedAt'], 'material');
  return { id: text(row.id, 'material.id'), version: positiveRevision(row.version, 'material.version'), digest: digest(row.digest, 'material.digest'), bodyDigest: digest(row.bodyDigest, 'material.bodyDigest'), pdfDigest: digest(row.pdfDigest, 'material.pdfDigest'), docxDigest: digest(row.docxDigest, 'material.docxDigest'), publishedAt: instant(row.publishedAt, 'material.publishedAt') };
}
export function parseCareerSubmission(value: unknown): CareerSubmission {
  const row = object(value, 'submission'); authority(row, 'submission'); keys(row, ['id', 'channel', 'confirmedAt', 'materialVersion', 'versionUnknown'], 'submission');
  if (typeof row.versionUnknown !== 'boolean') throw new ContractError('submission.versionUnknown', 'expected boolean');
  let materialVersion: CareerSubmission['materialVersion'] = null;
  if (row.materialVersion !== null) {
    const ref = object(row.materialVersion, 'submission.materialVersion'); authority(ref, 'submission.materialVersion'); keys(ref, ['id', 'version', 'digest'], 'submission.materialVersion');
    materialVersion = { id: text(ref.id, 'submission.materialVersion.id'), version: positiveRevision(ref.version, 'submission.materialVersion.version'), digest: digest(ref.digest, 'submission.materialVersion.digest') };
  }
  if (row.versionUnknown !== (materialVersion === null)) throw new ContractError('submission.versionUnknown', 'must match whether actual material version is unknown');
  return { id: text(row.id, 'submission.id'), channel: text(row.channel, 'submission.channel'), confirmedAt: instant(row.confirmedAt, 'submission.confirmedAt'), materialVersion, versionUnknown: row.versionUnknown };
}
export type CareerTimelineKind = import('./types.ts').CareerTimelineKind;
export function parseCareerTimelineEvent(value: unknown): CareerTimelineEvent {
  const row = object(value, 'timelineEvent'); authority(row, 'timelineEvent'); keys(row, ['id', 'revision', 'kind', 'occurredAt', 'note', 'correctionOf'], 'timelineEvent');
  return { id: text(row.id, 'timelineEvent.id'), revision: positiveRevision(row.revision, 'timelineEvent.revision'), kind: enumValue(row.kind, ['submitted', 'assessment', 'interview', 'offer', 'rejected', 'withdrawn', 'correction'] as const, 'timelineEvent.kind'), occurredAt: instant(row.occurredAt, 'timelineEvent.occurredAt'), ...(row.note === undefined ? {} : { note: text(row.note, 'timelineEvent.note') }), ...(row.correctionOf === undefined ? {} : { correctionOf: text(row.correctionOf, 'timelineEvent.correctionOf') }) };
}
export type CareerReminderKind = import('./types.ts').CareerReminderKind;
export function parseCareerReminder(value: unknown): CareerReminder {
  const row = object(value, 'reminder'); authority(row, 'reminder'); keys(row, ['id', 'revision', 'kind', 'dueAt', 'status'], 'reminder');
  return { id: text(row.id, 'reminder.id'), revision: positiveRevision(row.revision, 'reminder.revision'), kind: enumValue(row.kind, ['deadline', 'assessment', 'interview', 'follow_up'] as const, 'reminder.kind'), dueAt: instant(row.dueAt, 'reminder.dueAt'), status: enumValue(row.status, ['active', 'paused', 'completed'] as const, 'reminder.status') };
}
export function parseCareerExportReceipt(value: unknown): CareerExportReceipt {
  const row = object(value, 'exportReceipt'); authority(row, 'exportReceipt'); keys(row, ['requestId', 'status', 'revision', 'artifactId', 'failureReason'], 'exportReceipt');
  const status = enumValue(row.status, ['pending', 'ready', 'failed'] as const, 'exportReceipt.status');
  if (status === 'ready' && row.artifactId === undefined) throw new ContractError('exportReceipt.artifactId', 'ready export requires artifact ID');
  if (status === 'failed' && row.failureReason === undefined) throw new ContractError('exportReceipt.failureReason', 'failed export requires failure reason');
  return { requestId: text(row.requestId, 'exportReceipt.requestId'), status, revision: positiveRevision(row.revision, 'exportReceipt.revision'), ...(row.artifactId === undefined ? {} : { artifactId: text(row.artifactId, 'exportReceipt.artifactId') }), ...(row.failureReason === undefined ? {} : { failureReason: text(row.failureReason, 'exportReceipt.failureReason') }) };
}
export function parseCareerDeleteReceipt(value: unknown): CareerDeleteReceipt {
  const row = object(value, 'deleteReceipt'); authority(row, 'deleteReceipt'); keys(row, ['requestId', 'status', 'revision'], 'deleteReceipt');
  return { requestId: text(row.requestId, 'deleteReceipt.requestId'), status: enumValue(row.status, ['accepted', 'completed'] as const, 'deleteReceipt.status'), revision: positiveRevision(row.revision, 'deleteReceipt.revision') };
}
export function parseCareerEnvelope<T>(value: unknown, parseData: (data: unknown) => T, expectedRequestId?: string, requireReceipt = false): { success: true; data: T; requestId?: string } {
  const row = object(value, 'envelope'); keys(row, ['success', 'data', 'requestId'], 'envelope');
  if (row.success !== true) throw new ContractError('envelope.success', 'expected success: true');
  if (!('data' in row)) throw new ContractError('envelope.data', 'missing data');
  if (requireReceipt && (typeof row.requestId !== 'string' || !row.requestId.trim())) throw new ContractError('envelope.requestId', 'required write receipt request ID is missing');
  if (row.requestId !== undefined && (typeof row.requestId !== 'string' || !row.requestId.trim())) throw new ContractError('envelope.requestId', 'expected non-empty request ID');
  if (expectedRequestId !== undefined && row.requestId !== expectedRequestId) throw new ContractError('envelope.requestId', 'request ID mismatch');
  return { success: true, data: parseData(row.data), ...(row.requestId === undefined ? {} : { requestId: row.requestId }) };
}
