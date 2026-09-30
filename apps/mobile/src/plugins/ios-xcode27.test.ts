import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createRequire } from 'node:module';

// 纯 JS 的 Expo config plugin（CommonJS），不经 Expo CLI 直接测它的改写函数：
// 修复回归点 = scene 生命周期改写、冷启动 URL 转发、部署目标钳制。
const require = createRequire(import.meta.url);
const plugin = require('../../plugins/ios-xcode27.js') as {
  DEPLOYMENT_TARGET: string;
  SCENE_MANIFEST: Record<string, unknown>;
  applySceneLifecycle(contents: string): string;
  applyPodfileClamp(contents: string): string;
  applyPodfileWarningsInhibit(contents: string): string;
  applySplashStoryboard(contents: string): string;
  raiseDeploymentTargets(project: unknown): unknown;
  resolveNewArchEnabled(config: unknown): { effective: boolean; warned: boolean };
};

// Expo SDK 55 模板原文（expo/template.tgz → package/ios/HelloWorld/AppDelegate.swift）。
const TEMPLATE_APP_DELEGATE = `internal import Expo
import React
import ReactAppDependencyProvider

@main
class AppDelegate: ExpoAppDelegate {
  var window: UIWindow?

  var reactNativeDelegate: ExpoReactNativeFactoryDelegate?
  var reactNativeFactory: RCTReactNativeFactory?

  public override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]? = nil
  ) -> Bool {
    let delegate = ReactNativeDelegate()
    let factory = ExpoReactNativeFactory(delegate: delegate)
    delegate.dependencyProvider = RCTAppDependencyProvider()

    reactNativeDelegate = delegate
    reactNativeFactory = factory

#if os(iOS) || os(tvOS)
    window = UIWindow(frame: UIScreen.main.bounds)
    factory.startReactNative(
      withModuleName: "main",
      in: window,
      launchOptions: launchOptions)
#endif

    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  // Linking API
  public override func application(
    _ app: UIApplication,
    open url: URL,
    options: [UIApplication.OpenURLOptionsKey: Any] = [:]
  ) -> Bool {
    return super.application(app, open: url, options: options) || RCTLinkingManager.application(app, open: url, options: options)
  }

  // Universal Links
  public override func application(
    _ application: UIApplication,
    continue userActivity: NSUserActivity,
    restorationHandler: @escaping ([UIUserActivityRestoring]?) -> Void
  ) -> Bool {
    let result = RCTLinkingManager.application(application, continue: userActivity, restorationHandler: restorationHandler)
    return super.application(application, continue: userActivity, restorationHandler: restorationHandler) || result
  }
}

class ReactNativeDelegate: ExpoReactNativeFactoryDelegate {
  // Extension point for config-plugins

  override func sourceURL(for bridge: RCTBridge) -> URL? {
    // needed to return the correct URL for expo-dev-client.
    bridge.bundleURL ?? bundleURL()
  }

  override func bundleURL() -> URL? {
#if DEBUG
    return RCTBundleURLProvider.sharedSettings().jsBundleURL(forBundleRoot: ".expo/.virtual-metro-entry")
#else
    return Bundle.main.url(forResource: "main", withExtension: "jsbundle")
#endif
  }
}
`;

test('applySceneLifecycle replaces the template UIWindow launch with the scene life cycle', () => {
  const patched = plugin.applySceneLifecycle(TEMPLATE_APP_DELEGATE);

  // 旧式 UIWindow 生命周期必须消失（iOS 27 SDK 下启动即 SIGTRAP 的根因）。
  assert.equal(patched.includes('UIWindow(frame: UIScreen.main.bounds)'), false);
  // didFinishLaunching 只保存 launchOptions，RN 启动延迟到 scene。
  assert.equal(patched.includes('Self.savedLaunchOptions = launchOptions'), true);
  assert.match(patched, /configurationForConnecting connectingSceneSession: UISceneSession/);
  assert.match(patched, /configuration\.delegateClass = SceneDelegate\.self/);
  assert.match(patched, /startReactNativeOnce\(in: window\)/);
  // SceneDelegate：冷启动（connectionOptions.urlContexts）与 warm（openURLContexts）URL 都转发回 AppDelegate。
  assert.match(patched, /willConnectTo session: UISceneSession/);
  assert.match(patched, /forwardURLContexts\(connectionOptions\.urlContexts\)/);
  assert.match(patched, /openURLContexts URLContexts: Set<UIOpenURLContext>/);
  assert.match(patched, /appDelegate\.application\(UIApplication\.shared, open: context\.url, options: options\)/);
  assert.equal(patched.includes('final class SceneDelegate: UIResponder, UIWindowSceneDelegate'), true);
});

test('applySceneLifecycle is idempotent and refuses unknown templates', () => {
  const once = plugin.applySceneLifecycle(TEMPLATE_APP_DELEGATE);
  assert.equal(plugin.applySceneLifecycle(once), once, 're-running the plugin must not duplicate the patch');
  assert.throws(
    () => plugin.applySceneLifecycle('class AppDelegate: ExpoAppDelegate {}\n'),
    /no longer matches the Expo SDK 55 template/,
    'template drift must fail loudly instead of silently generating a broken delegate',
  );
});

test('the scene manifest opts into the scene life cycle without multiple scenes', () => {
  assert.deepEqual(plugin.SCENE_MANIFEST, { UIApplicationSupportsMultipleScenes: false });
});

// Expo SDK 55 模板 Podfile 的 post_install 原文（片段）。
const TEMPLATE_PODFILE = `target 'WeKnora' do
  use_expo_modules!
  use_react_native!(
    :path => config[:reactNativePath],
    :app_path => "#{Pod::Config.instance.installation_root}/..",
  )

  post_install do |installer|
    react_native_post_install(
      installer,
      config[:reactNativePath],
      :mac_catalyst_enabled => false,
      :ccache_enabled => ccache_enabled?(podfile_properties),
    )
  end
end
`;

test('applyPodfileClamp injects the per-pod deployment target clamp and re-forces RCTNewArchEnabled', () => {
  const clamped = plugin.applyPodfileClamp(TEMPLATE_PODFILE);
  assert.match(clamped, /installer\.pods_project\.targets\.each do \|t\|/);
  assert.match(clamped, new RegExp(`IPHONEOS_DEPLOYMENT_TARGET'\\] = '${plugin.DEPLOYMENT_TARGET}'`));
  // RN 0.83 的 react_native_post_install 会把 RCTNewArchEnabled 改写为 true（硬编码），
  // 钳制块必须在它之后依据 podfile properties 重新写回。
  assert.match(clamped, /Xcodeproj::Plist\.read_from_path/);
  assert.match(clamped, /expo\.newArchEnabled/);
  const postInstall = clamped.indexOf('react_native_post_install(');
  const newArchFix = clamped.indexOf("info_plist['RCTNewArchEnabled']");
  assert.ok(postInstall >= 0 && newArchFix > postInstall, 'the flag rewrite must run after react_native_post_install');
  assert.equal(plugin.applyPodfileClamp(clamped), clamped, 're-running must not duplicate the clamp');
  assert.throws(
    () => plugin.applyPodfileClamp('target x\nend\n'),
    /no longer matches the Expo SDK 55 template/,
  );
});

test('raiseDeploymentTargets lifts every iOS deployment target below the clamp', () => {
  // 真实 pbx 项目里 pbxXCBuildConfigurationSection() 每次返回同一 section 对象。
  const section: Record<string, { buildSettings?: Record<string, string> } | string> = {
    '/* Debug */': 'comment entry is skipped',
    AAAA: { buildSettings: { IPHONEOS_DEPLOYMENT_TARGET: '15.1' } },
    BBBB: { buildSettings: { IPHONEOS_DEPLOYMENT_TARGET: '16.0' } },
    CCCC: { buildSettings: { IPHONEOS_DEPLOYMENT_TARGET: '18.2' } },
    DDDD: { buildSettings: {} },
  };
  const project = { pbxXCBuildConfigurationSection: () => section };
  plugin.raiseDeploymentTargets(project);
  const buildSettings = (key: string) => (section[key] as { buildSettings?: Record<string, string> }).buildSettings;
  assert.equal(buildSettings('AAAA')!.IPHONEOS_DEPLOYMENT_TARGET, plugin.DEPLOYMENT_TARGET);
  assert.equal(buildSettings('BBBB')!.IPHONEOS_DEPLOYMENT_TARGET, '16.0', 'already at/above the clamp: untouched');
  assert.equal(buildSettings('CCCC')!.IPHONEOS_DEPLOYMENT_TARGET, '18.2', 'higher targets are never lowered');
  assert.equal(buildSettings('DDDD')!.IPHONEOS_DEPLOYMENT_TARGET, undefined);
});

test('willConnectTo forwards userActivities for cold-launch universal links (R1-F5)', () => {
  const rewritten = plugin.applySceneLifecycle(TEMPLATE_APP_DELEGATE);
  const willConnectStart = rewritten.indexOf('func scene(_ scene: UIScene, willConnectTo');
  const nextFunc = rewritten.indexOf('func scene(_ scene: UIScene, openURLContexts', willConnectStart);
  const willConnect = rewritten.slice(willConnectStart, nextFunc);
  assert.match(willConnect, /userActivities/);
  assert.match(rewritten, /forwardUserActivities\(connectionOptions\.userActivities\)/);
});

test('the clamp is idempotent by a unique anchor, not by its own substring (R1-F7)', () => {
  // 构造：模板 post_install + 一段含 'Xcodeproj::Plist.read_from_path' 的无关注入——
  // 旧实现以自身子串判已注入，会误判跳过（R1-F7 复现）。
  const unrelated = `${TEMPLATE_PODFILE.replace('end\n', '')}    # unrelated tooling block\n    other_plist = Xcodeproj::Plist.read_from_path('config/Other.plist')\n  end\nend\n`;
  const once = plugin.applyPodfileClamp(unrelated);
  assert.ok(once.includes('# weknora_ios_xcode27_clamp'));
  const twice = plugin.applyPodfileClamp(once);
  assert.equal(twice.split('# weknora_ios_xcode27_clamp').length - 1, 1, '不重复注入');
});

test('quoted deployment targets are still raised (R1-F8)', () => {
  const section: Record<string, { buildSettings?: Record<string, string> } | string> = {
    BC1: { buildSettings: { IPHONEOS_DEPLOYMENT_TARGET: '"15.1"' } },
  };
  const project = { pbxXCBuildConfigurationSection: () => section };
  const result = plugin.raiseDeploymentTargets(project) as typeof project;
  const raised = (result.pbxXCBuildConfigurationSection() as typeof section).BC1 as { buildSettings?: Record<string, string> };
  assert.equal(raised.buildSettings!.IPHONEOS_DEPLOYMENT_TARGET, plugin.DEPLOYMENT_TARGET);
});

test('the plugin honors and warns about an opt-in newArchEnabled declaration (R1-F6)', () => {
  // 决策函数单测：opt-in 声明显式告警且仍按 false 处理（iOS 27 模拟器 Fabric 不渲染）。
  const warnings: string[] = [];
  const originalWarn = console.warn;
  console.warn = (message: unknown) => { warnings.push(String(message)); };
  try {
    const optedIn = plugin.resolveNewArchEnabled({ newArchEnabled: true });
    assert.equal(optedIn.effective, false);
    assert.equal(optedIn.warned, true);
    assert.equal(warnings.length, 1, 'opt-in 声明必须 console.warn 说明强制旧架构的原因');
    const absent = plugin.resolveNewArchEnabled({});
    assert.equal(absent.effective, false);
    assert.equal(absent.warned, false);
    assert.equal(warnings.length, 1, '未声明时不得告警');
  } finally {
    console.warn = originalWarn;
  }
});

// ---- B4 复验修复轮（b4-recheck-fix.md）----

test('applySceneLifecycle wires the RCTRootView splash loadingView (B4 F1)', () => {
  // Release 冷启动 JS bundle 执行期 root view 纯白：修复 = factory 根视图
  // RCTSurfaceHostingProxyRootView 的 loadingView（surface hosting view 的
  // activity indicator，Preparing 阶段全屏显示、Running 后移除）。
  const patched = plugin.applySceneLifecycle(TEMPLATE_APP_DELEGATE);
  assert.match(patched, /rootView\.loadingView = Self\.makeSplashLoadingView\(size: window\.bounds\.size\)/);
  assert.match(patched, /private static func makeSplashLoadingView\(size: CGSize\) -> UIView/);
  assert.match(patched, /window\.rootViewController\?\.view as\? RCTSurfaceHostingProxyRootView/);
  // splash 注入必须发生在 RN 启动之后（拿到 root view 才有意义）。
  const start = patched.indexOf('factory.startReactNative(');
  const splash = patched.indexOf('rootView.loadingView');
  assert.ok(start >= 0 && splash > start, 'splash wiring must run after startReactNative');
  // 既有幂等/模板断言仍适用：重跑不重复、模板漂移抛错。
  assert.equal(plugin.applySceneLifecycle(patched), patched);
});

test('applyPodfileWarningsInhibit injects a top-level inhibit_all_warnings! (B4 F2)', () => {
  // 真实模板里 prepare_react_native_project! 在 target 之前；测试夹具补齐该行作锚。
  const templateWithPrepare = `prepare_react_native_project!\n${TEMPLATE_PODFILE}`;
  const inhibited = plugin.applyPodfileWarningsInhibit(templateWithPrepare);
  // 必须在 target 定义之前（顶层 DSL 调用，作用于全部 pod targets）。
  const inhibit = inhibited.indexOf('inhibit_all_warnings!');
  const target = inhibited.indexOf("target 'WeKnora' do");
  assert.ok(inhibit >= 0 && inhibit < target, 'inhibit_all_warnings! must precede the target block');
  assert.ok(inhibited.includes('# weknora_ios_inhibit_warnings'));
  // 幂等（唯一锚注释判已注入，同 R1-F7 规则）。
  assert.equal(plugin.applyPodfileWarningsInhibit(inhibited), inhibited);
  // 模板漂移必须失败而非静默跳过。
  assert.throws(
    () => plugin.applyPodfileWarningsInhibit("platform :ios, '15.1'\ntarget x\nend\n"),
    /no longer matches the Expo SDK 55 template/,
  );
});

test('withPodfile applies the warning inhibit and the clamp together (B4 F2)', () => {
  // 组合入口：两个注入共存于同一 Podfile 输出且互不吞并。
  const templateWithPrepare = `prepare_react_native_project!\n${TEMPLATE_PODFILE}`;
  const combined = plugin.applyPodfileClamp(plugin.applyPodfileWarningsInhibit(templateWithPrepare));
  assert.ok(combined.includes('# weknora_ios_inhibit_warnings'));
  assert.ok(combined.includes('# weknora_ios_xcode27_clamp'));
});

// Expo SDK 55 模板 SplashScreen.storyboard 的空容器（白屏根因：launch snapshot 只有白底，
// 且容器约束悬空引用不存在的 EXPO-SplashScreen item）。
const TEMPLATE_SPLASH_STORYBOARD = `<subviews/>
                        <viewLayoutGuide key="safeArea" id="Rmq-lb-GrQ"/>
                        <constraints>
                            <constraint firstItem="EXPO-SplashScreen" firstAttribute="centerY" secondItem="EXPO-ContainerView" secondAttribute="centerY" id="0VC-Wk-OaO"/>
                            <constraint firstItem="EXPO-SplashScreen" firstAttribute="centerX" secondItem="EXPO-ContainerView" secondAttribute="centerX" id="zR4-NK-mVN"/>
                        </constraints>`;

test('applySplashStoryboard revives the dangling splash constraints with a wordmark (B4 F1)', () => {
  const patched = plugin.applySplashStoryboard(TEMPLATE_SPLASH_STORYBOARD);
  // label 复用悬空约束引用的 id：约束原样生效，无需改动 constraints 段。
  assert.match(patched, /<subviews>\s*\n\s*<label[\s\S]*?id="EXPO-SplashScreen">/);
  assert.match(patched, /text="WeKnora"/);
  assert.equal(patched.includes('<subviews/>'), false, 'empty container must be replaced');
  // 幂等。
  assert.equal(plugin.applySplashStoryboard(patched), patched);
  // 模板漂移必须失败而非静默跳过。
  assert.throws(
    () => plugin.applySplashStoryboard('<subviews><label text="x"/></subviews>'),
    /no longer matches the Expo SDK 55 template/,
  );
});
