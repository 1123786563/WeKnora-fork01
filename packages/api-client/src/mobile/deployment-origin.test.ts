import test from 'node:test';
import assert from 'node:assert/strict';
import { requireDeploymentOrigin } from './deployment-origin.ts';

test('requireDeploymentOrigin enforces the strictest merged rule set', () => {
  assert.equal(requireDeploymentOrigin('https://weknora.example.test'), 'https://weknora.example.test');
  for (const bad of ['not-a-url', 'http://weknora.example.test', 'https://user:pw@weknora.example.test',
    'https://weknora.example.test/path', 'https://weknora.example.test?q=1', 'https://weknora.example.test#f', 'https://']) {
    assert.throws(() => requireDeploymentOrigin(bad), undefined, `must reject ${bad}`); // 'https://' 覆盖 hostname 空检查（inbox 版最严）
  }
});
