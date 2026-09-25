import assert from 'node:assert/strict';
import test from 'node:test';

import { errorFromResult } from './errors.ts';

// S00 negpath3 T-2: the backend returns 403 bodies like
// {"error":"Access denied: URL workspace does not match the active workspace"}
// (error is a STRING, not the usual {code,message} record). Vue surfaces the
// backend message verbatim; React used to degrade to
// "Request failed with status 403", dropping the backend copy.
test('403 with string error body preserves the backend message (S00 T-2)', () => {
  const error = errorFromResult(403, {
    error: 'Access denied: URL workspace does not match the active workspace',
  });
  assert.equal(
    error.message,
    'Access denied: URL workspace does not match the active workspace',
  );
  assert.equal(error.status, 403);
});

test('403 with {code,message} record keeps the nested message', () => {
  const error = errorFromResult(404, { error: { code: 1003, message: 'knowledge base not found' } });
  assert.equal(error.message, 'knowledge base not found');
  assert.equal(error.code, '1003');
});

test('plain string body still becomes the message (kept verbatim)', () => {
  const error = errorFromResult(500, ' upstream exploded ');
  assert.equal(error.message, ' upstream exploded ');
});

// (R1-24) POST /purchases 的平台故障信封是 503 + 顶层闭合 reason 令牌
// （{"error":"purchase temporarily unavailable","reason":"unreachable"}）——
// 非 2xx 直接抛 ApiError，顶层 reason 必须并入 details 存活下来，调用方才能
// 按闭合令牌映射文案，而不是只拿到英文 message。
test('503 purchase envelope preserves the top-level reason token in details', () => {
  const error = errorFromResult(503, {
    error: 'purchase temporarily unavailable',
    reason: 'unreachable',
  });
  assert.equal(error.status, 503);
  assert.equal(error.message, 'purchase temporarily unavailable');
  const details = error.details as { reason?: string };
  assert.equal(details?.reason, 'unreachable');
});

test('existing details object is preserved when merging the reason token', () => {
  const error = errorFromResult(503, {
    error: 'purchase temporarily unavailable',
    reason: 'unconfigured',
    details: { hint: 'channel not wired' },
  });
  const details = error.details as { hint?: string; reason?: string };
  assert.equal(details?.hint, 'channel not wired');
  assert.equal(details?.reason, 'unconfigured');
});

test('bodies without a reason token leave details untouched', () => {
  const error = errorFromResult(409, { error: 'quote expired' });
  assert.equal(error.details, undefined);
});

// (R3-31) 数组/字符串型 details 不得被展开合并污染：数组被 {...} 展开会变成
// 索引键对象（数组语义静默丢失），字符串会被整体丢弃——两者都改为仅携带
// reason 令牌的新对象。
test('an array details body is not spread into index keys when merging the reason', () => {
  const error = errorFromResult(503, { error: 'unavailable', reason: 'unreachable', details: ['a', 'b'] });
  const details = error.details as { reason?: string };
  assert.equal(details?.reason, 'unreachable');
  assert.equal(Array.isArray(error.details), false);
  assert.equal('0' in (error.details as object), false);
});

test('a string details body degrades to a token-only object, not a silent drop', () => {
  const error = errorFromResult(503, { error: 'unavailable', reason: 'unconfigured', details: 'hint-text' });
  const details = error.details as { reason?: string };
  assert.equal(details?.reason, 'unconfigured');
});
