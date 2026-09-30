import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  parseCraftInputView,
  parseCraftRunView,
  parseCraftVersionView,
  parseCraftInputRecognition,
  parseCraftWebCheckEvidence,
  parseCraftStopOutcome,
  parseCraftWriterAcquireOutcome,
  parseCraftBudgetPause,
  parseCraftRestrictedContribution,
  parseCraftShareDecision,
  parseCraftExportManifest,
  parseCraftExportDecision,
  parseCraftExportConsentView,
} from './index.ts';

const version = { id: 'v1', workspace_id: 'w1', run_id: 'r1', kind: 'web', files: [], checks: [] };

test('input acceptance is separate from understanding and old responses still parse', () => {
  assert.deepEqual(parseCraftInputRecognition({ accepted: true, understood: false, reason: 'unrecognized_extension' }), { accepted: true, understood: false, reason: 'unrecognized_extension' });
  assert.throws(() => parseCraftInputRecognition({ accepted: false, understood: true, reason: '' }), /understood/);
  assert.equal(parseCraftInputView({ ref: 'r', name: 'x', sha256: 'h', bytes: 1, citation_id: '' }).recognition, null);
  assert.equal(parseCraftInputView({ ref: 'r', name: 'x', sha256: 'h', bytes: 1, citation_id: '', recognition: { accepted: true, understood: false, reason: 'opaque' } }).recognition?.understood, false);
  assert.throws(() => parseCraftInputRecognition({ accepted: true, understood: true, reason: 17 }), /reason/);
  assert.throws(() => parseCraftInputView({ ref: 'r', name: 'x', sha256: 'h', bytes: 1, citation_id: '', recognition: { accepted: true, understood: true, reason: { unexpected: true } } }), /reason/);
});

test('version keeps four independent gates and rejects unknown statuses', () => {
  const evidence = { build: 'passed', entry: 'passed', preview_reachable: 'passed', page_loaded: 'not_run' };
  assert.deepEqual(parseCraftWebCheckEvidence(evidence), evidence);
  assert.equal(parseCraftVersionView(version).web_evidence, null);
  assert.equal(parseCraftVersionView({ ...version, web_evidence: evidence }).web_evidence?.page_loaded, 'not_run');
  assert.throws(() => parseCraftWebCheckEvidence({ ...evidence, page_loaded: 'maybe' }), /page_loaded/);
});

test('stop and writer outcomes preserve unknown explicitly', () => {
  assert.equal(parseCraftStopOutcome({ run_id: 'r1', status: 'unknown' }).status, 'unknown');
  assert.equal(parseCraftWriterAcquireOutcome({ workspace_id: 'w1', status: 'unknown' }).status, 'unknown');
  assert.throws(() => parseCraftStopOutcome({ run_id: 'r1', status: 'canceled' }), /status/);
  assert.throws(() => parseCraftWriterAcquireOutcome({ workspace_id: 'w1', status: 'locked' }), /status/);
});

test('budget and restricted consent records bind immutable identities', () => {
  assert.equal(parseCraftBudgetPause({ run_id: 'r1', reason: 'exhausted', limit: 100, used: 100 }).used, 100);
  assert.throws(() => parseCraftBudgetPause({ run_id: 'r1', reason: 'exhausted', limit: -1, used: 0 }), /limit/);
  assert.equal(parseCraftRestrictedContribution({ version_id: 'v1', evidence_digest: 'sha256:a', restricted: true }).restricted, true);
  assert.equal(parseCraftShareDecision({ version_id: 'v1', evidence_digest: 'sha256:a', owner_id: 'u1', decision: 'approved' }).decision, 'approved');
  assert.throws(() => parseCraftShareDecision({ version_id: 'v1', evidence_digest: 'sha256:a', owner_id: 'u1', decision: 'maybe' }), /decision/);
  assert.equal(parseCraftExportManifest({ version_id: 'v1', manifest_digest: 'sha256:m', files: [{ path: 'index.html', sha256: 'sha256:f', restricted: false, origins: [] }] }).files.length, 1);
  assert.equal(parseCraftExportDecision({ version_id: 'v1', manifest_digest: 'sha256:m', owner_id: 'u1', decision: 'rejected' }).decision, 'rejected');
  assert.throws(() => parseCraftExportDecision({ version_id: 'v1', manifest_digest: '', owner_id: 'u1', decision: 'approved' }), /manifest_digest/);
});

test('export consent view preserves immutable derived-file origins and binds decision digest', () => {
  const raw = {
    manifest: { version_id: 'v1', manifest_digest: 'sha256:manifest', files: [{ path: 'data.csv', sha256: 'sha256:derived', restricted: true, origins: [{ kind: 'knowledge', ref: 'source-1', sha256: 'sha256:source', restricted: true }] }] },
    decision: { version_id: 'v1', manifest_digest: 'sha256:manifest', owner_id: 'owner-1', decision: 'approved' },
  };
  const view = parseCraftExportConsentView(raw);
  assert.deepEqual(JSON.parse(JSON.stringify(view)), raw);
  assert.equal(view.manifest.files[0]?.origins[0]?.ref, 'source-1');
  assert.throws(() => parseCraftExportConsentView({ ...raw, decision: { ...raw.decision, manifest_digest: 'sha256:other' } }), /manifest_digest/);
  assert.throws(() => parseCraftExportManifest({ ...raw.manifest, files: [{ ...raw.manifest.files[0], origins: [{ kind: 'web', ref: 'source-1', sha256: 'sha256:source', restricted: true }] }] }), /kind/);
  assert.throws(() => parseCraftExportManifest({ ...raw.manifest, files: [{ ...raw.manifest.files[0], origins: [{ kind: 'knowledge', ref: '', sha256: 'sha256:source', restricted: true }] }] }), /ref/);
});

test('run budget pause is a typed optional fact on the existing waiting state', () => {
  const base = { run_id: 'r1', session_id: 's1', status: 'waiting_user', wait_reason: 'budget', revision: 1, epoch: 1, seq: 2, pending_id: 'p1' };
  assert.equal(parseCraftRunView(base).budget_pause, null);
  assert.equal(parseCraftRunView({ ...base, budget_pause: { run_id: 'r1', reason: 'exhausted', limit: 100, used: 100 } }).budget_pause?.used, 100);
  assert.throws(() => parseCraftRunView({ ...base, budget_pause: { run_id: 'r1', reason: 'exhausted', limit: -1, used: 100 } }), /limit/);
});
