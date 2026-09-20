import { ContractError } from '../index.ts';

export interface ReleaseSubmission {
  id: string;
  tenant_id: number;
  listing_id: string;
  agent_version_id: string;
  source_agent_id: string;
  author_id: string;
  semantic_version: string;
  bundle_digest: string;
  manifest: Record<string, unknown>;
  dependency_lock: Record<string, unknown>;
  payload: Record<string, unknown>;
  status: string;
  created_at: string;
  [key: string]: unknown;
}

export interface ReleaseReview {
  id: string;
  submission_id: string;
  reviewer_id: string;
  reviewed_digest: string;
  decision: 'approved' | 'rejected' | 'changes_requested';
  reason: string;
  created_at: string;
  [key: string]: unknown;
}

export interface AgentRelease {
  id: string;
  listing_id: string;
  submission_id: string;
  agent_version_id: string;
  source_agent_id: string;
  release_number: number;
  semantic_version: string;
  bundle_digest: string;
  manifest: Record<string, unknown>;
  dependency_lock: Record<string, unknown>;
  published_by: string;
  created_at: string;
  [key: string]: unknown;
}

export interface TenantReleaseListing {
  id: string;
  tenant_id: number;
  source_agent_id: string;
  display_name: string;
  summary: string;
  state: string;
  current_release_id?: string;
  created_at: string;
  updated_at: string;
  [key: string]: unknown;
}

export interface ReleaseReviewResult {
  review: ReleaseReview;
  release: AgentRelease | null;
}

function object(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ContractError(path, 'expected an object');
  return value as Record<string, unknown>;
}

function requiredString(row: Record<string, unknown>, key: string, path: string): string {
  const value = row[key];
  if (typeof value !== 'string' || value.trim() === '') throw new ContractError(`${path}.${key}`, 'expected a non-empty string');
  return value;
}

function digest(row: Record<string, unknown>, key: string, path: string): string {
  const value = requiredString(row, key, path);
  if (!/^[a-f\d]{64}$/i.test(value)) throw new ContractError(`${path}.${key}`, 'expected a SHA-256 hex digest');
  return value;
}

function tenantID(row: Record<string, unknown>, path: string): number {
  const value = row.tenant_id;
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) throw new ContractError(`${path}.tenant_id`, 'expected a non-negative safe integer');
  return value;
}

function record(row: Record<string, unknown>, key: string, path: string): Record<string, unknown> {
  return object(row[key], `${path}.${key}`);
}

function envelopeData(value: unknown): unknown {
  const envelope = object(value, '');
  if (envelope.success !== true) throw new ContractError('success', 'expected true');
  if (!Object.prototype.hasOwnProperty.call(envelope, 'data')) throw new ContractError('data', 'is required');
  return envelope.data;
}

function parseSubmission(value: unknown, path: string): ReleaseSubmission {
  const row = object(value, path);
  return {
    ...row,
    id: requiredString(row, 'id', path),
    tenant_id: tenantID(row, path),
    listing_id: requiredString(row, 'listing_id', path),
    agent_version_id: requiredString(row, 'agent_version_id', path),
    source_agent_id: requiredString(row, 'source_agent_id', path),
    author_id: requiredString(row, 'author_id', path),
    semantic_version: requiredString(row, 'semantic_version', path),
    bundle_digest: digest(row, 'bundle_digest', path),
    manifest: record(row, 'manifest', path),
    dependency_lock: record(row, 'dependency_lock', path),
    payload: record(row, 'payload', path),
    status: requiredString(row, 'status', path),
    created_at: requiredString(row, 'created_at', path),
  } as ReleaseSubmission;
}

function parseReview(value: unknown, path: string): ReleaseReview {
  const row = object(value, path);
  const decision = row.decision;
  if (decision !== 'approved' && decision !== 'rejected' && decision !== 'changes_requested') {
    throw new ContractError(`${path}.decision`, 'expected approved, rejected, or changes_requested');
  }
  if (typeof row.reason !== 'string') throw new ContractError(`${path}.reason`, 'expected a string');
  return {
    ...row,
    id: requiredString(row, 'id', path),
    submission_id: requiredString(row, 'submission_id', path),
    reviewer_id: requiredString(row, 'reviewer_id', path),
    reviewed_digest: digest(row, 'reviewed_digest', path),
    decision,
    reason: row.reason,
    created_at: requiredString(row, 'created_at', path),
  } as ReleaseReview;
}

function parseRelease(value: unknown, path: string): AgentRelease {
  const row = object(value, path);
  const releaseNumber = row.release_number;
  if (typeof releaseNumber !== 'number' || !Number.isSafeInteger(releaseNumber) || releaseNumber < 1) {
    throw new ContractError(`${path}.release_number`, 'expected a positive safe integer');
  }
  return {
    ...row,
    id: requiredString(row, 'id', path),
    listing_id: requiredString(row, 'listing_id', path),
    submission_id: requiredString(row, 'submission_id', path),
    agent_version_id: requiredString(row, 'agent_version_id', path),
    source_agent_id: requiredString(row, 'source_agent_id', path),
    release_number: releaseNumber,
    semantic_version: requiredString(row, 'semantic_version', path),
    bundle_digest: digest(row, 'bundle_digest', path),
    manifest: record(row, 'manifest', path),
    dependency_lock: record(row, 'dependency_lock', path),
    published_by: requiredString(row, 'published_by', path),
    created_at: requiredString(row, 'created_at', path),
  } as AgentRelease;
}

function parseListing(value: unknown, path: string): TenantReleaseListing {
  const row = object(value, path);
  if (row.current_release_id !== undefined && row.current_release_id !== null && typeof row.current_release_id !== 'string') {
    throw new ContractError(`${path}.current_release_id`, 'expected a string when present');
  }
  return {
    ...row,
    id: requiredString(row, 'id', path),
    tenant_id: tenantID(row, path),
    source_agent_id: requiredString(row, 'source_agent_id', path),
    display_name: requiredString(row, 'display_name', path),
    summary: typeof row.summary === 'string' ? row.summary : (() => { throw new ContractError(`${path}.summary`, 'expected a string'); })(),
    state: requiredString(row, 'state', path),
    ...(typeof row.current_release_id === 'string' ? { current_release_id: row.current_release_id } : {}),
    created_at: requiredString(row, 'created_at', path),
    updated_at: requiredString(row, 'updated_at', path),
  } as TenantReleaseListing;
}

export function parseReleaseSubmissionResponse(value: unknown): ReleaseSubmission {
  return parseSubmission(envelopeData(value), 'data');
}

export function parseReleaseReviewResponse(value: unknown): ReleaseReviewResult {
  const data = object(envelopeData(value), 'data');
  const review = parseReview(data.review, 'data.review');
  const release = data.release === null ? null : parseRelease(data.release, 'data.release');
  if (review.decision === 'approved' && release === null) throw new ContractError('data.release', 'is required for approval');
  if (review.decision !== 'approved' && release !== null) throw new ContractError('data.release', 'must be null for a non-approval decision');
  if (release !== null && release.submission_id !== review.submission_id) {
    throw new ContractError('data.release.submission_id', 'must match the reviewed submission');
  }
  if (release !== null && release.bundle_digest !== review.reviewed_digest) throw new ContractError('data.release.bundle_digest', 'must match the reviewed digest');
  return { review, release };
}

export function parseReleaseSubmissionListResponse(value: unknown): ReleaseSubmission[] {
  const data = envelopeData(value);
  if (!Array.isArray(data)) throw new ContractError('data', 'expected an array');
  return data.map((item, index) => parseSubmission(item, `data[${index}]`));
}

export function parseTenantReleaseListResponse(value: unknown): TenantReleaseListing[] {
  const data = envelopeData(value);
  if (!Array.isArray(data)) throw new ContractError('data', 'expected an array');
  return data.map((item, index) => parseListing(item, `data[${index}]`));
}
