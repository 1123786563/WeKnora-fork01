import type { ExpoConfig } from 'expo/config';

const config: ExpoConfig = {
  name: 'WeKnora',
  slug: 'weknora',
  version: '0.0.0',
  orientation: 'portrait',
  scheme: 'weknora',
  userInterfaceStyle: 'automatic',
  plugins: ['expo-router', 'expo-secure-store'],
  experiments: { typedRoutes: true },
  extra: { apiBaseUrl: process.env.EXPO_PUBLIC_API_BASE_URL ?? '' },
};

export default config;
