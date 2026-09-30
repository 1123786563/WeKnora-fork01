# Craft #107 T19 E0 Evidence Fix 5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the independently reviewed Fix3 Task2 false-PASS paths in E0 control ordering, fixed command execution, and raw topology. Preserve an explicit BLOCKED verdict for model/config provenance until independent pinned OpenCode evidence exists.

**Architecture:** One controller-owned ordinal stream orders host operations, phase acknowledgments, recorder seals and cleanup. Use structured exact process argv, with no free-form shell, for every fixed direct/proxy control. Reconstruct participant security and route facts from raw Docker inspection; do not accept manifest summaries as authority. Treat config trace as diagnostic until an independent runtime provenance source is demonstrated.

**Tech Stack:** Python verifier/recorder fixture, JSONL event stream, Docker raw inspect schema, adversarial synthetic tests.

**Spec:** `docs/plans/2026-09-24-craft-107-t19-e0-fix3-task2-review.md`, prior Fix3/Fix4 plans, approved Craft #107 E0/E1 acceptance, `docs/plans/2026-09-24-craft-107-t19-e0-evidence-architecture.md`.

## Global Constraints

- No synthetic fixture result constitutes measured E0 PASS. Runner is currently incompatible and production model route remains default-off.
- Every accepted fact must be rederived from independently retained raw evidence; reject missing, duplicated, reordered, stale, mismatched or unparseable records.
- Preserve direct A/D request one-to-one lifecycle and existing 49 mutation rejection. No manifest, environment field, captured free text or client-authored trace may declare its own proof.
- No Docker run, production route, commit or push under this task. Exact uncommitted checkpoint and independent review required.

## Review Focus

- F1 control/cleanup operation order, including stops, waits, helpers, barriers, both recorder seals, removals and not-found inspections.
- F2 exact single fixed curl executable/argv/target/proxy and result binding; expected tokens in comments or a second command cannot count.
- F3 wrong hostile origin/provider/model, config bytes and trace that agree with each other must still fail or remain explicitly unavailable.
- F4 raw A/D/helper/client/image/UID/mount/capability/network/route facts must disallow a writable evidence mount or extra network path even when summary says safe.

---

### Task 1: Repair verifier false-PASS paths and keep provenance blocked

**Depends on:** Fix3 Task2 review FAIL. **Owner:** `backend_implementer`; **validator:** `backend_validator`, then independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/assert_v2.py`, `test_assert_v2.py`, `test_source_owned_v2.py` only. If the current recorder cannot express an authoritative ordinal, document the precise required runner/recorder interface and leave E0 blocked; do not infer order from file order or manifest labels.

**Consumes / produces:** Existing source-owned A/D stream and host evidence schema; produces strict timeline, command and topology checks plus adversarial tests.

- [ ] Capture exact three-file preimage/hash. RED mutants keeping every field and hash self-consistent but moving control stop/helper/barrier/seals/removal across required boundaries; shell command executes wrong curl target while expected tokens appear inert; A/D/helper raw inspect shows writable evidence mount, privilege, extra route or wrong image/UID while manifest stays safe; wrong origin/provider/model config and trace agree with each other.
- [ ] Require a single controller-origin ordinal and operation ID linked to each phase/action. Enforce stop → terminal wait/inspect → helper start → exactly one control attempt/barrier → helper stop; both D/A stream-end seals → removals → not-found inspection. Reject duplicate/missing ordinals and cross-phase references.
- [ ] Replace shell substring matching with a fixed executable plus exact parsed argv and bound exit/stderr for the sole attempted process. Reject shell scripts, comments, chained commands and an unobserved target.
- [ ] Require complete raw pre/post inspect for all participants and independent fixed command results for routes, binary/version and effective identity. Validate mounts including aliases/ancestors, privilege/caps, network attachments and port binds against the fixed topology; reject omissions. Fix hostile case expected targets in verifier code; for unavailable OpenCode selected-provider/attempt provenance return explicit BLOCKED, never PASS.
- [ ] Run source-owned and adversarial tests, `py_compile`, diff check, and package a reconstructible exact checkpoint/patch/report. Independent review must attempt new self-consistent evidence forgeries.

**Acceptance / failure handling:** All structurally forged false-PASS examples fail with a specific reason. The final verdict remains BLOCKED if pinned OpenCode provenance or compatible runner is absent. Report an unachievable source/interface precisely instead of fabricating a green E0 result.

### Task 2: Migrate runner and measure E0

**Depends on:** Task1 independently reviewed PASS and a demonstrated independent pinned OpenCode provider/attempt provenance source. **Owner:** `backend_implementer`; **validator:** `backend_validator` and independent `reviewer`. **Owned files:** runner/fixture files assigned in a separate brief after Task1. RED/GREEN exact raw schema, one bounded Docker matrix, retained evidence and complete cleanup. Never dispatch while the provenance dependency is unknown.
