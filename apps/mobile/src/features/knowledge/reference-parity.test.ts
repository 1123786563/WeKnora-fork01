import test from 'node:test';
import assert from 'node:assert/strict';
import { editorRoute, faqReferenceListParams, referenceRoute, selectFaqReferenceEditKey, selectFaqReferenceLabel, selectWikiReferenceEditKey, selectWikiReferenceLabel } from './reference.ts';

test('mobile Wiki rows keep a readable title and revision marker', () => {
  assert.equal(
    selectWikiReferenceLabel({ title: 'Getting started', slug: 'getting-started', version: 4 }),
    'Getting started · v4',
  );
  assert.equal(selectWikiReferenceLabel({ title: '', slug: 'fallback/page', version: 1 }), 'fallback/page · v1');
});

test('mobile Wiki editing addresses the server page by slug rather than opaque id', () => {
  assert.equal(selectWikiReferenceEditKey({ id: 'page-id', slug: 'entity/acme' }), 'entity/acme');
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

test('mobile FAQ editing keeps its numeric server identifier', () => {
  assert.equal(selectFaqReferenceEditKey({ id: 42 }), '42');
});

test('mobile knowledge reference routes remain knowledge-base scoped', () => {
  assert.equal(referenceRoute('wiki', 'kb/one'), '/knowledge/kb%2Fone/wiki');
  assert.equal(referenceRoute('faq', 'kb-one'), '/knowledge/kb-one/faq');
  assert.equal(editorRoute('wiki', 'kb-one'), '/knowledge/kb-one/editor?kind=wiki');
  assert.equal(editorRoute('faq', 'kb-one', '42'), '/knowledge/kb-one/editor?kind=faq&slug=42');
});

test('mobile knowledge reference labels accept localized copy for every visible state', () => {
  assert.equal(
    selectWikiReferenceLabel(
      { title: '', slug: 'fallback/page', version: 4 },
      { untitled: '无标题', version: (version) => `版本 ${version}` },
    ),
    'fallback/page · 版本 4',
  );
  assert.equal(
    selectFaqReferenceLabel(
      { standard_question: '  ', is_enabled: false, is_recommended: true },
      { untitled: '无标题', enabled: '已启用', disabled: '已禁用', recommended: '推荐' },
    ),
    '无标题 · 已禁用 · 推荐',
  );
});

test('mobile FAQ reference forwards a trimmed search keyword like the Vue manager', () => {
  assert.deepEqual(faqReferenceListParams('  reset access  '), {
    page: 1,
    page_size: 100,
    keyword: 'reset access',
  });
  assert.deepEqual(faqReferenceListParams('   '), { page: 1, page_size: 100 });
});
