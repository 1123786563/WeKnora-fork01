import assert from 'node:assert/strict';
import test from 'node:test';
import { isExternalHttpUrl, normalizeLegacyPath } from './navigation.ts';
import { resolveDesktopApiBaseUrl, readWailsBridge, isWailsWebView } from './wails.ts';

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
});
