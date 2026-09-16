import assert from 'node:assert/strict';
import test from 'node:test';
import {
  canAcceptTerminalFrame,
  initialTerminalTicketState,
  reduceTerminalTicketState,
  terminalStatusFromCode,
  terminalWebSocketUrl,
} from './terminal.ts';

test('builds a WS URL with a short-lived ticket and no bearer token', () => {
  const url = terminalWebSocketUrl('https://example.test/root', 'session/1', 'ticket-value', 'root');
  assert.equal(url, 'wss://example.test/root/api/v1/sessions/session%2F1/sandbox/terminal?ticket=ticket-value');
  assert.equal(url.includes('authorization'), false);
});

test('maps server terminal codes while keeping provisioning explicit', () => {
  assert.equal(terminalStatusFromCode('SANDBOX_NOT_BOUND', false), 'needs_provision');
  assert.equal(terminalStatusFromCode('SANDBOX_NOT_BOUND', true), 'no_sandbox');
  assert.equal(terminalStatusFromCode('AUTH_REVOKED', false), 'unauthorized');
});

test('rejects late frames from an old connection generation', () => {
  const state = { status: 'ready' as const, sessionId: 's', generation: 3 };
  assert.equal(canAcceptTerminalFrame(state, 3), true);
  assert.equal(canAcceptTerminalFrame(state, 2), false);
  assert.equal(canAcceptTerminalFrame({ ...state, status: 'exited' }, 3), false);
});

test('moves a short-lived ticket through request, ready and consumed without retaining the secret', () => {
  let state = reduceTerminalTicketState(initialTerminalTicketState(), { type: 'request' });
  assert.deepEqual(state, { status: 'requesting' });
  state = reduceTerminalTicketState(state, {
    type: 'issued', ticket: 'ticket-1', expiresIn: 120, nowMs: 1_000,
  });
  assert.deepEqual(state, { status: 'ready', ticket: 'ticket-1', expiresAtMs: 121_000 });
  state = reduceTerminalTicketState(state, { type: 'consume', nowMs: 2_000 });
  assert.deepEqual(state, { status: 'consumed' });
  assert.equal('ticket' in state, false);
  assert.deepEqual(reduceTerminalTicketState(state, { type: 'consume', nowMs: 3_000 }), state);
});

test('expires unused tickets and clears invalid ticket payloads', () => {
  let state = reduceTerminalTicketState(initialTerminalTicketState(), {
    type: 'issued', ticket: 'ticket-1', expiresIn: 2, nowMs: 1_000,
  });
  state = reduceTerminalTicketState(state, { type: 'clock', nowMs: 3_000 });
  assert.deepEqual(state, { status: 'expired' });
  assert.deepEqual(
    reduceTerminalTicketState(initialTerminalTicketState(), {
      type: 'issued', ticket: '', expiresIn: 0, nowMs: 1_000,
    }),
    { status: 'error', error: 'invalid terminal ticket' },
  );
});
