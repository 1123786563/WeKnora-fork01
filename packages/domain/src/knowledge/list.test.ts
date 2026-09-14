import assert from 'node:assert/strict';
import test from 'node:test';

import { filterKnowledgeBases } from './list.ts';

const items = [
  { id: 'kb-1', name: 'Product docs', type: 'document', creator_id: 'user-1', is_favorite: true },
  { id: 'kb-2', name: 'Support FAQ', type: 'faq', creator_id: 'user-2', source: 'organization', permission: 'viewer' },
  { id: 'kb-3', name: 'Private notes', type: 'document', creator_id: 'user-1' },
];

test('filters knowledge bases by query, type, creator, and preserves source metadata', () => {
  const result = filterKnowledgeBases(items, { query: 'faq', type: 'faq', creator: 'others', currentUserId: 'user-1' });
  assert.deepEqual(result, { items: [items[1]], total: 1, page: 1, pageCount: 1 });
  assert.equal((result.items[0] as Record<string, unknown>).source, 'organization');
});

test('paginates filtered knowledge bases and clamps an out-of-range page', () => {
  const result = filterKnowledgeBases(items, { page: 3, pageSize: 2 });
  assert.deepEqual(result, { items: [items[2]], total: 3, page: 2, pageCount: 2 });
});

test('supports a favorites-only view without treating missing favorite state as true', () => {
  const result = filterKnowledgeBases(items, { favoritesOnly: true });
  assert.deepEqual(result.items, [items[0]]);
});

// ---- kbListMerge / card gating parity (ported from frontend/src/views/knowledge) ----

import { canDuplicateKBCard, canManageKBCard, isKnowledgeBaseInitialized, isSharedKbEditable, mergeAllScopeKnowledgeBases } from './list.ts';

const owned = [
  { id: 'kb-a', name: 'A', creator_id: 'user-1', created_at: '2024-01-01T00:00:00Z' },
  { id: 'kb-b', name: 'B', creator_id: 'user-2', is_pinned: true, pinned_at: '2024-02-01T00:00:00Z' },
  { id: 'kb-c', name: 'C', creator_id: 'user-1', is_pinned: true, pinned_at: '2024-03-01T00:00:00Z' },
];

test('merges shared knowledge bases without duplicating owned ids', () => {
  const shared = [
    { knowledge_base: { id: 'kb-a', name: 'A' }, permission: 'editor', shared_at: '2024-04-01T00:00:00Z', share_id: 's-1' },
    { knowledge_base: { id: 'kb-d', name: 'D' }, permission: 'viewer', shared_at: '2024-04-02T00:00:00Z', share_id: 's-2' },
  ];
  const merged = mergeAllScopeKnowledgeBases(owned, shared, 'user-1');
  const ids = merged.map((kb) => kb.id);
  assert.equal(new Set(ids).size, ids.length, 'duplicate kb id rendered twice');
  assert.equal(ids.filter((id) => id === 'kb-a').length, 1, 'owned KB lost to shared duplicate');
  assert.equal(merged[0]?.id, 'kb-c', 'newest pinned first');
});

test('collapses duplicate shares of one knowledge base to the most-privileged permission', () => {
  const shared = [
    { knowledge_base: { id: 'kb-d' }, permission: 'viewer', shared_at: '2024-04-02T00:00:00Z', share_id: 's-1' },
    { knowledge_base: { id: 'kb-d' }, permission: 'editor', shared_at: '2024-04-03T00:00:00Z', share_id: 's-2' },
  ];
  const merged = mergeAllScopeKnowledgeBases([], shared, 'user-1');
  assert.equal(merged.length, 1);
  assert.equal((merged[0] as { permission: string }).permission, 'editor');
});

test('orders merged cards pinned, mine, teammate, then shared editable before readonly', () => {
  const shared = [
    { knowledge_base: { id: 'kb-ro' }, permission: 'viewer', shared_at: '2024-04-02T00:00:00Z', share_id: 's-1' },
    { knowledge_base: { id: 'kb-ed' }, permission: 'editor', shared_at: '2024-04-03T00:00:00Z', share_id: 's-2' },
  ];
  const merged = mergeAllScopeKnowledgeBases(owned, shared, 'user-1');
  assert.deepEqual(merged.map((kb) => kb.id), ['kb-c', 'kb-b', 'kb-a', 'kb-ed', 'kb-ro']);
});

test('viewer role cannot manage or duplicate; creator and admin can manage', () => {
  const kb = { id: 'kb-1', creator_id: 'user-9' };
  assert.equal(canManageKBCard(kb, { userId: 'user-9', isAdmin: false }), true, 'creator match manages');
  assert.equal(canManageKBCard(kb, { userId: 'user-1', isAdmin: false }), false, 'other member cannot manage');
  assert.equal(canManageKBCard(kb, { userId: 'user-1', isAdmin: true }), true, 'admin fallback manages');
  // Legacy KBs created before creator_id existed fall back to the role gate.
  assert.equal(canManageKBCard({ id: 'kb-legacy' }, { userId: 'user-1', isAdmin: false }), false);
  assert.equal(canManageKBCard({ id: 'kb-legacy' }, { userId: 'user-1', isAdmin: true }), true);
  assert.equal(canDuplicateKBCard(kb, { userId: 'user-9', isAdmin: false, isContributor: true }), true);
  assert.equal(canDuplicateKBCard(kb, { userId: 'user-9', isAdmin: false, isContributor: false }), false, 'viewer cannot duplicate');
  assert.equal(canDuplicateKBCard({ id: 'kb-1', creator_id: 'user-2', isMine: false }, { userId: 'user-9', isAdmin: false, isContributor: true }), false, 'shared KB cannot be duplicated');
});

test('isSharedKbEditable mirrors the Vue EDITABLE_PERMS set', () => {
  assert.equal(isSharedKbEditable('admin'), true);
  assert.equal(isSharedKbEditable('editor'), true);
  assert.equal(isSharedKbEditable('viewer'), false);
  assert.equal(isSharedKbEditable(undefined), false);
});

test('isKnowledgeBaseInitialized follows the Vue summary/embedding model rule', () => {
  assert.equal(isKnowledgeBaseInitialized({ summary_model_id: 'llm', embedding_model_id: 'emb' }), true);
  assert.equal(isKnowledgeBaseInitialized({ summary_model_id: 'llm' }), false, 'embedding missing for RAG KB');
  assert.equal(isKnowledgeBaseInitialized({ summary_model_id: '', embedding_model_id: 'emb' }), false, 'summary model required');
  assert.equal(
    isKnowledgeBaseInitialized({ summary_model_id: 'llm', indexing_strategy: { vector_enabled: false, keyword_enabled: false } }),
    true,
    'non-RAG KB does not need an embedding model',
  );
  assert.equal(
    isKnowledgeBaseInitialized({ summary_model_id: 'llm', indexing_strategy: { wiki_enabled: true } }),
    true,
    'wiki-only strategy disables vector/keyword, so no embedding model is required (Vue rule)',
  );
  assert.equal(
    isKnowledgeBaseInitialized({ summary_model_id: 'llm', indexing_strategy: { wiki_enabled: true, vector_enabled: true } }),
    false,
    'wiki KB with vector indexing still needs an embedding model',
  );
});

import { groupKnowledgeBaseSections } from './list.ts';

function kb(id: string, extra: Record<string, unknown> = {}): Record<string, unknown> & { id: string } {
  return { id, name: id, ...extra } as Record<string, unknown> & { id: string };
}

test('groupKnowledgeBaseSections: pinned first, then mine, tenantOthers, sharedEditable, sharedReadonly', () => {
  const rows = [
    kb('p1', { is_pinned: true, pinned_at: '2026-01-02', isMine: true, creator_id: 'me' }),
    kb('m1', { isMine: true, creator_id: 'me' }),
    kb('t1', { isMine: true, creator_id: 'someone-else' }),
    kb('se1', { isMine: false, permission: 'editor', creator_id: 'x' }),
    kb('sr1', { isMine: false, permission: 'view', creator_id: 'x' }),
    kb('p2', { is_pinned: true, pinned_at: '2026-01-03', isMine: false, permission: 'view' }),
  ] as never[];
  const sections = groupKnowledgeBaseSections(rows, 'me');
  const keys = sections.map((s: { key: string }) => s.key);
  assert.deepEqual(keys, ['pinned', 'mine', 'tenantOthers', 'sharedEditable', 'sharedReadonly']);
  const byKey = Object.fromEntries(sections.map((s: { key: string; items: { id: string }[] }) => [s.key, s.items.map((i) => i.id)]));
  assert.deepEqual(byKey.pinned, ['p2', 'p1']); // newest pinned_at first (Vue:932-949)
  assert.deepEqual(byKey.mine, ['m1']);
  assert.deepEqual(byKey.tenantOthers, ['t1']);
  assert.deepEqual(byKey.sharedEditable, ['se1']);
  assert.deepEqual(byKey.sharedReadonly, ['sr1']);
  for (const s of sections) assert.ok(s.items.length > 0, 'empty sections are omitted');
});

// Vue tenantSectionLabelKey (KnowledgeBaseList.vue:1083-1086): contributor/
// viewer group header reads「本空间 · 仅查看」; admin/owner read「本空间 · 其他成员」.
test('groupKnowledgeBaseSections: tenantReadonly option swaps the tenant group label', () => {
  const rows = [kb('t1', { isMine: true, creator_id: 'someone-else' })] as never[];
  const readonly = groupKnowledgeBaseSections(rows, 'me', { tenantReadonly: true });
  assert.equal(readonly.find((s: { key: string }) => s.key === 'tenantOthers')?.labelKey, 'knowledgeList.sections.tenantReadonly');
  const admin = groupKnowledgeBaseSections(rows, 'me');
  assert.equal(admin.find((s: { key: string }) => s.key === 'tenantOthers')?.labelKey, 'knowledgeList.sections.tenantOthers');
  // The section key itself stays stable so collapse state and icons keep working.
  assert.equal(readonly[0]!.key, 'tenantOthers');
});

test('groupKnowledgeBaseSections keeps empty sections out of the result', () => {
  const sections = groupKnowledgeBaseSections([kb('m1', { isMine: true, creator_id: 'me' }) as never], 'me');
  assert.deepEqual(sections.map((s: { key: string }) => s.key), ['mine']);
});

import { filterByScope } from './list.ts';

function kbx(id: string, extra: Record<string, unknown> = {}) {
  return { id, name: id, ...extra } as Record<string, unknown> & { id: string };
}

test('filterByScope: all returns everything; mine returns only own creations', () => {
  const rows = [
    kbx('a', { isMine: true, creator_id: 'me' }),
    kbx('b', { isMine: true, creator_id: 'other', is_shared: true }),
    kbx('c', { isMine: false }),
  ] as never[];
  assert.equal(filterByScope(rows, 'all', 'me').length, 3);
  assert.deepEqual(filterByScope(rows, 'mine', 'me').map((r: { id: string }) => r.id), ['a']);
});

test('filterByScope: favorites returns only favorited ids', () => {
  const rows = [kbx('fav1'), kbx('fav2'), kbx('nofav')] as never[];
  const result = filterByScope(rows, 'favorites', 'me', new Set(['fav1']));
  assert.deepEqual(result.map((r: { id: string }) => r.id), ['fav1']);
});

test('filterByScope: recents returns only recents ids', () => {
  const rows = [kbx('r1'), kbx('r2')] as never[];
  const result = filterByScope(rows, 'recents', 'me', new Set(), new Set(['r2']));
  assert.deepEqual(result.map((r: { id: string }) => r.id), ['r2']);
});
