import assert from 'node:assert/strict';
import test from 'node:test';

import { Marked } from 'marked';

// React port of frontend/src/utils/markdownOptions.test.ts (#3962, shared
// bug fix — Vue↔React parity): CJK range separators must stay literal.
const { applyMarkdownOptions } = await import('./markdownOptions.ts');

function renderWithSharedOptions(markdown: string): string {
  const instance = new Marked();
  applyMarkdownOptions(instance);
  return instance.parse(markdown, { async: false }) as string;
}

test('#3962 single-tilde CJK range separators render literally (no <del>)', () => {
  const rangeCases = [
    '规划期为 2020~2035 年',
    '设计水温 50~60°',
    '设计规模 50,000~100,000 m³/d',
    '运行时段 8:00~12:00，二期扩至 14:00~18:00',
  ];
  for (const markdown of rangeCases) {
    const html = renderWithSharedOptions(markdown);
    assert.ok(!html.includes('<del'), `expected no <del> for "${markdown}", got: ${html}`);
    assert.ok(html.includes(markdown.trim()), `expected literal "${markdown.trim()}" in: ${html}`);
  }
});

test('~~text~~ still renders <del> after the single-tilde branch removal', () => {
  assert.equal(renderWithSharedOptions('~~已废弃~~'), '<p><del>已废弃</del></p>\n');
});

test('GFM tables and breaks keep rendering (no del regression)', () => {
  const table = renderWithSharedOptions('| 指标 | 范围 |\n| --- | --- |\n| 水温 | 50~60° |');
  assert.ok(table.includes('<table'), `expected a table, got: ${table}`);
  assert.ok(table.includes('50~60°'), `expected literal range inside the table, got: ${table}`);
  const breaks = renderWithSharedOptions('line one\nline two');
  assert.ok(breaks.includes('<br'), `expected a <br>, got: ${breaks}`);
});
