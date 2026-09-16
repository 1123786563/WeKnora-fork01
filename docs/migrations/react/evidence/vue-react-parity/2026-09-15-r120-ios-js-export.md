# 2026-09-15 iOS JavaScript export evidence

- Command: `pnpm --dir apps/mobile exec expo export --platform ios --output-dir /tmp/weknora-ios-export`.
- Result: Expo Metro bundled 1215 modules and exported the iOS Hermes bundle plus metadata to `/tmp/weknora-ios-export`.
- Boundary: JavaScript export is distinct from native compile/install/launch and simulator business/OIDC callback acceptance.
