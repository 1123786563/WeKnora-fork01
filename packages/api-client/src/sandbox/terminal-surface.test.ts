import test from 'node:test';
import assert from 'node:assert/strict';
import { createSandboxTerminalApi } from './terminal.ts';

// T25 (#55) AC2: terminal API exposes ticket issuance only; it has no input
// sending or other write channel.
test('the terminal api surface is ticket-issue only — no input or write channel exists (T25 #55 AC2)', () => {
  const api = createSandboxTerminalApi(async () => {
    throw new Error('no request may leave this probe');
  });
  assert.deepEqual(Object.keys(api), ['issueTicket']);
});
