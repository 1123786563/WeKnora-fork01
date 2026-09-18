import assert from 'node:assert/strict';
import test from 'node:test';
import { resolveDesktopPersonalNode } from './bootstrap.ts';

test('composes the desktop personal node after a delayed Wails API binding', async () => {
  let apiReads = 0;
  const values = new Map([
    ['weknora.desktop.access-token', 'access-token'],
    ['weknora.desktop.personal-node-credential', 'node-credential'],
  ]);
  const app = {
    GetAPIBaseURL: async () => { apiReads += 1; if (apiReads < 2) throw new Error('late'); return 'http://127.0.0.1:4321/api/v1'; },
    GetPaseoURL: async () => 'https://paseo.example.test',
    GetPaseoAllowedOrigins: async () => ['https://paseo.example.test'],
    GetCredential: (key: string) => values.get(key) ?? null,
    DeleteCredential: (key: string) => { values.delete(key); },
  };
  const node = await resolveDesktopPersonalNode(app, () => undefined, { attempts: 2, delayMs: 0 });
  assert.ok(node);
  assert.equal(apiReads, 2);
  assert.equal(node?.transport.baseURL, 'https://paseo.example.test');
  assert.equal(node?.transport.allowedOrigins[0], 'https://paseo.example.test');
});

test('observes a Wails bridge installed after module evaluation', async () => {
  let bridge: Record<string, unknown> = {};
  let reads = 0;
  const values = new Map([
    ['weknora.desktop.access-token', 'access-token'],
    ['weknora.desktop.personal-node-credential', 'node-credential'],
  ]);
  const installed = {
    GetAPIBaseURL: () => 'http://127.0.0.1:4321/api/v1',
    GetPaseoURL: () => 'https://paseo.example.test',
    GetPaseoAllowedOrigins: () => ['https://paseo.example.test'],
    GetCredential: (key: string) => values.get(key) ?? null,
    DeleteCredential: (key: string) => { values.delete(key); },
  };
  const node = await resolveDesktopPersonalNode(() => {
    reads += 1;
    if (reads >= 2) bridge = installed;
    return bridge;
  }, () => undefined, { attempts: 3, delayMs: 0 });
  assert.ok(node);
});

test('keeps access and personal-node credentials isolated', async () => {
  const reads: string[] = [];
  const node = await resolveDesktopPersonalNode({
    GetAPIBaseURL: () => 'http://127.0.0.1:4321/api/v1',
    GetPaseoURL: () => 'https://paseo.example.test',
    GetPaseoAllowedOrigins: () => ['https://paseo.example.test'],
    GetCredential: (key: string) => { reads.push(key); return key + '-value'; },
    DeleteCredential: () => undefined,
  }, () => undefined, { attempts: 1, delayMs: 0 });
  assert.ok(node);
  assert.deepEqual(reads, ['weknora.desktop.access-token', 'weknora.desktop.personal-node-credential']);
  assert.equal(node?.credentialBridge?.readCredential?.('weknora.desktop.personal-node-credential'), 'weknora.desktop.personal-node-credential-value');
  assert.equal(node?.credentialBridge?.readCredential?.('weknora.desktop.access-token'), 'weknora.desktop.access-token-value');
});

test('fails closed when the Wails bridge has no credential or trusted Paseo policy', async () => {
  const app = {
    GetAPIBaseURL: () => 'http://127.0.0.1:4321/api/v1',
    GetPaseoURL: () => 'https://paseo.example.test',
    GetPaseoAllowedOrigins: () => ['https://paseo.example.test'],
    GetCredential: () => null,
  };
  assert.equal(await resolveDesktopPersonalNode(app, () => undefined, { attempts: 1, delayMs: 0 }), undefined);
  assert.equal(await resolveDesktopPersonalNode({ GetAPIBaseURL: () => 'http://127.0.0.1:4321/api/v1' }, () => undefined, { attempts: 1, delayMs: 0 }), undefined);
});

test('revoke clears the cached bridge credential and calls Wails DeleteCredential', async () => {
  const deleted: string[] = [];
  const values = new Map([
    ['weknora.desktop.access-token', 'access-token'],
    ['weknora.desktop.personal-node-credential', 'node-credential'],
  ]);
  const node = await resolveDesktopPersonalNode({
    GetAPIBaseURL: () => 'http://127.0.0.1:4321/api/v1',
    GetPaseoURL: () => 'https://paseo.example.test',
    GetPaseoAllowedOrigins: () => ['https://paseo.example.test'],
    GetCredential: (key: string) => values.get(key) ?? null,
    DeleteCredential: (key: string) => { deleted.push(key); values.delete(key); },
  }, () => undefined, { attempts: 1, delayMs: 0 });
  assert.ok(node);
  assert.equal(typeof node?.credentialBridge?.removeCredential, 'function');
  node?.credentialBridge?.removeCredential?.('weknora.desktop.personal-node-credential');
  await Promise.resolve();
  assert.deepEqual(deleted, ['weknora.desktop.personal-node-credential']);
  assert.equal(values.has('weknora.desktop.personal-node-credential'), false);
});
