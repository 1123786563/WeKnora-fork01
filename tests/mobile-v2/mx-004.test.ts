import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-004.ts';

test('go-bytes-to-native-parser', async () => {
  const observed = await runProbe({
  "fixture": "Go-emitted-UTF8-CRLF-control-seq42",
  "fault": "split-every-byte"
});
  assert.deepEqual(observed, {
    "seqs": [
      42
    ],
    "controlCount": 1,
    "cursor": 42
});
});

// 附加验收（分册步骤3）：>256 条无丢失；控制帧不占业务 seq；cursor 到 300。
test('go-bytes-256-plus-no-loss', async () => {
  const observed = await runProbe({ fixture: 'mx-004-stream-300', fault: 'none' });
  assert.equal(observed.seqs.length, 300);
  assert.deepEqual(observed.seqs.slice(0, 3), [1, 2, 3]);
  assert.deepEqual(observed.seqs.slice(-3), [298, 299, 300]);
  assert.equal(observed.controlCount, 4);
  assert.equal(observed.cursor, 300);
});
