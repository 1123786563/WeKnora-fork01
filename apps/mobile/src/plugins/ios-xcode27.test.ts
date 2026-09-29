import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { verifyIosSceneProject } from '../../scripts/verify-ios-scene-project.js';

const fixture = (root: string) => {
  mkdirSync(join(root, 'WeKnora'), { recursive: true });
  mkdirSync(join(root, 'WeKnora.xcodeproj'), { recursive: true });
  writeFileSync(join(root, 'WeKnora', 'Info.plist'), `<?xml version="1.0"?><plist><dict><key>UIApplicationSceneManifest</key><dict><key>UISceneConfigurations</key><dict><key>UIApplicationSceneConfigurationName</key><array><dict><key>UISceneDelegateClassName</key><string>$(PRODUCT_MODULE_NAME).EXExpoAppSceneDelegate</string></dict></array></dict></dict></dict></plist>`);
  writeFileSync(join(root, 'WeKnora', 'AppDelegate.swift'), `class AppDelegate: ExpoAppDelegate, ExpoReactNativeFactoryProvider { func application(_ app: UIApplication, open url: URL, options: [UIApplication.OpenURLOptionsKey: Any]) -> Bool { RCTLinkingManager.application(app, open: url, options: options) } }`);
  writeFileSync(join(root, 'WeKnora.xcodeproj', 'project.pbxproj'), `IPHONEOS_DEPLOYMENT_TARGET = 16.4;`);
  writeFileSync(join(root, 'Podfile.properties.json'), JSON.stringify({ 'ios.deploymentTarget': '16.4' }));
};

test('generated SDK57 project contract accepts scene, factory, URL and deployment settings', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    assert.deepEqual(verifyIosSceneProject(root), []);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 project contract rejects missing scene, URL callback, and deployment target drift', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const scene = join(root, 'WeKnora', 'Info.plist');
    writeFileSync(scene, readFileSync(scene, 'utf8').replace('EXExpoAppSceneDelegate', 'OldSceneDelegate'));
    assert.match(verifyIosSceneProject(root).join('\n'), /EXExpoAppSceneDelegate/);
    fixture(root);
    const appDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
    writeFileSync(appDelegate, readFileSync(appDelegate, 'utf8').replace('RCTLinkingManager', 'OtherLinkManager'));
    assert.match(verifyIosSceneProject(root).join('\n'), /RCTLinkingManager/);
    fixture(root);
    writeFileSync(join(root, 'WeKnora.xcodeproj', 'project.pbxproj'), 'IPHONEOS_DEPLOYMENT_TARGET = 16.0;');
    assert.match(verifyIosSceneProject(root).join('\n'), /16.4/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});
