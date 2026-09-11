import test from 'node:test';
import assert from 'node:assert/strict';
import {
  parseConnectionView,
  parseInstallationView,
  parseSyncStatusView,
} from '../src/appconnector.ts';

test('parseConnectionView round-trips personal and space connections', () => {
  const personal = parseConnectionView({ id: 'c1', kind: 'personal', state: 'active', owner_id: 'u1' });
  assert.equal(personal.kind, 'personal');
  const space = parseConnectionView({ id: 'c2', kind: 'space', state: 'pending_reauthorization', owner_id: null });
  assert.equal(space.kind, 'space');
  assert.equal(space.owner_id, null);
});

test('parseConnectionView rejects non-object, bad kind, and bad owner_id', () => {
  assert.throws(() => parseConnectionView('nope'));
  assert.throws(() => parseConnectionView({ id: 'c', kind: 'team', state: 'active', owner_id: null }));
  assert.throws(() => parseConnectionView({ id: 'c', kind: 'personal', state: 'active', owner_id: 42 }));
  assert.throws(() => parseConnectionView({ kind: 'personal', state: 'active', owner_id: null }));
});

test('parseInstallationView projects scope lists and rejects malformed scopes', () => {
  const v = parseInstallationView({ id: 'i1', app_key: 'feishu', version: '1.2.0', scopes: ['doc:read', 'drive:read'] });
  assert.deepEqual(v.scopes, ['doc:read', 'drive:read']);
  assert.throws(() => parseInstallationView({ id: 'i1', app_key: 'feishu', version: '1.2.0', scopes: 'doc:read' }));
  assert.throws(() => parseInstallationView({ id: 'i1', app_key: 'feishu', version: '1.2.0', scopes: ['ok', 7] }));
});

test('installation projection never carries credential fields even if present in the payload', () => {
  const v = parseInstallationView({ id: 'i1', app_key: 'feishu', version: '1', scopes: [], access_token: 'x', credential: 'y' } as Record<string, unknown>);
  const json = JSON.stringify(v);
  assert.equal(json.includes('access_token'), false);
  assert.equal(json.includes('credential'), false);
});

test('parseSyncStatusView handles bound, unbound, and paused sources', () => {
  const bound = parseSyncStatusView({
    datasource_id: 'ds1', state: 'syncing', pause_reason: null,
    binding: { installation_id: 'i1', connection_id: 'c1', auth_version: '3' },
    requires_reauthorization: false,
  });
  assert.equal(bound.binding?.connection_id, 'c1');
  const paused = parseSyncStatusView({
    datasource_id: 'ds2', state: 'paused', pause_reason: 'permission',
    binding: null, requires_reauthorization: true,
  });
  assert.equal(paused.pause_reason, 'permission');
  assert.equal(paused.binding, null);
  assert.equal(paused.requires_reauthorization, true);
  assert.throws(() => parseSyncStatusView({ datasource_id: 'ds', state: 'ok', binding: null, requires_reauthorization: 'yes' }));
});
