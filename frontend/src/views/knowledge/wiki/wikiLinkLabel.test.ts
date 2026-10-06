import assert from 'node:assert/strict'
import test from 'node:test'

import { wikiLinkLabel } from './wikiLinkLabel.ts'

test('wikiLinkLabel prefers the backend-resolved title over the fallback', () => {
  const titles: Record<string, string> = {
    'concept/zhu-jie-gu-dong-wei-bei-zhixing-ren': '主睫固定尾背未执行人',
    'entity/mou-gong-si': '某公司',
    'summary/mou-wen-dang': '某文档 - Summary',
  }
  assert.equal(
    wikiLinkLabel(titles, 'concept/zhu-jie-gu-dong-wei-bei-zhixing-ren', 'zhu-jie-gu-dong-wei-bei-zhixing-ren'),
    '主睫固定尾背未执行人',
  )
  assert.equal(wikiLinkLabel(titles, 'entity/mou-gong-si', 'mou-gong-si'), '某公司')
  assert.equal(wikiLinkLabel(titles, 'summary/mou-wen-dang', 'mou-wen-dang'), '某文档 - Summary')
})

test('wikiLinkLabel falls back when the map is missing, empty, or lacks the slug', () => {
  assert.equal(wikiLinkLabel(undefined, 'concept/wei-zhi', '未知概念'), '未知概念')
  assert.equal(wikiLinkLabel({}, 'concept/wei-zhi', '未知概念'), '未知概念')
  assert.equal(wikiLinkLabel({ 'entity/other': '其他' }, 'concept/wei-zhi', 'wei-zhi'), 'wei-zhi')
  // An empty resolved title must not blank the label — fall through instead.
  assert.equal(wikiLinkLabel({ 'concept/empty': '' }, 'concept/empty', 'empty'), 'empty')
})
