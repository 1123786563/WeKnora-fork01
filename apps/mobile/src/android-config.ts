type AndroidManifestLike = {
  manifest: {
    application?: Array<{ $?: Record<string, string | undefined> }>;
  };
};

export function applyAndroidCleartextTraffic<T extends AndroidManifestLike>(manifest: T): T {
  const application = manifest.manifest.application?.[0];
  if (!application) throw new Error('Android manifest has no application entry');
  application.$ ??= {};
  application.$['android:usesCleartextTraffic'] = 'true';
  return manifest;
}
