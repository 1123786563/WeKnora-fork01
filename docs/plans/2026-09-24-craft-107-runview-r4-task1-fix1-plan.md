# Craft #107 T01 R4 Task 1 Fix 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the R4 Task 1 independent review's legal draft size, descriptor identity, writer-fence and production-boundary test findings.

**Architecture:** Read sealed draft objects under their own 50 MiB bound; verify every opened descriptor against the preflight directory entry; recheck durable writer epoch before message adoption/dispatch and condition adoption on the epoch. Exercise the actual `Execute` → durable seed → private output boundary with a scoped fixture.

**Tech Stack:** Go, SQLite repository fixture, descriptor-relative filesystem operations, focused container tests.

**Spec:** `docs/plans/2026-09-24-craft-107-runview-r4-task1-review.md`; approved Craft web Artifact Spec #107/T01; `docs/plans/2026-09-24-craft-107-runview-r4-version-plan.md`.

## Global Constraints

- The current persistent Workspace draft, not last promoted Version, is the frozen predecessor. Run/tenant/owner/session/Workspace/epoch and verified material generation determine the only output root.
- Initial empty revision is explicit; missing predecessor fails closed. No client-provided path, previous Run directory, HOME, input or knowledge material enters output seeding.
- Same-byte hard links or changed directory entries are not acceptable retries. Unknown writer fence or stale epoch cannot reach prompt/dispatch or mutate a prior draft/Version.
- Keep runtime default-off. No production enablement, commit/push, shared stash or concurrent edits to the H2 test fixture currently owned by R5.
- Exact pre/post file checkpoint, RED/GREEN focused tests, full container suite classification, independent Spec and quality review required.

## Review Focus

- 21–50 MiB sealed draft files continue successfully; >50 MiB and mismatched size/digest reject before prompt.
- Replacing a preflighted file with same-byte hard link or directory entry between stat/open/read must reject.
- Epoch changes during object fetch or staged write must reject before adoption/dispatch.
- A duplicate message-ID adoption cannot bypass epoch or Run/fence identity.
- Production `Execute` path, not helper-only tests, must prove initial empty and failed/stopped draft continuation with unchanged default Version.

---

### Task 1: Safe draft materialization and durable dispatch fence

**Depends on:** R4 Task 1 reviewed FAIL checkpoint. **Owner:** `backend_implementer`. **Validator:** `backend_validator`, followed by independent `reviewer`.

**Owned files:** `internal/container/craft_runtime.go`, `internal/container/craft_runtime_r4_seed_test.go` and a new focused container integration test if useful. No H2 fixture, R5 engine/provider, repository migration or T19 files.

**Consumes / produces:** Existing typed durable `CraftWorkspaceSeedSnapshot`, `ReadRevision`, pinned file service, material handle and `agent_runs.epoch`; produces fully verified private output seeding with durable writer authority still current at dispatch.

- [ ] Capture exact preimage hashes. Write RED production-boundary fixture for empty revision, 21 MiB valid draft, D1→B frozen predecessor, wrong Run/Workspace/epoch, byte mismatch, partial retry and no dispatch on rejection; include a separate changed-epoch-during-fetch case and same-byte hard-link replacement case.
- [ ] Replace the input-only read helper in draft seeding with a bounded draft-object read using declared size + 1 under the 50 MiB per-file cap. Preserve exact length and SHA-256 validation and total manifest quota.
- [ ] On every opened output file and intermediate directory, compare descriptor `Fstat` identity/type/link count with preflight `Fstatat` and recheck after read. Reject any changed entry even if bytes/digest match; retain no-follow/no-replace behavior.
- [ ] Recheck durable `agent_runs.epoch` after seeding and immediately before prompt. Make message-ID adoption conditional on the current Run/fence epoch and scope, with no stale update or dispatch; retain idempotent retry semantics.
- [ ] Run focused RED/GREEN and race tests, `go test ./internal/container -count=1`, gofmt and diff check. Record exact failures unrelated to the task; do not modify competing H2 fixture. Save task-local patch, hashes and report for independent review.

**Acceptance / failure handling:** Findings High/Medium 1–4 closed by exact tests and reviewer. If adapter/repository cannot provide an atomic fence check, stop and report a concrete seam instead of approximating with a stale in-memory value.

### Task 2: H2 fixture compatibility after R5 release

**Depends on:** Task 1 reviewed PASS and R5 engine Fix1 checkpoint/review released its H2 fixture file. **Owner:** `mechanical_worker` for test-only schema data; **validator:** `backend_validator`, followed by independent `reviewer`.

**Owned files:** `internal/container/craft_knowledge_runview_h2_test.go` only. The earlier plan named the provider-created live mount test by mistake; the two reported RED tests and their durable Run fixture are in this knowledge H2 file. No other agent owns this file.

**Consumes / produces:** Strict R4 seed admission and R5 observed ImageID H2 adapter; produces valid fixture `agent_runs.epoch` and typed `craft_workspace_seed` without bypassing production checks.

- [ ] Capture preimage. RED rerun the two named H2 tests; confirm only fixture schema/seed absence stops them.
- [ ] Add realistic epoch and immutable revision-zero seed fixture data, using repository creation APIs when possible. Do not fake a missing field in runtime or weaken `ParseDurableRunSnapshot`.
- [ ] Run the two H2 tests and the full container package when no conflicting Docker suite is active; save exact checkpoint/report and independent review.

**Acceptance / failure handling:** H2 tests reach and validate their original knowledge-mount assertions; remaining assembly failures are tracked separately. No full-package PASS is claimed until every failure is resolved.
