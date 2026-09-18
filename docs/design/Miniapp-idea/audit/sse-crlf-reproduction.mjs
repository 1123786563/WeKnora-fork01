/**
 * Standalone behavioral reproduction, NOT the repository's test suite.
 * Equivalent minimal extraction of the chunk-local newline normalization in:
 * packages/api-client/src/chat/stream.ts at 9cc91c31ef7853fb50073f92edd92807817c9259.
 * No repository files were modified.
 */
import assert from 'node:assert/strict';
function parse(chunks) {
  let buffer = '';
  let data = [];
  const events = [];
  const dispatch = () => { if (data.length) events.push(data.join('\n')); data = []; };
  const line = (value) => {
    if (value === '') return dispatch();
    if (value.startsWith('data:')) data.push(value.slice(5).replace(/^ /, ''));
  };
  for (const chunk of chunks) {
    buffer += chunk.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
    const lines = buffer.split('\n');
    buffer = lines.pop() ?? '';
    for (const value of lines) line(value);
  }
  if (buffer) line(buffer);
  dispatch();
  return events;
}
const whole = 'data: {"a":\r\ndata: 1}\r\n\r\n';
const contiguous = parse([whole]);
const fragmented = parse(['data: {"a":\r', '\ndata: 1}\r\n\r\n']);
assert.deepEqual(contiguous, ['{"a":\n1}']);
assert.notDeepEqual(fragmented, contiguous);
assert.equal(fragmented.length, 2);
const report = {scope: '最小等价算法复现；未运行仓库测试', contiguous, fragmented, crlfBoundaryRiskReproduced: true};
console.log(JSON.stringify(report, null, 2));
