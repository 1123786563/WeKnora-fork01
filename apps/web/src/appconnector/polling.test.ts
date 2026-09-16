import test from 'node:test';
import assert from 'node:assert/strict';
import { nextAuthorizationPoll, shouldContinueAuthorizationPolling } from './polling.ts';

test('polls only non-terminal authorization states and stops at expiry', () => {
  assert.equal(shouldContinueAuthorizationPolling('pending', '2099-01-01T00:00:00Z', Date.parse('2028-01-01T00:00:00Z')), true);
  assert.equal(shouldContinueAuthorizationPolling('authorizing', undefined, Date.now()), true);
  assert.equal(shouldContinueAuthorizationPolling('active', '2099-01-01T00:00:00Z', Date.now()), false);
  assert.equal(shouldContinueAuthorizationPolling('pending', '2020-01-01T00:00:00Z', Date.now()), false);
});

test('backs off transient poll errors and caps the delay', () => {
  assert.equal(nextAuthorizationPoll(0), 3000);
  assert.equal(nextAuthorizationPoll(1), 6000);
  assert.equal(nextAuthorizationPoll(20), 30000);
});
