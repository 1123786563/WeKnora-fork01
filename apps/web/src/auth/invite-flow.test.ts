import { test } from 'node:test';
import assert from 'node:assert/strict';
import { storePendingInviteToken, readPendingInviteToken, clearPendingInviteToken, landingModeForInvite, PENDING_INVITE_KEY } from './invite-flow.ts';

function memoryStorage() {
  const map = new Map<string, string>();
  return { getItem: (k: string) => map.get(k) ?? null, setItem: (k: string, v: string) => void map.set(k, v), removeItem: (k: string) => void map.delete(k) };
}

test('pending invite token survives storage round-trip and clears (Vue Login.vue:641-643)', () => {
  const storage = memoryStorage();
  storePendingInviteToken(storage, 'tok-123');
  assert.equal(storage.getItem(PENDING_INVITE_KEY), 'tok-123');
  assert.equal(readPendingInviteToken(storage), 'tok-123');
  clearPendingInviteToken(storage);
  assert.equal(readPendingInviteToken(storage), '');
});

test('empty token is never stored', () => {
  const storage = memoryStorage();
  storePendingInviteToken(storage, '');
  assert.equal(storage.getItem(PENDING_INVITE_KEY), null);
});

test('invite_only stays on login; open modes go to register (Vue Login.vue:803-808)', () => {
  assert.equal(landingModeForInvite('invite_only'), 'login');
  assert.equal(landingModeForInvite('self_serve'), 'register');
});
