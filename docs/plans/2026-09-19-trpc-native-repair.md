# tRPC Native P0 Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Each task uses the project-mandated `sdd_implementer` and `sdd_task_reviewer` roles.

**Goal:** Repair the three P0 blockers without treating the native Agent migration as approved for product execution.

**Architecture:** First validate the fixed `trpc-agent-go v1.11.0` candidate against the existing native Runner probe and all repository consumers. Separately correct the SQLite test fixture so it represents the model it persists, and make the P0 document validator reject the two documented negative cases. The P0 NO-GO remains until every migration acceptance gate is satisfied.

**Tech Stack:** Go 1.26, trpc-agent-go, GORM SQLite, Bash, Go testing.

**Spec:** `docs/superpowers/specs/2026-09-19-trpc-native-agent-migration-design.md`

## Global Constraints

- Preserve tenant isolation, archived-data read-only behavior, and `waiting_user` for unknown external effects.
- Do not release or wire the native Runner into product traffic based solely on this repair.
- Use exact `v1.11.0`; no floating module version or local absolute-path replace.
- A green candidate requires `GOWORK=off go test -race ./internal/agent/nativeprobe -count=20 -v`; current v1.10.0 is known to fail it.
- P0 documentation validator must fail if `interfaces.md` lacks the repeated race gate or any P0-1 through P0-5 primary ledger row is absent or incomplete.
- The SQLite test must create the actual persisted schema using repository migration semantics or a fixture DDL synchronized with `types.WorkbenchInteraction`; it must include `external_pending_id`.

### Task 1: Upgrade and validate the pinned tRPC candidate

**Files:** `go.mod`, `go.sum`, `internal/agent/nativeprobe/runner_test.go`, `docs/superpowers/plans/trpc-native/{sdk-probes.md,p0-decision.md,progress.md}`.

- [ ] Write a regression test or test assertion that records the selected SDK version / expected Session mutation synchronization through the real Runner path; run it on v1.10.0 and capture the expected race failure with the repeated gate.
- [ ] Pin `trpc-agent-go` to `v1.11.0`, run `go mod tidy`, and make only compile-required source adjustments.
- [ ] Run the native probe normally and with `-race -count=20`; run all direct tRPC consumer packages identified by `go list -deps` and the existing recovery package.
- [ ] Update P0 evidence with exact commands/results. If any candidate validation fails, keep NO-GO and report the failed contract; never weaken the gate.
- [ ] Commit scoped SDK/probe/evidence changes.

### Task 2: Make the durable-run SQLite fixture schema-complete

**Files:** `internal/agent/engine_test.go` and any narrowly scoped test helper it creates.

- [ ] Add a regression assertion that `TestAgentRunToolCallProjectsDurableRunID` can persist and read an interaction containing `external_pending_id`; run it against the old fixture and observe the missing-column failure.
- [ ] Replace or extend fixture DDL with the actual `external_pending_id VARCHAR(255) NOT NULL DEFAULT ''` column, keeping the fixture explicit and aligned to `migrations/sqlite/000077_paseo_control.up.sql`.
- [ ] Run the focused test, `go test ./internal/agent -run TestAgentRunToolCallProjectsDurableRunID -count=1`, then the affected package suite. Record any unrelated full-suite failures separately.
- [ ] Commit only fixture/test changes.

### Task 3: Make P0 validation reject documented drift

**Files:** `scripts/validate-trpc-native-p0-docs.sh` and optional shell-test harness under `scripts/`.

- [ ] Add a shell regression harness that copies the minimal P0 docs into a temporary repository fixture, removes the repeated gate from `interfaces.md`, and expects the validator to fail; add a second case deleting the P0-3 ledger row and expecting failure. Run it before validator changes and observe both incorrect passes.
- [ ] Change the validator to check the repeated gate separately in each required document and parse every P0-1 through P0-5 primary ledger row for exact commits plus implementation/spec/quality completion statuses.
- [ ] Run the negative harness and validator against the unmodified checkout; both negative cases must fail and the real documents must pass.
- [ ] Commit scoped validator/test changes.

## Review and completion

Each task is implemented by `sdd_implementer` and reviewed by `sdd_task_reviewer`. A final `sdd_final_reviewer` checks the combined diff. Product migration remains NO-GO unless Task 1, PostgreSQL, real Provider, persistence, recovery and client acceptance evidence all pass.
