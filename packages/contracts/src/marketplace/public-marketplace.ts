import { ContractError } from '../index.ts';

export interface VerifiedPublisher {
  tenant_id: number;
  state: 'verified' | 'revoked';
  verified_by: string;
  note: string;
  verified_at: string;
  updated_at: string;
  [key: string]: unknown;
}

export interface PublicCatalogRelease {
  id: string;
  semantic_version: string;
  bundle_digest: string;
  manifest: Record<string, unknown>;
  dependency_lock: Record<string, unknown>;
  created_at: string;
  [key: string]: unknown;
}

/**
 * Public catalog row. The field set mirrors the Go wire
 * (publicCatalogListingResponse) exactly: it carries listing, publisher
 * trust signal and current release documentation ONLY — never adopter
 * identity, adoption counts, mappings or task data (spec §12).
 */
export interface PublicCatalogListing {
  id: string;
  display_name: string;
  summary: string;
  state: string;
  publisher_tenant_id: number;
  publisher_verified: boolean;
  current_release?: PublicCatalogRelease;
  created_at: string;
  updated_at: string;
  [key: string]: unknown;
}

export interface PublicReleaseSubmission {
  id: string;
  publisher_tenant_id: number;
  public_listing_id: string;
  source_listing_id: string;
  source_release_id: string;
  publisher_actor_id: string;
  semantic_version: string;
  bundle_digest: string;
  manifest: Record<string, unknown>;
  dependency_lock: Record<string, unknown>;
  status: string;
  created_at: string;
  [key: string]: unknown;
}

export interface PublicReleaseReview {
  id: string;
  submission_id: string;
  reviewer_id: string;
  reviewed_digest: string;
  decision: 'approved' | 'rejected' | 'changes_requested';
  reason: string;
  created_at: string;
  [key: string]: unknown;
}

export interface PublicIntroduction {
  id: string;
  public_listing_id: string;
  public_release_id: string;
  display_name: string;
  summary: string;
  semantic_version: string;
  bundle_digest: string;
  introduced_by: string;
  introduced_at: string;
  [key: string]: unknown;
}

export interface PublicAdoption {
  id: string;
  listing_id: string;
  accepted_release_id: string;
  state: string;
  created_by: string;
  created_at: string;
  updated_at: string;
  [key: string]: unknown;
}

export interface AdoptPublicListingResult {
  introduction: PublicIntroduction;
  adoption: PublicAdoption;
  [key: string]: unknown;
}

const REVIEW_DECISIONS = ['approved', 'rejected', 'changes_requested'] as const;
const PUBLISHER_STATES = ['verified', 'revoked'] as const;

function object(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ContractError(path, 'expected an object');
  return value as Record<string, unknown>;
}

function envelope(value: unknown): Record<string, unknown> {
  const row = object(value, 'response');
  if (row.success !== true) throw new ContractError('response.success', 'expected success to be true');
  return row;
}

function requiredString(row: Record<string, unknown>, key: string, path: string): string {
  const value = row[key];
  if (typeof value !== 'string' || value.trim() === '') throw new ContractError(`${path}.${key}`, 'expected a non-empty string');
  return value;
}

function requiredPositiveInt(row: Record<string, unknown>, key: string, path: string): number {
  const value = row[key];
  if (typeof value !== 'number' || !Number.isInteger(value) || value <= 0) throw new ContractError(`${path}.${key}`, 'expected a positive integer');
  return value;
}

function requiredDocument(row: Record<string, unknown>, key: string, path: string): Record<string, unknown> {
  return object(row[key], `${path}.${key}`);
}

function requiredDigest(row: Record<string, unknown>, key: string, path: string): string {
  const digest = requiredString(row, key, path);
  if (!/^[0-9a-f]{64}$/.test(digest)) throw new ContractError(`${path}.${key}`, 'expected a lowercase sha-256 hex digest');
  return digest;
}

function oneOf<T extends readonly string[]>(row: Record<string, unknown>, key: string, values: T, path: string): T[number] {
  const value = row[key];
  if (typeof value !== 'string' || !(values as readonly string[]).includes(value)) {
    throw new ContractError(`${path}.${key}`, `expected one of ${values.join('|')}`);
  }
  return value as T[number];
}

function verifiedPublisher(row: Record<string, unknown>, path: string): VerifiedPublisher {
  return {
    tenant_id: requiredPositiveInt(row, 'tenant_id', path),
    state: oneOf(row, 'state', PUBLISHER_STATES, path),
    verified_by: requiredString(row, 'verified_by', path),
    note: typeof row.note === 'string' ? row.note : '',
    verified_at: requiredString(row, 'verified_at', path),
    updated_at: requiredString(row, 'updated_at', path),
  };
}

function publicCatalogRelease(row: Record<string, unknown>, path: string): PublicCatalogRelease {
  return {
    id: requiredString(row, 'id', path),
    semantic_version: requiredString(row, 'semantic_version', path),
    bundle_digest: requiredDigest(row, 'bundle_digest', path),
    manifest: requiredDocument(row, 'manifest', path),
    dependency_lock: requiredDocument(row, 'dependency_lock', path),
    created_at: requiredString(row, 'created_at', path),
  };
}

function publicCatalogListing(row: Record<string, unknown>, path: string): PublicCatalogListing {
  const listing: PublicCatalogListing = {
    id: requiredString(row, 'id', path),
    display_name: requiredString(row, 'display_name', path),
    summary: typeof row.summary === 'string' ? row.summary : '',
    state: requiredString(row, 'state', path),
    publisher_tenant_id: requiredPositiveInt(row, 'publisher_tenant_id', path),
    publisher_verified: row.publisher_verified === true,
    created_at: requiredString(row, 'created_at', path),
    updated_at: requiredString(row, 'updated_at', path),
  };
  if (row.current_release !== null && row.current_release !== undefined) {
    listing.current_release = publicCatalogRelease(object(row.current_release, `${path}.current_release`), `${path}.current_release`);
  }
  return listing;
}

function publicSubmission(row: Record<string, unknown>, path: string): PublicReleaseSubmission {
  return {
    id: requiredString(row, 'id', path),
    publisher_tenant_id: requiredPositiveInt(row, 'publisher_tenant_id', path),
    public_listing_id: requiredString(row, 'public_listing_id', path),
    source_listing_id: requiredString(row, 'source_listing_id', path),
    source_release_id: requiredString(row, 'source_release_id', path),
    publisher_actor_id: typeof row.publisher_actor_id === 'string' ? row.publisher_actor_id : '',
    semantic_version: requiredString(row, 'semantic_version', path),
    bundle_digest: requiredDigest(row, 'bundle_digest', path),
    manifest: requiredDocument(row, 'manifest', path),
    dependency_lock: requiredDocument(row, 'dependency_lock', path),
    status: requiredString(row, 'status', path),
    created_at: requiredString(row, 'created_at', path),
  };
}

function publicAdoption(row: Record<string, unknown>, path: string): PublicAdoption {
  return {
    id: requiredString(row, 'id', path),
    listing_id: requiredString(row, 'listing_id', path),
    accepted_release_id: requiredString(row, 'accepted_release_id', path),
    state: requiredString(row, 'state', path),
    created_by: typeof row.created_by === 'string' ? row.created_by : '',
    created_at: requiredString(row, 'created_at', path),
    updated_at: requiredString(row, 'updated_at', path),
  };
}

export function parseVerifiedPublisherResponse(value: unknown): VerifiedPublisher {
  return verifiedPublisher(object(envelope(value).data, 'response.data'), 'response.data');
}

export function parseVerifiedPublisherListResponse(value: unknown): VerifiedPublisher[] {
  const rows = envelope(value).data;
  if (!Array.isArray(rows)) throw new ContractError('response.data', 'expected an array');
  return rows.map((row, index) => verifiedPublisher(object(row, `response.data[${index}]`), `response.data[${index}]`));
}

export function parsePublicCatalogListResponse(value: unknown): PublicCatalogListing[] {
  const rows = envelope(value).data;
  if (!Array.isArray(rows)) throw new ContractError('response.data', 'expected an array');
  return rows.map((row, index) => publicCatalogListing(object(row, `response.data[${index}]`), `response.data[${index}]`));
}

export function parsePublicListingResponse(value: unknown): PublicCatalogListing {
  return publicCatalogListing(object(envelope(value).data, 'response.data'), 'response.data');
}

export function parsePublicSubmissionResponse(value: unknown): PublicReleaseSubmission {
  return publicSubmission(object(envelope(value).data, 'response.data'), 'response.data');
}

export function parsePublicSubmissionListResponse(value: unknown): PublicReleaseSubmission[] {
  const rows = envelope(value).data;
  if (!Array.isArray(rows)) throw new ContractError('response.data', 'expected an array');
  return rows.map((row, index) => publicSubmission(object(row, `response.data[${index}]`), `response.data[${index}]`));
}

export function parsePublicReviewResponse(value: unknown): { review: PublicReleaseReview } {
  const row = object(envelope(value).data, 'response.data');
  const reviewRow = object(row.review, 'response.data.review');
  return {
    review: {
      id: requiredString(reviewRow, 'id', 'response.data.review'),
      submission_id: requiredString(reviewRow, 'submission_id', 'response.data.review'),
      reviewer_id: requiredString(reviewRow, 'reviewer_id', 'response.data.review'),
      reviewed_digest: requiredDigest(reviewRow, 'reviewed_digest', 'response.data.review'),
      decision: oneOf(reviewRow, 'decision', REVIEW_DECISIONS, 'response.data.review'),
      reason: typeof reviewRow.reason === 'string' ? reviewRow.reason : '',
      created_at: requiredString(reviewRow, 'created_at', 'response.data.review'),
    },
  };
}

export function parseAdoptPublicListingResponse(value: unknown): AdoptPublicListingResult {
  const row = object(envelope(value).data, 'response.data');
  return {
    introduction: (() => {
      const introduction = object(row.introduction, 'response.data.introduction');
      return {
        id: requiredString(introduction, 'id', 'response.data.introduction'),
        public_listing_id: requiredString(introduction, 'public_listing_id', 'response.data.introduction'),
        public_release_id: requiredString(introduction, 'public_release_id', 'response.data.introduction'),
        display_name: requiredString(introduction, 'display_name', 'response.data.introduction'),
        summary: typeof introduction.summary === 'string' ? introduction.summary : '',
        semantic_version: requiredString(introduction, 'semantic_version', 'response.data.introduction'),
        bundle_digest: requiredDigest(introduction, 'bundle_digest', 'response.data.introduction'),
        introduced_by: typeof introduction.introduced_by === 'string' ? introduction.introduced_by : '',
        introduced_at: requiredString(introduction, 'introduced_at', 'response.data.introduction'),
      };
    })(),
    adoption: publicAdoption(object(row.adoption, 'response.data.adoption'), 'response.data.adoption'),
  };
}
