import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, unlinkSync, writeFileSync } from 'node:fs';
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
C1 /* Debug */ = { buildSettings = { IPHONEOS_DEPLOYMENT_TARGET = 16.4; }; };
C2 /* Release */ = { buildSettings = { IPHONEOS_DEPLOYMENT_TARGET = 16.4; }; };
C3 /* Staging */ = { buildSettings = { IPHONEOS_DEPLOYMENT_TARGET = 16.4; }; };
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

test('generated SDK57 scene checker rejects nested elements in unrelated plist scalars', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const plistPath = join(root, 'WeKnora', 'Info.plist');
    const source = readFileSync(plistPath, 'utf8');
    writeFileSync(plistPath, source.replace('<key>UIApplicationSceneManifest</key>', '<key>Bad</key><string><true/></string><key>UIApplicationSceneManifest</key>'));
    const messages = verifyIosSceneProject(root).join('\n');
    assert.match(messages, /WeKnora\/Info\.plist is malformed and could not be parsed/);
    assert.doesNotMatch(messages, /application scene role/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 scene checker reports unreliable Swift tokenization without contract violations', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const appDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
    const source = readFileSync(appDelegate, 'utf8');
    writeFileSync(appDelegate, `${source}\nlet damaged = "unterminated`);
    const messages = verifyIosSceneProject(root).join('\n');
    assert.match(messages, /AppDelegate\.swift could not be tokenized reliably/);
    assert.doesNotMatch(messages, /must conform to ExpoReactNativeFactoryProvider|open-URL callback|universal-link callback/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 scene checker distinguishes malformed callback structure from absent callbacks', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const appDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
    const source = readFileSync(appDelegate, 'utf8');
    const malformedSource = source.replace('      return super.application(app, open: url, options: options) || RCTLinkingManager.application(app, open: url, options: options)\n    }', '      return super.application(app, open: url, options: options) || RCTLinkingManager.application(app, open: url, options: options)');
    assert.notEqual(malformedSource, source);
    writeFileSync(appDelegate, malformedSource);
    let messages = verifyIosSceneProject(root).join('\n');
    assert.match(messages, /callback structure/);
    assert.doesNotMatch(messages, /open-URL callback must forward/);
    assert.doesNotMatch(messages, /universal-link callback must forward/);

    fixture(root);
    const absentSource = readFileSync(appDelegate, 'utf8').replace(
      /    override func application\(_ app: UIApplication, open url: URL[^\n]*\n      return[^\n]*\n    }\n/,
      '',
    );
    writeFileSync(appDelegate, absentSource);
    messages = verifyIosSceneProject(root).join('\n');
    assert.match(messages, /open-URL callback must forward/);
    assert.doesNotMatch(messages, /callback structure could not be analyzed/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 scene checker rejects an unclosed final callback before AppDelegate class boundary', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const appDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
    const source = readFileSync(appDelegate, 'utf8');
    const malformedSource = source.replace(
      '      return super.application(application, continue: userActivity, restorationHandler: restorationHandler) || result\n    }\n  }',
      '      return super.application(application, continue: userActivity, restorationHandler: restorationHandler) || result\n  }',
    );
    assert.notEqual(malformedSource, source);
    writeFileSync(appDelegate, malformedSource);
    const messages = verifyIosSceneProject(root).join('\n');
    assert.match(messages, /AppDelegate\.swift callback structure could not be analyzed \(universal-link\)/);
    assert.doesNotMatch(messages, /universal-link callback must forward/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 scene checker ignores matching callbacks outside AppDelegate', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const appDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
    const source = readFileSync(appDelegate, 'utf8');
    const helper = `class HelperDelegate {
    override func application(_ app: UIApplication, open url: URL, options: [UIApplication.OpenURLOptionsKey: Any] = [:]) -> Bool {
      return RCTLinkingManager.application(app, open: url, options: options)
    }
    override func application(_ application: UIApplication, continue userActivity: NSUserActivity, restorationHandler: @escaping ([UIUserActivityRestoring]?) -> Void) -> Bool {
      return RCTLinkingManager.application(application, continue: userActivity, restorationHandler: restorationHandler)
    }
  }
`;
    const withoutCallbacks = source.replace(/    override func application\(_ app[\s\S]*?    \}\n    override func application\(_ application[\s\S]*?    \}\n/, '');
    assert.notEqual(withoutCallbacks, source);
    writeFileSync(appDelegate, `${helper}${withoutCallbacks}`);
    const messages = verifyIosSceneProject(root).join('\n');
    assert.match(messages, /open-URL callback must forward/);
    assert.match(messages, /universal-link callback must forward/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 scene checker ignores callbacks in a nested helper type', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const appDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
    const source = readFileSync(appDelegate, 'utf8');
    const nestedHelper = `    class HelperDelegate: ExpoAppDelegate {
      override func application(_ app: UIApplication, open url: URL, options: [UIApplication.OpenURLOptionsKey: Any] = [:]) -> Bool {
        return RCTLinkingManager.application(app, open: url, options: options)
      }
      override func application(_ application: UIApplication, continue userActivity: NSUserActivity, restorationHandler: @escaping ([UIUserActivityRestoring]?) -> Void) -> Bool {
        return RCTLinkingManager.application(application, continue: userActivity, restorationHandler: restorationHandler)
      }
    }
`;
    const withoutCallbacks = source.replace(/    override func application\(_ app[\s\S]*?    \}\n    override func application\(_ application[\s\S]*?    \}\n/, '');
    assert.notEqual(withoutCallbacks, source);
    const nestedSource = withoutCallbacks.replace('class AppDelegate: ExpoAppDelegate, ExpoReactNativeFactoryProvider {', 'class AppDelegate: ExpoAppDelegate, ExpoReactNativeFactoryProvider {\n' + nestedHelper);
    assert.notEqual(nestedSource, withoutCallbacks);
    writeFileSync(appDelegate, nestedSource);
    const messages = verifyIosSceneProject(root).join('\n');
    assert.match(messages, /open-URL callback must forward/);
    assert.match(messages, /universal-link callback must forward/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 scene checker rejects unknown nested markup in plist scalars', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const plistPath = join(root, 'WeKnora', 'Info.plist');
    const source = readFileSync(plistPath, 'utf8');
    writeFileSync(plistPath, source.replace('<key>UIApplicationSceneManifest</key>', '<key>Bad</key><string><foo/></string><key>UIApplicationSceneManifest</key>'));
    assert.notDeepEqual(verifyIosSceneProject(root), []);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 scene checker parses valid empty and unrelated plist scalar values', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const plistPath = join(root, 'WeKnora', 'Info.plist');
    const source = readFileSync(plistPath, 'utf8');
    const unrelated = '<key>Empty</key><string></string><key>EmptySelfClosing</key><string/><key>Count</key><integer>7</integer><key>Ratio</key><real>1.5</real><key>Created</key><date>2026-09-30T00:00:00Z</date><key>Blob</key><data>AA==</data><key>EmptyDictionary</key><dict/><key>EmptyArray</key><array/>';
    writeFileSync(plistPath, source.replace('<key>UIApplicationSceneManifest</key>', `${unrelated}<key>UIApplicationSceneManifest</key>`));
    assert.deepEqual(verifyIosSceneProject(root), []);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 scene checker distinguishes an unreadable PBX target block from a deployment mismatch', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const projectPath = join(root, 'WeKnora.xcodeproj', 'project.pbxproj');
    writeFileSync(projectPath, '/* readable project file with no target sections */');
    let messages = verifyIosSceneProject(root).join('\n');
    assert.ok(messages.includes('Unable to locate WeKnora build configuration list in project.pbxproj'));
    assert.doesNotMatch(messages, /All app target deployment settings must be 16\.4/);

    fixture(root);
    writeFileSync(projectPath, readFileSync(projectPath, 'utf8').replace(
      'IPHONEOS_DEPLOYMENT_TARGET = 16.4;',
      'IPHONEOS_DEPLOYMENT_TARGET = 16.0;',
    ));
    messages = verifyIosSceneProject(root).join('\n');
    assert.ok(messages.includes('All app target deployment settings must be 16.4'));
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 scene checker suppresses downstream errors for missing generated files', () => {
  const missingCases = [
    ['WeKnora/Info.plist', /application scene role/],
    ['WeKnora/AppDelegate.swift', /must conform|callback must forward/],
    ['WeKnora.xcodeproj/project.pbxproj', /build configuration list|deployment settings/],
    ['Podfile.properties.json', /must be valid JSON with iOS deployment target/],
  ] as const;
  for (const [path, dependentMessage] of missingCases) {
    const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
    try {
      fixture(root);
      unlinkSync(join(root, path));
      const messages = verifyIosSceneProject(root).join('\n');
      assert.ok(messages.includes(`missing generated file: ${path}`));
      assert.doesNotMatch(messages, dependentMessage);
    } finally { rmSync(root, { recursive: true, force: true }); }
  }
});

test('generated SDK57 project contract ignores provider name in Swift comments and strings', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const appDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
    const source = readFileSync(appDelegate, 'utf8');
    const withoutConformance = source.replace('ExpoAppDelegate, ExpoReactNativeFactoryProvider {', 'ExpoAppDelegate {');
    assert.notEqual(withoutConformance, source);
    writeFileSync(appDelegate, withoutConformance.replace('class AppDelegate:', '// TODO: ExpoReactNativeFactoryProvider\nclass AppDelegate:').replace('class AppDelegate: ExpoAppDelegate {', 'class AppDelegate: ExpoAppDelegate {\n    let marker = \"ExpoReactNativeFactoryProvider\"'));
    assert.match(readFileSync(appDelegate, 'utf8'), /ExpoReactNativeFactoryProvider/);
    assert.match(verifyIosSceneProject(root).join('\n'), /AppDelegate must conform to ExpoReactNativeFactoryProvider/);
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
    source = source.replace('C3 /* Staging */ = { buildSettings = { IPHONEOS_DEPLOYMENT_TARGET = 16.4;', 'C3 /* Staging */ = { buildSettings = { IPHONEOS_DEPLOYMENT_TARGET = 16.0;');
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

test('generated SDK57 project contract rejects deployment setting found only in a PBX comment', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const project = join(root, 'WeKnora.xcodeproj', 'project.pbxproj');
    const source = readFileSync(project, 'utf8');
    writeFileSync(project, source.replace(
      'C3 /* Staging */ = { buildSettings = { IPHONEOS_DEPLOYMENT_TARGET = 16.4; }; };',
      'C3 /* Staging */ = { buildSettings = { /* IPHONEOS_DEPLOYMENT_TARGET = 16.4; */ }; };',
    ));
    assert.match(verifyIosSceneProject(root).join('\n'), /All app target deployment settings/);
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
    writeFileSync(join(root, 'WeKnora.xcodeproj', 'project.pbxproj'), readFileSync(join(root, 'WeKnora.xcodeproj', 'project.pbxproj'), 'utf8').replace('IPHONEOS_DEPLOYMENT_TARGET = 16.4;', 'IPHONEOS_DEPLOYMENT_TARGET = 16.0;'));
    assert.match(verifyIosSceneProject(root).join('\n'), /16.4/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 project contract ignores Swift comments in URL callbacks', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const appDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
    const source = readFileSync(appDelegate, 'utf8');
    writeFileSync(appDelegate, source.replace(
      'return super.application(app, open: url, options: options) || RCTLinkingManager.application(app, open: url, options: options)',
      'let callbackURL = "https://example.test/path"\n      return super.application(app, open: url, options: options) || RCTLinkingManager.application(app, open: url, options: options)',
    ));
    assert.deepEqual(verifyIosSceneProject(root), []);

    fixture(root);
    {
      const appDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
      const source = readFileSync(appDelegate, 'utf8');
      writeFileSync(appDelegate, source.replace(
        'return super.application(app, open: url, options: options) || RCTLinkingManager.application(app, open: url, options: options)',
        'return super.application(app, open: url, options: options) // RCTLinkingManager.application(app, open: url, options: options)',
      ));
      assert.match(verifyIosSceneProject(root).join('\n'), /open-URL callback/);
    }

    fixture(root);
    const universalDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
    const universalSource = readFileSync(universalDelegate, 'utf8');
    writeFileSync(universalDelegate, universalSource.replace(
      'let result = RCTLinkingManager.application(application, continue: userActivity, restorationHandler: restorationHandler)',
      'let result = false // RCTLinkingManager.application(application, continue: userActivity, restorationHandler: restorationHandler)',
    ));
    assert.match(verifyIosSceneProject(root).join('\n'), /universal-link callback/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 project contract rejects URL forwarding named only in string literals', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const appDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
    const source = readFileSync(appDelegate, 'utf8');
    const openWithMarker = source.replace(
      'return super.application(app, open: url, options: options) || RCTLinkingManager.application(app, open: url, options: options)',
      'let marker = "RCTLinkingManager.application"\n      return super.application(app, open: url, options: options)',
    );
    assert.notEqual(openWithMarker, source);
    writeFileSync(appDelegate, openWithMarker);
    assert.match(verifyIosSceneProject(root).join('\n'), /open-URL callback/);

    fixture(root);
    const universalWithMarker = source.replace(
      'let result = RCTLinkingManager.application(application, continue: userActivity, restorationHandler: restorationHandler)',
      'let marker = "RCTLinkingManager.application"\n      let result = false',
    );
    assert.notEqual(universalWithMarker, source);
    writeFileSync(appDelegate, universalWithMarker);
    assert.match(verifyIosSceneProject(root).join('\n'), /universal-link callback/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 project contract rejects a commented open-URL signature', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const appDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
    const source = readFileSync(appDelegate, 'utf8');
    const withoutOpenUrl = source.replace(
      /    override func application\(_ app: UIApplication, open url: URL[^\n]*\n      return[^\n]*\n    }\n/,
      '    // application(_ app: UIApplication, open url: URL\n',
    );
    assert.notEqual(withoutOpenUrl, source);
    writeFileSync(appDelegate, withoutOpenUrl);
    assert.match(verifyIosSceneProject(root).join('\n'), /open-URL callback/);
    assert.doesNotMatch(verifyIosSceneProject(root).join('\n'), /universal-link callback/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('generated SDK57 project contract masks Swift literal braces and nested comments', () => {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    const appDelegate = join(root, 'WeKnora', 'AppDelegate.swift');
    const source = readFileSync(appDelegate, 'utf8');
    const withLiterals = source.replace(
      'return super.application(app, open: url, options: options) || RCTLinkingManager.application(app, open: url, options: options)',
      'let diagnosticURL = "https://example.test/}"\n      let note = """\n        } // quoted\n        """\n      /* outer /* inner RCTLinkingManager.application(app) */ } */\n      return super.application(app, open: url, options: options) || RCTLinkingManager.application(app, open: url, options: options)',
    );
    assert.notEqual(withLiterals, source);
    writeFileSync(appDelegate, withLiterals);
    assert.deepEqual(verifyIosSceneProject(root), []);

    const noRealCall = withLiterals.replace(
      ' || RCTLinkingManager.application(app, open: url, options: options)',
      '',
    );
    writeFileSync(appDelegate, noRealCall);
    assert.match(verifyIosSceneProject(root).join('\n'), /open-URL callback/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});
