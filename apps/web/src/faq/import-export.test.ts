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
