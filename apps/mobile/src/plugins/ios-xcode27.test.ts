import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { verifyIosSceneProject } from '../../scripts/verify-ios-scene-project.js';

const fixture = (root: string) => {
  mkdirSync(join(root, 'WeKnora'), { recursive: true });
  mkdirSync(join(root, 'WeKnora.xcodeproj'), { recursive: true });
  writeFileSync(join(root, 'WeKnora', 'Info.plist'), `<?xml version="1.0"?><plist><dict><key>UIApplicationSceneManifest</key><dict><key>UISceneConfigurations</key><dict><key>UIWindowSceneSessionRoleApplication</key><array><dict><key>UISceneDelegateClassName</key><string>EXExpoAppSceneDelegate</string></dict></array></dict></dict></dict></plist>`);
  writeFileSync(join(root, 'WeKnora', 'AppDelegate.swift'), `class AppDelegate: ExpoAppDelegate, ExpoReactNativeFactoryProvider {
    override func application(_ app: UIApplication, open url: URL, options: [UIApplication.OpenURLOptionsKey: Any] = [:]) -> Bool {
      return super.application(app, open: url, options: options) || RCTLinkingManager.application(app, open: url, options: options)
    }
    override func application(_ application: UIApplication, continue userActivity: NSUserActivity, restorationHandler: @escaping ([UIUserActivityRestoring]?) -> Void) -> Bool {
      let result = RCTLinkingManager.application(application, continue: userActivity, restorationHandler: restorationHandler)
      return super.application(application, continue: userActivity, restorationHandler: restorationHandler) || result
    }
  }`);
  writeFileSync(join(root, 'WeKnora.xcodeproj', 'project.pbxproj'), `/* Begin PBXNativeTarget section */
A1 /* WeKnora */ = {
  buildConfigurationList = B1;
};
/* End PBXNativeTarget section */
/* Begin XCConfigurationList section */
B1 = { buildConfigurations = ( C1 /* Debug */, C2 /* Release */, C3 /* Staging */, ); };
/* End XCConfigurationList section */
/* Begin XCBuildConfiguration section */
C1 /* Debug */ = { IPHONEOS_DEPLOYMENT_TARGET = 16.4; };
C2 /* Release */ = { IPHONEOS_DEPLOYMENT_TARGET = 16.4; };
C3 /* Staging */ = { IPHONEOS_DEPLOYMENT_TARGET = 16.4; };
/* unrelated Pods setting */ IPHONEOS_DEPLOYMENT_TARGET = 12.0;
/* End XCBuildConfiguration section */`);
  writeFileSync(join(root, 'Podfile.properties.json'), JSON.stringify({ 'ios.deploymentTarget': '16.4' }));
};

test('generated SDK57 project contract accepts scene, factory, URL and deployment settings', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    assert.deepEqual(verifyIosSceneProject(root), []);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 project contract rejects broken effective scene, callback, or app configuration relationships', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const plist = join(root, 'WeKnora', 'Info.plist');
    writeFileSync(plist, readFileSync(plist, 'utf8').replace('<string>EXExpoAppSceneDelegate</string>', '<string>WrongDelegate</string>'));
    assert.match(verifyIosSceneProject(root).join('\n'), /application scene role/);

    fixture(root);
    const appDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
    const source = readFileSync(appDelegate, 'utf8');
    writeFileSync(appDelegate, source.replace('RCTLinkingManager.application(app, open: url, options: options)', 'OtherLinker.application(app, open: url, options: options)'));
    assert.match(verifyIosSceneProject(root).join('\n'), /open-URL callback/);

    fixture(root);
    const project = join(root, 'WeKnora.xcodeproj', 'project.pbxproj');
    writeFileSync(project, readFileSync(project, 'utf8').replace('IPHONEOS_DEPLOYMENT_TARGET = 16.4;', 'IPHONEOS_DEPLOYMENT_TARGET = 16.0;'));
    assert.match(verifyIosSceneProject(root).join('\n'), /All app target deployment settings/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 contract checks unannotated app configs and callback-specific relationships', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const project = join(root, 'WeKnora.xcodeproj', 'project.pbxproj');
    let source = readFileSync(project, 'utf8');
    source = source.replace('C1 /* Debug */, C2 /* Release */, C3 /* Staging */,', 'C1 /* Debug */, C2 /* Release */, C3,');
    source = source.replace('C3 /* Staging */ = { IPHONEOS_DEPLOYMENT_TARGET = 16.4;', 'C3 /* Staging */ = { IPHONEOS_DEPLOYMENT_TARGET = 16.0;');
    writeFileSync(project, source);
    assert.match(verifyIosSceneProject(root).join('\n'), /All app target deployment settings/);

    fixture(root);
    const plist = join(root, 'WeKnora', 'Info.plist');
    writeFileSync(plist, readFileSync(plist, 'utf8').replace('<key>UIWindowSceneSessionRoleApplication</key><array><dict><key>UISceneDelegateClassName</key><string>EXExpoAppSceneDelegate</string></dict></array>', '<key>UIWindowSceneSessionRoleApplication</key><array><dict><key>UISceneDelegateClassName</key><string>WrongDelegate</string></dict></array><key>OtherRole</key><array><dict><key>UISceneDelegateClassName</key><string>EXExpoAppSceneDelegate</string></dict></array>'));
    assert.match(verifyIosSceneProject(root).join('\n'), /application scene role/);

    fixture(root);
    const appDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
    const delegate = readFileSync(appDelegate, 'utf8').replace('RCTLinkingManager.application(application, continue:', 'OtherLinker.application(application, continue:');
    writeFileSync(appDelegate, delegate);
    assert.match(verifyIosSceneProject(root).join('\n'), /universal-link callback/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 project contract rejects missing scene, URL callback, and deployment target drift', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const scene = join(root, 'WeKnora', 'Info.plist');
    writeFileSync(scene, readFileSync(scene, 'utf8').replace('EXExpoAppSceneDelegate', 'OldSceneDelegate'));
    assert.match(verifyIosSceneProject(root).join('\n'), /application scene role/);
    fixture(root);
    const appDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
    writeFileSync(appDelegate, readFileSync(appDelegate, 'utf8').replace('RCTLinkingManager', 'OtherLinkManager'));
    assert.match(verifyIosSceneProject(root).join('\n'), /RCTLinkingManager/);
    fixture(root);
    writeFileSync(join(root, 'WeKnora.xcodeproj', 'project.pbxproj'), 'IPHONEOS_DEPLOYMENT_TARGET = 16.0;');
    assert.match(verifyIosSceneProject(root).join('\n'), /16.4/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});
