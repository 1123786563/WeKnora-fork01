import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import {
  parseCareerProfile, parseCareerOpportunity, parseCareerSearchRequest, parseCareerSearchReceipt,
  parseCareerEvaluation, parseCareerApplication, parseCareerMaterialVersion, parseCareerSubmission,
  parseCareerTimelineEvent, parseCareerReminder, parseCareerExportReceipt, parseCareerDeleteReceipt,
} from './index.ts';

const digest = `sha256:${'a'.repeat(64)}`;
const profileFact = { id: 'fact-1', value: 'BSc', confirmed: true, provenance: { kind: 'user', reference: 'interview-1' }, confirmation: { confirmedBy: 'user', at: '2026-09-28T10:00:00Z' } };
const snapshot = { revision: 1, digest, observedAt: '2026-09-28T10:00:00Z', sourceUrl: 'https://jobs.example/1', content: 'JD', completeness: 'complete' };
const material = { id: 'material-1', version: 1, digest, bodyDigest: digest, pdfDigest: digest, docxDigest: digest, publishedAt: '2026-09-28T10:00:00Z' };

test('every public Career DTO family decodes a valid fixture', () => {
  const fixture = JSON.parse(readFileSync(new URL('../../test/career/wire-variants.json', import.meta.url), 'utf8'));
  assert.equal(parseCareerProfile(fixture.profile).facts.length, 1);
  assert.equal(parseCareerOpportunity(fixture.opportunity).snapshot.completeness, 'complete');
  assert.equal(parseCareerSearchRequest(fixture.searchRequest).query, 'backend engineer');
  assert.equal(parseCareerSearchReceipt(fixture.searchReceipt).status, 'completed');
  assert.equal(parseCareerEvaluation(fixture.evaluation).status, 'unknown');
  assert.equal(parseCareerApplication(fixture.application).id, 'app-1');
  assert.equal(parseCareerMaterialVersion(fixture.materialVersion).version, 1);
  assert.equal(parseCareerSubmission(fixture.submission).versionUnknown, true);
  assert.equal(parseCareerTimelineEvent(fixture.timelineEvent).revision, 1);
  assert.equal(parseCareerReminder(fixture.reminder).status, 'active');
  assert.equal(parseCareerExportReceipt(fixture.exportReceipt).status, 'ready');
  assert.equal(parseCareerDeleteReceipt(fixture.deleteReceipt).status, 'completed');
});

test('strict parsers reject invalid IDs, revisions, unknown enums and authority at every nested boundary', () => {
  assert.throws(() => parseCareerProfile({ id: 'p', revision: 1, facts: [{ ...profileFact, provenance: { kind: 'user', tenantId: 'tenant-b' } }] }), /tenant/i);
  assert.throws(() => parseCareerProfile({ id: 'p', revision: 1, facts: [{ ...profileFact, value: { actor_id: 'u2' } }] }), /actor/i);
  assert.throws(() => parseCareerOpportunity({ id: '', snapshot }), /id/);
  assert.throws(() => parseCareerOpportunity({ id: 'opp-1', snapshot: { ...snapshot, revision: 0 } }), /revision/);
  assert.throws(() => parseCareerApplication({ id: 'app-1', revision: 1, opportunitySnapshot: { opportunityId: '', revision: -1, digest, tenantId: 'tenant-b' }, stage: 'preparing' }), /opportunitySnapshot/i);
  assert.throws(() => parseCareerSubmission({ id: 's', channel: 'manual', confirmedAt: 'now', materialVersion: { id: 'm', version: 0, digest, owner_id: 'u2' }, versionUnknown: false }), /materialVersion/i);
  assert.throws(() => parseCareerReminder({ id: 'r', revision: 1, kind: 'interview', dueAt: 'tomorrow', status: 'active', actorId: 'u2' }), /actor/i);
  assert.throws(() => parseCareerEvaluation({ status: 'eligible', revision: 1, profileRevision: 1, opportunityRevision: 1, modelVersion: 'm1', evidence: [{ claim: 'graduation', result: 'supports', source: 'confirmed_profile', tenant_id: 'tenant-b' }] }), /tenant/i);
  assert.throws(() => parseCareerExportReceipt({ requestId: 'r', status: 'ready', revision: 1, artifactId: 'a', downloadUrl: 'https://elsewhere' }), /downloadUrl|unknown/i);
  assert.throws(() => parseCareerDeleteReceipt({ requestId: '', status: 'completed', revision: 1 }), /requestId/i);
});
