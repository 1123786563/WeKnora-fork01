// T15 (#130): the default preview version as a pure domain rule. The web
// workbench's default seat only ever goes to the newest version whose four
// web checks (build, entry, preview_reachable, page_loaded) each
// independently passed — preview reachability can never substitute for the
// actual page load, and a version without the four-check evidence (a legacy
// row, a failed or unobserved round) stays out of the seat.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { defaultPreviewVersion, webCheckEvidenceReady, type WebVersionEvidenceFact } from './web-promotion.ts';

const all: WebVersionEvidenceFact['webEvidence'] = { build: 'passed', entry: 'passed', preview_reachable: 'passed', page_loaded: 'passed' };

test('webCheckEvidenceReady requires all four facts passed', () => {
  assert.equal(webCheckEvidenceReady(all), true);
  assert.equal(webCheckEvidenceReady(null), false, 'no evidence is never ready');
  assert.equal(webCheckEvidenceReady({ ...all, page_loaded: 'not_run' }), false, 'not-run page load is not ready');
  assert.equal(webCheckEvidenceReady({ ...all, page_loaded: 'failed' }), false, 'failed page load is not ready');
  assert.equal(webCheckEvidenceReady({ ...all, build: 'failed' }), false);
  assert.equal(webCheckEvidenceReady({ ...all, entry: 'not_run' }), false);
  assert.equal(webCheckEvidenceReady({ ...all, preview_reachable: 'failed' }), false);
});

test('the newest four-check ready version takes the default seat', () => {
  const newest = { id: 'ver_new', webEvidence: all };
  const older = { id: 'ver_old', webEvidence: all };
  assert.equal(defaultPreviewVersion([newest, older])?.id, 'ver_new');
  assert.equal(defaultPreviewVersion([older])?.id, 'ver_old');
});

test('reachability never substitutes for the actual page load', () => {
  const reachableOnly = { id: 'ver_reach', webEvidence: { ...all, page_loaded: 'not_run' as const } };
  const priorDefault = { id: 'ver_prior', webEvidence: all };
  // newest-first input: the reachable-but-never-loaded version must lose.
  assert.equal(defaultPreviewVersion([reachableOnly, priorDefault])?.id, 'ver_prior');
  assert.equal(defaultPreviewVersion([reachableOnly]), null);
});

test('versions without four-check evidence never become the default', () => {
  const legacy = { id: 'ver_legacy', webEvidence: null };
  const failedRound = { id: 'ver_failed', webEvidence: { build: 'failed' as const, entry: 'failed' as const, preview_reachable: 'not_run' as const, page_loaded: 'not_run' as const } };
  assert.equal(defaultPreviewVersion([legacy]), null);
  assert.equal(defaultPreviewVersion([failedRound, legacy]), null);
  assert.equal(defaultPreviewVersion([]), null);
});


// Round-4 OCR: the createdAt ordering witness finally has assertions —
// newest wins out of order, single-sided absence falls back to array order,
// and equal timestamps keep the first (array) winner.
import { defaultPreviewVersion } from './web-promotion.ts';
import type { WebVersionEvidenceFact } from './web-promotion.ts';

const ready = (id: string, createdAt?: string): WebVersionEvidenceFact => ({
  id,
  createdAt,
  webEvidence: { build: 'passed', entry: 'passed', preview_reachable: 'passed', page_loaded: 'passed' } as never,
});

test('createdAt witness: newest ready version wins regardless of array order', () => {
  const older = ready('ver-old', '2026-09-01T00:00:00Z');
  const newer = ready('ver-new', '2026-09-02T00:00:00Z');
  assert.equal(defaultPreviewVersion([older, newer])?.id, 'ver-new');
  assert.equal(defaultPreviewVersion([newer, older])?.id, 'ver-new', 'shuffled input still picks the newest');
});

test('createdAt witness: single-sided absence falls back to newest-first array order', () => {
  const withStamp = ready('ver-stamped', '2026-09-01T00:00:00Z');
  const withoutStamp = ready('ver-array-first');
  assert.equal(defaultPreviewVersion([withoutStamp, withStamp])?.id, 'ver-array-first',
    'when timestamps are not comparable the array order remains the ordering contract');
});

test('createdAt witness: equal timestamps keep the first array entry', () => {
  const a = ready('ver-a', '2026-09-01T00:00:00Z');
  const b = ready('ver-b', '2026-09-01T00:00:00Z');
  assert.equal(defaultPreviewVersion([a, b])?.id, 'ver-a');
});
