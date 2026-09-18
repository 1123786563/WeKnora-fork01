import type { ExpoConfig } from 'expo/config';

const config: ExpoConfig = {
  name: 'WeKnora',
  slug: 'weknora',
  version: '0.0.0',
  orientation: 'portrait',
  scheme: 'weknora',
  ios: { bundleIdentifier: 'com.weknora.mobile' },
  android: { package: 'com.weknora.mobile' },
  userInterfaceStyle: 'automatic',
  // 路由根固定为 WeKnora 产品 App（与 app.config.js 一致）。
  // 本文件后于 app.config.js 引入且会遮蔽它——缺此配置时 Expo 解析到
  // 上游 desktop-runtime 骨架 apps/mobile/app/，产品链（sources/app）失联
  // （2026-09-18 Android 模拟器实测发现：深链 Unmatched + 索引不重定向）。
  plugins: [['expo-router', { root: './sources/app' }], 'expo-secure-store', 'expo-dev-client', './withAndroidCleartextTraffic.js'],
  experiments: { typedRoutes: true },
  extra: { apiBaseUrl: process.env.EXPO_PUBLIC_API_BASE_URL ?? '', router: { root: './sources/app' } },
};

export default config;
