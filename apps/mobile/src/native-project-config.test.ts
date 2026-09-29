import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';

const packageJson = JSON.parse(
  readFileSync(join(import.meta.dirname, '../package.json'), 'utf8'),
) as { dependencies: Record<string, string> };
const appJson = JSON.parse(
  readFileSync(join(import.meta.dirname, '../app.json'), 'utf8'),
) as {
  expo: {
    ios?: { deploymentTarget?: string };
    plugins: Array<string | [string, Record<string, unknown>]>;
  };
};

test('Expo app config requests SDK 57 scene generation and iOS 16.4 minimum (config contract)', () => {
  const expoVersion = packageJson.dependencies.expo;
  assert.ok(expoVersion, 'Expo must be a direct mobile dependency');
  const sdkVersion = expoVersion.match(/(\d+)\.(\d+)\.(\d+)/);
  assert.ok(sdkVersion, `Expo version must be pinned, received ${expoVersion}`);
  assert.equal(Number(sdkVersion[1]), 57, 'Expo major must be 57');
  assert.equal(Number(sdkVersion[2]), 0, 'Expo minor must be 0');
  assert.ok(Number(sdkVersion[3]) >= 23, 'Expo patch must be at least 23');

  assert.ok(packageJson.dependencies['expo-build-properties']);
  const buildPropertiesPlugin = appJson.expo.plugins.find(
    (plugin): plugin is [string, Record<string, unknown>] =>
      Array.isArray(plugin) && plugin[0] === 'expo-build-properties',
  );
  assert.ok(buildPropertiesPlugin, 'expo-build-properties plugin must be configured');
  assert.deepEqual(buildPropertiesPlugin[1].ios, { enableSceneSupport: true });
  assert.equal(appJson.expo.ios?.deploymentTarget, '16.4');
});
