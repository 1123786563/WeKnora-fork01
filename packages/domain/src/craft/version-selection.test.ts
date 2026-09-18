// CFT-S01-T011: version/source state independence as PURE rules. The four
// acceptance assertions:
//   1. viewing v1 never touches base=v3 / workspace revision
//   2. the NEXT round's source selection never rewrites v1's recorded
//      citations (manifests are immutable once published)
//   3. a revoked source renders a placeholder — the cached excerpt never
//      shows through
//   4. downloadable is NOT restorable: a version without a complete snapshot
//      disables restore while download stays available
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  craftVersionSelection,
  sourceAccessibility,
  restoreEligibility,
  type CraftVersionSelectionState,
} from './version-selection.ts';

const base: CraftVersionSelectionState = {
  viewVersionId: 'v3',
  baseVersionId: 'v3',
  workspaceRevision: 7,
  versions: [
    { id: 'v1', published: true, fileCount: 2, hasSnapshot: true, citations: ['kc_1', 'kc_2'] },
    { id: 'v3', published: true, fileCount: 2, hasSnapshot: true, citations: ['kc_1'] },
  ],
  restorableVersionIds: ['v1'],
};

test('viewing v1 keeps base=v3 and the revision untouched', () => {
  const next = craftVersionSelection(base, { type: 'view', versionId: 'v1' });
  assert.equal(next.viewVersionId, 'v1', 'only the view moves');
  assert.equal(next.baseVersionId, 'v3', 'the editing baseline stays v3');
  assert.equal(next.workspaceRevision, 7, 'the workspace revision stays 7');
  assert.deepEqual(next.versions, base.versions, 'published versions are immutable');
});

test('the next round source pick never rewrites v1 citations', () => {
  const next = craftVersionSelection(base, { type: 'select-sources', citations: ['kc_9'] });
  assert.deepEqual(next.versions, base.versions, 'manifests never change on a selection');
  const v1 = next.versions.find((v) => v.id === 'v1');
  assert.deepEqual(v1?.citations, ['kc_1', 'kc_2'], 'v1 keeps its recorded citations');
});

test('a revoked source renders a placeholder, never the cached excerpt', () => {
  assert.equal(sourceAccessibility({ citationId: 'kc_1' }, 'granted'), 'open');
  assert.equal(sourceAccessibility({ citationId: 'kc_1' }, 'revoked'), 'revoked');
  // the revoked projection carries NO excerpt payload — only the placeholder id
  const revoked = sourceAccessibility({ citationId: 'kc_2' }, 'revoked');
  assert.notEqual(revoked, 'open');
});

test('downloadable is not restorable: no snapshot disables restore, download stays', () => {
  const v1 = restoreEligibility(base, 'v1', { canWrite: true, hasActiveRun: false });
  assert.equal(v1.restorable, true, 'v1 has a snapshot, write and no active run');
  assert.equal(v1.downloadable, true);

  const partial = {
    ...base,
    restorableVersionIds: [],
    versions: [
      { id: 'v2', published: true, fileCount: 3, hasSnapshot: false, citations: [] as string[] },
    ],
  } as CraftVersionSelectionState;
  const v2 = restoreEligibility(partial, 'v2', { canWrite: true, hasActiveRun: false });
  assert.equal(v2.restorable, false, 'no complete snapshot blocks restore');
  assert.equal(v2.downloadable, true, 'its files remain downloadable');
  assert.ok(v2.blockReason && v2.blockReason.length > 0, 'the block carries a readable reason');

  const busy = restoreEligibility(base, 'v1', { canWrite: true, hasActiveRun: true });
  assert.equal(busy.restorable, false, 'an active run blocks restore');
  const ro = restoreEligibility(base, 'v1', { canWrite: false, hasActiveRun: false });
  assert.equal(ro.restorable, false, 'read-only blocks restore');
  assert.equal(ro.downloadable, true, 'read-only still downloads');
});

test('a successful restore moves the baseline and bumps the revision once', () => {
  const next = craftVersionSelection(base, { type: 'restore', versionId: 'v1' });
  assert.equal(next.baseVersionId, 'v1');
  assert.equal(next.workspaceRevision, 8, 'revision advances exactly once');
  assert.equal(next.viewVersionId, 'v1');
  assert.deepEqual(next.versions, base.versions, 'restore publishes NOTHING — versions unchanged');
});
