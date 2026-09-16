import assert from 'node:assert/strict'
import test from 'node:test'

import { copyAnswerText } from './copy-answer.ts'

test('copyAnswerText returns the raw answer trimmed', () => {
  assert.equal(copyAnswerText('  Hello world  '), 'Hello world')
})

test('copyAnswerText strips think blocks from either side', () => {
  assert.equal(copyAnswerText('<think>reasoning</think>Answer'), 'Answer')
  assert.equal(copyAnswerText('Answer<think>reasoning</think>'), 'Answer')
  assert.equal(copyAnswerText('<think>partial'), '')
})

test('copyAnswerText handles empty and non-string input', () => {
  assert.equal(copyAnswerText(''), '')
  assert.equal(copyAnswerText(undefined), '')
  assert.equal(copyAnswerText(null), '')
  assert.equal(copyAnswerText('   '), '')
})
