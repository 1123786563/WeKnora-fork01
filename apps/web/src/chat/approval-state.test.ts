import assert from 'node:assert/strict';
import test from 'node:test';

import { applyOAuthApprovalCancellation, applyOAuthApprovalResolution, applyToolApprovalResolution } from './approval-state.ts';

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
