import test from 'node:test';
import assert from 'node:assert/strict';
import { referenceRoute, selectFaqReferenceLabel, selectWikiReferenceLabel } from './reference.ts';

test('mobile Wiki rows keep a readable title and revision marker', () => {
  assert.equal(
    selectWikiReferenceLabel({ title: 'Getting started', slug: 'getting-started', version: 4 }),
    'Getting started · v4',
  );
  assert.equal(selectWikiReferenceLabel({ title: '', slug: 'fallback/page', version: 1 }), 'fallback/page · v1');
});

test('mobile FAQ rows expose enabled and recommended state without inventing content', () => {
  assert.equal(
    selectFaqReferenceLabel({ standard_question: 'How do I reset access?', is_enabled: true, is_recommended: false }),
    'How do I reset access? · enabled',
  );
  assert.equal(
    selectFaqReferenceLabel({ standard_question: '  ', is_enabled: false, is_recommended: true }),
    'Untitled FAQ · disabled · recommended',
  );
});

test('mobile knowledge reference routes remain knowledge-base scoped', () => {
  assert.equal(referenceRoute('wiki', 'kb/one'), '/knowledge/kb%2Fone/wiki');
  assert.equal(referenceRoute('faq', 'kb-one'), '/knowledge/kb-one/faq');
});
