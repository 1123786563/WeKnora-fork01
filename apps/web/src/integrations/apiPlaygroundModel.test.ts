import assert from 'node:assert/strict';
import test from 'node:test';

import {
  DEFAULT_DIRECT_HEADER_NAME,
  DEFAULT_TOKEN_HEADER_NAME,
  agentOptionLabel,
  buildChatRequestBody,
  buildPlaygroundHeaders,
  compactText,
  ensurePlaygroundAgent,
  externalUserHintKey,
  formatResponseBody,
  hasPlaygroundResult,
  interpretSessionResponse,
  playgroundDisabledReason,
  playgroundRequestPreview,
  settlePlaygroundStatuses,
} from './apiPlaygroundModel.ts';

// Pure helpers ported from the Vue baseline ApiIntegrationSettings.vue
// (buildPlaygroundHeaders L1583-1599, playgroundRequestPreview L1022-1037,
// playgroundDisabledReason L1039-1048, compactText/formatJSON/readResponseBody
// L1601-1622, runPlayground session handling L1661-1677, ensurePlaygroundAgent
// L1216-1220, settle transitions from the catch block L1716-1726).

test('buildPlaygroundHeaders sends the api key and derives the SSE chat accept header', () => {
  const headers = buildPlaygroundHeaders({
    apiKey: 'wk-live-key', mode: 'tenant', externalUserId: '', signedToken: '', maskSecrets: false,
  });
  assert.deepEqual(headers.sessionHeaders, {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    'X-API-Key': 'wk-live-key',
  });
  assert.deepEqual(headers.chatHeaders, { ...headers.sessionHeaders, Accept: 'text/event-stream' });
});

test('buildPlaygroundHeaders masks secrets for the preview', () => {
  const headers = buildPlaygroundHeaders({
    apiKey: 'wk-live-key', mode: 'signed_token', externalUserId: 'user_123', signedToken: 'jwt-secret', maskSecrets: true,
  });
  assert.equal(headers.sessionHeaders['X-API-Key'], '<API_KEY>');
  assert.equal(headers.sessionHeaders[DEFAULT_TOKEN_HEADER_NAME], '<JWT>');
  assert.equal(headers.chatHeaders.Accept, 'text/event-stream');
});

test('buildPlaygroundHeaders carries the direct header only with a trimmed external user', () => {
  const withUser = buildPlaygroundHeaders({
    apiKey: 'k', mode: 'direct_header', externalUserId: '  user_9  ', signedToken: '', maskSecrets: false,
  });
  assert.equal(withUser.sessionHeaders[DEFAULT_DIRECT_HEADER_NAME], 'user_9');
  const withoutUser = buildPlaygroundHeaders({
    apiKey: 'k', mode: 'direct_header', externalUserId: '   ', signedToken: '', maskSecrets: false,
  });
  assert.equal(DEFAULT_DIRECT_HEADER_NAME in withoutUser.sessionHeaders, false);
});

test('playgroundRequestPreview mirrors the Vue two-step request preview with masked secrets', () => {
  const preview = playgroundRequestPreview({
    query: '', agentId: '', mode: 'tenant', externalUserId: '', signedToken: '', apiKey: 'wk-live-key',
  });
  assert.match(preview, /^POST \/api\/v1\/sessions\n/);
  assert.match(preview, /"X-API-Key": "<API_KEY>"/);
  assert.match(preview, /"body": \{\}/);
  assert.match(preview, /POST \/api\/v1\/agent-chat\/\<session_id\>/);
  assert.match(preview, /"Accept": "text\/event-stream"/);
  assert.match(preview, /"query": "<query>"/);
  assert.match(preview, /"agent_id": "<agent_id>"/);
  assert.match(preview, /"channel": "api"/);
  assert.match(preview, /"agent_enabled": true/);
  assert.ok(!preview.includes('wk-live-key'));
});

test('playgroundDisabledReason follows the Vue gating order', () => {
  const base = { running: false, apiKey: 'k', agentId: 'a', query: 'hi', mode: 'tenant' as const, externalUserId: '' };
  assert.equal(playgroundDisabledReason(base), '');
  assert.equal(playgroundDisabledReason({ ...base, running: true }), '');
  assert.equal(playgroundDisabledReason({ ...base, apiKey: '' }), 'integrations.api.playgroundNeedApiKey');
  assert.equal(playgroundDisabledReason({ ...base, agentId: '' }), 'integrations.api.playgroundNeedAgent');
  assert.equal(playgroundDisabledReason({ ...base, query: '   ' }), 'integrations.api.playgroundNeedQuestion');
  assert.equal(
    playgroundDisabledReason({ ...base, mode: 'signed_token', externalUserId: '' }),
    'integrations.api.playgroundNeedExternalUser',
  );
  assert.equal(playgroundDisabledReason({ ...base, mode: 'signed_token', externalUserId: 'u' }), '');
});

test('interpretSessionResponse extracts the session id and maps failures like Vue', () => {
  assert.deepEqual(
    interpretSessionResponse({ ok: true, status: 200, payload: { success: true, data: { id: 'sess-1' } } }),
    { ok: true, sessionId: 'sess-1' },
  );
  assert.deepEqual(
    interpretSessionResponse({ ok: true, status: 200, payload: { success: true, data: { ID: 'sess-2' } } }),
    { ok: true, sessionId: 'sess-2' },
  );
  assert.deepEqual(
    interpretSessionResponse({ ok: false, status: 401, payload: null }),
    { ok: false, error: 'HTTP 401' },
  );
  assert.deepEqual(
    interpretSessionResponse({ ok: true, status: 200, payload: { success: false, message: 'nope' } }),
    { ok: false, error: 'nope' },
  );
  assert.deepEqual(
    interpretSessionResponse({ ok: false, status: 500, payload: { error: { message: 'boom' } } }),
    { ok: false, error: 'boom' },
  );
  assert.deepEqual(
    interpretSessionResponse({ ok: true, status: 200, payload: { success: true, data: {} } }),
    { ok: false, error: 'integrations.api.playgroundMissingSessionId' },
  );
});

test('compactText, formatResponseBody and hasPlaygroundResult mirror the Vue helpers', () => {
  assert.equal(compactText('abcdef', 4), 'abcd\n...');
  assert.equal(compactText('abc', 4), 'abc');
  assert.equal(formatResponseBody(''), '');
  assert.equal(formatResponseBody('{"a":1}'), '{\n  "a": 1\n}');
  assert.equal(formatResponseBody('plain text'), 'plain text');
  assert.equal(hasPlaygroundResult({ signedToken: '', sessionResponse: '', streamOutput: '', finalAnswer: '' }), false);
  assert.equal(hasPlaygroundResult({ signedToken: 'jwt', sessionResponse: '', streamOutput: '', finalAnswer: '' }), true);
  assert.equal(hasPlaygroundResult({ signedToken: '', sessionResponse: '{}', streamOutput: '', finalAnswer: '' }), true);
});

test('ensurePlaygroundAgent prefers the current id, then the builtin, then the first agent', () => {
  const agents = [{ id: 'a1', name: 'A' }, { id: 'builtin-smart-reasoning', name: '深度推理' }];
  assert.equal(ensurePlaygroundAgent('a1', agents), 'a1');
  assert.equal(ensurePlaygroundAgent('', agents), 'builtin-smart-reasoning');
  assert.equal(ensurePlaygroundAgent('gone', agents), 'builtin-smart-reasoning');
  assert.equal(ensurePlaygroundAgent('', [{ id: 'a1', name: 'A' }]), 'a1');
  assert.equal(ensurePlaygroundAgent('', []), '');
});

test('agentOptionLabel appends the builtin suffix like the Vue select options', () => {
  assert.equal(agentOptionLabel('助手', true, '内置'), '助手 · 内置');
  assert.equal(agentOptionLabel('助手', false, '内置'), '助手');
});

test('buildChatRequestBody matches the Vue agent-chat payload', () => {
  assert.deepEqual(buildChatRequestBody({ query: '  hello  ', agentId: 'a1' }), {
    query: 'hello', agent_enabled: true, agent_id: 'a1', channel: 'api',
  });
});

test('externalUserHintKey maps each principal mode to its Vue hint', () => {
  assert.equal(externalUserHintKey('tenant'), 'integrations.api.playgroundTenantModeHint');
  assert.equal(externalUserHintKey('direct_header'), 'integrations.api.playgroundDirectModeHint');
  assert.equal(externalUserHintKey('signed_token'), 'integrations.api.playgroundSignedModeHint');
});

test('settlePlaygroundStatuses moves running steps to stopped or failed like the Vue catch block', () => {
  assert.deepEqual(
    settlePlaygroundStatuses({ sessionStatus: 'running', chatStatus: 'running' }, true),
    { sessionStatus: 'stopped', chatStatus: 'stopped' },
  );
  // Vue L1723: a failure never marks a running step as done — the session lands
  // on 'failed' too, not 'success'.
  assert.deepEqual(
    settlePlaygroundStatuses({ sessionStatus: 'running', chatStatus: 'running' }, false),
    { sessionStatus: 'failed', chatStatus: 'failed' },
  );
  assert.deepEqual(
    settlePlaygroundStatuses({ sessionStatus: 'running', chatStatus: '' }, true),
    { sessionStatus: 'stopped', chatStatus: '' },
  );
  assert.deepEqual(
    settlePlaygroundStatuses({ sessionStatus: 'success', chatStatus: 'running' }, false),
    { sessionStatus: 'success', chatStatus: 'failed' },
  );
  assert.deepEqual(
    settlePlaygroundStatuses({ sessionStatus: 'success', chatStatus: 'success' }, false),
    { sessionStatus: 'success', chatStatus: 'success' },
  );
});
