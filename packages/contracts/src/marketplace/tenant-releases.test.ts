import assert from 'node:assert/strict';
import test from 'node:test';
import { ContractError } from '../index.ts';
import { parseReleaseReviewResponse, parseReleaseSubmissionResponse, parseTenantReleaseListResponse } from './tenant-releases.ts';

const submission = { id: 'submission-1', tenant_id: 7, listing_id: 'listing-1', agent_version_id: 'version-1', source_agent_id: 'agent-1', author_id: 'user-1', semantic_version: '1.0.0', bundle_digest: 'b'.repeat(64), manifest: {}, dependency_lock: { dependencies: [] }, status: 'pending_review', created_at: '2026-09-21T00:00:00Z' };
const review = { id: 'review-1', submission_id: submission.id, reviewer_id: 'admin-1', reviewed_digest: submission.bundle_digest, decision: 'approved', reason: '', created_at: '2026-09-21T00:00:00Z' };
const release = { id: 'release-1', listing_id: 'listing-1', submission_id: submission.id, agent_version_id: 'version-1', source_agent_id: 'agent-1', release_number: 1, semantic_version: '1.0.0', bundle_digest: submission.bundle_digest, manifest: {}, dependency_lock: { dependencies: [] }, published_by: 'admin-1', created_at: '2026-09-21T00:00:00Z' };
const listing = { id: 'listing-1', tenant_id: 7, source_agent_id: 'agent-1', display_name: 'Helper', summary: 'Help', state: 'published', current_release_id: release.id, created_at: '2026-09-21T00:00:00Z', updated_at: '2026-09-21T00:00:00Z' };

test('parses immutable Submission and Release identifiers and digests', () => {
  assert.equal(parseReleaseSubmissionResponse({ success: true, data: submission }).id, submission.id);
  const approved = parseReleaseReviewResponse({ success: true, data: { review, release } });
  assert.equal(approved.review.reviewed_digest, submission.bundle_digest);
  assert.equal(approved.release?.id, release.id);
});

test('review carries a Release only for approval', () => {
  const rejected = parseReleaseReviewResponse({ success: true, data: { review: { ...review, decision: 'rejected', reason: 'Needs changes' }, release: null } });
  assert.equal(rejected.release, null);
  assert.throws(() => parseReleaseReviewResponse({ success: true, data: { review, release: null } }), ContractError);
  assert.throws(() => parseReleaseReviewResponse({ success: true, data: { review: { ...review, decision: 'rejected' }, release } }), ContractError);
  const changesRequested = parseReleaseReviewResponse({ success: true, data: { review: { ...review, decision: 'changes_requested' }, release: null } });
  assert.equal(changesRequested.release, null);
  assert.throws(() => parseReleaseReviewResponse({ success: true, data: { review: { ...review, decision: 'changes_requested' }, release } }), ContractError);
  assert.throws(() => parseReleaseReviewResponse({ success: true, data: { review: { ...review, decision: 'approved', submission_id: 'another-submission' }, release } }), ContractError);
});

test('rejects malformed identifiers, digests, and response envelopes', () => {
  assert.throws(() => parseReleaseSubmissionResponse({ success: true, data: { ...submission, id: '' } }), ContractError);
  assert.throws(() => parseReleaseSubmissionResponse({ success: true, data: { ...submission, bundle_digest: 'bad' } }), ContractError);
  assert.throws(() => parseReleaseReviewResponse({ success: true, data: { review: { ...review, reviewed_digest: 'bad' }, release } }), ContractError);
  assert.throws(() => parseTenantReleaseListResponse({ success: false, data: [] }), ContractError);
  assert.deepEqual(parseTenantReleaseListResponse({ success: true, data: [listing] }), [listing]);
});
