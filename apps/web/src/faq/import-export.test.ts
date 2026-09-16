import assert from 'node:assert/strict';
import test from 'node:test';
import { normalizeFAQPayload, parseFAQImportText, serializeFAQEntries } from './import-export.ts';

test('normalizes FAQ fields and rejects incomplete entries', () => {
  assert.deepEqual(normalizeFAQPayload({ standard_question: '  How? ', answers: [' This. ', ''] }), {
    standard_question: 'How?', similar_questions: [], negative_questions: [], answers: ['This.'],
  });
  assert.throws(() => normalizeFAQPayload({ standard_question: 'How?', answers: [] }), /answer/i);
});

test('round-trips JSON and quoted CSV FAQ imports', () => {
  const entry = { id: 1, standard_question: 'How, exactly?', similar_questions: ['When?'], negative_questions: [], answers: ['Use "this".'], is_enabled: true, is_recommended: false };
  const json = serializeFAQEntries([entry], 'json');
  assert.deepEqual(parseFAQImportText(json, 'json'), [{ standard_question: 'How, exactly?', similar_questions: ['When?'], negative_questions: [], answers: ['Use "this".'], is_enabled: true, is_recommended: false }]);
  const csv = serializeFAQEntries([entry], 'csv');
  assert.deepEqual(parseFAQImportText(csv, 'csv'), [{ standard_question: 'How, exactly?', similar_questions: ['When?'], negative_questions: [], answers: ['Use "this".'] }]);
});

test('CSV import mirrors Vue Chinese headers, ## lists, tag name, and disabled inversion', () => {
  const csv = [
    '问题(标准问题),相似问题,反例问题,机器人回答,是否停用,标签',
    '如何登录?,登录方法##怎样登录,无法登录,打开登录页##输入凭据,是,账号',
  ].join('\n');

  assert.deepEqual(parseFAQImportText(csv, 'csv'), [{
    standard_question: '如何登录?',
    similar_questions: ['登录方法', '怎样登录'],
    negative_questions: ['无法登录'],
    answers: ['打开登录页', '输入凭据'],
    tag_name: '账号',
    is_enabled: false,
  }]);
});
