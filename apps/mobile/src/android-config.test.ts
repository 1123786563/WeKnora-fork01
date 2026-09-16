import assert from 'node:assert/strict';
import test from 'node:test';
import { applyAndroidCleartextTraffic } from './android-config.ts';

test('Android manifest allows the HTTP(S) server addresses accepted by the mobile transport', () => {
  const manifest = {
    manifest: {
      $: { 'xmlns:android': 'http://schemas.android.com/apk/res/android' },
      queries: [],
      application: [{ $: { 'android:name': '.MainApplication' } }],
    },
  } satisfies {
    manifest: {
      $?: Record<string, string | undefined>;
      queries: unknown[];
      application?: Array<{ $?: Record<string, string | undefined> }>;
    };
  };
  applyAndroidCleartextTraffic(manifest);
  const applicationAttributes = manifest.manifest.application?.[0].$ as Record<string, string | undefined>;
  assert.equal(applicationAttributes?.['android:usesCleartextTraffic'], 'true');
});
