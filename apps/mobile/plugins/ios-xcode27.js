// iOS 27 SDK (Xcode 27) compatibility for `expo prebuild` output.
//
// 1. UIKit now REQUIRES the scene-based life cycle: apps built with the iOS 27
//    SDK that keep the legacy `UIWindow`-in-AppDelegate life cycle are killed at
//    launch (SIGTRAP, "UIScene life cycle is required for apps built with this
//    SDK"). Expo SDK 55's `ExpoAppDelegate` has no scene support yet (its
//    ios/AppDelegates/ExpoAppDelegate.swift still has a TODO for scenes), so
//    this plugin rewrites the generated AppDelegate to defer React Native
//    startup into a `SceneDelegate` and adds the `UIApplicationSceneManifest`.
// 2. Xcode 27 refuses deployment targets below 15.0 (SDWebImage declares 9.0)
//    and expo-router's Swift sources use iOS 16 APIs while declaring 15.1, so
//    every iOS deployment target is clamped up to 16.0.
// 3. React Native 0.83 hardcodes NewArchitectureHelper.new_arch_enabled=true,
//    so `pod install` rewrites RCTNewArchEnabled=true into the app Info.plist
//    regardless of app.json `newArchEnabled` — and Fabric renders nothing on
//    the iOS 27.0 simulator runtime. The plugin writes RCTNewArchEnabled=false
//    into the prebuild Info.plist AND re-forces it after react_native_post_install.
// 4. With the scene life cycle, custom-scheme/universal-link URLs are delivered
//    to the scene delegate instead of the app delegate. The injected
//    SceneDelegate forwards them back to `AppDelegate.application(_:open:)`,
//    keeping expo-linking (ExpoLinkingRegistry / onURLReceivedNotification) and
//    RCTLinkingManager working — including the cold-launch URL that UIKit
//    delivers through `scene(_:willConnectTo:options:)`.
//
// The AppDelegate/Podfile transforms anchor on the exact Expo SDK 55 template
// bodies; if the template drifts on a future Expo upgrade, the plugin fails
// loudly instead of silently generating a broken project.

const fs = require('fs');
const path = require('path');
const { withAppDelegate, withDangerousMod, withInfoPlist, withXcodeProject, withPodfile, withPodfileProperties } = require('expo/config-plugins');

const DEPLOYMENT_TARGET = '16.0';

const SCENE_MANIFEST = {
  UIApplicationSupportsMultipleScenes: false,
};

// Exact excerpt of the Expo SDK 55 template AppDelegate.swift
// (expo/template.tgz → package/ios/HelloWorld/AppDelegate.swift).
const TEMPLATE_LAUNCH_BODY = `  public override func application(
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
`;

const SCENE_LAUNCH_BODY = `  public override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]? = nil
  ) -> Bool {
    // iOS 27 SDK requires the UIKit scene life cycle and kills legacy
    // UIWindow-in-AppDelegate apps at launch; Expo SDK 55 has no scene support
    // in ExpoAppDelegate yet, so React Native startup is deferred to
    // SceneDelegate.scene(_:willConnectTo:options:).
    Self.savedLaunchOptions = launchOptions
    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  public func application(
    _ application: UIApplication,
    configurationForConnecting connectingSceneSession: UISceneSession,
    options: UIScene.ConnectionOptions
  ) -> UISceneConfiguration {
    let configuration = UISceneConfiguration(name: nil, sessionRole: connectingSceneSession.role)
    configuration.delegateClass = SceneDelegate.self
    return configuration
  }

  func startReactNativeOnce(in window: UIWindow) {
    self.window = window
    guard reactNativeFactory == nil else { return }
    let delegate = ReactNativeDelegate()
    let factory = ExpoReactNativeFactory(delegate: delegate)
    delegate.dependencyProvider = RCTAppDependencyProvider()

    reactNativeDelegate = delegate
    reactNativeFactory = factory

    factory.startReactNative(
      withModuleName: "main",
      in: window,
      launchOptions: Self.savedLaunchOptions)

    // B4 recheck F1: in Release the JS bundle takes seconds to execute and the
    // root view sat blank white until it finished. The factory root view is an
    // RCTSurfaceHostingProxyRootView (RN 0.83's RCTRootView-compatible surface
    // host, RCTRootViewFactory.mm:177-186); its 'loadingView' property maps to
    // the surface hosting view's activity indicator, which is shown full-screen
    // while the surface is not yet running and removed once it runs
    // (RCTSurfaceHostingView.mm:145-153).
    if let rootView = window.rootViewController?.view as? RCTSurfaceHostingProxyRootView {
      rootView.loadingView = Self.makeSplashLoadingView(size: window.bounds.size)
    }
  }

  /// Splash shown while the JS bundle loads: white background with a centered
  /// wordmark, matching the launch storyboard's white background
  /// (SplashScreen.storyboard's container has empty subviews, so there is no
  /// richer content to replicate).
  private static func makeSplashLoadingView(size: CGSize) -> UIView {
    let container = UIView(frame: CGRect(origin: .zero, size: size))
    container.autoresizingMask = [.flexibleWidth, .flexibleHeight]
    container.backgroundColor = .white

    let wordmark = UILabel()
    wordmark.text = "WeKnora"
    wordmark.font = .systemFont(ofSize: 30, weight: .semibold)
    wordmark.textColor = UIColor(red: 0.20, green: 0.45, blue: 0.85, alpha: 1.0)
    wordmark.translatesAutoresizingMaskIntoConstraints = false
    container.addSubview(wordmark)
    NSLayoutConstraint.activate([
      wordmark.centerXAnchor.constraint(equalTo: container.centerXAnchor),
      wordmark.centerYAnchor.constraint(equalTo: container.centerYAnchor),
    ])
    return container
  }
`;

const TEMPLATE_WINDOW_PROPERTY = `class AppDelegate: ExpoAppDelegate {
  var window: UIWindow?
`;

const SCENE_WINDOW_PROPERTY = `class AppDelegate: ExpoAppDelegate {
  var window: UIWindow?

  static var savedLaunchOptions: [UIApplication.LaunchOptionsKey: Any]?
`;

const SCENE_DELEGATE_SOURCE = `
// Scene-based life cycle adoption required by the iOS 27 SDK. URLs arrive on
// the scene, not the app delegate; forwarding keeps expo-linking (which stores
// ExpoLinkingRegistry.initialURL and posts onURLReceivedNotification through
// ExpoAppDelegate subscribers) and RCTLinkingManager working unchanged.
final class SceneDelegate: UIResponder, UIWindowSceneDelegate {
  var window: UIWindow?

  func scene(_ scene: UIScene, willConnectTo session: UISceneSession, options connectionOptions: UIScene.ConnectionOptions) {
    guard let windowScene = scene as? UIWindowScene else { return }
    let window = UIWindow(windowScene: windowScene)
    self.window = window
    (UIApplication.shared.delegate as? AppDelegate)?.startReactNativeOnce(in: window)
    // Cold launch by URL: UIKit delivers it here (not through
    // scene(_:openURLContexts:)), so forward it before the JS bundle runs and
    // expo-router reads Linking.getLinkingURL() as the initial route.
    forwardURLContexts(connectionOptions.urlContexts)
    // Cold launch by universal link: UIKit delivers NSUserActivity here (not
    // through scene(_:continue:)); forward before the JS bundle runs so
    // expo-router resolves the initial route (file-header promise, R1-F5).
    forwardUserActivities(connectionOptions.userActivities)
  }

  func scene(_ scene: UIScene, openURLContexts URLContexts: Set<UIOpenURLContext>) {
    forwardURLContexts(URLContexts)
  }

  func scene(_ scene: UIScene, continue userActivity: NSUserActivity) {
    guard let appDelegate = UIApplication.shared.delegate as? AppDelegate else { return }
    _ = appDelegate.application(
      UIApplication.shared,
      continue: userActivity,
      restorationHandler: { (_: [UIUserActivityRestoring]?) in })
  }

  private func forwardURLContexts(_ contexts: Set<UIOpenURLContext>) {
    guard let appDelegate = UIApplication.shared.delegate as? AppDelegate else { return }
    for context in contexts {
      var options: [UIApplication.OpenURLOptionsKey: Any] = [:]
      options[.openInPlace] = context.options.openInPlace
      if let annotation = context.options.annotation {
        options[.annotation] = annotation
      }
      _ = appDelegate.application(UIApplication.shared, open: context.url, options: options)
    }
  }

  private func forwardUserActivities(_ activities: Set<NSUserActivity>) {
    guard let appDelegate = UIApplication.shared.delegate as? AppDelegate else { return }
    for activity in activities where activity.activityType == NSUserActivityTypeBrowsingWeb {
      _ = appDelegate.application(
        UIApplication.shared,
        continue: activity,
        restorationHandler: { (_: [UIUserActivityRestoring]?) in })
    }
  }
}
`;

// B4 recheck F1: the Expo-generated SplashScreen.storyboard has an EMPTY
// container (<subviews/>) while its centering constraints already reference a
// missing `EXPO-SplashScreen` item — so the cold-start launch snapshot is a
// plain white screen for the whole bridge/Hermes startup (B4 measured 8-10s).
// Injecting a centered "WeKnora" wordmark label with that exact id revives the
// dangling constraints: the launch snapshot (the only thing on screen while
// startReactNative blocks the main thread) shows brand content instead of
// white. The RCTSurfaceHostingProxyRootView loadingView wired in
// SCENE_LAUNCH_BODY covers the window after the first app frame as a second
// layer.
const SPLASH_STORYBOARD_CONTAINER_ANCHOR = `<subviews/>`;
const SPLASH_STORYBOARD_CONTAINER_REPLACEMENT = `<subviews>
                        <label opaque="NO" userInteractionEnabled="NO" contentMode="left" horizontalHuggingPriority="251" verticalHuggingPriority="251" text="WeKnora" textAlignment="center" lineBreakMode="tailTruncation" baselineAdjustment="alignBaselines" adjustsFontSizeToFit="NO" translatesAutoresizingMaskIntoConstraints="NO" id="EXPO-SplashScreen">
                            <fontDescription key="fontDescription" type="boldSystem" pointSize="30"/>
                            <color key="textColor" red="0.20" green="0.45" blue="0.85" alpha="1" colorSpace="custom" customColorSpace="sRGB"/>
                            <nil key="highlightedColor"/>
                        </label>
                    </subviews>`;

/** Inject the splash wordmark into the generated SplashScreen.storyboard. Idempotent. */
function applySplashStoryboard(contents) {
  if (contents.includes('id="EXPO-SplashScreen"')) {
    return contents;
  }
  if (!contents.includes(SPLASH_STORYBOARD_CONTAINER_ANCHOR)) {
    throw new Error(
      'ios-xcode27 plugin: the generated SplashScreen.storyboard no longer matches the Expo SDK 55 template; update the plugin anchors.',
    );
  }
  return contents.replace(SPLASH_STORYBOARD_CONTAINER_ANCHOR, SPLASH_STORYBOARD_CONTAINER_REPLACEMENT);
}

/** Rewrite the template AppDelegate.swift into the scene-based variant. Idempotent. */
function applySceneLifecycle(contents) {
  if (contents.includes('configurationForConnecting connectingSceneSession')) {
    return contents;
  }
  for (const anchor of [TEMPLATE_LAUNCH_BODY, TEMPLATE_WINDOW_PROPERTY]) {
    if (!contents.includes(anchor)) {
      throw new Error(
        'ios-xcode27 plugin: the generated AppDelegate.swift no longer matches the Expo SDK 55 template; update the plugin anchors.',
      );
    }
  }
  return contents
    .replace(TEMPLATE_WINDOW_PROPERTY, SCENE_WINDOW_PROPERTY)
    .replace(TEMPLATE_LAUNCH_BODY, SCENE_LAUNCH_BODY)
    .concat(SCENE_DELEGATE_SOURCE);
}

// Exact excerpt of the Expo SDK 55 template Podfile post_install block.
const TEMPLATE_POST_INSTALL = `    react_native_post_install(
      installer,
      config[:reactNativePath],
      :mac_catalyst_enabled => false,
      :ccache_enabled => ccache_enabled?(podfile_properties),
    )
`;

const PODFILE_CLAMP = `    # weknora_ios_xcode27_clamp
    # Xcode 27 rejects pod deployment targets below 15.0 (SDWebImage declares
    # 9.0) while expo-router Swift sources use iOS 16 APIs at 15.1: clamp every
    # pod up to the app deployment target.
    installer.pods_project.targets.each do |t|
      t.build_configurations.each do |build_config|
        if build_config.build_settings['IPHONEOS_DEPLOYMENT_TARGET'].to_f < ${DEPLOYMENT_TARGET}
          build_config.build_settings['IPHONEOS_DEPLOYMENT_TARGET'] = '${DEPLOYMENT_TARGET}'
        end
      end
    end
    # RN 0.83 hardcodes NewArchitectureHelper.new_arch_enabled=true, so the
    # react_native_post_install above rewrote RCTNewArchEnabled=true into the
    # app Info.plist even though app.json sets newArchEnabled=false (Fabric
    # renders nothing on the iOS 27.0 simulator runtime). Force it back.
    Dir.glob(File.join(__dir__, '*', 'Info.plist')).each do |plist_path|
      next if plist_path.include?('Pods') || plist_path.include?('build/')
      info_plist = Xcodeproj::Plist.read_from_path(plist_path)
      next if info_plist.nil? || info_plist['RCTNewArchEnabled'].nil?
      info_plist['RCTNewArchEnabled'] = podfile_properties['expo.newArchEnabled'] != 'false' ? true : false
      Xcodeproj::Plist.write_to_path(info_plist, plist_path)
    end
`;

/** Inject the per-pod deployment-target clamp into the generated Podfile. Idempotent（唯一锚 R1-F7：
 * 不得以 clamp 自身子串（如 Xcodeproj::Plist.read_from_path）判已注入——无关注入含同串会被误判跳过）。 */
function applyPodfileClamp(contents) {
  if (contents.includes('# weknora_ios_xcode27_clamp')) {
    return contents;
  }
  if (!contents.includes(TEMPLATE_POST_INSTALL)) {
    throw new Error(
      'ios-xcode27 plugin: the generated Podfile no longer matches the Expo SDK 55 template; update the plugin anchors.',
    );
  }
  return contents.replace(TEMPLATE_POST_INSTALL, `${TEMPLATE_POST_INSTALL}${PODFILE_CLAMP}`);
}

// B4 recheck F2: Release builds emitted ~4.8k warnings, all of them from
// third-party pod sources (SDWebImage/libdav1d/libwebp/libavif, Expo modules,
// prebuilt RN umbrella headers); the app target itself compiled with 0 warnings
// (full attribution in docs/plans/issue30-sweep/ios-evidence/b4-recheck-fix.md).
// Silence pod warnings so future real regressions stay visible.
const TEMPLATE_PREPARE_REACT_NATIVE = `prepare_react_native_project!
`;

const PODFILE_INHIBIT_WARNINGS = `# weknora_ios_inhibit_warnings
inhibit_all_warnings!
`;

/** Inject the pod-wide warning inhibit before the target definitions. Idempotent by a unique
 *  anchor comment (same rule as R1-F7). `inhibit_all_warnings!` is a top-level CocoaPods DSL
 *  call: it must stay outside the target block, so the anchor is the template's
 *  `prepare_react_native_project!` line. */
function applyPodfileWarningsInhibit(contents) {
  if (contents.includes('# weknora_ios_inhibit_warnings')) {
    return contents;
  }
  if (!contents.includes(TEMPLATE_PREPARE_REACT_NATIVE)) {
    throw new Error(
      'ios-xcode27 plugin: the generated Podfile no longer matches the Expo SDK 55 template; update the plugin anchors.',
    );
  }
  return contents.replace(
    TEMPLATE_PREPARE_REACT_NATIVE,
    `${TEMPLATE_PREPARE_REACT_NATIVE}\n${PODFILE_INHIBIT_WARNINGS}`,
  );
}

/** Raise every iOS deployment target below DEPLOYMENT_TARGET in the pbxproj. 引号包裹的目标值先 strip（R1-F8）。 */
function raiseDeploymentTargets(project) {
  const configurations = project.pbxXCBuildConfigurationSection();
  for (const key of Object.keys(configurations)) {
    const settings = configurations[key] && configurations[key].buildSettings;
    if (settings && settings.IPHONEOS_DEPLOYMENT_TARGET) {
      const current = parseFloat(String(settings.IPHONEOS_DEPLOYMENT_TARGET).replace(/^"|"$/g, ''));
      if (!Number.isNaN(current) && current < parseFloat(DEPLOYMENT_TARGET)) {
        settings.IPHONEOS_DEPLOYMENT_TARGET = DEPLOYMENT_TARGET;
      }
    }
  }
  return project;
}

/** R1-F6：app.json 声明优先于硬编码；iOS 27 模拟器 Fabric 不渲染，opt-in 时显式告警而非静默覆盖。 */
function resolveNewArchEnabled(config) {
  const declared = config && config.newArchEnabled === true;
  if (declared) {
    console.warn(
      'ios-xcode27 plugin: app.json sets newArchEnabled=true, but Fabric renders nothing on the iOS 27.0 simulator runtime; forcing old architecture on iOS. Remove this plugin when RN/Expo support the iOS 27 SDK.',
    );
  }
  return { effective: false, warned: declared };
}

module.exports = function withIosXcode27(config) {
  const newArch = resolveNewArchEnabled(config);
  config = withAppDelegate(config, (mod) => {
    mod.modResults.contents = applySceneLifecycle(mod.modResults.contents);
    return mod;
  });
  config = withInfoPlist(config, (mod) => {
    mod.modResults.UIApplicationSceneManifest = SCENE_MANIFEST;
    mod.modResults.RCTNewArchEnabled = newArch.effective;
    return mod;
  });
  config = withPodfileProperties(config, (mod) => {
    mod.modResults = {
      ...mod.modResults,
      'ios.deploymentTarget': DEPLOYMENT_TARGET,
      'expo.newArchEnabled': String(newArch.effective),
    };
    return mod;
  });
  config = withPodfile(config, (mod) => {
    mod.modResults.contents = applyPodfileWarningsInhibit(mod.modResults.contents);
    mod.modResults.contents = applyPodfileClamp(mod.modResults.contents);
    return mod;
  });
  config = withDangerousMod(config, [
    'ios',
    (mod) => {
      const storyboardPath = path.join(
        mod.modRequest.platformProjectRoot,
        mod.modRequest.projectName,
        'SplashScreen.storyboard',
      );
      const contents = fs.readFileSync(storyboardPath, 'utf8');
      fs.writeFileSync(storyboardPath, applySplashStoryboard(contents));
      return mod;
    },
  ]);
  config = withXcodeProject(config, (mod) => {
    raiseDeploymentTargets(mod.modResults);
    return mod;
  });
  return config;
};

module.exports.DEPLOYMENT_TARGET = DEPLOYMENT_TARGET;
module.exports.SCENE_MANIFEST = SCENE_MANIFEST;
module.exports.applySceneLifecycle = applySceneLifecycle;
module.exports.applyPodfileClamp = applyPodfileClamp;
module.exports.applyPodfileWarningsInhibit = applyPodfileWarningsInhibit;
module.exports.applySplashStoryboard = applySplashStoryboard;
module.exports.raiseDeploymentTargets = raiseDeploymentTargets;
module.exports.resolveNewArchEnabled = resolveNewArchEnabled;
