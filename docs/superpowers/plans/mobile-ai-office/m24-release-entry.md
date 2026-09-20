# M24 Release Entry and Cutover Preparation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`.

**Goal:** Produce reproducible mobile-next build-entry evidence and a reversible release preparation while retaining existing Web and Flutter clients until real platform gates pass.

**Architecture:** After M01–M23 are integrated, freeze the installed mobile container’s scripts/source provenance, build the current Paseo Web export only as a build check, and document an explicit non-cutover state. This is release preparation, not a business feature or client replacement.

**Tech Stack:** existing `apps/mobile-next` npm scripts, Expo 55, repository scripts.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; Parent #3.

## Global Constraints

- Blocked by M01–M23; controller checks all upstream integration states before starting.
- Use existing `npm run typecheck`, `npm test`, `npm run check:isolation`, and `npm run export:web` from `apps/mobile-next/package.json`; do not invent root scripts.
- Paseo Web is build-only; existing WeKnora Web is retained. Flutter remains until M25/M26 actual gates have passed and an authorized later cutover is made.
- Record exact runtime/package hashes and failures. Do not call an export, simulator, or static test a device/cloud success.
- Root manifests/lockfiles remain controller-owned; this ticket documents and verifies current state only.

## Review Focus

1. Export succeeds without replacing existing Web serving.
2. Isolation check catches imports from old mobile/Happy business code.
3. Missing platform credentials/device stays blocked, not skipped.
4. Build evidence names installed runtime and git revision.
5. Cutover checklist has rollback/parallel-install state.

### Task M24: build evidence and non-cutover release entry

**Files:**
- Create: `apps/mobile-next/release-entry.json`, `apps/mobile-next/tests/release-entry.test.ts`, `docs/evidence/mobile-ai-office/m24-device-runbook.md`.
- Create: `docs/evidence/mobile-ai-office/m24-release-entry.md`.
- Controller only: root build aliases, workspace manifests, lockfile, release switch.

**Interfaces:**
- Consumes: integrated M01–M23 evidence IDs and current `apps/mobile-next/package.json` scripts.
- Produces (proposed): `ReleaseEntry{GitRevision,NodeVersion,NpmVersion,ExpoVersion,Commands,WebExportState,FlutterRetained,PlatformGateState}`.
- M24 fixes artifact identity as Git revision plus identifiers resolved from existing `apps/mobile-next/app.config.ts`, and produces `docs/evidence/mobile-ai-office/m24-device-runbook.md`. The runbook first records installed-tool checks: `npm exec --offline -- expo --help`, `xcrun devicectl help`, and `adb help`; missing command/tool is `blocked-env`. For a recorded physical `IOS_DEVICE_ID`, its iOS command is `npm run ios -- --device "$IOS_DEVICE_ID"`; for a selected Android serial from `adb devices -l`, it is `npm run android -- --device "$ANDROID_DEVICE_SERIAL"`. Logs use installed `xcrun devicectl` for the recorded iOS target and `adb -s "$ANDROID_DEVICE_SERIAL" logcat`, never `npx` auto-download. It records artifact hash after output exists; missing Xcode/Android SDK/device/signing is explicitly `blocked-env`.

- [ ] **Step 1: RED.** Add a test requiring all four commands and `FlutterRetained=true`.

```ts
it("records build-only Web and retained Flutter", () => {
 expect(entry.commands).toEqual(expect.arrayContaining(["npm run typecheck","npm test","npm run check:isolation","npm run export:web"]));
 expect(entry.flutterRetained).toBe(true);
});
```

- [ ] **Step 2: Run RED.** `cd apps/mobile-next && npm test -- --runInBand release-entry.test.ts`; expected failure before entry exists.
- [ ] **Step 3: Implement evidence file.** Capture `git rev-parse HEAD`, `node --version`, `npm --version`, and command exit/status/time. Do not fabricate outputs.
- [ ] **Step 4: Run actual build checks and write device runbook.** Execute the four listed npm commands sequentially from `apps/mobile-next`; resolve identifiers from `app.config.ts`; run/record `npm exec --offline -- expo --help`, `xcrun devicectl help`, `adb help`, and `adb devices -l`. Only after recording physical IDs run `npm run ios -- --device "$IOS_DEVICE_ID"` and `npm run android -- --device "$ANDROID_DEVICE_SERIAL"`; collect logs with installed `xcrun devicectl` and `adb -s "$ANDROID_DEVICE_SERIAL" logcat`. Record actual exit/output/hash or precise `blocked-env`; never invoke `npx` to fetch a CLI.
- [ ] **Step 5: GREEN/review/demo.** Re-run entry test, inspect diff confirms no `apps/mobile`/Web replacement, and hand M25/M26 the exact evidence plus blockers.

## Review Gate

Reject a cutover, deletion, root lockfile mutation by this track, or device-pass claim from export output.

## Evidence format

- [ ] List command, working directory, start/end timestamp, exit code and immutable artifact path for each build check.
- [ ] List current git revision and working-tree state before execution; preserve other-agent modifications.
- [ ] Record Node/npm/Expo versions exactly as observed, not assumed from the plan.
- [ ] State whether `apps/mobile` and the existing Web entry remain reachable after every check.
- [ ] State iOS/Android conditions separately as pending M25/M26, with no implicit pass.
- [ ] If a command fails, keep its complete failure classification and recovery owner in the evidence file.

## Controller assembly request

- [ ] Controller decides any root-script alias only after this evidence proves the local command works.
- [ ] Controller alone changes lockfile or release routing after explicit future authorization.
- [ ] Do not remove Flutter artifacts or revise package identifiers in this ticket.
