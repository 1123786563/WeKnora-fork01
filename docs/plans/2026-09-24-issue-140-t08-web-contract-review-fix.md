# T08 Web contract decoder review fix

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject malformed JD evidence and impossible acquisition dates at the typed Web boundary.

**Architecture:** Keep the additive Opportunity wire and client methods. Tighten decoder predicates only; return every valid nonblank `rawText` byte-for-byte and maintain the backend's RFC3339 time representation.

**Tech Stack:** TypeScript contracts/tests.

**Sources:** initial T08 Web contract commit `efd18d67feaac09d8ee6347df509f537ee0cf10e`; independent review findings at `packages/career-core/src/contracts.ts:84,96`; #146 exact original snapshot requirement.

## Global Constraints

- Own only `packages/career-core/src/contracts.ts` and focused decoder tests in existing isolated T08 Web worktree. No API client/Go/router/UI changes.
- Local code commit authorized; no push/merge/deploy. Preserve valid text exactly, including internal whitespace and line endings.

## Review Focus

- Empty or whitespace-only evidence text throws `TypeError`, consistent with Go import validation.
- Calendar-impossible RFC3339 date throws; valid UTC/offset dates accepted and returned unchanged.
- Existing receipt/evidence and Career profile decoders remain compatible.

---

### Task 1: Tighten Opportunity evidence decoding

**Depends:** reviewed initial T08 Web contract. **Owner/validator:** frontend_implementer / frontend_validator. **Files:** `packages/career-core/src/contracts.ts` and `contracts.test.ts`. **Consumes:** Opportunity evidence JSON. **Produces:** strict valid evidence.

- [ ] RED: Add malformed evidence tests for blank/whitespace raw JD and impossible calendar date; valid multiline raw JD remains exact.
- [ ] GREEN: Add narrow text and RFC3339 calendar checks at the Opportunity decoder boundary.
- [ ] VERIFY: Focused contract/client tests, `pnpm typecheck:web`, `git diff --check`; independent reviewer rechecks both findings and regressions.

## Shared-file preflight

No other worker writes Web contract or client files during this repair. The later T08 Web UI task starts after this code is integrated and reviewed.
