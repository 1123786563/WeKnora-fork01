import assert from 'node:assert/strict';
import test from 'node:test';
import type { ClientRequest } from '../client.ts';
import { createTenantReleaseApi } from './tenant-releases.ts';

const submission = { id: 'submission-1', tenant_id: 7, listing_id: 'listing-1', agent_version_id: 'version-1', source_agent_id: 'agent-1', author_id: 'user-1', semantic_version: '1.0.0', bundle_digest: 'b'.repeat(64), manifest: {}, dependency_lock: { dependencies: [] }, status: 'pending_review', created_at: '2026-09-21T00:00:00Z' };
const review = { id: 'review-1', submission_id: submission.id, reviewer_id: 'admin-1', reviewed_digest: submission.bundle_digest, decision: 'approved', reason: '', created_at: '2026-09-21T00:00:00Z' };
const release = { id: 'release-1', listing_id: 'listing-1', submission_id: submission.id, agent_version_id: 'version-1', source_agent_id: 'agent-1', release_number: 1, semantic_version: '1.0.0', bundle_digest: submission.bundle_digest, manifest: {}, dependency_lock: { dependencies: [] }, published_by: 'admin-1', created_at: '2026-09-21T00:00:00Z' };
const listing = { id: 'listing-1', tenant_id: 7, source_agent_id: 'agent-1', display_name: 'Helper', summary: 'Help', state: 'published', current_release_id: release.id, created_at: '2026-09-21T00:00:00Z', updated_at: '2026-09-21T00:00:00Z' };

test('uses exact Tenant Release routes, methods, and request bodies', async () => {
  const calls: ClientRequest[] = [];
  const api = createTenantReleaseApi(async (call) => {
    calls.push(call);
    if (call.path.endsWith('/review')) return { success: true, data: { review, release } };
    if (call.path.endsWith('/review-queue')) return { success: true, data: [submission] };
    if (call.path.endsWith('/catalog')) return { success: true, data: [listing] };
    return { success: true, data: submission };
  });
  const metadata = { semantic_version: '1.0.0', display_name: 'Helper', summary: 'Help', supported_languages: ['en'], use_cases: ['support'], minimum_weknora_capability: '1', license_id: 'MIT' };
  await api.submit('version-1', metadata);
  await api.listReviewQueue();
  await api.review('submission/1', { expected_digest: submission.bundle_digest, decision: 'approved' });
  await api.listCatalog();
  assert.deepEqual(calls, [
    { method: 'POST', path: '/api/v1/marketplace/tenant/release-submissions', body: { agent_version_id: 'version-1', metadata } },
    { method: 'GET', path: '/api/v1/marketplace/tenant/release-submissions/review-queue' },
    { method: 'POST', path: '/api/v1/marketplace/tenant/release-submissions/submission%2F1/review', body: { expected_digest: submission.bundle_digest, decision: 'approved' } },
    { method: 'GET', path: '/api/v1/marketplace/tenant/catalog' },
  ]);
});

test('preserves rejected review response with no Release and rejects malformed response', async () => {
  const api = createTenantReleaseApi(async () => ({ success: true, data: { review: { ...review, decision: 'rejected' }, release: null } }));
  const result = await api.review('submission-1', { expected_digest: submission.bundle_digest, decision: 'rejected', reason: 'Needs changes' });
  assert.equal(result.release, null);
  const changesRequested = createTenantReleaseApi(async () => ({ success: true, data: { review: { ...review, decision: 'changes_requested' }, release: null } }));
  assert.equal((await changesRequested.review('submission-1', { expected_digest: submission.bundle_digest, decision: 'changes_requested' })).release, null);
  const malformed = createTenantReleaseApi(async () => ({ success: true, data: { review, release: null } }));
  await assert.rejects(malformed.review('submission-1', { expected_digest: submission.bundle_digest, decision: 'approved' }));
});
