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

const { withAppDelegate, withInfoPlist, withXcodeProject, withPodfile, withPodfileProperties } = require('expo/config-plugins');

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
}
`;

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

const PODFILE_CLAMP = `    # Xcode 27 rejects pod deployment targets below 15.0 (SDWebImage declares
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

/** Inject the per-pod deployment-target clamp into the generated Podfile. Idempotent. */
function applyPodfileClamp(contents) {
  if (contents.includes('Xcodeproj::Plist.read_from_path')) {
    return contents;
  }
  if (!contents.includes(TEMPLATE_POST_INSTALL)) {
    throw new Error(
      'ios-xcode27 plugin: the generated Podfile no longer matches the Expo SDK 55 template; update the plugin anchors.',
    );
  }
  return contents.replace(TEMPLATE_POST_INSTALL, `${TEMPLATE_POST_INSTALL}${PODFILE_CLAMP}`);
}

/** Raise every iOS deployment target below DEPLOYMENT_TARGET in the pbxproj. */
function raiseDeploymentTargets(project) {
  const configurations = project.pbxXCBuildConfigurationSection();
  for (const key of Object.keys(configurations)) {
    const settings = configurations[key] && configurations[key].buildSettings;
    if (settings && settings.IPHONEOS_DEPLOYMENT_TARGET) {
      const current = parseFloat(settings.IPHONEOS_DEPLOYMENT_TARGET);
      if (!Number.isNaN(current) && current < parseFloat(DEPLOYMENT_TARGET)) {
        settings.IPHONEOS_DEPLOYMENT_TARGET = DEPLOYMENT_TARGET;
      }
    }
  }
  return project;
}

module.exports = function withIosXcode27(config) {
  config = withAppDelegate(config, (mod) => {
    mod.modResults.contents = applySceneLifecycle(mod.modResults.contents);
    return mod;
  });
  config = withInfoPlist(config, (mod) => {
    mod.modResults.UIApplicationSceneManifest = SCENE_MANIFEST;
    mod.modResults.RCTNewArchEnabled = false;
    return mod;
  });
  config = withPodfileProperties(config, (mod) => {
    mod.modResults = {
      ...mod.modResults,
      'ios.deploymentTarget': DEPLOYMENT_TARGET,
      'expo.newArchEnabled': 'false',
    };
    return mod;
  });
  config = withPodfile(config, (mod) => {
    mod.modResults.contents = applyPodfileClamp(mod.modResults.contents);
    return mod;
  });
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
module.exports.raiseDeploymentTargets = raiseDeploymentTargets;
