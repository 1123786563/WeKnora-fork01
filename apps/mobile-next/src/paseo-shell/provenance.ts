export interface PaseoSource {
  readonly upstreamPath: string;
  readonly upstreamSha: string;
  readonly license: "Apache-2.0";
  readonly copiedAt: string;
  readonly localChanges: string;
}

export const PASEO_SOURCES: readonly PaseoSource[] = [
  {
    upstreamPath: "packages/app/src/components/headers/screen-title.tsx",
    upstreamSha: "d636abd7a4ce302e7ccb9eb6074f637c6dd4d83b",
    license: "Apache-2.0",
    copiedAt: "2026-09-20",
    localChanges: "Replaced react-native-unistyles theme access with the app runtime theme and React Native StyleSheet.",
  },
];
