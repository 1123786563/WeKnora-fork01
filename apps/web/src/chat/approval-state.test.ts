import assert from 'node:assert/strict';
import test from 'node:test';

import { applyOAuthApprovalCancellation, applyOAuthApprovalResolution, applyToolApprovalResolution, extractApprovalTiming, withApprovalTiming } from './approval-state.ts';

// The SSE tool_approval_required payload carries the Vue countdown inputs
// (requested_at in unix seconds, timeout_seconds); the domain reducer drops
// them, so the web layer re-extracts them from the raw event.
test('extractApprovalTiming pulls requested_at and timeout_seconds from approval events', () => {
  assert.deepEqual(extractApprovalTiming({
    response_type: 'tool_approval_required',
    data: { pending_id: 'approval-1', requested_at: 1_700_000_000, timeout_seconds: 600 },
  }), { requestedAt: 1_700_000_000, timeoutSeconds: 600 });
  // Non-numeric or missing fields stay optional; non-approval events yield null.
  assert.deepEqual(extractApprovalTiming({
    response_type: 'tool_approval_required',
    data: { pending_id: 'approval-1', requested_at: 'NaN-ish', timeout_seconds: null },
  }), {});
  assert.equal(extractApprovalTiming({ response_type: 'answer', data: { requested_at: 5 } }), null);
  assert.equal(extractApprovalTiming({ data: { requested_at: 5 } }), null);
});

test('withApprovalTiming merges countdown fields onto approval prompts by pendingId', () => {
  const timing = new Map([
    ['approval-1', { requestedAt: 1_700_000_000, timeoutSeconds: 600 }],
    ['approval-2', { requestedAt: 1_700_000_500 }],
  ]);
  assert.deepEqual(withApprovalTiming([
    { pendingId: 'approval-1', status: 'pending' },
    { pendingId: 'approval-2', status: 'pending' },
    { pendingId: 'approval-3', status: 'pending' },
  ], timing), [
    { pendingId: 'approval-1', status: 'pending', requestedAt: 1_700_000_000, timeoutSeconds: 600 },
    { pendingId: 'approval-2', status: 'pending', requestedAt: 1_700_000_500 },
    { pendingId: 'approval-3', status: 'pending' },
  ]);
});

test('resolves a live tool approval immediately after the approval API succeeds', () => {
  assert.deepEqual(applyToolApprovalResolution({
    'approval-1': { pendingId: 'approval-1', status: 'pending', toolCallId: 'tool-1' },
    'approval-2': { pendingId: 'approval-2', status: 'pending' },
  }, 'approval-1', 'approve'), {
    'approval-1': { pendingId: 'approval-1', status: 'resolved', decision: 'approve', toolCallId: 'tool-1' },
    'approval-2': { pendingId: 'approval-2', status: 'pending' },
  });
});

test('resolves an OAuth approval immediately after authorization completes', () => {
  assert.deepEqual(applyOAuthApprovalResolution({
    'oauth-1': { pendingId: 'oauth-1', serviceId: 'mcp-1', status: 'pending' },
  }, 'oauth-1', true), {
    'oauth-1': { pendingId: 'oauth-1', serviceId: 'mcp-1', status: 'resolved', authorized: true },
  });
});

test('resolves an OAuth approval immediately after cancellation completes', () => {
  assert.deepEqual(applyOAuthApprovalCancellation({
    'oauth-1': { pendingId: 'oauth-1', serviceId: 'mcp-1', status: 'pending' },
  }, 'oauth-1', 'Authorization cancelled'), {
    'oauth-1': { pendingId: 'oauth-1', serviceId: 'mcp-1', status: 'resolved', authorized: false, reason: 'Authorization cancelled' },
  });
});
