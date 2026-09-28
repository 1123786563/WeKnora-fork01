import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { parseCareerProfile, parseCareerOpportunity, parseCareerEvaluation, parseCareerMaterialVersion } from './index.ts';

test('career DTO parsers keep profile facts, immutable opportunity snapshots and tri-state evaluation strict', () => {
  assert.deepEqual(parseCareerProfile({ id: 'p1', revision: 2, facts: [{ id: 'f1', value: 'BSc', confirmed: true, provenance: { kind: 'user' }, confirmation: { confirmedBy: 'user', at: '2026-09-28T00:00:00Z' } }] }), {
    id: 'p1', revision: 2, facts: [{ id: 'f1', value: 'BSc', confirmed: true, provenance: { kind: 'user' }, confirmation: { confirmedBy: 'user', at: '2026-09-28T00:00:00Z' } }],
  });
  assert.equal(parseCareerOpportunity({ id: 'o1', snapshot: { revision: 1, digest: 'sha256:' + 'a'.repeat(64), observedAt: '2026-09-28T00:00:00Z', sourceUrl: 'https://jobs.example/a', content: 'JD', completeness: 'complete' } }).snapshot.revision, 1);
  assert.throws(() => parseCareerEvaluation({ status: 'maybe' }), /status/);
  assert.equal(parseCareerEvaluation({ status: 'unknown', revision: 1, profileRevision: 1, opportunityRevision: 1, modelVersion: 'eval-v1', evidence: [] }).status, 'unknown');
  const fixture = JSON.parse(readFileSync(new URL('../../test/career/material-version.json', import.meta.url), 'utf8'));
  assert.equal(parseCareerMaterialVersion(fixture).version, 1);
  assert.throws(() => parseCareerMaterialVersion({ id: 'm1', version: 0, digest: 'bad', bodyDigest: 'bad', pdfDigest: 'bad', docxDigest: 'bad', publishedAt: 'now' }), /digest|version/);
  assert.throws(() => parseCareerOpportunity({ id: 'o1', snapshot: { revision: 1, digest: 'bad', observedAt: '2026-09-28T00:00:00Z', sourceUrl: 'https://jobs.example', content: 'JD', completeness: 'complete' } }), /digest/);
});
