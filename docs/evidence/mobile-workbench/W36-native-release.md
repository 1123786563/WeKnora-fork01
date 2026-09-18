# W36 Native release, compatibility window and layered acceptance

Date: 2026-09-17
Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`
Base: dispatched `86909e37` (worktree HEAD at task start was the same
`86909e37`; parallel tasks hold unstaged edits under `internal/voice`,
`internal/handler/mobile_voice.go`, `migrations/*voice*` and `apps/web`
test files — none touched by this task).

## Deliverables

- `packages/domain/src/mobile/compatibility.ts` — the compatibility window:
  `protocolMode(client, minimum, maximum)` tri-state (`full` /
  `upgrade_required` / `server_upgrade_required`) with
  `INVALID_PROTOCOL_RANGE` defense (non-integer generations, `< 1`, or
  `minimum > maximum` are rejected, never guessed); `CLIENT_PROTOCOL_VERSION`
  (= 3) and `SERVER_PROTOCOL_WINDOW` (`[2, 3]`, covering the previous
  generation); wire parsing (`protocol_minimum` / `protocol_maximum`) that
  yields `undefined` on unknown/malformed schema; `clientGate` (control
  commands allowed in `full` mode only; unknown schema → safe
  login+upgrade surface); client-side mirrors of the W34 switch semantics
  (`mayQueryRuns` / `mayAdmitNewRun` / `mayCleanUpRuns` /
  `normalizeWorkbenchCapabilitySnapshot`).
- `packages/domain/src/mobile/compatibility.test.ts` — brief-literal RED
  test preserved verbatim plus: malformed-window rejections, control-command
  gating, unknown-schema safe surface, expand/contract floor (window covers
  the previous generation), the compatibility tri-state harness (old app +
  new API / new app + minimum API / rollback with W34 read-gate and drain
  semantics), and the JS-layer performance harness (see
  `W36-performance.md`).
- `apps/mobile/package.json` — wired `export:ios` / `export:android`
  scripts (`expo export --platform ios|android`). No dependency changes.
- `deploy/mobile-workbench/README.md` — W36 section: compatibility window
  (actual behaviour), upgrade/rollback procedure, release manifest (pinned
  versions), expand/contract principle, the native-release rule (native
  changes cannot ship as JS updates), native-payment declaration.
- `docs/evidence/mobile-workbench/W36-native-release.md` (this file) and
  `W36-performance.md`.
- `pnpm-lock.yaml` — NOT modified (no dependency changed; verified unchanged
  after `--frozen-lockfile` install).

`compatibility.ts` is not yet re-exported from
`packages/domain/src/mobile/index.ts` (outside this task's file ownership)
and has no production consumer yet — the acceptance is harness-level by
design; wiring a consumer belongs to the lane that adds the capabilities
endpoint surface.

## RED (task behaviour, not environment failures)

```text
pnpm exec tsx --test packages/domain/src/mobile/compatibility.test.ts
Error [ERR_MODULE_NOT_FOUND]: Cannot find module '.../packages/domain/src/mobile/compatibility.ts'
✖ packages/domain/src/mobile/compatibility.test.ts
ℹ fail 1
```

Transcript: `/tmp/w36-red.txt` — `protocolMode` undefined, exactly the
brief's expected RED.

## GREEN

```text
pnpm exec tsx --test packages/domain/src/mobile/compatibility.test.ts
ℹ tests 11  pass 11  fail 0        # exit 0
```

Transcript: `/tmp/w36-green.txt` (plus three stability re-runs, all exit 0).
Sibling domain-mobile suites unchanged: `auth-return.test.ts` +
`execution-cache.test.ts` → 6/6 pass (exit 0).

## Layered acceptance (each layer recorded separately)

| # | Layer | Command (abridged) | Exit | Result |
| --- | --- | --- | --- | --- |
| 1 | Lockfile install | `pnpm install --frozen-lockfile` | 0 | installs from the frozen lockfile; `git status pnpm-lock.yaml` clean afterwards (lockfile not modified — no dependency changed) |
| 2 | Shared regression | `pnpm test:shared` | 0 | 603/603 pass. Note: the root glob does not include `packages/domain/src/mobile/*.test.ts`; that suite is run by its explicit command (W12/W27 precedent) — rows 3–4 |
| 3 | New compatibility suite | `pnpm exec tsx --test packages/domain/src/mobile/compatibility.test.ts` | 0 | 11/11 pass (tri-state + perf harness) |
| 4 | Sibling domain-mobile suites | `pnpm exec tsx --test packages/domain/src/mobile/{auth-return,execution-cache}.test.ts` | 0 | 6/6 pass |
| 5 | Mobile typecheck | `pnpm typecheck:mobile` (`tsc --noEmit` in apps/mobile) | 0 | no errors |
| 6 | Expo export, iOS | `npx expo export --platform ios` (script `export:ios`) | 0 | Hermes bundle `_expo/static/js/ios/index-9ec8adb3ad1496eae2f9545cb5f3b7a8.hbc` (4.5MB) + `metadata.json`; output `/tmp/w36-export-ios` |
| 7 | Expo export, Android | `npx expo export --platform android` (script `export:android`) | 0 | Hermes bundle `_expo/static/js/android/index-8faf53af1970d59d788a45081b7551a3.hbc` (4.6MB) + `metadata.json`; output `/tmp/w36-export-android` |
| 8 | Native Debug build | — | — | **blocked-env**: no `ios/`/`android/` native projects exist in the worktree (`expo prebuild` has not been run; the prebuild script `rm -rf android ios` would also write untracked trees into the shared worktree); machine is shared with parallel tasks (load average >100 during this task) so a full prebuild + `pod install` + `xcodebuild`/`gradle` build is not a reliable verification window here |
| 9 | Native Release build + signing | — | — | **blocked-env**: `security find-identity -p codesigning -v` → `0 valid identities found` (no distribution/signing identity on this machine). Xcode 27.0 and CocoaPods 1.17.0 ARE present; iOS simulators exist (iPhone 17 family); Android SDK platforms 33–37 exist but `ANDROID_HOME` is unset |
| 10 | Device install + screen-by-screen interaction | — | — | **blocked-env**: no signing identity and no physical device/crippled native project state (see 8–9) |
| 11 | Store submission | — | — | **blocked-env**: follows from 8–10; additionally the native purchase entrypoint stays closed (see below) |

No verification above was recorded as passing without its command and exit
status; every unavailable layer is labelled blocked-env, not passed. No
Metro dev-server run was substituted for any native layer.

## Compatibility tri-state harness (code-level, in compatibility.test.ts)

| State | Setup | Assertions |
| --- | --- | --- |
| Old App + new API | app protocol generation 1 vs server window `[2,3]` | `upgrade_required`; login still ok; safe surface `login-upgrade` (upgrade explanation retained); client sends NO control command and server receives none; the app does not enter the workbench surface; a tampered direct command attempt is refused server-side too |
| New App + minimum API | app generation 4 vs server window `[2,3]` | `server_upgrade_required`; same safe-surface degradation, no control commands sent or received |
| In-window control path | app generation 3 vs window `[2,3]` | `full`; command accepted and recorded server-side |
| Rollback: new admission closed | window `[2,3]`, `platform_admission: false` (and separately `worker_drain: true`) | NEW admission refused; existing runs still queryable (read gate untouched); existing run still cleanable — consuming the W34 read-gate/drain semantics via the `mayQueryRuns`/`mayAdmitNewRun`/`mayCleanUpRuns` mirrors of `internal/config` `workbench.*` |
| Read gate closed (control case) | `read_enabled: false` | queries stop; cleanup still available (one switch never cuts query and cleanup at once) |
| Unknown capability schema | wire payload without a parseable window | `unknown_schema`: control commands refused, safe login+upgrade surface retained — never guessed into an allow decision |

## Release manifest (pinned; mirror of the README table)

| Item | Value |
| --- | --- |
| Resolved Expo config | `apps/mobile/app.config.ts` — verified via `npx expo config --type public` (name `WeKnora`, version `0.0.0`, slug `weknora`); the sibling `app.config.js` (dev variant, version `0.1.0`, `com.weknora.mobile.dev`, `supportsTablet: true`) is NOT what Expo resolves |
| Bundle identifiers | iOS `com.weknora.mobile` / Android `com.weknora.mobile` |
| Expo SDK | 55 (resolved `expo@55.0.31`; spec `~55.0.8`) |
| React Native | `0.83.1` |
| React | `19.3.0` |
| JS update bundles | `expo export` per platform (Hermes `.hbc` + `metadata.json`) — JS/assets only |

Expand/contract (documented in the README, encoded in the tests): widening
the window keeps every existing app `full`; the window minimum covers the
previous protocol generation; contracting the minimum and destructive
old-app-data cleanup are deliberate release decisions gated on user upgrade
plus compatibility-data validation. Native dependency changes require a new
native release (build + sign + store review) — a JS-only `expo export`
update never covers a native runtime change.

## Native payments

The purchase entrypoint stays closed: the app carries entitlement-sync code
only (`apps/mobile/sources/sync/purchases.ts` parses subscription/
entitlement state; no store purchase flow is exposed). Web payment flows are
product surfaces and are NOT recorded as store acceptance for the mobile
lane. Recorded in `deploy/mobile-workbench/README.md` (W36 section).

## Device-matrix layering (large font / keyboard / landscape / tablet / share / push / audio)

Real-device verification of all seven items: **blocked-env** (rows 8–10
above). Code-level coverage that exists today (component/unit tests in
`apps/mobile/sources`, all green under `pnpm --filter @weknora/mobile test`
ownership of their lanes):

- Keyboard: `keyboard/shortcuts.test.ts`, `components/multiTextInputLayout.test.ts`,
  `components/agentInputLayout.test.ts` (input layout semantics;
  `react-native-keyboard-controller` is the runtime dependency).
- Large font / layout math: `utils/deviceCalculations.test.ts`,
  `navigation/headerMetrics.test.ts`, `utils/newSessionSidebarLayout.test.ts`,
  `utils/duplicateSheetLayout.test.ts`.
- Landscape/tablet: the resolved release config locks `orientation` to
  `portrait` and does not set `ios.supportsTablet` (iPad renders in
  iPhone-compatibility mode); the dev-only `app.config.js` sets
  `supportsTablet: true` but is not the resolved config. Real-device
  landscape/tablet acceptance is therefore not applicable to this build
  configuration and remains blocked-env for any future config that enables it.
- Push: `utils/notificationRouting.test.ts` (routing semantics;
  `expo-notifications` runtime dependency).
- Audio/voice: `weknora/voice/dictation.test.ts` (dictation semantics;
  `expo-audio` runtime dependency) — the voice LANE itself is
  blocked-dependency (W31, below).
- File share: `expo-sharing` is a runtime dependency; no dedicated
  share-sheet component test exists — nothing code-level is claimed here,
  real-device share verification stays blocked-env.

## Sub-gate status

| Sub-gate | Status | Basis |
| --- | --- | --- |
| core (this task's verifiable part) | DONE | compatibility module + tri-state harness + rows 1–7 of the layered table |
| Artifacts (产物) | recorded against W27 baseline | W27 code landed as `e1633cdd` (preview-policy/artifact preview, in-tree and green in this task's runs); W27 review outcome is tracked separately by the coordinator |
| Voice (语音) | blocked-dependency | W31 not implemented; voice admission switch (`workbench.voice_admission`) has no wired entrypoint yet (W30 handler/migration work is in-flight in parallel and untouched by this task) |
| Advanced capabilities (高级能力) | blocked-dependency | W32 not implemented |
| Native build / signing / real device / store | blocked-env | rows 8–11 above |

## blocked-env / blocked-dependency summary

- Native Debug/Release builds, device install, screen-by-screen interaction,
  store submission — no native projects in the worktree, zero codesigning
  identities, shared-machine load; facts recorded in rows 8–11.
- Real-device runs of the seven device-matrix items — same basis.
- Native-runtime performance measurement — see `W36-performance.md`.
- Voice lane (W31) and advanced capabilities (W32) — blocked-dependency, not
  environment.
