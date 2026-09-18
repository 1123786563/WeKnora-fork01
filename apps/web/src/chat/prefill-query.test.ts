import assert from 'node:assert/strict';
import test from 'node:test';

import { clearPrefillParamsFromUrl, readPrefillKbIds, readPrefillQuery, stripPrefillParamsFromHref } from './prefill-query.ts';

/*
 * Vue menuStore.prefillQuery contract (frontend/src/stores/menu.ts): the
 * "问 AI" entries (useStartChat → setPrefillQuery + router.push creatChat)
 * seed exactly one composer query. The React SPA carries it as the ?q= deep
 * link (R465 palette), so reading/striping must behave one-shot and must
 * never mutate the history stack (replaceState only).
 *
 * R467-A2 — Vue startChat(query, kbIds, fileIds) additionally preselects the
 * new chat's knowledge bases (settingsStore.selectKnowledgeBases). React
 * carries that scope as the ?kbIds= deep link stacked onto ?q= (palette
 * ask-AI in scoped state); both prefill params share the one-shot lifecycle
 * and are stripped together via replaceState.
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
    stripPrefillParamsFromHref('http://weknora.test/platform/creatChat?q=hi&agentId=agent-1#frag'),
    '/platform/creatChat?agentId=agent-1#frag',
  );
});

test('stripPrefillQueryFromHref drops the whole search when q was alone', () => {
  assert.equal(
    stripPrefillParamsFromHref('http://weknora.test/platform/creatChat?q=hi'),
    '/platform/creatChat',
  );
});

test('stripPrefillQueryFromHref is a no-op without a q parameter', () => {
  assert.equal(stripPrefillParamsFromHref('http://weknora.test/platform/creatChat?agentId=a'), null);
  assert.equal(stripPrefillParamsFromHref('http://weknora.test/platform/creatChat'), null);
});

test('clearPrefillParamsFromUrl replaces the history entry without pushing', () => {
  const calls: Array<{ url: string }> = [];
  const history = { replaceState: (_data: unknown, _unused: string, url?: string) => { calls.push({ url: String(url) }); } };
  const next = clearPrefillParamsFromUrl(history, 'http://weknora.test/platform/creatChat?q=hi');
  assert.equal(next, '/platform/creatChat');
  assert.deepEqual(calls, [{ url: '/platform/creatChat' }]);
});

test('clearPrefillParamsFromUrl leaves the history alone when nothing is prefilled', () => {
  const calls: unknown[] = [];
  const history = { replaceState: (...args: unknown[]) => { calls.push(args); } };
  assert.equal(clearPrefillParamsFromUrl(history, 'http://weknora.test/platform/creatChat'), null);
  assert.equal(calls.length, 0);
});

// ─── R467-A2 ?kbIds= knowledge-base scope preselect ───

test('readPrefillKbIds parses comma-separated and repeated values, trimmed and deduped', () => {
  assert.deepEqual(readPrefillKbIds('?kbIds=kb-1,kb-2'), ['kb-1', 'kb-2']);
  assert.deepEqual(readPrefillKbIds('?kbIds=kb-1&kbIds=kb-2'), ['kb-1', 'kb-2']);
  assert.deepEqual(readPrefillKbIds('?kbIds=%20kb-1%20%2Ckb-2%2Ckb-1'), ['kb-1', 'kb-2']);
});

test('readPrefillKbIds is empty without usable values', () => {
  assert.deepEqual(readPrefillKbIds(''), []);
  assert.deepEqual(readPrefillKbIds('?q=hi'), []);
  assert.deepEqual(readPrefillKbIds('?kbIds='), []);
  assert.deepEqual(readPrefillKbIds('?kbIds=,,'), []);
});

test('stripPrefillParamsFromHref removes both q and kbIds in one pass, keeping siblings', () => {
  assert.equal(
    stripPrefillParamsFromHref('http://weknora.test/platform/creatChat?q=hi&kbIds=kb-1,kb-2&agentId=a#frag'),
    '/platform/creatChat?agentId=a#frag',
  );
  assert.equal(
    stripPrefillParamsFromHref('http://weknora.test/platform/creatChat?kbIds=kb-1'),
    '/platform/creatChat',
  );
});

test('clearPrefillParamsFromUrl strips a kbIds-only deep link via replaceState', () => {
  const calls: Array<{ url: string }> = [];
  const history = { replaceState: (_data: unknown, _unused: string, url?: string) => { calls.push({ url: String(url) }); } };
  const next = clearPrefillParamsFromUrl(history, 'http://weknora.test/platform/creatChat?kbIds=kb-1');
  assert.equal(next, '/platform/creatChat');
  assert.deepEqual(calls, [{ url: '/platform/creatChat' }]);
});
