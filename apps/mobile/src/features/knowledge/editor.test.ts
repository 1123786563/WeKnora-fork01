import assert from 'node:assert/strict';
import test from 'node:test';
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
  assert.deepEqual(validateWikiDraft({ title: ' ', content: '' }), ['Title is required', 'Content is required']);
  assert.deepEqual(validateFaqDraft({ standardQuestion: '', answer: ' ' }), ['Question is required', 'Answer is required']);
});

test('mobile editor distinguishes conflict from ordinary API failure', () => {
  assert.equal(classifyMobileEditorError(new ApiError({ status: 409, code: 'CONFLICT', message: 'stale' })), 'conflict');
  assert.equal(classifyMobileEditorError(new ApiError({ status: 403, code: 'FORBIDDEN', message: 'denied' })), 'error');
});
