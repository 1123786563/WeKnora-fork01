import assert from 'node:assert/strict';
import test from 'node:test';

import { classifyKnowledgeBaseMetadataError, computeKBPermissions, kbTypeRedirectPath, resolveKBSurfaceTabs } from './permissions.ts';

test('viewer role hides editing controls for a shared KB', () => {
  const kb = { id: 'kb-1', user_id: 'someone-else' };
  const viewer = { user: { id: 'u-2' }, memberships: [{ role: 'viewer' }] };
  assert.deepEqual(computeKBPermissions(kb, viewer), { canContribute: false, viewerOnly: true });
});

test('creator, system admin and contributor roles keep editing', () => {
  const kb = { id: 'kb-1', user_id: 'u-1' };
  assert.equal(computeKBPermissions(kb, { user: { id: 'u-1' } }).canContribute, true);
  assert.equal(computeKBPermissions(kb, { user: { id: 'u-9', role: 'system_admin' } }).canContribute, true);
  assert.equal(computeKBPermissions(kb, { user: { id: 'u-9', role: 'admin' } }).canContribute, true);
});

test('tenant admin and contributor memberships can use an independently opened home-tenant KB', () => {
  assert.deepEqual(computeKBPermissions(
    { id: 'kb-1', user_id: 'owner-1' },
    { user: { id: 'u-9' }, memberships: [{ role: 'contributor' }] },
  ), { canContribute: true, viewerOnly: false });
  assert.deepEqual(computeKBPermissions(
    { id: 'kb-1', user_id: 'owner-1' },
    { user: { id: 'u-9' }, memberships: [{ role: 'admin' }] },
  ), { canContribute: true, viewerOnly: false });
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
  assert.deepEqual(computeKBPermissions({ id: 'kb-1' }, null), { canContribute: false, viewerOnly: true });
  assert.deepEqual(computeKBPermissions({ id: 'kb-1' }, { user: { id: 'u-1' } }), { canContribute: false, viewerOnly: true });
  assert.deepEqual(computeKBPermissions({ id: 'kb-1' }, { user: { id: 'u-1' }, memberships: [] }), { canContribute: false, viewerOnly: true });
  assert.deepEqual(computeKBPermissions({ id: 'kb-1' }, { user: { id: 'u-1' }, memberships: [{ role: 'unknown' }] }), { canContribute: false, viewerOnly: true });
});

test('faq-type KBs redirect to the FAQ route', () => {
  assert.equal(kbTypeRedirectPath({ id: 'kb-1', type: 'faq' }), '/knowledgeBase/kb-1/faq');
  assert.equal(kbTypeRedirectPath({ id: 'kb-1', type: 'FAQ' }), '/knowledgeBase/kb-1/faq');
  assert.equal(kbTypeRedirectPath({ id: 'kb-1', type: 'document' }), undefined);
  assert.equal(kbTypeRedirectPath({ type: 'faq' }), undefined);
});

test('wiki and graph tabs follow the indexing strategy', () => {
  assert.deepEqual(resolveKBSurfaceTabs({ id: 'kb-1' }), ['documents']);
  assert.deepEqual(resolveKBSurfaceTabs({ id: 'kb-1', indexing_strategy: { wiki_enabled: true } }), ['documents', 'wiki']);
  assert.deepEqual(resolveKBSurfaceTabs({ id: 'kb-1', indexing_strategy: { wiki_enabled: true, graph_enabled: true } }), ['documents', 'wiki', 'graph']);
  assert.deepEqual(resolveKBSurfaceTabs({ id: 'kb-1', indexing_strategy: { wiki_enabled: false, graph_enabled: true } }), ['documents', 'graph']);
});
