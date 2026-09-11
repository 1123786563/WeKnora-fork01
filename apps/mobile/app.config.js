export default {
  expo: {
    name: 'WeKnora Mobile (dev)',
    slug: 'weknora-mobile',
    version: '0.1.0',
    orientation: 'default',
    scheme: 'weknora',
    userInterfaceStyle: 'automatic',
    ios: {
      supportsTablet: true,
      bundleIdentifier: 'com.weknora.mobile.dev',
      config: { usesNonExemptEncryption: false },
    },
    android: {
      package: 'com.weknora.mobile.dev',
    },
    plugins: [['expo-router', { root: './sources/app' }]],
    experiments: { typedRoutes: true },
    extra: { router: { root: './sources/app' } },
  },
};
