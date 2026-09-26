import assert from 'node:assert/strict';
import test from 'node:test';

import { classifyKnowledgeBaseMetadataError, computeKBPermissions, kbTypeRedirectPath, kbWikiTabFallbackPath, resolveKBSurfaceTabs } from './permissions.ts';

test('viewer role hides editing controls for a shared KB', () => {
  const kb = { id: 'kb-1', user_id: 'someone-else' };
  const viewer = { user: { id: 'u-2' }, memberships: [{ role: 'viewer' }] };
  assert.deepEqual(computeKBPermissions(kb, viewer), { canContribute: false, canManage: false, viewerOnly: true });
});

test('creator, system admin and contributor roles keep editing', () => {
  const kb = { id: 'kb-1', user_id: 'u-1' };
  assert.equal(computeKBPermissions(kb, { user: { id: 'u-1' } }).canContribute, true);
  assert.equal(computeKBPermissions(kb, { user: { id: 'u-9', role: 'system_admin' } }).canContribute, true);
  assert.equal(computeKBPermissions(kb, { user: { id: 'u-9', role: 'admin' } }).canContribute, true);
});

// The live backend KB payload carries ONLY creator_id (R461 browser evidence;
// internal/types/knowledgebase.go) — isCreator must match it exactly like
// Vue's isOwner (KnowledgeBase.vue:250-258). user_id/created_by never arrive
// from this backend, so without creator_id the creator's edit/upload controls
// vanish (the wiki edit-button gap root cause).
test('creator_id (the only field the live backend sends) grants editing', () => {
  const kb = { id: 'kb-1', creator_id: 'u-1' };
  assert.equal(computeKBPermissions(kb, { user: { id: 'u-1' } }).canContribute, true, 'creator_id match must count as creator');
  assert.equal(computeKBPermissions({ id: 'kb-2', creator_id: 'someone-else' }, { user: { id: 'u-1' } }).canContribute, false, 'creator_id mismatch stays non-contributor for a plain member');
});

test('tenant admin and contributor memberships can use an independently opened home-tenant KB', () => {
  assert.deepEqual(computeKBPermissions(
    { id: 'kb-1', user_id: 'owner-1' },
    { user: { id: 'u-9' }, memberships: [{ role: 'contributor' }] },
  ), { canContribute: true, canManage: false, viewerOnly: false });
  assert.deepEqual(computeKBPermissions(
    { id: 'kb-1', user_id: 'owner-1' },
    { user: { id: 'u-9' }, memberships: [{ role: 'admin' }] },
  ), { canContribute: true, canManage: false, viewerOnly: false });
});

test('canManage follows Vue canManageKB: owner/admin only, never every contributor (OCR R1-17)', () => {
  // editor 可贡献但不可管理——canContribute 误用作 canManage 是授权回归根因。
  const editor = computeKBPermissions({ id: 'kb-1', my_permission: 'editor' }, { user: { id: 'u-2' } });
  assert.equal(editor.canContribute, true);
  assert.equal(editor.canManage, false, 'editor contributes but cannot manage');
  // 显式 KB owner/admin、creator、system admin 才可管理。
  assert.equal(computeKBPermissions({ id: 'kb-1', my_permission: 'admin' }, { user: { id: 'u-2' } }).canManage, true);
  assert.equal(computeKBPermissions({ id: 'kb-1', my_permission: 'owner' }, { user: { id: 'u-2' } }).canManage, true);
  assert.equal(computeKBPermissions({ id: 'kb-1', creator_id: 'u-1' }, { user: { id: 'u-1' } }).canManage, true);
  assert.equal(computeKBPermissions({ id: 'kb-1' }, { user: { id: 'u-9', role: 'system_admin' } }).canManage, true);
  // viewer / 普通 contributor membership 不可管理。
  assert.equal(computeKBPermissions({ id: 'kb-1', my_permission: 'viewer' }, { user: { id: 'u-2' } }).canManage, false);
  const member = computeKBPermissions({ id: 'kb-1' }, { user: { id: 'u-2' }, memberships: [{ role: 'contributor' }] });
  assert.equal(member.canContribute, true);
  assert.equal(member.canManage, false, 'tenant contributor membership is write access, not KB management');
});

test('classifies KB metadata 403 separately from other metadata failures', () => {
  assert.deepEqual(classifyKnowledgeBaseMetadataError({ status: 403, message: 'forbidden' }), {
    kind: 'forbidden',
    message: 'forbidden',
  });
  assert.deepEqual(classifyKnowledgeBaseMetadataError(new Error('network unavailable')), {
    kind: 'error',
    message: 'network unavailable',
  });
});

test('missing or incomplete capability evidence fails closed', () => {
  assert.deepEqual(computeKBPermissions({ id: 'kb-1' }, null), { canContribute: false, canManage: false, viewerOnly: true });
  assert.deepEqual(computeKBPermissions({ id: 'kb-1' }, { user: { id: 'u-1' } }), { canContribute: false, canManage: false, viewerOnly: true });
  assert.deepEqual(computeKBPermissions({ id: 'kb-1' }, { user: { id: 'u-1' }, memberships: [] }), { canContribute: false, canManage: false, viewerOnly: true });
  assert.deepEqual(computeKBPermissions({ id: 'kb-1' }, { user: { id: 'u-1' }, memberships: [{ role: 'unknown' }] }), { canContribute: false, canManage: false, viewerOnly: true });
});

test('faq-type KBs redirect to the FAQ route', () => {
  assert.equal(kbTypeRedirectPath({ id: 'kb-1', type: 'faq' }), '/knowledgeBase/kb-1/faq');
  assert.equal(kbTypeRedirectPath({ id: 'kb-1', type: 'FAQ' }), '/knowledgeBase/kb-1/faq');
  assert.equal(kbTypeRedirectPath({ id: 'kb-1', type: 'document' }), undefined);
  assert.equal(kbTypeRedirectPath({ type: 'faq' }), undefined);
});

test('the tab row exists only for wiki KBs (Vue isWiki gate, KnowledgeBase.vue:89/2359-2381)', () => {
  // No wiki → no tabs at all; the caller falls back to the plain 文档 crumb.
  assert.deepEqual(resolveKBSurfaceTabs({ id: 'kb-1' }), []);
  assert.deepEqual(resolveKBSurfaceTabs({ id: 'kb-1', indexing_strategy: { wiki_enabled: false, graph_enabled: false } }), []);
  // Wiki off but graph on still gets no tab row: the Vue graph view lives
  // inside the wiki surface (WikiBrowser) and is never gated separately.
  assert.deepEqual(resolveKBSurfaceTabs({ id: 'kb-1', indexing_strategy: { wiki_enabled: false, graph_enabled: true } }), []);
  // A wiki KB always shows the three tabs — graph is not independently gated.
  assert.deepEqual(resolveKBSurfaceTabs({ id: 'kb-1', indexing_strategy: { wiki_enabled: true } }), ['documents', 'wiki', 'graph']);
  assert.deepEqual(resolveKBSurfaceTabs({ id: 'kb-1', indexing_strategy: { wiki_enabled: true, graph_enabled: true } }), ['documents', 'wiki', 'graph']);
});

test('non-wiki ?tab deep links fall back to the canonical documents URL', () => {
  // Vue keeps the ?tab=graph URL but renders the documents branch when isWiki
  // is false (KnowledgeBase.vue:2412/2418); the React pages are separate
  // routes, so the deep link redirects to the documents URL instead.
  assert.equal(kbWikiTabFallbackPath({ id: 'kb-1', indexing_strategy: { wiki_enabled: false, graph_enabled: true } }), '/knowledgeBase/kb-1');
  assert.equal(kbWikiTabFallbackPath({ id: 'kb-1', indexing_strategy: { wiki_enabled: false } }), '/knowledgeBase/kb-1');
  assert.equal(kbWikiTabFallbackPath({ id: 'kb-1', indexing_strategy: { wiki_enabled: true } }), undefined);
  assert.equal(kbWikiTabFallbackPath({ id: 'kb-1', indexing_strategy: { wiki_enabled: true, graph_enabled: true } }), undefined);
  assert.equal(kbWikiTabFallbackPath({ indexing_strategy: { wiki_enabled: false } }), undefined);
});
