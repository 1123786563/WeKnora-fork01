# T33 Task 2 Repair Round 2 — upgrade accept conflict mapping

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Return the established upgrade state conflict when an Adoption ends after upgrade acceptance prechecks but before draft Variant creation.

**Architecture:** Keep the Task 2 repository serialization guard unchanged. In `AcceptUpgradeProposal`, translate only the guarded repository `ErrAgentAdoptionTransition` from `CreateVariant` into `ErrAgentUpgradeStateConflict`; the existing handler maps that service sentinel to HTTP 409. A gated real-repository service test proves the stale-precheck interleaving and that no proposal Variant is created.

**Tech Stack:** Go, GORM migration-backed SQLite repository, existing service/handler sentinels.

**Spec:** `docs/specs/2026-09-20-agent-marketplace-domain-model.md` §9; `.superpowers/sdd/plan-t63/task-2-fix-round1-report.md`.

## Global Constraints

- Preserve the guarded CreateVariant/EndAdoption repository core.
- Keep all state access tenant-scoped through the existing repository.
- Surface stale lifecycle state as the existing upgrade conflict, which maps to HTTP 409.

## Review Focus

- EndAdoption wins after Accept prechecks: return state conflict, create no Variant, and leave proposal open.
- Ordinary successful and duplicate Accept behavior remains unchanged.
- No handler mapping changes are needed if the service emits `ErrAgentUpgradeStateConflict`.

---

### Task 1: Map CreateVariant race to upgrade conflict

**Files:**
- Modify: `internal/application/service/agent_upgrade.go`
- Test: `internal/application/service/agent_upgrade_test.go`

**Interfaces:**
- Consumes: `repository.ErrAgentAdoptionTransition`, `AgentUpgradeService.AcceptUpgradeProposal`, existing `ErrAgentUpgradeStateConflict` and `upgradeClientError` mapping.
- Produces: `AcceptUpgradeProposal` wraps the repository transition sentinel with `ErrAgentUpgradeStateConflict`; other errors retain existing handling.

- [x] Write gated migration-backed service test; pause after service prechecks at `CreateVariant`, commit EndAdoption, resume Accept, assert upgrade state conflict and no Variant.
- [x] Run test and observe RED: error is repository transition sentinel without upgrade conflict.
- [x] Translate `repository.ErrAgentAdoptionTransition` at the CreateVariant error boundary using both `%w` sentinels.
- [x] Run targeted upgrade service tests and race check where established, then format/diff-check.
- [x] Commit repair and record exact patch range/hash.
