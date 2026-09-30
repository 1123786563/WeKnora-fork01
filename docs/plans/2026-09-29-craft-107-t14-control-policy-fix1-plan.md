# T14 Control Policy Review Fix Round 1

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the five high/medium findings from the independent review of T14 WebDriver control-policy Task 1 and provide bounded live kernel evidence before any immutable renderer probe.

**Architecture:** Keep the exact host-attested PID1→ChromeDriver socket tuple and deny-by-default nft policy. Normalize proc state once and compare the normalized values. Parse a bounded conntrack table as complete records and require exactly one matching TCP ESTABLISHED original/reply tuple in the right family. Bound host discovery inventory and child output bytes. Make the same-listener canary produce structured proof of a real failed connect from a distinct source port with a positive loopback drop delta. In the disposable namespace integration, separately prove the old socket still exchanges data after policy and a new socket to the same listener is denied. Preserve the `-150` OUTPUT hook, preview rules, target filters, and all T14 attempts.

**Tech Stack:** Python 3.12, Linux procfs/conntrack, Docker disposable network namespaces, nftables.

**Sources:** T14 policy plan `docs/plans/2026-09-28-craft-107-t14-webdriver-control-policy-plan.md`; prior Task report `docs/plans/2026-09-28-craft-107-t14-webdriver-control-policy-report.md`; independent review `docs/plans/2026-09-29-craft-107-t14-control-policy-review.md`; validator `.superpowers/sdd/2026-09-28-craft-107-t14-webdriver-control-policy-plan/validator-report-task-1.md`; root cause `docs/plans/2026-09-28-craft-107-t14-loopback-webdriver-root-cause.md`.

## Global Constraints

- No port-wide, generic `ct state established`, renderer-supplied, or listener-only exception.
- Keep chain priority strictly after conntrack (`-150`), and keep `policy drop`, exact preview allows, loopback drops, isolated targets, and 8081 canary.
- Discovery output contains only bounded socket metadata; no argv, page data, URLs, or arbitrary process data.
- Conntrack uncertainty, table overflow, malformed record, ambiguous match, discovery overflow, or canary ambiguity fails before successful install and enters verified cleanup.
- Do not retry with broader rules or weakened timeout/resource/isolation settings.
- Do not run the immutable renderer candidate until disposable conntrack proof and both old-flow reuse/new-flow denial assertions pass.
- No commit/staging; preserve all unrelated T14 worktree changes.

## Review Focus

- Proc TCP numeric states are normalized consistently before selection; test the exact discovery serialization with numeric kernel fixtures.
- Conntrack parsing rejects wrong protocol/family, partial/reversed/multiple tuples, duplicate matches, malformed lines, and oversized/truncated input; accepts only one exact bidirectional TCP ESTABLISHED flow.
- Discovery bounds PID FD count, TCP row count, JSON output bytes, Docker child stdout/stderr bytes, and wall time; overflow terminates/fails closed.
- Same-listener canary receipt proves actual attempted target, chosen source port differs from established client port, connection is not established, failure class is an expected policy drop, and matching counter increases in the narrow window.
- Disposable namespace test exchanges a marker over the preexisting connection after policy install, then verifies a new connection to the same listener fails and increments the exact loopback drop counter; 8081 canary remains.
- Cleanup removes sidecar/table/routes and disposable renderer even on malformed evidence/canary failure.

## Task 1: Bound exact-flow evidence and prove both established reuse and fresh-connection denial

**Dependencies:** Prior Task 1 fixes are under independent Review; findings are documented above. No integration has occurred. This task owns the same T14 policy-helper source in the same worktree and is sequential.

**Owner role:** `implementer` (host controller, policy helper, test namespace, and nftables contract are one cross-boundary unit).

**Validator role:** `backend_validator`; separate read-only security reviewer.

**Owned files:**
- `deploy/craft/render-boundary/policy-helper/controller.py`
- `deploy/craft/render-boundary/policy-helper/helper.py`
- `deploy/craft/render-boundary/policy-helper/tests/test_barrier_adapter.py` only if discovery parser needs a new pure seam
- `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
- `docs/plans/2026-09-29-craft-107-t14-control-policy-fix1-report.md`
- Any focused report-owned fixture file only if existing integration test requires it; declare before editing.

**Consumes:** Frozen prior review package 01 and its exact source hashes; existing host-issued flow attestation; conntrack tables; nft priority -150 rule set.

**Produces:** Fail-closed bounded flow discovery and conntrack verification, trustworthy canary receipt, disposable live proof of reuse and denial, and exact report/checkpoint hashes.

- [ ] Add failing tests for state normalization/selection, exact bidirectional conntrack parsing, malformed/truncated/oversized/ambiguous records, and strict family/protocol checks.
- [ ] Run targeted tests to capture RED before changes.
- [ ] Add failing canary tests for script failure before connect, duplicate source port, wrong destination, connected result, unexpected errno, no counter delta, and success only with explicit attempt evidence plus exact counter evidence.
- [ ] Extend the disposable namespace integration fixture to use its existing socket after install for a request/response marker, then issue a second connection to the same driver listener from another source port and assert denial plus exact counter increase.
- [ ] Bound the host discovery script inventory (FDs, proc TCP rows, number of candidates, serialized JSON bytes) and implement controller-side bounded stdout/stderr collection with kill-on-overflow and timeout cleanup.
- [ ] Implement the minimal fixes without changing browser behavior, attempt inventory, policy intent, or security resource limits.
- [ ] Run parser/canary focused tests, all policy-helper unit tests, and the disposable integration test. Expected: green including live conntrack and both socket assertions. If Docker daemon still cannot complete `docker run`, report the exact command/process evidence and stop before immutable T14 probe.
- [ ] Run `py_compile`, `git diff --check`, and a source inventory audit for exact tuples, conntrack priority, all deny rules and unchanged ATTEMPTS.
- [ ] Only if disposable proof passes, send parent a preflight with candidate identity, exact immutable run command, resource/policy limits unchanged, and cleanup plan. Wait for parent sequencing before one full immutable T14 candidate probe.
- [ ] Save task report with baseline/final hashes, task patch hash, RED/GREEN evidence, live tuple/counter/container receipts, cleanup verification, and explicit T14 acceptance status.

## Failure Handling

- If `/proc/net/nf_conntrack` is unavailable, no matching entry exists, proc output overflows, or conntrack semantics are uncertain, fail closed; do not move the chain before conntrack or broaden exceptions.
- If the disposable Docker command hangs/times out, verify all spawned containers/helpers are absent; report this environment blocker and do not run a renderer candidate.
- If a canary cannot distinguish firewall denial from listener loss/script error, do not claim pass; tighten its structured protocol or stop.
- If the established socket does not reuse the same tracked flow, redesign the control transport rather than allow the driver port.

## Pre-Dispatch Consistency Check

- Review coverage: the three HIGH and two MEDIUM findings all map to specific code/test steps above.
- Interface consistency: one seven-field exact flow is consumed by parser, controller, conntrack check, helper rule generator and canary.
- Task boundary: controller/helper/tests share same canary and tuple evidence; split would create unchecked interfaces.
- Resource use: one disposable Docker namespace test at a time; do not run concurrent Docker checks or immutable browser probe.
- Dependency graph: prior T14 Task 1 -> this fix -> disposable proof -> immutable T14 probe; no cycle.
