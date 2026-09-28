import {
  ContractError, parseCareerApplication, parseCareerDeleteReceipt, parseCareerEnvelope, parseCareerEvaluation,
  parseCareerExportReceipt, parseCareerMaterialVersion, parseCareerOpportunity, parseCareerProfile,
  parseCareerReminder, parseCareerSearchReceipt, parseCareerSubmission, parseCareerTimelineEvent,
} from '@weknora/contracts';
import type {
  CareerApplication, CareerDeleteReceipt, CareerEnvelope, CareerEvaluation, CareerExportReceipt,
  CareerMaterialVersion, CareerOpportunity, CareerProfile, CareerReminder, CareerSearchReceipt,
  CareerSubmission, CareerTimelineEvent, CareerWorkspace, CareerReceipt,
} from '@weknora/contracts';

export function decodeCareerEnvelope<T>(value: unknown, parseData: (data: unknown) => T, requestId?: string, requireReceipt = false): { success: true; data: T; requestId?: string } {
  return parseCareerEnvelope(value, parseData, requestId, requireReceipt);
}
function array<T>(value: unknown, parse: (item: unknown) => T, path: string): T[] {
  if (!Array.isArray(value)) throw new ContractError(path, 'expected array');
  return value.map(parse);
}
function obj(value: unknown, path: string): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new ContractError(path, 'expected object');
  return value as Record<string, unknown>;
}
function exact(row: Record<string, unknown>, fields: readonly string[], path: string): void {
  for (const key of Object.keys(row)) if (!fields.includes(key)) throw new ContractError(`${path}.${key}`, 'unknown field');
}
function revision(value: unknown, path: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 1) throw new ContractError(path, 'expected positive safe revision');
  return value;
}
export function decodeCareerProfile(value: unknown, requestId?: string, requireReceipt = false): CareerProfile {
  return decodeCareerEnvelope(value, parseCareerProfile, requestId, requireReceipt).data;
}
export function decodeCareerOpportunityList(value: unknown): CareerOpportunity[] { return array(value, parseCareerOpportunity, 'opportunities'); }
export function decodeCareerEvaluation(value: unknown): CareerEvaluation { return parseCareerEvaluation(value); }
export function decodeCareerApplication(value: unknown): CareerApplication { return parseCareerApplication(value); }
export function decodeCareerMaterialVersion(value: unknown): CareerMaterialVersion { return parseCareerMaterialVersion(value); }
export function decodeCareerSearchReceipt(value: unknown, requestId: string): CareerSearchReceipt {
  const receipt = parseCareerSearchReceipt(value);
  if (receipt.requestId !== requestId) throw new ContractError('searchReceipt.requestId', 'request ID mismatch');
  return receipt;
}
export function decodeCareerSubmission(value: unknown): CareerSubmission { return parseCareerSubmission(value); }
export function decodeCareerTimeline(value: unknown): CareerTimelineEvent[] { return array(value, parseCareerTimelineEvent, 'timeline'); }
export function decodeCareerReminders(value: unknown): CareerReminder[] { return array(value, parseCareerReminder, 'reminders'); }
export function decodeCareerExportReceipt(value: unknown, requestId: string): CareerExportReceipt {
  const receipt = parseCareerExportReceipt(value);
  if (receipt.requestId !== requestId) throw new ContractError('exportReceipt.requestId', 'request ID mismatch');
  return receipt;
}
export function decodeCareerDeleteReceipt(value: unknown, requestId: string): CareerDeleteReceipt {
  const receipt = parseCareerDeleteReceipt(value);
  if (receipt.requestId !== requestId) throw new ContractError('deleteReceipt.requestId', 'request ID mismatch');
  return receipt;
}
export function decodeCareerWorkspace(value: unknown): CareerWorkspace {
  const row = obj(value, 'workspace'); exact(row, ['profile', 'opportunities', 'applications'], 'workspace');
  return {
    ...(row.profile === undefined ? {} : { profile: parseCareerProfile(row.profile) }),
    opportunities: array(row.opportunities, parseCareerOpportunity, 'workspace.opportunities'),
    applications: array(row.applications, parseCareerApplication, 'workspace.applications'),
  };
}
export function decodeCareerEnvelopeValue<T>(value: unknown, parseValue: (v: unknown) => T): CareerEnvelope<T> {
  const row = obj(value, 'careerEnvelope'); exact(row, ['revision', 'value'], 'careerEnvelope');
  return { revision: revision(row.revision, 'careerEnvelope.revision'), value: parseValue(row.value) };
}
export function decodeCareerReceipt(value: unknown, requestId: string): CareerReceipt<CareerWorkspace> {
  const row = obj(value, 'careerReceipt');
  if (row.requestId !== requestId) throw new ContractError('careerReceipt.requestId', 'request ID mismatch');
  if (row.kind === 'unknown' || row.kind === 'forbidden') {
    exact(row, ['kind', 'requestId'], 'careerReceipt');
    return { kind: row.kind, requestId };
  }
  if (row.kind === 'applied' || row.kind === 'conflict') {
    exact(row, ['kind', 'requestId', 'envelope'], 'careerReceipt');
    return { kind: row.kind, requestId, envelope: decodeCareerEnvelopeValue(row.envelope, decodeCareerWorkspace) };
  }
  throw new ContractError('careerReceipt.kind', 'unknown receipt kind');
}
