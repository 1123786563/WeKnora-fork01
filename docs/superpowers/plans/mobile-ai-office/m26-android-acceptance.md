# M26 Android Real Cloud Acceptance Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`.

**Goal:** Record a real Android-device acceptance result against authenticated WeKnora cloud and persistent sandbox, or an explicit blocked condition.

**Architecture:** This independent platform gate consumes M24’s frozen integrated artifact. It verifies real Android behavior, not mobile unit/build output, and writes only evidence. No business work, cutover, or environment workaround belongs here.

**Tech Stack:** integrated Expo Android artifact, physical Android device, WeKnora cloud, persistent sandbox, selected code platform.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; Parent #3.

## Global Constraints

- Blocked by M24. Scope is backend/accountId/tenantId/generation; Task=sessionId.
- Actual device/cloud/sandbox/provider prerequisites are mandatory. If `adb` or comparable environment support is unavailable, report `blocked-env`; do not claim a run.
- Use only commands supplied by the concrete device/CI environment; this plan intentionally does not fabricate Android automation commands.
- No source/business changes, no Flutter removal, no public release, and no fake acceptance from Jest/export.

## Review Focus

1. Background/reopen resumes real session snapshot without cross-scope cache data.
2. Revoked reader loses access after reconnect while independent publication remains readable by target ACL.
3. Exact push/PR approval and unknown query have no duplicate remote write.
4. Active Run blocks interactive PTY edits and stopping is confirmed before write access.
5. Android lock screen reveals no Task title/body.

### Task M26: Android real acceptance ledger

**Files:**
- Create: `docs/evidence/mobile-ai-office/m26-android-acceptance.md`.
- Test: evidence review against M24 artifact identity and M01–M23 scenario matrix only.

**Interfaces:**
- Consumes: M24 `ReleaseEntry` and integrated M01–M23 contracts.
- Produces: `AndroidAcceptanceEvidence{ArtifactRevision,DeviceModel,AndroidVersion,Backend,AccountScope,Scenarios,Status,BlockedReason}` (proposed evidence schema).

- [ ] **Step 1: Precondition ledger.** Record physical device/Android version, install artifact revision, acceptance backend/account/tenant, persistent sandbox, provider Connection and notification permission. Missing one is a named `blocked-env` record with recovery condition.
- [ ] **Step 1a: Discover and build/install from M24 runbook.** Read `docs/evidence/mobile-ai-office/m24-device-runbook.md`, identifiers resolved from `app.config.ts`, artifact hash, and selected serial from `adb devices -l`. Execute `npm run android -- --device "$ANDROID_DEVICE_SERIAL"` and `adb -s "$ANDROID_DEVICE_SERIAL" logcat` verbatim. If `adb`, signing, physical device, or committed command source is absent, record `blocked-env` with that exact condition; do not use emulator output.
- [ ] **Step 2: Execute real Task flow.** Create/continue Task, background/reopen, verify scoped Home/Task state, Owner/read-only/revoke behavior, immutable publication read, clone/local commit, approved push/PR, quota, and Run/PTY mutual exclusion.
- [ ] **Step 3: Execute privacy/recovery checks.** Confirm state-only notification, permission recheck on open, scope-cache clearing after tenant/account change, and unknown remote outcome is queried rather than retried where safe test setup supports it.
- [ ] **Step 4: Evidence recording.** Save timestamp, artifact revision, environment identity, scenario expected/actual, real device screen/log and corresponding cloud/run reference; classify static/mock evidence separately.
- [ ] **Step 5: Gate decision.** Mark pass only with complete real evidence; otherwise record failed/blocked-env and exact recovery prerequisite. Do not skip or infer device success.

## Review Gate

Reject an emulator/mock-only result, unspecified device/cloud target, missing `adb` disguised as pass, or a cutover conclusion.

## Required record per scenario

- [ ] Device model, Android version, installed artifact revision, timestamp, backend name and tenant/account scope.
- [ ] SessionId/runId or safely redacted correlator, expected behavior, observed behavior and proof reference.
- [ ] Whether evidence is real device, real cloud API, real persistent sandbox, or merely supporting static output.
- [ ] Owner/reader/revoked authorization results and retained publication ACL result.
- [ ] Exact push/PR approval values and unknown-result query outcome when safely runnable.
- [ ] PTY/Run and quota outcomes plus notification privacy evidence.
- [ ] Final `passed`, `failed`, or `blocked-env` status with recovery condition.

## Handoff

- [ ] Do not patch business code inside this gate; return a reproducible failure to the owning ticket.
- [ ] Do not reuse iOS proof as Android proof or infer a pass from Web export/Jest output.
- [ ] Controller aggregates completed iOS and Android records for final review only.
