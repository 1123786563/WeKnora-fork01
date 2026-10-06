import assert from 'node:assert/strict'
import test from 'node:test'

import { Marked } from 'marked'

import { applyMarkdownOptions, doubleTildeDelTokenizer } from './markdownOptions.ts'
import { createChatMarkdownRenderer, renderChatMarkdown } from './chatMarkdownRenderer.ts'

function renderWithSharedOptions(markdown: string): string {
  const instance = new Marked()
  applyMarkdownOptions(instance)
  return instance.parse(markdown, { async: false }) as string
}

// #3962 — CJK technical writing uses `~` as a range separator. marked's
// default GFM del tokenizer accepts a single tilde, so a line with two
// ranges had the span between them swallowed into <del> with the tildes
// eaten. Only `~~text~~` may render as <del>.
test('#3962 single-tilde CJK range separators render literally (no <del>)', () => {
  const rangeCases = [
    '规划期为 2020~2035 年',
    '设计水温 50~60°',
    '设计规模 50,000~100,000 m³/d',
    '运行时段 8:00~12:00，二期扩至 14:00~18:00',
  ]
  for (const markdown of rangeCases) {
    const html = renderWithSharedOptions(markdown)
    assert.ok(
      !html.includes('<del'),
      `expected no <del> for "${markdown}", got: ${html}`,
    )
    const range = markdown.trim()
    assert.ok(html.includes(range), `expected literal "${range}" in: ${html}`)
  }
})

test('#3962 the chat chain keeps single-tilde ranges literal', () => {
  const html = renderChatMarkdown('水温 50~60°，运行时段 8:00~12:00', {
    renderer: createChatMarkdownRenderer(),
    escapeMarkdown: (text) => text,
    sanitizeHtml: (value) => value,
    streaming: false,
  })
  assert.ok(!html.includes('<del'), `expected no <del>, got: ${html}`)
  assert.ok(html.includes('50~60°'), `expected literal range in: ${html}`)
  assert.ok(html.includes('8:00~12:00'), `expected literal range in: ${html}`)
})

test('~~text~~ still renders <del> after the single-tilde branch removal', () => {
  assert.equal(
    renderWithSharedOptions('~~已废弃~~'),
    '<p><del>已废弃</del></p>\n',
  )
})

test('GFM tables keep rendering (no del regression)', () => {
  const html = renderWithSharedOptions('| 指标 | 范围 |\n| --- | --- |\n| 水温 | 50~60° |')
  assert.ok(html.includes('<table'), `expected a table, got: ${html}`)
  assert.ok(html.includes('<th>'), `expected table headers, got: ${html}`)
  assert.ok(html.includes('50~60°'), `expected literal range inside the table, got: ${html}`)
})

test('breaks keeps turning single newlines into <br> (no del regression)', () => {
  const html = renderWithSharedOptions('line one\nline two')
  assert.ok(html.includes('<br'), `expected a <br>, got: ${html}`)
})

test('applyMarkdownOptions is idempotent — re-applying keeps single tildes literal', () => {
  const instance = new Marked()
  applyMarkdownOptions(instance)
  applyMarkdownOptions(instance)
  const html = instance.parse('水温 50~60°', { async: false }) as string
  assert.ok(!html.includes('<del'), `expected no <del>, got: ${html}`)
  assert.ok(html.includes('50~60°'), `expected literal range in: ${html}`)
})

test('doubleTildeDelTokenizer is exported for the React parity port', () => {
  assert.equal(typeof doubleTildeDelTokenizer, 'function')
})
