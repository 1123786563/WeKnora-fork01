import assert from 'node:assert/strict';
import test from 'node:test';

import { clearPrefillQueryFromUrl, readPrefillQuery, stripPrefillQueryFromHref } from './prefill-query.ts';

/*
 * Vue menuStore.prefillQuery contract (frontend/src/stores/menu.ts): the
 * "问 AI" entries (useStartChat → setPrefillQuery + router.push creatChat)
 * seed exactly one composer query. The React SPA carries it as the ?q= deep
 * link (R465 palette), so reading/striping must behave one-shot and must
 * never mutate the history stack (replaceState only).
 */

test('readPrefillQuery returns the trimmed q parameter', () => {
  assert.equal(readPrefillQuery('?q=hello%20world'), 'hello world');
  assert.equal(readPrefillQuery('?q=+%20spaced%20'), 'spaced');
});

test('readPrefillQuery is empty without a q parameter', () => {
  assert.equal(readPrefillQuery(''), '');
  assert.equal(readPrefillQuery('?agentId=agent-1'), '');
  assert.equal(readPrefillQuery('?q='), '');
});

test('stripPrefillQueryFromHref keeps sibling parameters and the hash', () => {
  assert.equal(
    stripPrefillQueryFromHref('http://weknora.test/platform/creatChat?q=hi&agentId=agent-1#frag'),
    '/platform/creatChat?agentId=agent-1#frag',
  );
});

test('stripPrefillQueryFromHref drops the whole search when q was alone', () => {
  assert.equal(
    stripPrefillQueryFromHref('http://weknora.test/platform/creatChat?q=hi'),
    '/platform/creatChat',
  );
});

test('stripPrefillQueryFromHref is a no-op without a q parameter', () => {
  assert.equal(stripPrefillQueryFromHref('http://weknora.test/platform/creatChat?agentId=a'), null);
  assert.equal(stripPrefillQueryFromHref('http://weknora.test/platform/creatChat'), null);
});

test('clearPrefillQueryFromUrl replaces the history entry without pushing', () => {
  const calls: Array<{ url: string }> = [];
  const history = { replaceState: (_data: unknown, _unused: string, url?: string) => { calls.push({ url: String(url) }); } };
  const next = clearPrefillQueryFromUrl(history, 'http://weknora.test/platform/creatChat?q=hi');
  assert.equal(next, '/platform/creatChat');
  assert.deepEqual(calls, [{ url: '/platform/creatChat' }]);
});

test('clearPrefillQueryFromUrl leaves the history alone when nothing is prefilled', () => {
  const calls: unknown[] = [];
  const history = { replaceState: (...args: unknown[]) => { calls.push(args); } };
  assert.equal(clearPrefillQueryFromUrl(history, 'http://weknora.test/platform/creatChat'), null);
  assert.equal(calls.length, 0);
});
