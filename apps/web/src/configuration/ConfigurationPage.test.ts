import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const page = readFileSync(new URL('./ConfigurationPage.tsx', import.meta.url), 'utf8');
const usageNotice = readFileSync(new URL('./ModelUsageNotice.tsx', import.meta.url), 'utf8');

test('describes missing sandbox skill availability without claiming the catalog is unavailable', () => {
  assert.match(page, /sandbox-installed skills/i);
  assert.match(page, /catalog remains available/i);
  assert.doesNotMatch(page, /Skill catalog is unavailable/i);
});

test('renders sandbox availability as a notice instead of replacing catalog results', () => {
  assert.match(page, /available === false \? <Status tone="warning">[\s\S]*?<\/Status> : null}\s*\{errors\[section\.key\]/);
});

test('renders structured model deletion occupancy instead of hiding it in a generic error', () => {
  assert.match(page, /modelInUseDetails\(cause\)/);
  assert.match(usageNotice, /Knowledge bases \(/);
  assert.match(usageNotice, /Long-term memory/);
});
