# T64 Task4 Review Fix — reject structurally invalid persisted dependency locks

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Narrow repair for T64 Task4 commit `0f0239eb0`.

**Goal:** Ensure structurally invalid stored lock values cannot be treated as an empty valid dependency set or admitted as OK.

**Finding:** MEDIUM T64-T4-R1: `json.Unmarshal` accepts `null`, `{}`, or a null/missing dependency list into zero values; current adjudication can therefore return an `ok` verdict for invalid persisted lock structure. LOW: behavior tests omit malformed lock, tuple mismatch, release precedence and unknown/foreign admission boundaries.

## Constraints
- Continue only in isolated T64 worktree after commit `0f0239eb0`.
- Owned files: `internal/application/service/agent_security.go` and `agent_security_test.go` only. Do not edit interface, repository store/schema, or any T63-shared service files.
- Preserve release-revocation precedence, exact case-sensitive `(type,id,version,digest)` matching, tenant introduction lookup and existing fail-closed error for unknown release.
- No external services/credentials; local commit authorized.

## Task 1 — validate persisted lock shape
**Role:** backend_implementer; independent reviewer + validator follow.
1. RED tests for JSON `null`, `{}`, missing dependencies, `dependencies:null`, valid empty array, exact mismatch by each tuple component/case, release revocation taking precedence, and unknown/foreign release error.
2. GREEN: after decoding, reject null top-level, missing or null dependencies, and invalid dependency entries that cannot represent required tuple identity; return explicit error rather than empty lock/OK. A valid `dependencies: []` remains a valid empty lock. Avoid over-constraining optional metadata not required by the domain.
3. Run focused tests, relevant security/adoption service filter, diff-check; commit only two files and write exact report.

**Review focus:** Invalid stored bytes can never produce OK; legal empty dependencies remain valid; predicate order and exact matching unchanged.
