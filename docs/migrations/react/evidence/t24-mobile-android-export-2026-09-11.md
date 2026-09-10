# T24 Android mobile bundle evidence — 2026-09-11

## Red → green

The first Android export failed before Metro could produce a bundle:

```text
Unable to resolve module @expo-google-fonts/material-symbols
from .../expo-symbols/build/SymbolView.js
```

`expo-symbols@55.0.9` declares this Android font package in the resolved
workspace lockfile, but `apps/mobile/package.json` did not declare it directly;
pnpm therefore did not expose the package through the mobile importer.

The minimal fix added the already-resolved compatible package
`@expo-google-fonts/material-symbols@0.4.47` to the mobile dependencies. A
lockfile-only refresh followed by `pnpm install --offline --frozen-lockfile`
installed the direct link without downloading new packages.

## Green export

```text
pnpm --filter @weknora/mobile exec expo export \
  --platform android \
  --output-dir /tmp/weknora-react-mobile-android-export-20260911-rerun
```

Result:

- exit code `0`;
- Metro bundled `1,130` modules;
- one Android Hermes bundle was emitted at approximately `3 MB`;
- 31 referenced assets were emitted, including the Material Symbols font
  variants;
- `metadata.json` was emitted.

## Boundary

This proves Android JS/native-module graph exportability. It does not prove an
Android APK/AAB install, emulator/device launch, live authentication, AppState
recovery, file picker/upload, or backend permission behavior. Those remain
native runtime evidence gaps for T20–T24.
