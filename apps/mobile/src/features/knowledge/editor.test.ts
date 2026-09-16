import assert from 'node:assert/strict';
import test from 'node:test';
import { knowledgeListLabel } from './list.ts';
import { ApiError } from '@weknora/api-client';
import {
  classifyMobileEditorError,
  createFaqDraft,
  createWikiDraft,
  validateFaqDraft,
  validateWikiDraft,
} from './editor.ts';

test('mobile Wiki draft keeps the server version for optimistic conflict checks', () => {
  assert.deepEqual(createWikiDraft({ title: ' Guide ', summary: 'Summary', content: 'Body', version: 7 }), {
    title: 'Guide', summary: 'Summary', content: 'Body', version: 7,
  });
});

test('mobile FAQ draft maps the first answer and preserves server flags', () => {
  assert.deepEqual(createFaqDraft({ standard_question: ' Question ', answers: ['Answer', 'Other'], is_enabled: true, is_recommended: false }), {
    standardQuestion: 'Question', answer: 'Answer', isEnabled: true, isRecommended: false,
  });
});

test('mobile editors reject empty required fields before a network write', () => {
  assert.deepEqual(validateWikiDraft({ title: ' ', content: '' }), ['knowledgeEditor.mobile.titleRequired', 'knowledgeEditor.mobile.contentRequired']);
  assert.deepEqual(validateFaqDraft({ standardQuestion: '', answer: ' ' }), ['knowledgeEditor.mobile.questionRequired', 'knowledgeEditor.mobile.answerRequired']);
});

test('mobile editor distinguishes conflict from ordinary API failure', () => {
  assert.equal(classifyMobileEditorError(new ApiError({ status: 409, code: 'CONFLICT', message: 'stale' })), 'conflict');
  assert.equal(classifyMobileEditorError(new ApiError({ status: 403, code: 'FORBIDDEN', message: 'denied' })), 'error');
});

test('empty editor validation is localized in every supported locale', () => {
  const expected = {
    'zh-CN': ['请填写标题', '请填写正文', '请填写问题', '请填写答案'],
    'en-US': ['Title is required', 'Content is required', 'Question is required', 'Answer is required'],
    'ja-JP': ['タイトルを入力してください', '本文を入力してください', '質問を入力してください', '回答を入力してください'],
    'ko-KR': ['제목을 입력하세요', '본문을 입력하세요', '질문을 입력하세요', '답변을 입력하세요'],
    'ru-RU': ['Введите заголовок', 'Введите содержимое', 'Введите вопрос', 'Введите ответ'],
  } as const;
  for (const locale of Object.keys(expected) as (keyof typeof expected)[]) {
    const keys = [...validateWikiDraft({ title: '', content: '' }), ...validateFaqDraft({ standardQuestion: '', answer: '' })];
    assert.deepEqual(keys.map((key) => knowledgeListLabel(locale, key)), expected[locale], locale);
    assert.equal(keys.some((key) => knowledgeListLabel(locale, key) === key), false, locale);
  }
});
