# Native device Runtime login acceptance

This checklist records installed iOS and Android development-build acceptance
against an authorized WeKnora staging Deployment. It is separate from unit
tests, Expo JavaScript exports, and HTTP integration tests. Do not mark a
platform accepted from any of those substitutes.

## Preconditions

- Use a development build installed on the target device or simulator; Expo Go
  is not device acceptance evidence.
- Use a short-lived staging test account and an HTTPS deployment origin with no
  path, query, fragment, or embedded credentials.
- Record only the redacted origin, build identifier, application version,
  platform/OS, timestamp, protocol/capability result, and result category.
- Store screenshots, screen recordings, and command logs under
  `docs/testing/evidence/mobile-runtime-login/<yyyy-mm-dd>/`. Redact account
  identifiers, bearer/refresh tokens, authorization codes, PKCE verifiers,
  cookies, request bodies, and server responses before committing evidence.

## Per-platform checklist

Complete every item independently for **iOS** and **Android**. Record the
evidence file paths next to each result.

| Check | iOS result and evidence | Android result and evidence |
| --- | --- | --- |
| Development build installed; package/bundle identifier and build identifier recorded | Pending | Pending |
| HTTPS staging Deployment entered; origin normalizes successfully | Pending | Pending |
| Non-HTTPS, path-bearing, query/fragment-bearing, and credential-bearing origins remain rejected before login | Pending | Pending |
| Password login reaches the authorized landing with a server-issued identity and active Tenant | Pending | Pending |
| Password sign-out removes the authenticated landing; relaunch does not reuse the prior session | Pending | Pending |
| OIDC entry uses the registered callback only; a successful returned authorization completes login | Pending | Pending |
| OIDC cancel, callback mismatch, state mismatch, and provider failure leave no authorized state | Pending | Pending |
| Compatible capability result permits authorized landing | Pending | Pending |
| Incompatible/malformed/missing capability result reaches upgrade-required state and exposes no authorized controls | Pending | Pending |
| Evidence path, operator, redacted deployment origin, protocol version, capability mode, and timestamp recorded | Pending | Pending |

## Evidence record template

Create one record per scenario. `outcome` may be `accepted`, `rejected`,
`blocked`, or `unavailable`; it must never be inferred from a JavaScript
export. A record marked `accepted` requires the corresponding redacted native
device evidence file.

```json
{
  "platform": "ios | android",
  "deviceOrSimulator": "redacted model and OS version",
  "build": "bundle/package identifier and build identifier",
  "deploymentOrigin": "https://staging.example",
  "loginFlow": "password | oidc | setup | protocol",
  "scenario": "development-build-installed | deployment-origin-normalized | origin-rejected-non-https | origin-rejected-path | origin-rejected-query-or-fragment | origin-rejected-embedded-credentials | password-authorized | password-sign-out | password-relaunch-session-cleared | oidc-authorized | oidc-cancel | oidc-callback-mismatch | oidc-state-mismatch | oidc-provider-failure | capability-compatible | capability-incompatible | capability-malformed | capability-missing | evidence-recorded",
  "clientProtocol": 3,
  "capabilityMode": "compatible | incompatible | malformed | missing",
  "outcome": "accepted | rejected | blocked | unavailable",
  "evidencePath": "docs/testing/evidence/mobile-runtime-login/YYYY-MM-DD/file.md",
  "timestamp": "RFC 3339 timestamp",
  "operator": "approved operator alias"
}
```

## Current Task 7 status (2026-09-29): iOS 27 scene build in progress

The Expo SDK 57 generated iOS project contract now passes in the isolated
prebuild at `/tmp/weknora-issue31-final-prebuild-_x7w61dy/ios`: its
`UIApplicationSceneManifest` names `EXExpoAppSceneDelegate`, its AppDelegate
conforms to `ExpoReactNativeFactoryProvider` without creating a React Native
window in `didFinishLaunchingWithOptions`, and generated Xcode/Pod deployment
targets are all 16.4. CocoaPods installation completed for that project.

The iOS 27 simulator build is running with the booted iPhone 18 Pro simulator
(`0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`). Its command log is
`/tmp/issue31-xcodebuild.log`; install, launch, screenshot and launch-log
inspection will be recorded after the build completes. JavaScript exports are
build evidence only: iOS output is under `/tmp/issue31-ios-export` and Android
output is under `/tmp/issue31-android-export`.

The simulator gate does not exercise a real staging login. No staging
Deployment or short-lived mobile credentials are configured, and no Android
device acceptance has been performed. Therefore password/OIDC/capability
flows on staging and Android installation remain Pending. Task 6 also found no
local HTTPS fixture or protected staging credentials; see
`.superpowers/sdd/2026-09-20-t01-mobile-runtime-login/task-6-report.md` for the
safe opt-in HTTP harness and its limitation. No credentials were inspected or
added to source, logs, screenshots or evidence.
