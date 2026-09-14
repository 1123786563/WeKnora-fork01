import assert from 'node:assert/strict';
import test from 'node:test';
import {
  RECENTS_CAP,
  addKbFavorite,
  createPinsGeneration,
  fetchKbFavoriteIds,
  kbRecentsStorageKey,
  parseKbRecents,
  readKbRecents,
  removeKbFavorite,
  removeKbRecent,
  sortRecentsDesc,
  touchKbRecent,
  type KeyValueStorage,
} from './pins.ts';

function memoryStorage(): KeyValueStorage & { dump(): Record<string, string> } {
  const store: Record<string, string> = {};
  return {
    getItem: async (key) => store[key] ?? null,
    setItem: async (key, value) => { store[key] = value; },
    dump: () => store,
  };
}

test('recents persist under the user-and-tenant Vue-style key', async () => {
  const storage = memoryStorage();
  await touchKbRecent(storage, 'user-1', '4', 'kb-1');
  assert.deepEqual(Object.keys(storage.dump()), ['WeKnora_user-1_t4_resource_recents']);
  assert.equal(kbRecentsStorageKey('user-1', '4'), 'WeKnora_user-1_t4_resource_recents');
  assert.equal(kbRecentsStorageKey(null, null), 'WeKnora_anon_resource_recents');
});

test('recents survive a storage round-trip across restarts', async () => {
  const storage = memoryStorage();
  await touchKbRecent(storage, 'user-1', '4', 'kb-1', 100);
  await touchKbRecent(storage, 'user-1', '4', 'kb-2', 200);
  const reloaded = await readKbRecents(storage, 'user-1', '4');
  assert.deepEqual(reloaded.map((entry) => entry.id), ['kb-2', 'kb-1']);
  // A different tenant never sees the same recents.
  assert.deepEqual(await readKbRecents(storage, 'user-1', '9'), []);
  assert.deepEqual(await readKbRecents(storage, 'user-2', '4'), []);
});

test('touching a recent moves it to the front and refreshes its timestamp', async () => {
  const storage = memoryStorage();
  await touchKbRecent(storage, 'user-1', null, 'kb-1', 100);
  await touchKbRecent(storage, 'user-1', null, 'kb-2', 200);
  await touchKbRecent(storage, 'user-1', null, 'kb-1', 300);
  const recents = await readKbRecents(storage, 'user-1', null);
  assert.deepEqual(recents.map((entry) => entry.id), ['kb-1', 'kb-2']);
  assert.equal(recents[0].ts, 300);
});

test('recents are capped at the Vue RECENTS_CAP of 30', async () => {
  const storage = memoryStorage();
  for (let index = 0; index < RECENTS_CAP + 5; index += 1) await touchKbRecent(storage, 'user-1', null, `kb-${index}`, index);
  const recents = await readKbRecents(storage, 'user-1', null);
  assert.equal(recents.length, RECENTS_CAP);
  assert.equal(recents[0].id, `kb-${RECENTS_CAP + 4}`);
});

test('removeKbRecent only drops the matching kb entry', async () => {
  const storage = memoryStorage();
  await touchKbRecent(storage, 'user-1', null, 'kb-1', 100);
  await storage.setItem('WeKnora_user-1_resource_recents', JSON.stringify([{ type: 'kb', id: 'kb-1', ts: 100 }, { type: 'agent', id: 'agent-1', ts: 50 }]));
  const next = await removeKbRecent(storage, 'user-1', null, 'kb-1');
  assert.deepEqual(next.map((entry) => entry.id), ['agent-1']);
});

test('parseKbRecents drops malformed rows like the Vue validator', () => {
  const raw = JSON.stringify([
    { type: 'kb', id: 'kb-1', ts: 1 },
    { type: 'chat', id: 'nope', ts: 2 },
    { type: 'kb', id: 7, ts: 3 },
    { type: 'agent', id: 'agent-1' },
    null,
  ]);
  assert.deepEqual(parseKbRecents(raw).map((entry) => entry.id), ['kb-1']);
  assert.deepEqual(parseKbRecents(null), []);
  assert.deepEqual(parseKbRecents('not json'), []);
  assert.deepEqual(parseKbRecents('[1,2]'), []);
});

test('sortRecentsDesc orders newest first without mutating the input', () => {
  const entries = [{ type: 'kb' as const, id: 'a', ts: 1 }, { type: 'kb' as const, id: 'b', ts: 3 }];
  assert.deepEqual(sortRecentsDesc(entries).map((entry) => entry.id), ['b', 'a']);
  assert.equal(entries[0].id, 'a');
});

test('pins generation invalidates an older async hydration result', () => {
  const generations = createPinsGeneration();
  const first = generations.next();
  const second = generations.next();
  assert.equal(generations.isCurrent(first), false);
  assert.equal(generations.isCurrent(second), true);
});

test('favorites hydrate from the server favorites endpoint', async () => {
  const calls: Array<{ method: string; path: string; body?: unknown }> = [];
  const client = {
    request: async (input: { method: string; path: string; body?: unknown }) => {
      calls.push(input);
      return { success: true, data: [
        { resource_type: 'kb', resource_id: 'kb-1', created_at: '2026-01-01T00:00:00Z' },
        { resource_type: 'agent', resource_id: 'agent-1', created_at: '2026-01-01T00:00:00Z' },
        { resource_type: 'kb', resource_id: 'kb-2', created_at: '2026-01-02T00:00:00Z' },
      ] };
    },
  };
  assert.deepEqual(await fetchKbFavoriteIds(client), new Set(['kb-1', 'kb-2']));
  assert.deepEqual(calls, [{ method: 'GET', path: '/api/v1/user/favorites?type=kb' }]);
  assert.deepEqual(await fetchKbFavoriteIds({ request: async () => ({ success: true }) }), new Set());
  assert.deepEqual(await fetchKbFavoriteIds({ request: async () => null }), new Set());
});

test('favorite toggles hit the Vue add/remove endpoints', async () => {
  const calls: Array<{ method: string; path: string; body?: unknown }> = [];
  const client = {
    request: async (input: { method: string; path: string; body?: unknown }) => { calls.push(input); return { success: true }; },
  };
  await addKbFavorite(client, 'kb-9');
  await removeKbFavorite(client, 'kb-9');
  assert.deepEqual(calls, [
    { method: 'POST', path: '/api/v1/user/favorites', body: { type: 'kb', id: 'kb-9' } },
    { method: 'DELETE', path: '/api/v1/user/favorites/kb/kb-9' },
  ]);
});
