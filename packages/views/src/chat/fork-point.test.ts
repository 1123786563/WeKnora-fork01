import assert from 'node:assert/strict';
import test from 'node:test';

import { resolveForkAffordance, stashForkLanding, takeForkLanding } from './fork-point.ts';

// Upstream forkPoint.test.ts contract plus the index.vue landing stash.

const MESSAGES = [
  { id: 'u1', role: 'user', is_completed: true },
  { id: 'a1', role: 'assistant', is_completed: true },
  { id: 'u2', role: 'user', is_completed: true },
];

test('resolveForkAffordance allows user and assistant points', () => {
  assert.deepEqual(resolveForkAffordance(MESSAGES, 'u1'), { canFork: true });
  assert.deepEqual(resolveForkAffordance(MESSAGES, 'a1'), { canFork: true });
  assert.deepEqual(resolveForkAffordance(MESSAGES, 'missing'), { canFork: false });
  assert.deepEqual(resolveForkAffordance(MESSAGES, 'u1'), { canFork: true });
});

test('resolveForkAffordance refuses while any assistant turn is streaming', () => {
  const streaming = [...MESSAGES, { id: 'a2', role: 'assistant', is_completed: false }];
  assert.deepEqual(resolveForkAffordance(streaming, 'u1'), { canFork: false });
  assert.deepEqual(resolveForkAffordance(streaming, 'a1'), { canFork: false });
});

test('resolveForkAffordance refuses non-conversational roles', () => {
  const withSystem = [...MESSAGES, { id: 'sys', role: 'system', is_completed: true }];
  assert.deepEqual(resolveForkAffordance(withSystem, 'sys'), { canFork: false });
});

test('fork landing stash survives navigation and is consumed once by its session', () => {
  const store = new Map<string, string>();
  const storage = {
    setItem: (k: string, v: string) => { store.set(k, v); },
    getItem: (k: string) => store.get(k) ?? null,
    removeItem: (k: string) => { store.delete(k); },
  };

  stashForkLanding('fork-1', '原问题', storage);
  assert.equal(takeForkLanding('other-session', storage), null, 'a different session must not consume the landing');
  assert.deepEqual(takeForkLanding('fork-1', storage), { sessionId: 'fork-1', text: '原问题' });
  assert.equal(takeForkLanding('fork-1', storage), null, 'consumed exactly once');
  assert.equal(store.has('weknora:fork-prefill'), false);
});

test('fork landing tolerates corrupted storage', () => {
  const store = new Map<string, string>([['weknora:fork-prefill', '{not-json']]);
  const storage = {
    setItem: () => undefined,
    getItem: (k: string) => store.get(k) ?? null,
    removeItem: (k: string) => { store.delete(k); },
  };
  assert.equal(takeForkLanding('fork-1', storage), null);
});
