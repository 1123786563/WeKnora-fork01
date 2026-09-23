import test from 'node:test';
import assert from 'node:assert/strict';
import {
  connectionCapability,
  knowledgeRefForPrompt,
  revokedKnowledgeProjection,
  scanStatusPresentation,
  toConnectionResource,
  toKnowledgeResource,
} from './resource-presentation.ts';

test('toKnowledgeResource maps wire rows and reports unknown scan status as pending', () => {
  assert.deepEqual(
    toKnowledgeResource({ id: 'kb-1', title: 'Handbook', scan_status: 'indexed', document_count: 3, updated_at: '2026-09-01T00:00:00Z' }),
    { id: 'kb-1', title: 'Handbook', scanStatus: 'indexed', documentCount: 3, updatedAt: '2026-09-01T00:00:00Z' },
  );
  // 未知/缺失扫描状态按 pending 展示——不臆造 indexed（冻结规则）。
  assert.deepEqual(
    toKnowledgeResource({ id: 'kb-2', name: 'Shared', documentCount: 0 }),
    { id: 'kb-2', title: 'Shared', scanStatus: 'pending', documentCount: 0, updatedAt: '' },
  );
});

test('revocation hides sensitive knowledge fields and blocks ask-with-knowledge', () => {
  assert.deepEqual(revokedKnowledgeProjection(), { visibleSensitiveFields: [], canAskWithKnowledge: false });
  const resource = toKnowledgeResource({ id: 'kb-1', title: 'Handbook' });
  assert.deepEqual(knowledgeRefForPrompt(resource, { revoked: true }), null);
  assert.deepEqual(knowledgeRefForPrompt(resource, { revoked: false }), { kind: 'knowledge_ref', knowledgeId: 'kb-1' });
});

test('scan status presentation carries label and tone on separate channels', () => {
  assert.deepEqual(scanStatusPresentation('indexed'), { label: '已索引', tone: 'brand' });
  assert.deepEqual(scanStatusPresentation('scanning'), { label: '扫描中', tone: 'warning' });
  assert.deepEqual(scanStatusPresentation('failed'), { label: '扫描失败', tone: 'danger' });
  assert.deepEqual(scanStatusPresentation('pending'), { label: '待扫描', tone: 'neutral' });
});

test('connection capability explains supported, unavailable and never guesses unknown states', () => {
  assert.deepEqual(connectionCapability('active'), { state: 'supported', reason: '' });
  assert.deepEqual(connectionCapability('revoked'), { state: 'unavailable', reason: 'connection_revoked' });
  assert.deepEqual(connectionCapability('pending_reauthorization'), { state: 'unavailable', reason: 'reauthorization_required' });
  assert.deepEqual(connectionCapability('unknown'), { state: 'unavailable', reason: 'connection_state_not_reported' });
});

test('toConnectionResource projects wire rows without credentials or fabricated names', () => {
  assert.deepEqual(
    toConnectionResource({ id: 'conn-1', kind: 'personal', state: 'active', owner_id: 'member-1', auth_version: 4 }),
    { id: 'conn-1', kind: 'personal', state: 'active', connected: true, capability: { state: 'supported', reason: '' } },
  );
  assert.deepEqual(
    toConnectionResource({ id: 'conn-2', kind: 'space', state: 'revoked' }),
    { id: 'conn-2', kind: 'space', state: 'revoked', connected: false, capability: { state: 'unavailable', reason: 'connection_revoked' } },
  );
  assert.deepEqual(
    toConnectionResource({ id: 'conn-3', kind: 'personal' }),
    { id: 'conn-3', kind: 'personal', state: 'unknown', connected: false, capability: { state: 'unavailable', reason: 'connection_state_not_reported' } },
  );
  const projected = toConnectionResource({ id: 'conn-4', kind: 'space', state: 'active', access_token: 'SECRET', credential_ref: 'SECRET-REF' });
  assert.equal(JSON.stringify(projected).includes('SECRET'), false, 'credential-looking fields must not survive projection');
});

test('unknown connection kind maps to unknown, not silently personal; blank ids are filtered by the shelf', () => {
  assert.equal(toConnectionResource({ id: 'c1', kind: 'weird' }).kind, 'unknown');
});
