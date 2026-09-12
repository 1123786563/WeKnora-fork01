import assert from 'node:assert/strict'
import test from 'node:test'

import {
  DEFAULT_SESSION_GROUP_MODE,
  SESSION_GROUP_MODE_STORAGE_KEY,
  readStoredGroupMode,
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
