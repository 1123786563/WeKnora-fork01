import assert from 'node:assert/strict'
import test from 'node:test'

import {
  CONVERSATION_TIMESTAMP_GAP_MS,
  ensureMessageCreatedAt,
  formatConversationTimestampLabel,
  formatMessageTimestamp,
  getConversationTimestampKind,
  getConversationTimestampModel,
  normalizeMessageCreatedAt,
  shouldInsertConversationTimestamp,
  shouldShowConversationTimestamp,
} from './message-timestamps.ts'

const labels = {
  today: 'Today',
  yesterday: 'Yesterday',
  thisYear: (model: { month: number; day: number }) => `${model.month}/${model.day}`,
  otherYear: (model: { year: number; month: number; day: number }) => `${model.year}/${model.month}/${model.day}`,
}

test('normalizeMessageCreatedAt keeps parseable strings and rejects the rest', () => {
  assert.equal(normalizeMessageCreatedAt('2024-03-05T10:20:00Z'), '2024-03-05T10:20:00Z')
  assert.equal(normalizeMessageCreatedAt('  2024-03-05T10:20:00Z '), '2024-03-05T10:20:00Z')
  assert.equal(normalizeMessageCreatedAt('not-a-date'), '')
  assert.equal(normalizeMessageCreatedAt(''), '')
  assert.equal(normalizeMessageCreatedAt(undefined), '')
  assert.equal(normalizeMessageCreatedAt(42), '')
})

test('ensureMessageCreatedAt fills only missing or invalid timestamps', () => {
  const filled = ensureMessageCreatedAt({ id: 'a' }, '2024-03-05T10:20:00Z')
  assert.equal(filled.created_at, '2024-03-05T10:20:00Z')

  const kept = ensureMessageCreatedAt({ id: 'b', created_at: '2024-01-01T00:00:00Z' }, '2024-03-05T10:20:00Z')
  assert.equal(kept.created_at, '2024-01-01T00:00:00Z')

  const replaced = ensureMessageCreatedAt({ id: 'c', created_at: 'bogus' }, '2024-03-05T10:20:00Z')
  assert.equal(replaced.created_at, '2024-03-05T10:20:00Z')
})

test('formatMessageTimestamp renders local YYYY-MM-DD HH:mm', () => {
  const value = '2024-03-05T10:20:00Z'
  const date = new Date(value)
  const pad = (part: number) => String(part).padStart(2, '0')
  const expected = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
  assert.equal(formatMessageTimestamp(value), expected)
})

test('formatMessageTimestamp returns empty for invalid input', () => {
  assert.equal(formatMessageTimestamp('nonsense'), '')
  assert.equal(formatMessageTimestamp(undefined), '')
})

test('getConversationTimestampKind classifies today, yesterday, this year, and older', () => {
  const now = new Date(2024, 2, 5, 12, 0, 0)
  assert.equal(getConversationTimestampKind(new Date(2024, 2, 5, 8, 0, 0), now), 'today')
  assert.equal(getConversationTimestampKind(new Date(2024, 2, 4, 23, 0, 0), now), 'yesterday')
  assert.equal(getConversationTimestampKind(new Date(2024, 0, 1), now), 'thisYear')
  assert.equal(getConversationTimestampKind(new Date(2023, 11, 31), now), 'otherYear')
})

test('getConversationTimestampModel returns null for unusable values', () => {
  assert.equal(getConversationTimestampModel('nope'), null)
  const model = getConversationTimestampModel(new Date(2024, 2, 5, 9, 7, 0).toISOString(), new Date(2024, 2, 5, 12, 0, 0))
  assert.equal(model?.kind, 'today')
  assert.equal(model?.time, '09:07')
})

test('shouldInsertConversationTimestamp follows the day-change and idle-gap rules', () => {
  const early = { role: 'user', created_at: new Date(2024, 2, 5, 8, 0, 0).toISOString() }
  const later = { role: 'assistant', created_at: new Date(2024, 2, 5, 8, 10, 0).toISOString() }
  const nextDay = { role: 'user', created_at: new Date(2024, 2, 6, 8, 0, 0).toISOString() }

  assert.equal(shouldInsertConversationTimestamp(undefined, early), true)
  assert.equal(shouldInsertConversationTimestamp(early, { role: 'user', created_at: new Date(2024, 2, 5, 8, 10, 0).toISOString() }), true, 'gap above 5 minutes inserts')
  assert.equal(shouldInsertConversationTimestamp(early, later), false, 'assistant reply right after user does not insert')
  assert.equal(shouldInsertConversationTimestamp(early, { role: 'user', created_at: new Date(2024, 2, 5, 8, 2, 0).toISOString() }), false, 'small gap does not insert')
  assert.equal(shouldInsertConversationTimestamp(later, nextDay), true, 'day change inserts')
  assert.equal(shouldInsertConversationTimestamp(early, { role: 'user', created_at: 'nope' }), false)
  assert.equal(CONVERSATION_TIMESTAMP_GAP_MS, 5 * 60 * 1000)
})

test('shouldShowConversationTimestamp indexes into the message list', () => {
  const a = { role: 'user', created_at: new Date(2024, 2, 5, 8, 0, 0).toISOString() }
  const b = { role: 'assistant', created_at: new Date(2024, 2, 5, 8, 1, 0).toISOString() }
  assert.equal(shouldShowConversationTimestamp([a, b], 0), true)
  assert.equal(shouldShowConversationTimestamp([a, b], 1), false)
})

test('formatConversationTimestampLabel composes relative day labels with clock time', () => {
  const now = new Date(2024, 2, 5, 12, 0, 0)
  const today = new Date(2024, 2, 5, 9, 5, 0).toISOString()
  const yesterday = new Date(2024, 2, 4, 22, 1, 0).toISOString()
  const thisYear = new Date(2024, 0, 2, 8, 0, 0).toISOString()
  const otherYear = new Date(2023, 6, 15, 7, 30, 0).toISOString()

  assert.equal(formatConversationTimestampLabel(today, labels, now), `Today 09:05`)
  assert.equal(formatConversationTimestampLabel(yesterday, labels, now), `Yesterday 22:01`)
  assert.equal(formatConversationTimestampLabel(thisYear, labels, now), `1/2 08:00`)
  assert.equal(formatConversationTimestampLabel(otherYear, labels, now), `2023/7/15 07:30`)
  assert.equal(formatConversationTimestampLabel('bogus', labels, now), '')
})