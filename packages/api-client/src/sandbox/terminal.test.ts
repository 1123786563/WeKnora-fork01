import assert from 'node:assert/strict';
import test from 'node:test';
import { ContractError } from '@weknora/contracts';
import { createSandboxTerminalApi } from './terminal.ts';

test('mints a ticket through the authenticated POST and validates its expiry', async () => {
  let seen: unknown;
  const api = createSandboxTerminalApi(async (request) => {
    seen = request;
    return { success: true, data: { ticket: 'short-lived', expires_in: 60 } };
  });
  assert.deepEqual(await api.issueTicket('session/1'), { ticket: 'short-lived', expiresIn: 60 });
  assert.deepEqual(seen, { method: 'POST', path: '/api/v1/sessions/session%2F1/sandbox/terminal-ticket', body: {} });
});

test('rejects missing or malformed tickets instead of opening a WS', async () => {
  const api = createSandboxTerminalApi(async () => ({ success: true, data: { ticket: '', expires_in: 0 } }));
  await assert.rejects(() => api.issueTicket('s-1'), /terminal ticket/);
  await assert.rejects(() => api.issueTicket(''), /sessionId/);
});

test('rejects every malformed ticket field with a contract error', async () => {
  const responses = [
    null,
    { success: false, data: { ticket: 'ticket', expires_in: 120 } },
    { success: true, data: null },
    { success: true, data: { ticket: ' ', expires_in: 120 } },
    { success: true, data: { ticket: 'ticket', expires_in: '120' } },
    { success: true, data: { ticket: 'ticket', expires_in: 0 } },
    { success: true, data: { ticket: 'ticket', expires_in: 1.5 } },
  ];

  for (const response of responses) {
    const api = createSandboxTerminalApi(async () => response);
    await assert.rejects(() => api.issueTicket('session-1'), ContractError);
  }
});
