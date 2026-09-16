const { withAndroidManifest } = require('@expo/config-plugins');

module.exports = function withAndroidCleartextTraffic(config) {
  return withAndroidManifest(config, (mod) => {
    const application = mod.modResults.manifest.application?.[0];
    if (!application) throw new Error('Android manifest has no application entry');
    application.$ = {
      ...application.$,
      'android:usesCleartextTraffic': 'true',
    };
    return mod;
  });
};
