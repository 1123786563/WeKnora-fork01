import { ContractError } from '../index.ts';
export * from './profile.ts';
export * from './opportunity.ts';
export * from './application.ts';
export * from './search.ts';
export * from './privacy.ts';
import type { CareerProfile } from './profile.ts';
import type { CareerOpportunity } from './opportunity.ts';
import type { CareerEvaluation } from './search.ts';
import type { CareerMaterialVersion } from './application.ts';

function object(value: unknown, path: string): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new ContractError(path, 'expected object');
  return value as Record<string, unknown>;
}
function string(value: unknown, path: string): string {
  if (typeof value !== 'string' || !value.trim()) throw new ContractError(path, 'expected non-empty string');
  return value;
}
function revision(value: unknown, path: string): number {
  if (!Number.isSafeInteger(value) || (value as number) < 1) throw new ContractError(path, 'expected positive revision');
  return value as number;
}
function digest(value: unknown, path: string): string {
  if (typeof value !== 'string' || !/^sha256:[a-f0-9]{64}$/.test(value)) throw new ContractError(path, 'expected sha256 digest');
  return value;
}
function exactKeys(row: Record<string, unknown>, allowed: string[], path: string) {
  for (const key of Object.keys(row)) if (!allowed.includes(key)) throw new ContractError(`${path}.${key}`, 'unknown or untrusted field');
}
export function parseCareerProfile(value: unknown): CareerProfile {
  const row = object(value, 'profile'); exactKeys(row, ['id', 'revision', 'facts'], 'profile');
  if (!Array.isArray(row.facts)) throw new ContractError('profile.facts', 'expected array');
  const facts = row.facts.map((value, i) => {
    const path = `profile.facts[${i}]`; const fact = object(value, path);
    exactKeys(fact, ['id', 'value', 'confirmed', 'source'], path);
    if (typeof fact.confirmed !== 'boolean') throw new ContractError(`${path}.confirmed`, 'expected boolean');
    return { id: string(fact.id, `${path}.id`), value: fact.value, confirmed: fact.confirmed, source: string(fact.source, `${path}.source`) };
  });
  return { id: string(row.id, 'profile.id'), revision: revision(row.revision, 'profile.revision'), facts };
}
export function parseCareerOpportunity(value: unknown): CareerOpportunity {
  const row = object(value, 'opportunity'); exactKeys(row, ['id', 'snapshot'], 'opportunity');
  const snapshot = object(row.snapshot, 'opportunity.snapshot'); exactKeys(snapshot, ['revision', 'digest', 'observedAt', 'sourceUrl', 'content'], 'opportunity.snapshot');
  const sourceUrl = string(snapshot.sourceUrl, 'opportunity.snapshot.sourceUrl');
  try { if (new URL(sourceUrl).protocol !== 'https:') throw new Error(); } catch { throw new ContractError('opportunity.snapshot.sourceUrl', 'expected https URL'); }
  return { id: string(row.id, 'opportunity.id'), snapshot: { revision: revision(snapshot.revision, 'opportunity.snapshot.revision'), digest: digest(snapshot.digest, 'opportunity.snapshot.digest'), observedAt: string(snapshot.observedAt, 'opportunity.snapshot.observedAt'), sourceUrl, content: string(snapshot.content, 'opportunity.snapshot.content') } };
}
export function parseCareerEvaluation(value: unknown): CareerEvaluation {
  const row = object(value, 'evaluation'); exactKeys(row, ['status', 'revision', 'explanation'], 'evaluation');
  if (!['eligible', 'ineligible', 'unknown'].includes(String(row.status))) throw new ContractError('evaluation.status', 'unknown eligibility enum');
  return { status: row.status as CareerEvaluation['status'], revision: revision(row.revision, 'evaluation.revision'), ...(row.explanation === undefined ? {} : { explanation: string(row.explanation, 'evaluation.explanation') }) };
}
export function parseCareerMaterialVersion(value: unknown): CareerMaterialVersion {
  const row = object(value, 'material'); exactKeys(row, ['id', 'version', 'digest', 'bodyDigest', 'pdfDigest', 'docxDigest', 'publishedAt'], 'material');
  for (const field of ['digest', 'bodyDigest', 'pdfDigest', 'docxDigest']) digest(row[field], `material.${field}`);
  if (!Number.isSafeInteger(row.version) || Number(row.version) < 1) throw new ContractError('material.version', 'expected positive version');
  return { id: string(row.id, 'material.id'), version: row.version as number, digest: row.digest as string, bodyDigest: row.bodyDigest as string, pdfDigest: row.pdfDigest as string, docxDigest: row.docxDigest as string, publishedAt: string(row.publishedAt, 'material.publishedAt') };
}
