# Ticket #31 / Task 7 — native Runtime login build gates

## Scope and gate wiring

No root script or workflow edit was needed. `package.json` already maps
`test:mobile` and `typecheck:mobile` to `@weknora/mobile`, and
`.github/workflows/mobile.yml` already installs with `pnpm install
--frozen-lockfile`, runs the React-boundary check, mobile test/typecheck, and
separate iOS and Android Expo exports.

The operator checklist is
`docs/testing/mobile-runtime-login-device-acceptance.md`. It deliberately
leaves device acceptance pending and provides separate password, OIDC,
origin-validation, and protocol/capability-gate evidence rows for both
platforms. Its evidence template contains only redacted operational metadata;
it prohibits credential, token, authorization-code, verifier, cookie, request,
and raw-response capture.

## Fresh local verification (2026-09-21)

```text
pnpm install --frozen-lockfile
# exit 0
# Lockfile is up to date, resolution step is skipped

pnpm test:mobile
# exit 0; tests 9 / pass 9 / fail 0

pnpm typecheck:mobile
# exit 0

pnpm --filter @weknora/mobile exec expo export --platform ios \
  --output-dir /tmp/weknora-mobile-ios-task7.6DNn1H
# exit 0; 1 iOS Hermes bundle, 1124 modules

pnpm --filter @weknora/mobile exec expo export --platform android \
  --output-dir /tmp/weknora-mobile-android-task7.RiaROs
# exit 0; 1 Android Hermes bundle, 1147 modules

node scripts/check-react-boundaries.mjs
# exit 1; 227 existing failures, beginning with
# "React surface reaches the legacy frontend source" in shared/web files such
# as packages/api-client/src/chat/sessions.ts and packages/domain/src/auth/onboarding.ts

git diff --check
# exit 0
```

The Expo outputs are temporary JavaScript bundles and are not native-device
acceptance evidence. The boundary check failure does not arise from a Task 7
script/workflow change: this task made none. It is a repository-wide existing
legacy-source boundary condition that must be remediated by its owning work.

## Installed-device and staging availability

Checked without printing environment values:

```text
xcodebuild -version
# Xcode 27.0 (27A266a)

xcrun simctl list devices available
# 11 available iOS simulators

test -d apps/mobile/ios; test -d apps/mobile/android
# both native project directories unavailable

command -v adb
# unavailable

command -v eas
# unavailable

environment-name scan for WEKNORA_MOBILE_TEST_*, EXPO_*, and EAS_*
# no configured staging/mobile credential variables
```

No installed WeKnora iOS or Android development build, Android device bridge,
EAS build client, authorized staging Deployment, or staging credential is
available in this checkout. Therefore password login, OIDC completion and
negative callback handling, and protocol/capability behavior have not been
executed on a native device or simulator. They remain **unavailable**, not
successful.

Task 6's limitation remains unchanged: it found no local HTTPS fixture or
protected staging credentials. See `task-6-report.md` for the redacted opt-in
HTTP harness; it cannot substitute for the installed-device acceptance listed
in the Task 7 checklist.

## TDD

This task adds operational documentation only and makes no behavior or script
change, so a RED/GREEN implementation cycle is not applicable.
