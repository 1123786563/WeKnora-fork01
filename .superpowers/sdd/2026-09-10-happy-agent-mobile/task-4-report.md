# Task 4 build verification

Date: 2026-09-11

## Green checks

- `pnpm --filter @weknora/mobile typecheck` — passed.
- `pnpm --filter @weknora/mobile exec expo export --platform ios` — passed; Metro bundled 3,699 modules and emitted the iOS Hermes bundle.
- `pnpm --filter @weknora/mobile exec expo export --platform android` — passed; Metro bundled 3,698 modules and emitted the Android Hermes bundle.

The workspace now includes the complete Happy app image/font closure used by static `require()` calls, `theme.css`, and the changelog image. Metro maps `@slopus/happy-wire` to its workspace source, while TypeScript resolves the source types without requiring generated `dist` artifacts.

## Native run limits

- `pnpm --filter @weknora/mobile exec expo run:ios --no-install` reached Xcode and failed with exit code 65 because the generated native project Pod sandbox is out of sync with `Podfile.lock`; `pod install` is required on a machine with the configured CocoaPods installation.
- `pnpm --filter @weknora/mobile exec expo run:android --no-install` stopped before Gradle because no Android device or emulator was connected/available for Expo to start.

These are host toolchain/device prerequisites; JavaScript typecheck and both platform exports are green.
