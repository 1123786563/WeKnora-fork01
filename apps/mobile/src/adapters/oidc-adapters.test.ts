import test from 'node:test';
import assert from 'node:assert/strict';
import { createOidcBrowser } from './oidc-browser.ts';
import { createSecurePendingOidcStore } from './secure-store.ts';

test('secure pending OIDC storage consumes the verifier once', async () => {
  const values = new Map<string, string>();
  const secureStore = {
    async getItemAsync(key: string) { return values.get(key) ?? null; },
    async setItemAsync(key: string, value: string) { values.set(key, value); },
    async deleteItemAsync(key: string) { values.delete(key); },
  };
  const pending = createSecurePendingOidcStore(secureStore);

  await pending.savePending({ deploymentOrigin: 'https://weknora.example.test', state: 'state-1', codeVerifier: 'verifier-1', redirectUri: 'weknora://oidc' });

  assert.deepEqual(await pending.consumePending(), {
    deploymentOrigin: 'https://weknora.example.test', state: 'state-1', codeVerifier: 'verifier-1', redirectUri: 'weknora://oidc',
  });
  assert.equal(await pending.consumePending(), undefined);
});

test('native browser returns only the registered callback URL', async () => {
  const browser = createOidcBrowser('weknora://oidc', {
    async openAuthSessionAsync() { return { type: 'success', url: 'weknora://oidc?code=code-1&state=state-1' }; },
  });

  assert.equal(await browser.open('https://idp.example.test/authorize'), 'weknora://oidc?code=code-1&state=state-1');
});

test('native browser rejects a redirect outside the registered scheme host and path', async () => {
  const browser = createOidcBrowser('weknora://oidc', {
    async openAuthSessionAsync() { return { type: 'success', url: 'weknora://other?code=code-1&state=state-1' }; },
  });

  await assert.rejects(browser.open('https://idp.example.test/authorize'), /OIDC_CALLBACK/);
});
