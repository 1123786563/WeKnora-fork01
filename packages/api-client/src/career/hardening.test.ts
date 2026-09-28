import assert from 'node:assert/strict';
import test from 'node:test';
import { createCareerApi } from './index.ts';
import type { CareerScope } from '@weknora/contracts';

const scope: CareerScope = { deploymentOrigin: 'https://weknora.example', tenantId: 'tenant-1', actorId: 'actor-1' };
const profile = { id: 'profile-1', revision: 1, facts: [] };
const envelope = (data: unknown, requestId?: string) => ({ success: true, data, ...(requestId ? { requestId } : {}) });
const app = { id: 'app-1', revision: 1, opportunitySnapshot: { opportunityId: 'opp-1', revision: 1, digest: `sha256:${'a'.repeat(64)}` }, stage: 'preparing' };
const material = { id: 'm1', version: 1, digest: `sha256:${'a'.repeat(64)}`, bodyDigest: `sha256:${'b'.repeat(64)}`, pdfDigest: `sha256:${'c'.repeat(64)}`, docxDigest: `sha256:${'d'.repeat(64)}`, publishedAt: '2026-09-28T10:00:00Z' };

test('Career endpoints require strict success envelopes and correlate each write receipt', async () => {
  const api = createCareerApi(async () => ({ success: false, data: profile }));
  await assert.rejects(api.getProfile(scope), /success/i);
  await assert.rejects(api.getApplication(scope, 'app-1'), /success/i);
  const missingReceipt = createCareerApi(async () => envelope(profile));
  await assert.rejects(missingReceipt.updateProfile(scope, { requestId: 'r1', expectedRevision: 1, facts: [] }), /requestId/i);
  const mismatch = createCareerApi(async () => envelope(profile, 'other'));
  await assert.rejects(mismatch.updateProfile(scope, { requestId: 'r1', expectedRevision: 1, facts: [] }), /requestId/i);
  await assert.rejects(createCareerApi(async () => ({ ...envelope(profile), debug: true })).getProfile(scope), /unknown|debug/i);
});

test('every Career endpoint rejects a failed success envelope', async () => {
  const api = createCareerApi(async () => ({ success: false, data: [] }));
  const checks = [
    api.getProfile(scope), api.updateProfile(scope, { requestId: 'p1', expectedRevision: 1, facts: [] }),
    api.search(scope, { requestId: 's1', expectedRevision: 1, query: 'backend' }), api.listOpportunities(scope), api.getEvaluation(scope, 'o1'),
    api.createApplication(scope, { requestId: 'a1', expectedRevision: 1, opportunityId: 'o1' }), api.getApplication(scope, 'a1'),
    api.listApplications(scope), api.listMaterials(scope, 'a1'), api.createMaterial(scope, 'a1', { requestId: 'm1', expectedRevision: 1, body: {} }),
    api.recordSubmission(scope, 'a1', { requestId: 's2', expectedRevision: 1, channel: 'manual', versionUnknown: true }),
    api.listTimeline(scope, 'a1'), api.appendTimeline(scope, 'a1', { requestId: 'e1', expectedRevision: 1, kind: 'interview' }),
    api.listReminders(scope), api.createReminder(scope, { requestId: 'r1', expectedRevision: 1, kind: 'interview', dueAt: '2026-09-29T10:00:00Z' }),
    api.requestExport(scope, { requestId: 'x1' }), api.requestDelete(scope, { requestId: 'd1', expectedRevision: 1 }),
    api.lookupRequest(scope, 'lookup-1'), api.open(scope), api.list(scope),
  ];
  const results = await Promise.allSettled(checks);
  assert.ok(results.every(result => result.status === 'rejected'));
});

test('CareerApi exposes all planned endpoints at canonical versioned paths and propagates local scope', async () => {
  const calls: any[] = [];
  const api = createCareerApi(async request => {
    calls.push(request);
    const path = request.path;
    let data: unknown = [];
    if (path === '/api/v1/career/profile') data = profile;
    else if (path.includes('/evaluation')) data = { status: 'unknown', revision: 1, profileRevision: 1, opportunityRevision: 1, modelVersion: 'eval-v1', evidence: [] };
    else if (path === '/api/v1/career/applications' && request.method === 'POST') data = { ...app, id: 'app-created' };
    else if (path.endsWith('/applications/app1') || path === '/api/v1/career/applications') data = path.endsWith('/applications/app1') ? app : [];
    else if (path.endsWith('/materials') && request.method === 'POST') data = material;
    else if (path.endsWith('/submission')) data = { id: 'sub1', channel: 'manual', confirmedAt: '2026-09-28T10:00:00Z', materialVersion: null, versionUnknown: true };
    else if (path.endsWith('/timeline') && request.method === 'POST') data = { id: 'event1', revision: 1, kind: 'interview', occurredAt: '2026-09-28T10:00:00Z' };
    else if (path === '/api/v1/career/reminders' && request.method === 'POST') data = { id: 'rem1', revision: 1, kind: 'interview', dueAt: '2026-09-29T10:00:00Z', status: 'active' };
    else if (path.endsWith('/privacy/export')) data = { requestId: request.requestId, status: 'pending', revision: 1 };
    else if (path.endsWith('/privacy/delete')) data = { requestId: request.requestId, status: 'accepted', revision: 1 };
    else if (path.endsWith('/search')) data = { requestId: request.requestId, revision: 1, status: 'completed', opportunityIds: [] };
    else if (path.includes('/requests/')) data = { kind: 'unknown', requestId: 'r1' };
    return envelope(data, request.requestId);
  });
  await api.getProfile(scope);
  await api.search(scope, { requestId: 's1', expectedRevision: 1, query: 'backend' });
  await api.listOpportunities(scope);
  await api.getEvaluation(scope, 'opp 1');
  await api.createApplication(scope, { requestId: 'a1', expectedRevision: 1, opportunityId: 'o1' });
  await api.getApplication(scope, 'app1');
  await api.listApplications(scope);
  await api.listMaterials(scope, 'app1');
  await api.createMaterial(scope, 'app1', { requestId: 'm1', expectedRevision: 1, body: {} });
  await api.recordSubmission(scope, 'app1', { requestId: 'sub1', expectedRevision: 1, channel: 'manual', versionUnknown: true });
  await api.listTimeline(scope, 'app1');
  await api.appendTimeline(scope, 'app1', { requestId: 'e1', expectedRevision: 1, kind: 'interview' });
  await api.listReminders(scope);
  await api.createReminder(scope, { requestId: 'rem1', expectedRevision: 1, kind: 'interview', dueAt: '2026-09-29T10:00:00Z' });
  await api.requestExport(scope, { requestId: 'x1' });
  await api.requestDelete(scope, { requestId: 'd1', expectedRevision: 1 });
  await api.lookupRequest(scope, 'r1');
  assert.deepEqual(calls.map(call => [call.method, call.path]), [
    ['GET', '/api/v1/career/profile'], ['POST', '/api/v1/career/search'], ['GET', '/api/v1/career/opportunities'],
    ['GET', '/api/v1/career/opportunities/opp%201/evaluation'], ['POST', '/api/v1/career/applications'],
    ['GET', '/api/v1/career/applications/app1'], ['GET', '/api/v1/career/applications'], ['GET', '/api/v1/career/applications/app1/materials'],
    ['POST', '/api/v1/career/applications/app1/materials'], ['POST', '/api/v1/career/applications/app1/submission'],
    ['GET', '/api/v1/career/applications/app1/timeline'], ['POST', '/api/v1/career/applications/app1/timeline'],
    ['GET', '/api/v1/career/reminders'], ['POST', '/api/v1/career/reminders'], ['POST', '/api/v1/career/privacy/export'],
    ['POST', '/api/v1/career/privacy/delete'], ['GET', '/api/v1/career/requests/r1'],
  ]);
  assert.ok(calls.every(call => JSON.stringify(call.scope) === JSON.stringify(scope)));
});

test('invalid write IDs, revisions, and authority bodies fail before the requester is called', async () => {
  let calls = 0;
  const api = createCareerApi(async () => { calls++; return envelope(profile, 'r1'); });
  await assert.rejects(api.updateProfile(scope, { requestId: '', expectedRevision: 1, facts: [] }));
  await assert.rejects(api.updateProfile(scope, { requestId: 'r1', expectedRevision: Number.NaN, facts: [] }));
  await assert.rejects(api.updateProfile(scope, { requestId: 'r1', expectedRevision: 1, facts: [], tenantId: 'tenant-x' } as any));
  assert.equal(calls, 0);
});

test('API-owned Desk transport satisfies the CareerRemote contract without a second adapter', () => {
  const api = createCareerApi(async () => envelope({ revision: 1, value: { profile, opportunities: [], applications: [] } }));
  const remote: import('@weknora/contracts').CareerRemote<import('@weknora/contracts').CareerWorkspace, import('@weknora/contracts').CareerCommand> = api;
  assert.equal(typeof remote.lookup, 'function');
});
