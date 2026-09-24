# Craft #107 T19 Normal Output Store Fix 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the independent review's unobservable failed seal and quota-truncated replay bugs before the normal-output coordinator consumes the store.

**Architecture:** Provider seal must report error and mark output partial/unavailable. The repository accepts a sink opening only once for a receipt; after uncertain seal no second writer may reopen it, while cursor reads remain available. Store each append's original length/digest separately from committed prefix, including a zero-byte quota attempt, so exact sequence replay is idempotent and changed input conflicts.

**Tech Stack:** Go provider contract, Gorm SQLite/PG schema, focused/race tests.

**Spec:** `docs/plans/2026-09-24-craft-107-t19-normal-output-task2a-task1-review.md`; approved Craft #107 T19 normal-output and no false-success contract.

## Global Constraints

- Exactly one physical Docker attach remains unchanged. Seal error, timeout, DB loss, quota truncation or late append cannot become `TransportComplete` or build success.
- `OpenSink` must not regrant write ability after a sealed, sealing, failed or ambiguous operation. A cursor reader can read committed bytes without opening a writer.
- Preserve tenant/Run/activity/receipt scope and byte limits. No production routing, commit or Docker run. Exact task-local checkpoint and independent review.
- SQLite124/PG203 migration files are ignored by Git; include full SQL bytes and hashes in review package, and preserve serial predecessor migration order.

## Review Focus

- Provider sees durable `Seal` failure and returns partial/unavailable; retry cannot append to the same receipt after a failure or process restart.
- Blocked append racing Seal is bounded; a late release after return cannot commit.
- Truncated exact replay and zero-byte quota attempt are idempotent by original input identity; changed bytes for the same sequence conflict.

---

### Task 1: Make seal outcome observable and append replay exact

**Depends on:** normal-output Task2a independent review FAIL. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/application/repository/craft_docker_output.go`/tests, `internal/application/service/craft_docker_output.go`/tests, `internal/modules/execution/sandbox/docker_normal_exec.go`/tests for the narrow sink interface, and migration pairs `migrations/sqlite/000124_craft_docker_output.{up,down}.sql`, `migrations/versioned/000203_craft_docker_output.{up,down}.sql`. No S2/Run repo or R4/R5 files.

- [ ] Capture exact eight-file preimage. RED: `Seal` timeout/DB error returns provider partial, second Seal reports same failure or safely retries to a durable sealed row, and a fresh `OpenSink` after failed/unknown seal is denied before any Append; late blocked append cannot commit. RED: exact replay of truncated prefix and zero-committed-byte quota attempt succeeds idempotently; same sequence with different full bytes conflicts.
- [ ] Change narrow sink interface to report `Seal() error` (or equivalent typed observable outcome) and propagate it through provider freeze/classification. Keep `Seal` bounded. Make `Open` create-only for writer claims and keep read-only cursor path separate; no restart may regain an unsealed writer after ambiguous seal. Persist original input digest/length for each append attempt, including zero-prefix quota cases, with a stable sequence/receipt identity.
- [ ] Run focused repository/service/provider and race tests, isolated SQLite migration up/down/up, PostgreSQL where available, gofmt/diff. Save reconstructible incremental patch, hashes/report and independent review.

**Acceptance / failure handling:** F1 High and F2 Medium closed without changing physical start protocol. F3 migration tracking/PG execution remains a final integration gate if PG unavailable now.
