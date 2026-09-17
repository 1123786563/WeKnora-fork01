/**
 * W32 — advanced interaction capability ports (tests).
 *
 * node:test file: run with `pnpm exec tsx --test
 * apps/mobile/sources/weknora/conversations/advanced.test.ts`.
 */
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  invokeAdvanced,
  deriveRemoteAdvancedCapabilities,
  deriveProductAdvancedCapabilities,
  createSandboxTerminalOperation,
  REMOTE_ADVANCED_DRIVER,
  type AdvancedCapabilities,
  type AdvancedOperation,
} from './advanced.ts';

test('unsupported advanced operation cannot fall back to a shell command', async () => {
  let calls = 0;
  await assert.rejects(invokeAdvanced('fork', { fork: false }, async () => { calls++; }), /CAPABILITY_UNAVAILABLE/);
  assert.equal(calls, 0);
});

test('a capability that is merely absent also refuses instead of guessing', async () => {
  let calls = 0;
  // Product conversations have no rewind endpoint: the absent flag must be a
  // refusal, never an optimistic attempt against some other driver.
  await assert.rejects(invokeAdvanced('rewind', {}, async () => { calls++; }), /CAPABILITY_UNAVAILABLE/);
  assert.equal(calls, 0);
});

test('supported operation is delivered to its fixed operation handler only', async () => {
  const seen: AdvancedOperation[] = [];
  await invokeAdvanced('archive', { archive: true }, async (op) => { seen.push(op); });
  assert.deepEqual(seen, ['archive']);
});

test('the gate checks the capability before awaiting anything the handler defers', async () => {
  let started = false;
  const slowHandler = async () => { started = true; await new Promise((resolve) => setTimeout(resolve, 10)); };
  await assert.rejects(
    invokeAdvanced('terminal', { terminal: false }, slowHandler),
    /CAPABILITY_UNAVAILABLE/,
  );
  assert.equal(started, false, 'the handler must not start while the capability is unproven');
});

test('remote capabilities map session facts and archive never maps to a cancel driver', () => {
  const capabilities: AdvancedCapabilities = deriveRemoteAdvancedCapabilities({
    goalActions: true,
    forkSource: true,
    sessionPresent: true,
    rewindSupported: true,
    duplicateSupported: true,
  });
  assert.deepEqual(capabilities, {
    goal: true,
    fork: true,
    side_chat: true,
    archive: true,
    rewind: true,
    duplicate: true,
  });
  // The remote terminal is a separate machine provider (H33: the product
  // sandbox terminal and the remote machine terminal must never mix paths or
  // tickets), so a remote session alone must not unlock the product
  // 'terminal' operation — the capability stays absent (refuses at the gate).
  assert.equal('terminal' in capabilities, false);
  assert.equal(REMOTE_ADVANCED_DRIVER.archive.driver, 'sync.sessionArchive');
  assert.notEqual(REMOTE_ADVANCED_DRIVER.archive.driver, 'sync.sessionAbort');
  assert.equal(REMOTE_ADVANCED_DRIVER.archive.cancelCommand, false);
});

test('remote capabilities follow the facts when the fork source is missing', () => {
  const capabilities: AdvancedCapabilities = deriveRemoteAdvancedCapabilities({
    goalActions: true,
    forkSource: false,
    sessionPresent: true,
    rewindSupported: true,
    duplicateSupported: false,
  });
  assert.equal(capabilities.fork, false);
  assert.equal(capabilities.side_chat, false);
  assert.equal(capabilities.duplicate, false);
  assert.equal(capabilities.archive, true, 'archiving a present session needs no fork source');
});

test('product conversations expose only the verified sandbox terminal entry', () => {
  const capabilities: AdvancedCapabilities = deriveProductAdvancedCapabilities({ authenticated: true, sessionId: 'sess-1' });
  assert.deepEqual(capabilities, { terminal: true });
  // No product endpoints exist for the six Happy-origin operations (verified
  // against packages/api-client chat/sessions + executions): the product side
  // must refuse them instead of routing the Happy remote ops as a fallback.
  for (const op of ['goal', 'fork', 'side_chat', 'archive', 'rewind', 'duplicate'] as AdvancedOperation[]) {
    assert.equal(op in capabilities, false, `product ${op} must stay CAPABILITY_UNAVAILABLE`);
  }
  const unauthenticated = deriveProductAdvancedCapabilities({ authenticated: false, sessionId: 'sess-1' });
  assert.deepEqual(unauthenticated, {});
});

test('rewind maps to point-listing plus fork-from-point, never a side-effect rollback', () => {
  const rewind = REMOTE_ADVANCED_DRIVER.rewind;
  assert.equal(rewind.driver, 'sync.listRewindPoints+forkAndSpawn');
  assert.equal(rewind.claimsExternalRollback, false);
  // duplicate is the same non-destructive shape: a new session forked from a
  // selected item, not a mutation of the original history.
  assert.equal(REMOTE_ADVANCED_DRIVER.duplicate.claimsExternalRollback, false);
});

test('terminal port reuses the sandbox terminal-ticket entry with a scoped ticket', async () => {
  const requests: Array<{ method: string; url: string; authorization: string }> = [];
  const fetcher = (async (url: string | RequestInfo | URL, init?: RequestInit) => {
    requests.push({
      method: init?.method ?? 'GET',
      url: String(url),
      authorization: new Headers(init?.headers).get('authorization') ?? '',
    });
    return {
      ok: true,
      status: 200,
      headers: { get: () => 'application/json' },
      json: async () => ({ success: true, data: { ticket: 'ticket-once', expires_in: 30 } }),
    } as unknown as Response;
  }) as typeof fetch;
  const targets: Array<{ webSocketUrl: string; ticketExpiresIn: number }> = [];
  const operation = createSandboxTerminalOperation({
    origin: 'https://api.example',
    sessionId: 'sess/1',
    credential: { kind: 'bearer', accessToken: 'bearer-long-lived' },
    fetcher,
    onTarget: (target) => targets.push(target),
  });
  await invokeAdvanced('terminal', { terminal: true }, operation);
  // Exactly one request: the verified product ticket entry. The long-lived
  // bearer rides only this authenticated HTTP call.
  assert.equal(requests.length, 1);
  assert.equal(requests[0].method, 'POST');
  assert.equal(requests[0].url, 'https://api.example/api/v1/sessions/sess%2F1/sandbox/terminal-ticket');
  assert.equal(requests[0].authorization, 'Bearer bearer-long-lived');
  // The produced target reference carries the one-time short-term ticket in
  // the scoped WS url — the bearer itself never travels past the ticket call
  // and no generic shell RPC is exposed.
  assert.deepEqual(targets, [{ webSocketUrl: 'wss://api.example/api/v1/sessions/sess%2F1/sandbox/terminal?ticket=ticket-once', ticketExpiresIn: 30 }]);
  assert.equal(targets[0].webSocketUrl.includes('bearer-long-lived'), false);
});

test('a terminal operation bound to another operation refuses to run', async () => {
  let issued = 0;
  const operation = createSandboxTerminalOperation({
    origin: 'https://api.example',
    sessionId: 'sess-1',
    credential: { kind: 'bearer', accessToken: 'b' },
    fetcher: (async () => {
      issued += 1;
      return { ok: true, status: 200, headers: { get: () => 'application/json' }, json: async () => ({ success: true, data: { ticket: 't', expires_in: 30 } }) } as unknown as Response;
    }) as typeof fetch,
  });
  // Even with the capability proven, a terminal port handed the wrong
  // operation must not issue a ticket it cannot scope.
  await assert.rejects(invokeAdvanced('fork', { fork: true, terminal: true }, operation), /OPERATION_MISMATCH/);
  assert.equal(issued, 0);
});
