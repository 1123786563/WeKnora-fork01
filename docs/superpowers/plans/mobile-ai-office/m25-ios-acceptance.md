# M25 iOS Real Cloud Acceptance Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`.

**Goal:** Record a real iOS-device acceptance result against authenticated WeKnora cloud and a real persistent sandbox, or an explicit blocked condition.

**Architecture:** This is a non-business acceptance gate after M24. It uses the integrated artifact, actual iOS device, real account/tenant, cloud backend, and provider credentials where exercised. Static, Jest, simulator, and mock results remain supporting evidence only.

**Tech Stack:** integrated Expo iOS artifact, iOS device, WeKnora cloud, persistent sandbox, selected code platform.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; Parent #3.

## Global Constraints

- Blocked by M24; no scope, interface, or business-code change is permitted.
- Validate backend/accountId/tenantId/generation and Task=sessionId with real non-production acceptance identities.
- Do not invent device commands: use only device/CI commands documented by the available acceptance environment.
- Missing device, signing, cloud, sandbox, or provider credential is `blocked-env` with owner and recovery condition; it cannot be skipped.
- Do not claim Flutter removal/cutover from this gate.

## Review Focus

1. Task persists through iOS background/reopen and renders the real current scope.
2. Owner/viewer and revoked access are rechecked server-side.
3. Publication remains after Task unshare while Git push/PR retains exact approval/reconcile behavior.
4. Run/PTY interlock and quota failure remain visible, not silently retried.
5. Lock-screen notification contains state only, no title/body.

### Task M25: iOS real acceptance ledger

**Files:**
- Create: `docs/evidence/mobile-ai-office/m25-ios-acceptance.md`.
- Test: evidence review against M24 artifact identity and M01–M23 acceptance matrix; no source-code test is substituted.

**Interfaces:**
- Consumes: M24 `ReleaseEntry` and integrated M01–M23 API/mobile contracts.
- Produces: `IOSAcceptanceEvidence{ArtifactRevision,DeviceModel,IOSVersion,Backend,AccountScope,Scenarios,Status,BlockedReason}` (proposed evidence schema).

- [ ] **Step 1: Establish preconditions.** Record artifact revision, physical device model/iOS version, authenticated acceptance backend/account/tenant, reachable persistent sandbox, selected provider Connection, and notification permission. If any is absent, write `blocked-env` and stop dependent scenarios.
- [ ] **Step 1a: Discover and build/install from M24 runbook.** Read `docs/evidence/mobile-ai-office/m24-device-runbook.md`, identifiers resolved from `app.config.ts`, artifact hash, and recorded physical `IOS_DEVICE_ID`. Run `npm run ios -- --device "$IOS_DEVICE_ID"` and the runbook’s installed `xcrun devicectl` log command verbatim. If its tool/signing/device check is blocked, record `blocked-env: missing-ios-runbook-or-signing` and do not substitute a simulator.
- [ ] **Step 2: Execute real scenarios.** On that physical device: create/continue a session Task; background/reopen; verify Owner vs reader and revoke refresh; publish/read immutable artifact; clone/status/diff/local commit; approve exact push and PR; exercise unknown-result query only when a safe test-provider fault exists; inspect quota and Run/PTY behavior.
- [ ] **Step 3: Verify notification privacy.** Trigger completed/failed/pending approval and record that lock screen has state only, with no task title/body. Open it and verify server permission recheck before content.
- [ ] **Step 4: Record evidence.** For every scenario record real timestamp, sessionId/runId redacted as needed, expected/actual result, API/device screenshot/log reference, and backend/sandbox classification. Mark failures; never convert them to pass with manual notes.
- [ ] **Step 5: Gate decision.** Mark `passed` only if every required scenario has real proof. Otherwise retain `blocked-env`/`failed` with recovery condition and hand M26 the unchanged matrix.

## Review Gate

Reject simulator-only evidence, fabricated commands/screenshots, omitted device identity, or a status that hides missing real-cloud prerequisites.

## Required record per scenario

- [ ] Device model, iOS version, installed artifact revision, timestamp, backend name and tenant/account scope.
- [ ] SessionId/runId or safely redacted correlator, expected behavior, observed behavior and proof reference.
- [ ] Whether proof is real device, real cloud API, real persistent sandbox, or supporting static result.
- [ ] Owner and reader authorization outcomes and post-revoke outcome.
- [ ] Exact approval snapshot and remote reconciliation state for Git scenarios.
- [ ] Notification screen evidence confirming no title/body exposure.
- [ ] Final status `passed`, `failed`, or `blocked-env`, with recovery condition for every non-pass.

## Handoff

- [ ] Do not change any feature after an acceptance failure; report the reproducer to its owning ticket.
- [ ] M26 runs independently on Android and cannot reuse iOS screenshots as evidence.
- [ ] Controller collects only completed evidence records for final gate review.
