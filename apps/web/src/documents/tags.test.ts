import assert from 'node:assert/strict';
import test from 'node:test';
import {
  computeTagVisibleLimit,
  TAG_PANEL_PAGE_SIZE,
  commonTagIds,
  documentTags,
  filterTagOptions,
  joinTagIds,
  paginateTagOptions,
  selectTagId,
  tagCreateFailureMessage,
  tagFilterLabel,
  tagFilterTitle,
  tagUpdatesFor,
} from './tags.ts';

test('tag chip limit reserves Vue overflow pill space and is width responsive', () => {
  assert.equal(computeTagVisibleLimit(300, 5), 3);
  assert.equal(computeTagVisibleLimit(100, 5), 1);
  assert.equal(computeTagVisibleLimit(500, 3), 99);
});
import type { KnowledgeTag } from '@weknora/api-client';

const tag = (id: string, name: string, knowledgeCount?: number): KnowledgeTag => ({
  id,
  name,
  knowledge_count: knowledgeCount,
});

// --- documentTags: Vue KnowledgeCard.tags (DocumentCardView.vue L35) ---------------

test('documentTags reads the {id,name} tag objects the backend embeds per document', () => {
  const document = { id: 'doc-1', tags: [{ id: '7', name: '重要' }, { id: '9', name: '合同' }] } as never;
  assert.deepEqual(documentTags(document).map((item) => item.id), ['7', '9']);
});

test('documentTags tolerates missing or malformed tags like the Vue baseline', () => {
  assert.deepEqual(documentTags({ id: 'doc-1' } as never), []);
  assert.deepEqual(documentTags({ id: 'doc-1', tags: 'nope' } as never), []);
  assert.deepEqual(documentTags({ id: 'doc-1', tags: [{ name: 'no id' }] } as never), []);
});

// --- commonTagIds: Vue batchTagPreSelectedIds (KnowledgeBase.vue L445-460) ---------

test('commonTagIds keeps only tags shared by every selected document', () => {
  const documents = [
    { id: 'a', tags: [{ id: '1' }, { id: '2' }, { id: '3' }] },
    { id: 'b', tags: [{ id: '2' }, { id: '3' }, { id: '4' }] },
    { id: 'c', tags: [{ id: '3' }, { id: '2' }] },
  ] as never[];
  assert.deepEqual(commonTagIds(documents), ['2', '3']);
});

test('commonTagIds returns nothing when the selection is empty or shares no tag', () => {
  assert.deepEqual(commonTagIds([]), []);
  const documents = [
    { id: 'a', tags: [{ id: '1' }] },
    { id: 'b', tags: [{ id: '2' }] },
  ] as never[];
  assert.deepEqual(commonTagIds(documents), []);
});

// --- filterTagOptions: Vue BatchTagDialog availableTagsList (L145-152) -------------

test('filterTagOptions hides selected tags and filters by search case-insensitively', () => {
  const tags = [tag('1', '重要'), tag('2', '合同'), tag('3', 'Legal')];
  assert.deepEqual(
    filterTagOptions(tags, ['1'], '').map((item) => item.id),
    ['2', '3'],
  );
  assert.deepEqual(
    filterTagOptions(tags, [], '同').map((item) => item.id),
    ['2'],
  );
  assert.deepEqual(
    filterTagOptions(tags, [], 'legal').map((item) => item.id),
    ['3'],
  );
});

test('selectTagId keeps an existing tag selected instead of toggling it off', () => {
  assert.deepEqual(selectTagId(['1', '2'], '2'), ['1', '2']);
  assert.deepEqual(selectTagId(['1'], '2'), ['1', '2']);
});

test('tagCreateFailureMessage keeps a server error but safely falls back for unknown failures', () => {
  assert.equal(tagCreateFailureMessage(new Error('Tag already exists'), '操作失败'), 'Tag already exists');
  assert.equal(tagCreateFailureMessage({ message: '  ' }, '操作失败'), '操作失败');
  assert.equal(tagCreateFailureMessage(null, '操作失败'), '操作失败');
});

// --- joinTagIds: Vue filterParams tag_ids (KnowledgeBase.vue L671) -----------------

test('joinTagIds comma-joins for the list request and drops the param when empty', () => {
  assert.equal(joinTagIds(['7', '9']), '7,9');
  assert.equal(joinTagIds([]), undefined);
});

// --- tagFilterLabel: Vue activeTagFilterLabel (KnowledgeBase.vue L708-719) ---------

test('tagFilterLabel mirrors the Vue trigger label states', () => {
  assert.deepEqual(tagFilterLabel([], false), { kind: 'all' });
  assert.deepEqual(tagFilterLabel([], true), { kind: 'placeholder' });
  assert.deepEqual(tagFilterLabel(['7'], false), { kind: 'single', id: '7' });
  assert.deepEqual(tagFilterLabel(['7', '9'], false), { kind: 'multi' });
});

// --- tagFilterTitle: Vue activeTagFilterTitle (KnowledgeBase.vue L721-729) ---------

test('tagFilterTitle joins visible names with 、 and falls back to the title key', () => {
  const nameOf = (id: string) => (id === '7' ? '重要' : id === '9' ? '合同' : undefined);
  assert.equal(tagFilterTitle([], nameOf, '按标签筛选'), '按标签筛选');
  assert.equal(tagFilterTitle(['7', '9'], nameOf, '按标签筛选'), '重要、合同');
  assert.equal(tagFilterTitle(['404'], nameOf, '按标签筛选'), '按标签筛选');
});

// --- panel paging: Vue TAG_PAGE_SIZE=50 + load more (L551, L926, L2510-2515) -------

test('paginateTagOptions shows the first 50 tags and extends on load more', () => {
  const tags = Array.from({ length: 55 }, (_, index) => tag(String(index), `tag-${index}`));
  assert.equal(TAG_PANEL_PAGE_SIZE, 50);
  assert.equal(paginateTagOptions(tags, TAG_PANEL_PAGE_SIZE).length, 50);
  assert.equal(paginateTagOptions(tags, TAG_PANEL_PAGE_SIZE * 2).length, 55);
});

// --- tagUpdatesFor: Vue onBatchTagConfirm (KnowledgeBase.vue L2169-2178) -----------

test('tagUpdatesFor maps every selected document to the same tag list', () => {
  assert.deepEqual(tagUpdatesFor([{ id: 'a' }, { id: 'b' }] as never[], ['1', '2']), {
    a: ['1', '2'],
    b: ['1', '2'],
  });
  assert.deepEqual(tagUpdatesFor([], []), {});
});
