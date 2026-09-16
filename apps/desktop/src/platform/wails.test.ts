import assert from 'node:assert/strict';
import test from 'node:test';
import { isExternalHttpUrl, isSafeDesktopDeepLink, normalizeDesktopLocation, normalizeLegacyPath } from './navigation.ts';
import { resolveDesktopApiBaseUrl, resolveDesktopApiBaseUrlFromBridge, resolveDesktopApiBaseUrlWhenReady, readWailsBridge, isWailsWebView } from './wails.ts';
import { applyDesktopWindowDefaults, createDesktopRuntimeAdapters, DESKTOP_WINDOW_DEFAULTS, installDesktopExternalUrlBridge } from './runtime.ts';
import { createDesktopCredentialStorage } from './credentials.ts';

test('accepts only injected HTTP(S) desktop API roots', () => {
  assert.equal(resolveDesktopApiBaseUrl('http://127.0.0.1:1234/api/v1/'), 'http://127.0.0.1:1234/api/v1');
  assert.equal(resolveDesktopApiBaseUrl('javascript:alert(1)'), '');
  assert.equal(resolveDesktopApiBaseUrl(''), '');
});

test('resolves the async Wails API bridge before the shared renderer imports', async () => {
  assert.equal(await resolveDesktopApiBaseUrlFromBridge({ GetAPIBaseURL: () => Promise.resolve('http://127.0.0.1:4321/api/v1/') }), 'http://127.0.0.1:4321/api/v1');
  assert.equal(await resolveDesktopApiBaseUrlFromBridge({ GetAPIBaseURL: () => Promise.resolve('javascript:alert(1)') }), '');
  assert.equal(await resolveDesktopApiBaseUrlFromBridge({ GetAPIBaseURL: () => Promise.reject(new Error('bridge unavailable')) }), '');
  assert.equal(await resolveDesktopApiBaseUrlFromBridge({}), '');
});

test('waits for a late Wails binding before the renderer boots', async () => {
  let reads = 0;
  const baseUrl = await resolveDesktopApiBaseUrlWhenReady(
    () => {
      reads += 1;
      return reads < 3 ? {} : { GetAPIBaseURL: () => Promise.resolve('http://127.0.0.1:4321/api/v1') };
    },
    () => '',
    { attempts: 3, delayMs: 0 },
  );

  assert.equal(baseUrl, 'http://127.0.0.1:4321/api/v1');
  assert.equal(reads, 3);
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

test('installs an early external URL bridge and leaves internal navigation untouched', () => {
  const opened: string[] = [];
  const internal: string[] = [];
  const target = {
    open: (url: string) => { internal.push(url); return 'browser-window'; },
  } as unknown as Window;
  const restore = installDesktopExternalUrlBridge(target, { BrowserOpenURL: (url) => opened.push(url) });

  assert.equal(target.open?.('https://example.test'), null);
  assert.equal(target.open?.('/platform/settings'), 'browser-window');
  assert.deepEqual(opened, ['https://example.test']);
  assert.deepEqual(internal, ['/platform/settings']);

  restore();
  assert.equal(target.open?.('https://example.test'), 'browser-window');
});

test('uses local storage only when no desktop credential bridge exists', () => {
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

test('does not mirror desktop credentials into local storage or revive stale values', () => {
  const local = {
    getItem: (key: string) => key === 'access' ? 'stale-browser-token' : null,
    setItem: () => { throw new Error('desktop credentials must not use localStorage'); },
    removeItem: () => { throw new Error('desktop credentials must not use localStorage'); },
  } as unknown as Storage;
  const secure = new Map<string, string>();
  const credentials = createDesktopCredentialStorage(local, {
    readCredential: (key) => secure.get(key) ?? null,
    writeCredential: (key, value) => void secure.set(key, value),
    removeCredential: (key) => void secure.delete(key),
  });

  credentials.write('access', 'desktop-token');
  assert.equal(credentials.read('access'), 'desktop-token');
  credentials.remove('access');
  assert.equal(credentials.read('access'), null);
});
