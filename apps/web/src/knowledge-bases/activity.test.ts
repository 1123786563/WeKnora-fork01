import assert from 'node:assert/strict';
import { test } from 'node:test';
import { activityActionTone, activityDateTime, activityOutcomeTone, activityTargetSummary } from './activity.ts';

test('activity date time follows the Vue split date and clock presentation', () => {
  const result = activityDateTime('2026-09-15T08:04:00.000Z', 'en-US');
  assert.match(result.date, /2026/);
  assert.match(result.time, /:/);
});

test('activity tones classify Vue action and outcome states', () => {
  assert.equal(activityActionTone('knowledge.created'), 'success');
  assert.equal(activityActionTone('knowledge.parse_canceled'), 'warning');
  assert.equal(activityActionTone('knowledge.move_failed'), 'error');
  assert.equal(activityActionTone('datasource.sync_started'), 'primary');
  assert.equal(activityActionTone('kb.updated'), 'primary');
  assert.equal(activityActionTone('kb.share_permission_changed'), 'primary');
  assert.equal(activityOutcomeTone('partial'), 'warning');
  assert.equal(activityOutcomeTone('denied'), 'error');
  assert.equal(activityOutcomeTone('accepted'), 'primary');
});

test('activity target summary exposes subject and aggregate change without raw ids', () => {
  const result = activityTargetSummary({
    id: 1,
    action: 'knowledge.batch_deleted',
    outcome: 'partial',
    created_at: '2026-09-15T08:04:00.000Z',
    target_name: 'Docs',
    details: { count: 4, failed: 1, skipped: 1, target_id: 'secret-id' },
  } as never);
  assert.deepEqual(result, { subject: 'Docs', change: '4 (1 failed, 1 skipped)' });
  assert.equal(result.change.includes('secret-id'), false);
});
