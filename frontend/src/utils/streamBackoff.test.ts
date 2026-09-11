import { test } from 'node:test';
import assert from 'node:assert/strict';
import { streamRetryDelayMs, isStreamRetryableError, streamErrorStatus } from './streamBackoff';

test('retry delays grow and then run out', () => {
  assert.equal(streamRetryDelayMs(0), 1000);
  assert.equal(streamRetryDelayMs(1), 2000);
  assert.equal(streamRetryDelayMs(2), 4000);
  assert.equal(streamRetryDelayMs(3), null);
  assert.equal(streamRetryDelayMs(-1), null);
});

test('aborts are never retried', () => {
  assert.equal(isStreamRetryableError(new Error('HTTP 503'), true), false);
});

test('transient failures are retried, client errors are not', () => {
  assert.equal(isStreamRetryableError(new Error('HTTP 503'), false), true);
  assert.equal(isStreamRetryableError(new Error('HTTP 429'), false), true);
  assert.equal(isStreamRetryableError(new TypeError('network error'), false), true);
  assert.equal(isStreamRetryableError(new Error('HTTP 404'), false), false);
  assert.equal(isStreamRetryableError(new Error('HTTP 400'), false), false);
});

test('status is extracted from message and fields', () => {
  assert.equal(streamErrorStatus(new Error('HTTP 502')), 502);
  assert.equal(streamErrorStatus({ status: 429 }), 429);
  assert.equal(streamErrorStatus({ $httpStatus: 500 }), 500);
  assert.equal(streamErrorStatus(new Error('no status here')), undefined);
  assert.equal(streamErrorStatus(42), undefined);
});
