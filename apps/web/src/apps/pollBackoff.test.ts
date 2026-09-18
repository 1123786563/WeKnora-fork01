import assert from 'node:assert/strict'
import test from 'node:test'

import { POLL_INTERVAL_MS, POLL_MAX_INTERVAL_MS, pollBackoffDelayMs } from './pollBackoff'

test('backoff doubles per consecutive failure', () => {
  assert.equal(pollBackoffDelayMs(0), POLL_INTERVAL_MS)
  assert.equal(pollBackoffDelayMs(1), 6000)
  assert.equal(pollBackoffDelayMs(2), 12000)
  assert.equal(pollBackoffDelayMs(3), 24000)
})

test('backoff is capped at the max interval', () => {
  assert.equal(pollBackoffDelayMs(4), POLL_MAX_INTERVAL_MS)
  assert.equal(pollBackoffDelayMs(50), POLL_MAX_INTERVAL_MS)
})

test('negative failure counts fall back to the base interval', () => {
  assert.equal(pollBackoffDelayMs(-1), POLL_INTERVAL_MS)
})
