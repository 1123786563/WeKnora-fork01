import assert from 'node:assert/strict';
import test from 'node:test';
import { resolveMobileApiBaseUrl } from './transport.ts';

test('requires an explicit HTTP(S) mobile server address', () => {
  assert.equal(resolveMobileApiBaseUrl('https://weknora.example.test/'), 'https://weknora.example.test');
  assert.equal(resolveMobileApiBaseUrl('http://10.0.2.2:8080/api/v1'), 'http://10.0.2.2:8080/api/v1');
  assert.equal(resolveMobileApiBaseUrl('http://localhost:8080'), 'http://localhost:8080');
  assert.equal(resolveMobileApiBaseUrl('file:///tmp/weknora'), '');
  assert.equal(resolveMobileApiBaseUrl(''), '');
});
