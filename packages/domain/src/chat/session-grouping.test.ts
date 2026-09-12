import assert from 'node:assert/strict'
import test from 'node:test'

import {
  DEFAULT_SESSION_GROUP_MODE,
  SESSION_GROUP_MODE_STORAGE_KEY,
  readStoredGroupMode,
  resolveSessionOrigin,
  sessionSourceBadge,
  storeGroupMode,
} from './session-grouping.ts'

function fakeStorage(initial: Record<string, string> = {}): Storage {
  const data = new Map(Object.entries(initial))
  return {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => void data.set(key, value),
    removeItem: (key: string) => void data.delete(key),
    clear: () => data.clear(),
    key: (index: number) => [...data.keys()][index] ?? null,
    get length() { return data.size },
  } as Storage
}

test('readStoredGroupMode accepts persisted none/date values', () => {
  assert.equal(readStoredGroupMode(fakeStorage({ [SESSION_GROUP_MODE_STORAGE_KEY]: 'date' })), 'date')
  assert.equal(readStoredGroupMode(fakeStorage({ [SESSION_GROUP_MODE_STORAGE_KEY]: 'none' })), 'none')
})

test('readStoredGroupMode falls back to the default for unknown or legacy values', () => {
  assert.equal(readStoredGroupMode(fakeStorage({ [SESSION_GROUP_MODE_STORAGE_KEY]: 'source' })), DEFAULT_SESSION_GROUP_MODE)
  assert.equal(readStoredGroupMode(fakeStorage({ [SESSION_GROUP_MODE_STORAGE_KEY]: 'gibberish' })), DEFAULT_SESSION_GROUP_MODE)
  assert.equal(readStoredGroupMode(fakeStorage()), DEFAULT_SESSION_GROUP_MODE)
  assert.equal(readStoredGroupMode(null), DEFAULT_SESSION_GROUP_MODE)
})

test('storeGroupMode round-trips through storage and no-ops without one', () => {
  const storage = fakeStorage()
  storeGroupMode('date', storage)
  assert.equal(storage.getItem(SESSION_GROUP_MODE_STORAGE_KEY), 'date')
  assert.equal(readStoredGroupMode(storage), 'date')
  assert.doesNotThrow(() => storeGroupMode('none', null))
})

test('resolveSessionOrigin classifies im, embed, api and web sessions', () => {
  assert.deepEqual(resolveSessionOrigin({ id: '1', im_platform: 'Feishu' }), { kind: 'im', platform: 'feishu' })
  assert.deepEqual(
    resolveSessionOrigin({ id: '2', description: 'embed_channel:chan-9' }),
    { kind: 'embed', channelId: 'chan-9' },
  )
  assert.deepEqual(resolveSessionOrigin({ id: '3', user_id: 'api_tenant_key:abc' }), { kind: 'api' })
  assert.deepEqual(resolveSessionOrigin({ id: '4', user_id: 'api_external_user:xyz' }), { kind: 'api' })
  assert.deepEqual(resolveSessionOrigin({ id: '5' }), { kind: 'web' })
  // A bare embed marker without a channel id stays a web session.
  assert.deepEqual(resolveSessionOrigin({ id: '6', description: 'embed_channel:' }), { kind: 'web' })
})

test('sessionSourceBadge renders Web / IM / Embed / API labels', () => {
  assert.deepEqual(sessionSourceBadge({ id: '1' }), { kind: '', label: 'Web' })
  assert.deepEqual(sessionSourceBadge({ id: '2', im_platform: 'slack' }), { kind: 'is-im', label: 'SLACK' })
  assert.deepEqual(sessionSourceBadge({ id: '3', description: 'embed_channel:c1' }), { kind: 'is-embed', label: 'Embed' })
  assert.deepEqual(sessionSourceBadge({ id: '4', user_id: 'api_tenant_key:k' }), { kind: 'is-api', label: 'API' })
})
