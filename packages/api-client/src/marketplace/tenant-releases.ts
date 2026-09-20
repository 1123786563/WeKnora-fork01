import {
  parseReleaseReviewResponse,
  parseReleaseSubmissionListResponse,
  parseReleaseSubmissionResponse,
  parseTenantReleaseListResponse,
  type AgentRelease,
  type ReleaseReview,
  type ReleaseSubmission,
  type TenantReleaseListing,
} from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface ReleaseMetadataInput {
  semantic_version: string;
  display_name: string;
  summary: string;
  supported_languages: string[];
  use_cases: string[];
  non_use_cases?: string[];
  capability_requirements?: string[];
  data_categories?: string[];
  external_side_effects?: string[];
  minimum_weknora_capability: string;
  license_id: string;
  change_notes?: string;
}

export interface ReviewReleaseInput {
  expected_digest: string;
  decision: 'approved' | 'rejected' | 'changes_requested';
  reason?: string;
}

export interface TenantReleaseReviewResult {
  review: ReleaseReview;
  release: AgentRelease | null;
}

function required(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return value;
}

function digest(value: string): string {
  const checked = required(value, 'expected_digest');
  if (!/^[a-f\d]{64}$/i.test(checked)) throw new Error('expected_digest must be a SHA-256 hex digest');
  return checked;
}

const base = '/api/v1/marketplace/tenant';

export function createTenantReleaseApi(request: Request) {
  return {
    async submit(agentVersionID: string, metadata: ReleaseMetadataInput): Promise<ReleaseSubmission> {
      return parseReleaseSubmissionResponse(await request({
        method: 'POST',
        path: `${base}/release-submissions`,
        body: { agent_version_id: required(agentVersionID, 'agentVersionID'), metadata },
      }));
    },
    async listReviewQueue(): Promise<ReleaseSubmission[]> {
      return parseReleaseSubmissionListResponse(await request({ method: 'GET', path: `${base}/release-submissions/review-queue` }));
    },
    async review(submissionID: string, input: ReviewReleaseInput): Promise<TenantReleaseReviewResult> {
      if (input.decision !== 'approved' && input.decision !== 'rejected' && input.decision !== 'changes_requested') {
        throw new Error('decision must be approved, rejected, or changes_requested');
      }
      const reason = input.reason;
      if (reason !== undefined && typeof reason !== 'string') throw new Error('reason must be a string');
      return parseReleaseReviewResponse(await request({
        method: 'POST',
        path: `${base}/release-submissions/${encodeURIComponent(required(submissionID, 'submissionID'))}/review`,
        body: {
          expected_digest: digest(input.expected_digest),
          decision: input.decision,
          ...(reason === undefined ? {} : { reason }),
        },
      }));
    },
    async listCatalog(): Promise<TenantReleaseListing[]> {
      return parseTenantReleaseListResponse(await request({ method: 'GET', path: `${base}/catalog` }));
    },
  };
}

export type TenantReleaseApi = ReturnType<typeof createTenantReleaseApi>;
