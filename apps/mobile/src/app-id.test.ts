import assert from 'node:assert/strict';
import test from 'node:test';
import { isValidWeKnoraAppId, resolveWeKnoraAppId, OFFICIAL_WEKNORA_APP_ID } from './app-id.ts';

test('valid app ids are official or enterprise slugs', () => {
  assert.equal(isValidWeKnoraAppId('official'), true);
  assert.equal(isValidWeKnoraAppId('enterprise:acme'), true);
  assert.equal(isValidWeKnoraAppId('enterprise:a1-b2'), true);
  for (const invalid of ['', 'Official', 'enterprise', 'enterprise:', 'enterprise:Acme', 'enterprise:a_b', `enterprise:${'x'.repeat(33)}`, 'ios']) {
    assert.equal(isValidWeKnoraAppId(invalid), false, JSON.stringify(invalid));
  }
});

test('resolveWeKnoraAppId falls back to official when unset or malformed', () => {
  assert.equal(resolveWeKnoraAppId(undefined), OFFICIAL_WEKNORA_APP_ID);
  assert.equal(resolveWeKnoraAppId('  '), OFFICIAL_WEKNORA_APP_ID);
  assert.equal(resolveWeKnoraAppId('not-an-app'), OFFICIAL_WEKNORA_APP_ID, 'a misconfigured build fails closed to the official lane');
  assert.equal(resolveWeKnoraAppId(' enterprise:acme '), 'enterprise:acme');
});
