# T10 contract review fixes

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject malformed evaluation conclusions at the Web boundary and return a precise 404 for a missing evaluation.

**Architecture:** Keep immutable evaluation storage and wire fields unchanged. Extend Career HTTP error classification for `ErrEvaluationNotFound`. Tighten the typed Web decoder so each hard outcome is supported by a pinned JD span and confirmed profile fact, and so the aggregate status follows `ineligible > unknown > eligible`. Reject a malformed response rather than render a favorable or hidden conflict.

**Sources:** #150, approved Job Search Spec §3/§6.1, T10 plan; independent Task 3 review of Web code `cdf2cb0e1` with two medium findings. Backend evaluated code integrated at `7c8a7735e`; Web contract in isolated worktree based on `0370bebfe`.

## Global Constraints

- Local task commits authorized; no push, merge, deploy. Preserve immutable snapshots, fact versions and current API JSON shape. All user-provided JD text is inert.
- Tasks run serially and in isolated worktrees. Backend implementer owns Career handler + HTTP test only; frontend implementer owns four T10 contract/client files only. Independent reviewer/validator are read-only.
- Do not let soft matches suppress hard ineligible or turn unknown into eligible. Do not fabricate a citation when source evidence is missing.

## Review Focus

- GET missing evaluation ID yields 404 `not_found`; receipt lookup and other error mappings retain behavior.
- Web accepts actual Go generated evaluation fixtures with nonempty rule list and matching aggregate; rejects empty rules, a mismatched aggregate, conclusive rule missing either citation, evidence referencing a fact absent from pinned `facts`, and invalid snapshot/offset provenance.
- `unknown` is not accepted as a stand-in for a hidden `ineligible` rule; no numeric probability field is introduced.

---

### Task 1: Map missing evaluation to HTTP 404

**Depends:** reviewed T10 backend. **Owner/validator:** backend_implementer / backend_validator. **Files:** `internal/modules/career/handler.go`, `internal/router/routes_career_test.go` (or a focused Career handler test). **Consumes:** existing `ErrEvaluationNotFound`. **Produces:** 404 `not_found` for missing evaluation.

- [ ] RED: Add an authenticated GET `/api/v1/career/evaluations/:evaluationId` missing-ID test expecting 404 and code `not_found`; confirm current 500.
- [ ] GREEN: Add `ErrEvaluationNotFound` to the existing not-found error mapping. No other handler behavior changes.
- [ ] VERIFY: Focused handler/router/Career tests, `git diff --check`, independent Spec/quality review and backend validator. Integrate before Task 2.

### Task 2: Enforce evaluation evidence invariants in the Web decoder

**Depends:** Task 1 reviewed and integrated; typed Web contract code `cdf2cb0e1`. **Owner/validator:** frontend_implementer / frontend_validator. **Files:** `packages/career-core/src/contracts.ts`, `contracts.test.ts`, `packages/api-client/src/career.ts`, `career.test.ts`. **Consumes:** frozen Go evaluation JSON; **produces:** strict evaluation decoder/client.

- [ ] RED: Add negative fixtures for every review finding. Replace the empty-rules API client fixture with a valid pinned evidence record. Confirm current malformed fixtures are accepted.
- [ ] GREEN: Require nonempty hard rules; derive and compare overall with precedence; require both job/profile citation on conclusive rule outcomes and ensure cited facts belong to the pinned fact manifest. Validate job evidence against the fixed snapshot IDs/hash and byte span when the raw text is supplied. Preserve unknown rule semantics without fabricated citations.
- [ ] VERIFY: Focused contract/client tests, `pnpm typecheck:web`, `git diff --check`, independent Spec/quality review and frontend validator. Then integrate and continue T10 UI Task 4.

## Shared-file and interface preflight

The two tasks own disjoint Go and TypeScript files; they execute serially because Task 2 consumes the reviewed backend checkpoint. No migration, route registration, profile model or Web UI file changes. Existing T10 backend/guard reports remain valid for their unchanged code; rerun only checks affected by these fixes.
