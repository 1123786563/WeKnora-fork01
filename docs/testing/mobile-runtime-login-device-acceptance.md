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
  "loginFlow": "password | oidc",
  "scenario": "authorized | rejected-origin | oidc-cancel | protocol-gate",
  "clientProtocol": 3,
  "capabilityMode": "compatible | incompatible | malformed | missing",
  "outcome": "accepted | rejected | blocked | unavailable",
  "evidencePath": "docs/testing/evidence/mobile-runtime-login/YYYY-MM-DD/file.md",
  "timestamp": "RFC 3339 timestamp",
  "operator": "approved operator alias"
}
```

## Current Task 7 status (2026-09-21)

Neither platform is accepted. This checkout has iOS simulators and Xcode, but
does not contain an iOS/Android native project or an installed WeKnora
development build. `adb` and the EAS CLI are unavailable. No staging
deployment or mobile test credential variable is configured. Consequently no
password or OIDC flow, callback behavior, or protocol gate has been exercised
on an installed native build.

Task 6 also found no local HTTPS fixture or protected staging credentials; see
`.superpowers/sdd/2026-09-20-t01-mobile-runtime-login/task-6-report.md` for
the safe opt-in HTTP harness and its limitation. The Expo exports and local
test/typecheck results must remain build evidence only.
