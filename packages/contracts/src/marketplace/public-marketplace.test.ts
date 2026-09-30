import assert from 'node:assert/strict';
import test from 'node:test';
import { ContractError } from '../index.ts';
import {
  parseVerifiedPublisherResponse,
  parseVerifiedPublisherListResponse,
  parsePublicCatalogListResponse,
  parsePublicListingResponse,
  parsePublicSubmissionResponse,
  parsePublicSubmissionListResponse,
  parsePublicReviewResponse,
  parseAdoptPublicListingResponse,
} from './public-marketplace.ts';

const publisher = { tenant_id: 1, state: 'verified', verified_by: 'sysadmin', note: 'identity checked', verified_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:00:00Z' };
const catalogRow = {
  id: 'pub-listing-1', display_name: 'Public helper', summary: 'A portable helper', state: 'listed',
  publisher_tenant_id: 1, publisher_verified: true,
  current_release: {
    id: 'pub-release-1', semantic_version: '1.0.0', bundle_digest: 'a'.repeat(64),
    manifest: { capability_requirements: ['knowledge'] }, dependency_lock: { dependencies: [] },
    created_at: '2026-09-24T00:00:00Z',
  },
  created_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:00:00Z',
};
const submission = {
  id: 'pub-sub-1', publisher_tenant_id: 1, public_listing_id: 'pub-listing-1',
  source_listing_id: 'tenant-listing-1', source_release_id: 'tenant-release-1',
  publisher_actor_id: 'tenant-admin', semantic_version: '1.0.0', bundle_digest: 'a'.repeat(64),
  manifest: {}, dependency_lock: {}, status: 'submitted', created_at: '2026-09-24T00:00:00Z',
};
const review = { id: 'rev-1', submission_id: 'pub-sub-1', reviewer_id: 'platform-reviewer', reviewed_digest: 'a'.repeat(64), decision: 'approved', reason: '', created_at: '2026-09-24T00:00:00Z' };
const adoptResult = {
  introduction: {
    id: 'introduced-1', public_listing_id: 'pub-listing-1', public_release_id: 'pub-release-1',
    display_name: 'Public helper', summary: 'A portable helper', semantic_version: '1.0.0',
    bundle_digest: 'a'.repeat(64), introduced_by: 'adopter-admin', introduced_at: '2026-09-24T00:00:00Z',
  },
  adoption: {
    id: 'adoption-1', listing_id: 'pub-listing-1', accepted_release_id: 'introduced-1',
    state: 'active', created_by: 'adopter-admin', created_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:00:00Z',
  },
};

test('parses verified publishers, catalog rows and listing detail', () => {
  assert.equal(parseVerifiedPublisherResponse({ success: true, data: publisher }).state, 'verified');
  assert.deepEqual(parseVerifiedPublisherListResponse({ success: true, data: [publisher] }), [publisher]);
  const rows = parsePublicCatalogListResponse({ success: true, data: [catalogRow] });
  assert.equal(rows[0].current_release?.semantic_version, '1.0.0');
  assert.equal(rows[0].publisher_verified, true);
  assert.equal(parsePublicListingResponse({ success: true, data: catalogRow }).id, catalogRow.id);
  assert.deepEqual(parsePublicCatalogListResponse({ success: true, data: [] }), []);
});

test('parses submissions, review and adoption result', () => {
  assert.equal(parsePublicSubmissionResponse({ success: true, data: submission }).status, 'submitted');
  assert.deepEqual(parsePublicSubmissionListResponse({ success: true, data: [submission] }), [submission]);
  const parsed = parsePublicReviewResponse({ success: true, data: { review, release: null } });
  assert.equal(parsed.review.decision, 'approved');
  const adopted = parseAdoptPublicListingResponse({ success: true, data: adoptResult });
  assert.equal(adopted.adoption.accepted_release_id, adopted.introduction.id);
});

test('rejects malformed envelopes, ids, digests, states and revoked rows in list context', () => {
  assert.throws(() => parsePublicCatalogListResponse({ success: false, data: [catalogRow] }), ContractError);
  assert.throws(() => parsePublicCatalogListResponse({ success: true, data: catalogRow }), ContractError);
  assert.throws(() => parsePublicCatalogListResponse({ success: true, data: [{ ...catalogRow, id: '' }] }), ContractError);
  assert.throws(() => parsePublicCatalogListResponse({ success: true, data: [{ ...catalogRow, current_release: { ...catalogRow.current_release, bundle_digest: 'not-hex' } }] }), ContractError);
  assert.throws(() => parsePublicSubmissionResponse({ success: true, data: { ...submission, publisher_tenant_id: 0 } }), ContractError);
  assert.throws(() => parsePublicReviewResponse({ success: true, data: { review: { ...review, decision: 'maybe' }, release: null } }), ContractError);
  assert.throws(() => parseVerifiedPublisherResponse({ success: true, data: { ...publisher, state: 'unknown' } }), ContractError);
  assert.throws(() => parseAdoptPublicListingResponse({ success: true, data: { ...adoptResult, adoption: { ...adoptResult.adoption, listing_id: '' } } }), ContractError);
});
