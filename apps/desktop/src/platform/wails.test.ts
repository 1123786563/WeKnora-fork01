import assert from 'node:assert/strict';
import test from 'node:test';
import { isExternalHttpUrl, isSafeDesktopDeepLink, normalizeDesktopLocation, normalizeLegacyPath } from './navigation.ts';
import { resolveDesktopApiBaseUrl, readWailsBridge, isWailsWebView } from './wails.ts';
import { applyDesktopWindowDefaults, createDesktopRuntimeAdapters, DESKTOP_WINDOW_DEFAULTS } from './runtime.ts';
import { createDesktopCredentialStorage } from './credentials.ts';

test('accepts only injected HTTP(S) desktop API roots', () => {
  assert.equal(resolveDesktopApiBaseUrl('http://127.0.0.1:1234/api/v1/'), 'http://127.0.0.1:1234/api/v1');
  assert.equal(resolveDesktopApiBaseUrl('javascript:alert(1)'), '');
  assert.equal(resolveDesktopApiBaseUrl(''), '');
});

test('keeps Wails bridge state explicit and maps old deep links', () => {
  const bridge = readWailsBridge({ GetAPIBaseURL: () => 'http://127.0.0.1:1/api/v1' });
  assert.equal(typeof bridge.GetAPIBaseURL, 'function');
  assert.equal(isWailsWebView({ runtime: true }), true);
  assert.equal(normalizeLegacyPath('/creatChat/agent-1'), '/platform/creatChat/agent-1');
  assert.equal(isExternalHttpUrl('https://example.test/path'), true);
  assert.equal(isExternalHttpUrl('file:///tmp/a'), false);
  assert.equal(normalizeDesktopLocation('/creatChat', '?agent=a', '#composer'), '/platform/creatChat?agent=a#composer');
  assert.equal(isSafeDesktopDeepLink('/platform/settings'), true);
  assert.equal(isSafeDesktopDeepLink('//evil.test'), false);
});

test('applies the fixed Wails desktop viewport contract', () => {
  const calls: number[][] = [];
  applyDesktopWindowDefaults({
    WindowSetMinSize: (...args) => calls.push(args),
    WindowSetSize: (...args) => calls.push(args),
  });
  assert.deepEqual(calls, [
    [DESKTOP_WINDOW_DEFAULTS.minWidth, DESKTOP_WINDOW_DEFAULTS.minHeight],
    [DESKTOP_WINDOW_DEFAULTS.width, DESKTOP_WINDOW_DEFAULTS.height],
  ]);
});

test('routes external URLs through Wails and rejects non-http schemes', () => {
  const opened: string[] = [];
  const adapters = createDesktopRuntimeAdapters({}, { BrowserOpenURL: (url) => opened.push(url) });
  adapters.openExternal('https://example.test');
  adapters.openExternal('file:///tmp/private');
  assert.deepEqual(opened, ['https://example.test']);
});

test('uses the desktop credential bridge with local fallback semantics', () => {
  const values = new Map<string, string>();
  const local = { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => void values.set(key, value), removeItem: (key: string) => void values.delete(key) } as Storage;
  const secure = new Map<string, string>();
  const credentials = createDesktopCredentialStorage(local, {
    readCredential: (key) => secure.get(key) ?? null,
    writeCredential: (key, value) => void secure.set(key, value),
    removeCredential: (key) => void secure.delete(key),
  });
  credentials.write('access', 'secret');
  assert.equal(credentials.read('access'), 'secret');
  credentials.remove('access');
  assert.equal(credentials.read('access'), null);
  assert.equal(values.has('access'), false);
});
