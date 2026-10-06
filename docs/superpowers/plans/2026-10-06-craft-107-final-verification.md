# Craft #107 Final Integration Verification Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to execute any code repair task created by this plan. This plan starts with read-only validation; do not change production code unless an observed failure or valid OCR finding requires an owned SDD repair plan.

**Goal:** Close the remaining post-F08 integrated browser acceptance gate for Craft #107, then obtain complete OCR coverage of the delivered Craft implementation and resolve every valid critical/high/medium finding through SDD.

**Architecture:** Craft implementation exists on current `main` at base `6e1b2072a13a798e7ec25be44784e5242bd2f178`. Two production boot blockers discovered during Task 1 were repaired by scoped SDD plans, then independently validated and reviewed: the missing commercial webhook handler provider (`docs/superpowers/plans/2026-10-06-craft-container-webhook-provider-fix.md`) and the missing anonymous HMAC webhook POST exemption (`docs/superpowers/plans/2026-10-06-commercial-webhook-anonymous-auth-fix.md`). Their findings, TDD evidence, reports, and exact source hashes are preserved under `docs/superpowers/reports/`. Now run the repository's isolated `test:craft:web` stack with a unique `CRAFT_STACK_TAG`; preserve its exact logs and browser artifacts in a new testing evidence directory. After the run has fully torn down and tested source hashes are recorded, perform final OCR over the full Craft delivery range and post-F08 workspace state. OCR findings are evidence, not instructions; repair only valid findings using the appropriate backend/frontend implementer, independent validator and reviewer, then rerun OCR.

**Tech Stack:** Go, SQLite, Docker/OrbStack, Node/pnpm, Playwright, local nginx preview, `open-code-review` CLI.

**Authority:** Approved Spec `docs/specs/2026-09-23-craft-web-artifact-spec.md`; ADR `docs/adr/0004-task-is-session.md`; `CONTEXT.md`; live Issue snapshot `docs/plans/2026-10-06-craft-107-live-issue-refresh.md`; refreshed DAG/Ledger `docs/plans/2026-10-06-craft-107-execution-dag.md`; prior implementation Ledger `docs/plans/2026-09-23-craft-107-ledger.md`.

**Commit policy:** No staging or commits are authorized. Keep all run evidence and plans in the isolated Worktree and record HEAD plus hashes.

## Global Constraints

- Preserve the primary checkout, its 23 untracked documents and current branch; all commands run from this Worktree.
- The browser stack is an exclusive shared-resource task: use a unique `CRAFT_STACK_TAG`, confirm default ports are free before launching, do not run another Craft Docker/browser stack concurrently, and verify precise teardown afterward.
- Do not infer success from closed Issues, previous T20 runs before F08 wiring, route/component tests, or the “closure-ready” label. Capture the new exact result.
- Do not use user credentials/secrets in logs. Do not push, merge, publish, deploy, close Issues, or post external messages.
- A full web suite with known unrelated hangs is not a substitute for the required Craft end-to-end suite; report any broader baseline separately if rerun.
- Do not treat the partial OCR R3 report as a completed final review. Cover all selected Craft source files, preserve selected/excluded/failed counts and provider errors, and require complete coverage.

## Review Focus

- F08 production dispatcher runs only after successful web delegation, uses budget admission and provider-bound execution evidence, and leaves promotion fail-closed on missing or mismatched receipts.
- The integrated member journey actually creates, builds, previews, and promotes the webpage after F08 is wired; refusal/failure remains honest and the previous version stays safe.
- T03 uploaded-code screening and audit behavior remain fail-closed on the real production dispatch path.
- T14 preview loads a valid local HTML/CSS/JS page while retaining isolated origin, capability binding, no-egress and cleanup guarantees.
- T19 budget pause and replay behavior remain durable and do not falsely deliver work.
- Final diff/evidence scope excludes unrelated user changes.

## Tasks

### Task 1 — Re-run post-F08 integrated Playwright acceptance

**Depends on:** refreshed live Issue/DAG snapshot (complete); implementation baseline `6e1b2072a`.  
**Role:** parent controller for test orchestration; backend/frontend validators may inspect evidence after freeze.  
**Owned files:** new `docs/testing/craft/t20/2026-10-06-post-f08/` evidence only; no production source.  
**Consumes:** F08 dispatcher and trigger at `internal/container/craft_web_build_dispatch.go` / `craft_runtime.go`, prior `docs/testing/craft/web-acceptance.md`, `apps/web/e2e/craft-stack.sh`.  
**Produces:** exact command output, Playwright result summary, run artifact manifest and cleanup evidence tied to tested HEAD and source SHA manifest.

Steps:

1. Record current HEAD, relevant Craft source hashes and Worktree status. Confirm the documented Docker image/runtime and local executables are available. Inspect exact listeners for the harness ports `41871–41878` and `41883`; if occupied, identify the owning process and use the harness-supported environment overrides without terminating unrelated processes.
2. Use the documented runner `apps/web/e2e/craft-stack.sh`: run `CRAFT_STACK_TAG=craft107-final-20261006 bash apps/web/e2e/craft-stack.sh up mock`, then `... run mock`, and always `... down mock` in a shell cleanup path. Do not invoke `pnpm test:craft:web` bare: its Playwright config requires the harness-provided `CRAFT_AUTH_STATE`, API URL, DB path and web URL. Preserve the up/run/down exit codes independently.
3. Record all Playwright specs/counts, exit code, logs and screenshots/traces that explain any failure. Verify its stack processes, nginx instance and only its own temp run directory are torn down. Re-hash source files to confirm the tested tree did not change.
4. If all specs pass, record Task 1 verified. If the test fails, reproduce the specific failure using browser evidence and the isolated run DB/logs. Route application defects through a finding-scoped SDD repair. If a fail-closed deployment prerequisite is absent, prove it from the current harness env/config path, source guard, emitted tool result and persisted state; do not infer success or silently substitute a local image tag/digest.

**Verification:** harness `up`, `run` and `down` exit 0; test output reports all configured Craft e2e specs passed; cleanup checks show no owned process/container/network residue; source SHA-256 before and after is identical.

**Current result:** harness `up=0`, Playwright `run=1` (first of six stories failed because no version was published; five did not run), exact `down=0`. Run evidence is under `docs/testing/craft/t20/2026-10-06-post-f08/`; a sanitized diagnosis is required before finalizing. Persisted state proves the `craft_delegate` tool failed with `verified RunView material resolver is not assembled`, delegation stayed `prepared`, and no version was created. The six `CRAFT_RUNVIEW_*` inputs required by `craftRunViewProductionConfigFromEnv` were all unset. The checked-in lock leaves the authoritative container digest null; code explicitly refuses to infer a production pin from local Docker metadata. The T14 `WebPageLoadProbe` registration slot is also unimplemented/unregistered, so promotion fails closed even with a local OpenCode server. Task 1 is blocked pending an authoritative deployment pin set and T14 browser page-load/no-egress integration evidence. Do not retry the same local setup without those inputs.

**Failure handling:** Preserve the sanitized run evidence and exact teardown record; do not copy generated credentials, storage-state tokens, SQLite DB, or raw trace into the repository. Application failures remain bugs until the emitted error/state prove an external fail-closed prerequisite. The two verified DI/auth blockers are resolved at hashes in their Task reports; if either recurs, treat it as a source-integrity mismatch and stop the live run. Do not alter Issue state.

### Task 2 — Full-scope OCR and scoped repair loop

**Depends on:** Task 1 has a frozen verified result.  
**Role:** parent invokes OCR; issue fixes route to backend/frontend implementer; matching validator and independent reviewer follow.  
**Owned files:** one report per OCR round under `docs/plans/`; any finding fix gets a separate `docs/superpowers/plans/` plan and non-overlapping files.  
**Consumes:** live snapshot, approved Spec, ADR, CONTEXT, DAG/Ledger, complete Craft delivery range and Task 1 checkpoint.  
**Produces:** complete OCR report with selected/covered/excluded/failed file counts, exit status and provider warnings; rulings for rejected findings; SDD repair packages/reviews/verification and complete OCR rerun if needed.

Steps:

1. Read and follow `open-code-review/SKILL.md`. Determine the actual baseline and current HEAD from recorded Craft history; include the full delivered range, including post-F08 integration, plus any workspace changes. Run with `--audience agent` and business background referencing #107, #119–#139, the approved Spec and critical authority boundaries. Save the full report; inspect selection and failure coverage instead of trusting exit status alone.
2. For every critical/high/medium finding, reproduce the evidence against the frozen current source and record finding ID + validity + acceptance impact. For false positives, save the exact source/test/contract evidence. Valid findings require a finding-only plan/brief and SDD implementation with TDD, affected validation, independent Review and a scoped OCR re-review. Never repair production code directly from this controller task.
3. If OCR reports provider/rate-limit failures, missing selected files, unsupported-file skips or timeout/budget errors, classify the pass as incomplete. Follow the skill's recovery procedure; do not claim coverage until all delivery files are reviewed.
4. After any repair, run a new complete OCR pass covering original baseline through final HEAD plus any uncommitted delivery edits. Keep every round/report/ruling in durable records.

**Verification:** complete coverage and no valid critical/high/medium findings; all returned valid findings resolved and scoped re-review confirms the fixes; source hashes match the final tested/reviewed checkpoint.

**Failure handling:** keep the affected DAG node unverified. If the same valid finding makes no progress for two OCR repair rounds, use `systematic-debugging` in a fresh context and escalate reasoning. Stop only for a verified external blocker with no safe authorized route; record precise evidence and impact.

## Dependency and resource preflight

| Work | Depends on | Shared files/interfaces | Shared resource | Dispatch |
|---|---|---|---|---|
| Task 1 live browser acceptance | F08 and all T20 native predecessors verified/integrated | Craft production tree read-only | Docker, SQLite, fixed test ports, nginx preview | Single owner until teardown |
| Task 2 full OCR | Task 1 source/result frozen | All Craft delivered files read-only | OCR provider quota | Starts after Task 1 freeze |
| SDD repairs (conditional) | A reproduced valid finding | Finding-specific disjoint source/test files | Task-local tests; no shared browser stack unless acceptance requires it | Fill available safe seats in one frontier round; use separate Worktrees |
