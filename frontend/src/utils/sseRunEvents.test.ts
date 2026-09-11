import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseSseRunEvents } from './sseRunEvents';

test('parses run events from an SSE body and drops control frames', () => {
  const body = [
    'event: 3',
    'data: {"seq":3,"attempt_id":"m1","type":"answer_delta","payload":{"text":"你"}}',
    '',
    'event: run',
    'data: {"run_id":"r1","status":"running","seq":3}',
    '',
    'event: keepalive',
    'data: {"seq":3}',
    '',
    'event: 4',
    'data: {"seq":4,"attempt_id":"m1","type":"answer_delta","payload":{"text":"好"}}',
  ].join('\n');
  const events = parseSseRunEvents(body);
  assert.equal(events.length, 2);
  assert.equal(events[0].seq, 3);
  assert.equal(events[1].payload.text, '好');
});

test('ignores malformed data lines and non-string bodies', () => {
  assert.deepEqual(parseSseRunEvents(['data: not-json', 'data: 42', 'data: {"seq":"x","type":"a","attempt_id":"b"}'].join('\n')), []);
  assert.deepEqual(parseSseRunEvents(null), []);
  assert.deepEqual(parseSseRunEvents({ data: [] }), []);
});

test('accepts CRLF separators', () => {
  const body = 'data: {"seq":1,"attempt_id":"a","type":"answer","payload":{"text":"ok"}}\r\ndata: {"seq":1,"attempt_id":"a","type":"answer","payload":{"text":"ok"}}\r\n';
  assert.equal(parseSseRunEvents(body).length, 2);
});
