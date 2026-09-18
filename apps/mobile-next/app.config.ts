import type { ExpoConfig } from "expo/config";

const config: ExpoConfig = {
  name: "WeKnora 工作台",
  slug: "weknora-mobile-next",
  version: "0.1.0",
  scheme: "weknora",
  orientation: "portrait",
  userInterfaceStyle: "automatic",
  platforms: ["ios", "android"],
  experiments: {
    typedRoutes: true,
  },
  plugins: [
    "expo-router",
    "expo-secure-store",
    "expo-document-picker",
    [
      "expo-audio",
      { microphonePermission: "用于将你的语音转为文字草稿，确认后才发送。" },
    ],
  ],
  ios: {
    supportsTablet: false,
    bundleIdentifier: "com.weknora.mobilenext",
    // 部署目标 16.0 由 ios/Podfile.properties 的 deploymentTarget 承载（expo-router LinkPreview 需 iOS 16 API）
    infoPlist: {
      NSAppTransportSecurity: {
        NSAllowsArbitraryLoads: true, // dev 阶段连接本地 http 后端；发布前收敛为 NSExceptionDomains
      },
    },
  },
  android: {
    package: "com.weknora.mobilenext",
    // dev 阶段允许本地 http 后端：expo dev-client 默认 cleartext；发布前移除并收紧 ATS/网络白名单
  },
  // 运行时后端地址由 app 内"可信服务器"配置提供（M01），不从 app.config 注入
  extra: {
    buildProfile: process.env.EXPO_PROFILE ?? "dev",
  },
};

export default config;
