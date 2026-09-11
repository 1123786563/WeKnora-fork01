import assert from 'node:assert/strict';
import test from 'node:test';

import { isSafeSkillFilePath, installProgress, normalizeSandboxConfigIds, redactDebugValue, shouldPollInstalledSkill } from './management.ts';

test('normalizes one or more manually entered sandbox configuration IDs', () => {
  assert.deepEqual(normalizeSandboxConfigIds(' cfg-a, cfg-b\ncfg-a\n\tcfg-c '), ['cfg-a', 'cfg-b', 'cfg-c']);
});

test('rejects empty and traversal skill file paths', () => {
  assert.equal(isSafeSkillFilePath('README.md'), true);
  assert.equal(isSafeSkillFilePath('docs/setup.md'), true);
  assert.equal(isSafeSkillFilePath(''), false);
  assert.equal(isSafeSkillFilePath('/etc/passwd'), false);
  assert.equal(isSafeSkillFilePath('../secrets.env'), false);
  assert.equal(isSafeSkillFilePath('docs/../secrets.env'), false);
  assert.equal(isSafeSkillFilePath('docs\\secrets.env'), false);
});

test('polls only while the server reports an in-progress installed-skill status', () => {
  assert.equal(shouldPollInstalledSkill('installing'), true);
  assert.equal(shouldPollInstalledSkill('removing'), true);
  assert.equal(shouldPollInstalledSkill('ready'), false);
  assert.equal(shouldPollInstalledSkill('failed'), false);
});

test('summarizes install progress from strict server statuses', () => {
  assert.deepEqual(installProgress(['ready', 'installing', 'failed']), { total: 3, complete: 2, active: 1, failed: 1 });
});

test('redacts secret-shaped debug observation fields before rendering', () => {
  assert.deepEqual(redactDebugValue({ provider: 'openai', api_key: 'secret', nested: { token: 'secret', latency_ms: 12 } }), {
    provider: 'openai', api_key: '[redacted]', nested: { token: '[redacted]', latency_ms: 12 },
  });
});
