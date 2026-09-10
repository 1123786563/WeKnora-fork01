import assert from 'node:assert/strict';
import test from 'node:test';

import { ContractError, parseChatMessageListResponse, parseChatSessionListResponse, parseChatSessionResponse, parseTemporaryAttachmentResponse } from '../src/index.ts';

test('parses the session-list envelope returned by GET /sessions', () => {
  const result = parseChatSessionListResponse({
    success: true,
    data: [{ id: 'session-1', title: 'Release notes', is_pinned: true, updated_at: '2026-09-10T09:00:00Z' }],
    total: 1,
    page: 1,
    page_size: 30,
  });

  assert.deepEqual(result, {
    data: [{ id: 'session-1', title: 'Release notes', is_pinned: true, updated_at: '2026-09-10T09:00:00Z' }],
    total: 1,
    page: 1,
    page_size: 30,
  });
});

test('rejects a session list with a malformed row instead of rendering it as empty', () => {
  assert.throws(
    () => parseChatSessionListResponse({ success: true, data: [{ title: 'Missing id' }], total: 1, page: 1, page_size: 30 }),
    ContractError,
  );
});

test('parses the single-session envelope returned by POST /sessions', () => {
  assert.deepEqual(
    parseChatSessionResponse({ success: true, data: { id: 'session-2', title: '', is_pinned: false } }),
    { id: 'session-2', title: '', is_pinned: false },
  );
});

test('parses message history and rejects an unknown role', () => {
  assert.deepEqual(
    parseChatMessageListResponse({ success: true, data: [{ id: 'message-1', session_id: 'session-1', role: 'assistant', content: 'Hello' }] }),
    [{ id: 'message-1', session_id: 'session-1', role: 'assistant', content: 'Hello' }],
  );
  assert.throws(
    () => parseChatMessageListResponse({ success: true, data: [{ id: 'message-1', session_id: 'session-1', role: 'tool', content: 'x' }] }),
    ContractError,
  );
});

test('parses attachment processing states without exposing server storage fields as required DTOs', () => {
  const attachment = parseTemporaryAttachmentResponse({ success: true, data: {
    id: 'att-1', session_id: 'session-1', file_name: 'notes.md', file_type: 'md', file_size: 12, status: 'ready', resource_ref: '/private/path',
  } });
  assert.equal(attachment.status, 'ready');
  assert.equal(attachment.resource_ref, '/private/path');
  assert.throws(() => parseTemporaryAttachmentResponse({ success: true, data: { id: 'att-1', session_id: 's', file_name: 'x', file_type: 'md', file_size: -1, status: 'ready' } }), ContractError);
});
