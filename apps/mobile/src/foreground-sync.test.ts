import assert from 'node:assert/strict';
import test from 'node:test';
import { createForegroundSyncLoop } from './foreground-sync.ts';

function scriptedLifecycle() {
  const listeners = new Set<(state: 'active' | 'background') => void>();
  return {
    lifecycle: {
      subscribe(listener: (state: 'active' | 'background') => void): () => void {
        listeners.add(listener);
        return () => listeners.delete(listener);
      },
    },
    emit(state: 'active' | 'background') { for (const listener of [...listeners]) listener(state); },
  };
}

test('returning to the foreground triggers one authoritative sync', async () => {
  const script = scriptedLifecycle();
  const syncs: number[] = [];
  let gate: (() => void) | undefined;
  const loop = createForegroundSyncLoop({
    lifecycle: script.lifecycle,
    resolveSync: () => async () => { syncs.push(syncs.length); gate?.(); },
  });
  const stop = loop.start();
  script.emit('background');
  assert.equal(syncs.length, 0, 'background events never sync');
  script.emit('active');
  await new Promise<void>((resolve) => { gate = resolve; });
  assert.equal(syncs.length, 1);
  stop();
});

test('concurrent active events coalesce into one follow-up, never parallel syncs', async () => {
  const script = scriptedLifecycle();
  let inflight = 0;
  let maxInflight = 0;
  let release: (() => void) | undefined;
  const loop = createForegroundSyncLoop({
    lifecycle: script.lifecycle,
    resolveSync: () => async () => {
      inflight += 1;
      maxInflight = Math.max(maxInflight, inflight);
      await new Promise<void>((resolve) => { release = resolve; });
      inflight -= 1;
    },
  });
  loop.start();
  script.emit('active');
  script.emit('active');
  script.emit('active');
  await new Promise<void>((resolve) => setTimeout(resolve, 0));
  assert.equal(maxInflight, 1, 'syncs are serialized');
  release?.();
  await new Promise<void>((resolve) => setTimeout(resolve, 10));
  assert.ok(maxInflight <= 1, 'follow-up completed without parallel sync or unhandled rejection');
});

test('an unauthorized scope resolves undefined and never syncs', async () => {
  const script = scriptedLifecycle();
  let called = 0;
  const loop = createForegroundSyncLoop({
    lifecycle: script.lifecycle,
    resolveSync: () => { called += 1; return undefined; },
  });
  loop.start();
  script.emit('active');
  await new Promise<void>((resolve) => setTimeout(resolve, 0));
  assert.equal(called, 1, 'resolveSync is consulted per event');
});

test('sync failures are contained and the next foreground retries', async () => {
  const script = scriptedLifecycle();
  let attempts = 0;
  const loop = createForegroundSyncLoop({
    lifecycle: script.lifecycle,
    resolveSync: () => () => { attempts += 1; return attempts === 1 ? Promise.reject(new Error('offline')) : Promise.resolve(); },
  });
  loop.start();
  script.emit('active');
  await new Promise<void>((resolve) => setTimeout(resolve, 0));
  script.emit('active');
  await new Promise<void>((resolve) => setTimeout(resolve, 0));
  assert.equal(attempts, 2, 'the failed sync did not poison the loop');
});

test('stop unsubscribes the lifecycle listener', async () => {
  const script = scriptedLifecycle();
  let called = 0;
  const loop = createForegroundSyncLoop({ lifecycle: script.lifecycle, resolveSync: () => { called += 1; return () => Promise.resolve(); } });
  const stop = loop.start();
  stop();
  script.emit('active');
  await new Promise<void>((resolve) => setTimeout(resolve, 0));
  assert.equal(called, 0);
});
