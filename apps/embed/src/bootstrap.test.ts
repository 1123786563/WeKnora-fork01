import assert from 'node:assert/strict';
import test from 'node:test';
import { channelIdFromPath, parentOriginFromReferrer, readStoredSession, writeStoredSession } from './bootstrap.ts';

test('resolves only the dedicated channel route and decodes channel ids', () => {
  assert.equal(channelIdFromPath('/embed/channel%2F1'), 'channel/1');
  assert.equal(channelIdFromPath('/embed/channel-1/'), 'channel-1');
  assert.equal(channelIdFromPath('/platform/embed/channel-1'), '');
});

test('persists only the signed visitor session handle', () => {
  const storage = new Map<string, string>();
  const adapter = {
    getItem: (key: string) => storage.get(key) ?? null,
    setItem: (key: string, value: string) => { storage.set(key, value); },
    removeItem: (key: string) => { storage.delete(key); },
    length: 0,
    clear: () => { storage.clear(); },
    key: () => null,
  } as unknown as Storage;
  writeStoredSession('channel/1', { id: 'session-1', signature: 'sig-1', agentId: 'agent-1' }, adapter);
  assert.deepEqual(readStoredSession('channel/1', adapter), { id: 'session-1', signature: 'sig-1', agentId: 'agent-1' });
  assert.equal(storage.get('weknora-embed-session:channel/1')?.includes('token'), false);
});

test('accepts only a valid http(s) parent referrer origin', () => {
  assert.equal(parentOriginFromReferrer('https://shop.example.test/page'), 'https://shop.example.test');
  assert.equal(parentOriginFromReferrer('null'), '');
  assert.equal(parentOriginFromReferrer('javascript:alert(1)'), '');
});
