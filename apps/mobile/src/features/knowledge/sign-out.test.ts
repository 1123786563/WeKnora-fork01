import assert from 'node:assert/strict';
import test from 'node:test';
import { signOutAndRedirect } from './sign-out.ts';

test('sign out clears the session before returning to login', async () => {
  const events: string[] = [];
  await signOutAndRedirect(async () => { events.push('logout'); }, (path) => { events.push(path); });
  assert.deepEqual(events, ['logout', '/(auth)/login']);
});

test('sign out does not navigate when session cleanup fails', async () => {
  let redirected = false;
  await assert.rejects(
    signOutAndRedirect(async () => { throw new Error('cleanup failed'); }, () => { redirected = true; }),
    /cleanup failed/,
  );
  assert.equal(redirected, false);
});
