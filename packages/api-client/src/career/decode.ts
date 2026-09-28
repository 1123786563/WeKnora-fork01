import { ContractError, parseCareerEvaluation, parseCareerMaterialVersion, parseCareerOpportunity, parseCareerProfile } from '@weknora/contracts';
import type { CareerApplication, CareerOpportunity, CareerMaterialVersion } from '@weknora/contracts';
export interface CareerReceipt<T> { requestId: string; data: T }
function receipt(value: unknown, requestId?: string): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new ContractError('receipt', 'expected object');
  const row = value as Record<string, unknown>;
  if (typeof row.requestId !== 'string' || !row.requestId.trim()) throw new ContractError('receipt.requestId', 'expected request ID');
  if (requestId !== undefined && row.requestId !== requestId) throw new ContractError('receipt.requestId', 'request ID mismatch');
  if ('tenantId' in row || 'tenant_id' in row || 'ownerId' in row || 'owner_id' in row || 'actorId' in row) throw new ContractError('receipt', 'tenant/owner/actor are not client authority');
  return row;
}
export function decodeCareerProfile(value: unknown, requestId?: string) { return parseCareerProfile(receipt(value, requestId).data); }
export function decodeCareerOpportunityList(value: unknown): CareerOpportunity[] {
  if (!Array.isArray(value)) throw new ContractError('opportunities', 'expected array');
  return value.map(parseCareerOpportunity);
}
export function decodeCareerEvaluation(value: unknown) { return parseCareerEvaluation(value); }
export function decodeCareerMaterialVersion(value: unknown): CareerMaterialVersion { return parseCareerMaterialVersion(value); }
export function decodeCareerApplication(value: unknown): CareerApplication {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new ContractError('application', 'expected object');
  const row = value as Record<string, unknown>;
  if (Object.keys(row).some(key => ['tenantId', 'tenant_id', 'ownerId', 'owner_id', 'actorId'].includes(key))) throw new ContractError('application', 'tenant/owner/actor are not client authority');
  if (!['preparing', 'ready_to_submit', 'submitted', 'assessment', 'interview', 'offer', 'closed'].includes(String(row.stage))) throw new ContractError('application.stage', 'unknown stage');
  if (!Number.isSafeInteger(row.revision) || Number(row.revision) < 1) throw new ContractError('application.revision', 'invalid revision');
  const ref = row.opportunitySnapshot as Record<string, unknown> | undefined;
  if (!ref || typeof ref !== 'object' || typeof ref.opportunityId !== 'string' || !Number.isSafeInteger(ref.revision) || typeof ref.digest !== 'string' || !/^sha256:[a-f0-9]{64}$/.test(ref.digest)) throw new ContractError('application.opportunitySnapshot', 'invalid immutable snapshot reference');
  if (Object.keys(row).some(key => !['id', 'revision', 'opportunitySnapshot', 'stage'].includes(key))) throw new ContractError('application', 'unknown field');
  return row as unknown as CareerApplication;
}
