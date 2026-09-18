import assert from 'node:assert/strict';
import test from 'node:test';
import { createDesktopAppPersonalNode } from './personalNodeApp.ts';

test('resolves asynchronous Wails Paseo policy methods before composing the desktop node', async () => {
  const node = await createDesktopAppPersonalNode({
    GetPaseoURL: async () => '  https://paseo.example.test/  ',
    GetPaseoAllowedOrigins: async () => ['https://paseo.example.test', '', 'https://paseo.example.test'],
    readCredential: (key) => key === 'access' ? 'access-token' : 'node-token',
  }, {
    apiBaseURL: 'http://127.0.0.1:4321/api/v1',
    accessCredentialKey: 'access',
    credentialKey: 'node',
  });

  assert.ok(node);
  assert.equal(node.transport.baseURL, 'https://paseo.example.test/');
  assert.deepEqual(node.transport.allowedOrigins, [
    'https://paseo.example.test',
    'https://paseo.example.test',
  ]);
  assert.equal(node.credentialKey, 'weknora.desktop.personal-node-credential');
});
