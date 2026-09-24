# Craft #107 T01 R5 Engine Fix 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the namespace isolation and live read-only mount evidence gaps in the R5 engine Task 1 independent review.

**Architecture:** Validate inspected PID/IPC namespace values against a strict private-container allowlist and exercise the actual knowledge mount destination in the disposable Linux container. Keep Docker facts observed rather than copied from the requested create configuration.

**Tech Stack:** Go, pinned Moby Docker SDK, disposable Linux/arm64 Docker engine/image.

**Spec:** `docs/plans/2026-09-24-craft-107-runview-r5-engine-task1-review.md`; approved Craft web Artifact Spec #107/T01; `docs/plans/2026-09-24-craft-107-runview-r5-engine-plan.md`.

## Global Constraints

- The private RunView must have no shared host or other-container PID/IPC namespace and must not expose a writable knowledge/input mount.
- Docker inspect/runtime facts come from the daemon. Unknown or anomalous inspected modes fail closed; no permissive fallback to requested config.
- Do not enable production DI, substitute a local image ID for the absent deployable registry pin, or commit/push.
- Preserve concurrent R4 `craft_runtime.go` edits; if they temporarily block package tests, capture the exact compile error and rerun after that ownership settles.

## Review Focus

- `PidMode=container:<id>` and `IpcMode=container:<id>` must be rejected before provider acceptance.
- `host`, malformed and future unknown namespace modes must remain rejected or explicitly proven private.
- Live write attempt must target the inspected mounted knowledge destination, not its host source path.
- A nonzero write exit must be attributable to read-only mount denial, not a nonexistent path or permission from a different cause.
- Repeated full checkpoint hashes and cleanup must match the reviewed test run.

---

### Task 1: Strict namespace inspection and mounted destination proof

**Depends on:** R5 Engine Task 1 reviewed FAIL checkpoint. **Owner:** `backend_implementer`. **Validator:** `backend_validator`, followed by independent `reviewer`.

**Owned files:** `internal/container/craft_runview_docker_engine.go`, `_test.go`; if the existing provider itself validates inspected namespace facts, narrow `internal/container/craft_runview_container_provider.go` and its focused test. No `craft_runtime.go` or production DI files.

**Consumes / produces:** Existing complete `InspectContainer` projection and live knowledge mount test; produces strict PID/IPC isolation and actual destination write-denial evidence.

- [ ] Capture exact preimage hashes. Add RED fake inspect/provider tests for `container:<id>` PID and IPC separately, host and unknown variants, and confirm the current acceptance bug.
- [ ] Apply the smallest strict allowlist for inspected private namespace modes; require a known private value rather than only excluding `host`. Preserve daemon-derived projections and all other R1 validation.
- [ ] Correct the live Docker test to derive and assert the destination path from the inspected knowledge mount, attempt create/write there, and distinguish read-only-filesystem denial from missing-path/UID errors. Verify the target existed and was readable before the write attempt.
- [ ] Run focused normal/race tests, disposable real Docker proof, gofmt and diff checks; preserve exact commands/exits, cleanup, checkpoint hashes, task-local patch and report. If R4 in-progress edits block compilation, wait for its checkpoint and rerun rather than editing its file.

**Acceptance / failure handling:** High and Medium findings closed by independent Spec/quality review; actual knowledge mount remains read-only and PID/IPC namespace strictly private. R5 Task2 DI and production enablement remain separately gated by reviewed R4 and a deployable registry pin.
