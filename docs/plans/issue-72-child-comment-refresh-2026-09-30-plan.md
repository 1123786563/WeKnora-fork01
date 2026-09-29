# Issue #72 Child Issue Comment Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to execute this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve a complete, timestamped read of comments on all 33 current Issue #72 children and reconcile old comment claims against later approved decisions and current execution evidence.

**Architecture:** Archive a concise source-attributed comment audit and append its pointer to the Issue #72 execution Ledger. Keep remote containment and status evidence from the existing live-tree refresh; classify comment claims as historical, superseded, or current only when a later approved source supports that classification.

**Tech Stack:** Authenticated GitHub REST API via `gh`, JSON validation and SHA-256, Markdown.

**Spec:** Root task `/Users/wuyongjun/.codex/attachments/f2475d5c-b99d-461a-8a7c-2475ec8b0c21/pasted-text-1.txt`; current sources `docs/plans/issue-72-live-tree-refresh-2026-09-30.md`, `docs/plans/issue-72-issues-inventory.md`, `docs/plans/issue-72-dag.md`, `docs/plans/issue-72-execution-ledger.md`, `docs/plans/issue-72-user-rulings.md`, and `docs/adr/0012-lago-as-commercial-billing-authority.md`.

## Global Constraints

- Preserve the #72 descendant scope; do not add Issue #30.
- Read every child Issue comment endpoint with pagination; a missing, invalid, or empty output file is not evidence of zero comments.
- Do not change GitHub Issues, comments, labels, PRs, or branches.
- Do not edit or infer status from any owner-controlled dirty worktree.
- The Sep 20 #74/#75 comments are dated historical observations; any “superseded” conclusion must cite later approved evidence and retain the remote Issue's current OPEN/CLOSED status separately.
- Preserve approved R-2, R-3, R-4, R-6 and the documented #82/#86/#87 acceptance gates without reopening or promoting them.
- Never copy credentials or authentication material into the audit.

## Review Focus

- Confirm exactly 33 child comment responses exist, parse as arrays, cover #73–#105, and the source manifest hash corresponds to those exact response files.
- Preserve all non-empty comment bodies and source URLs in the summary; verify their claims against current ruling/ledger evidence rather than treating comment text as new authorization.
- Confirm no comment contains a child Issue or dependency assertion omitted from the current DAG; distinguish explicit dependency claims from ordinary references.
- Keep remote Issue status distinct from local implementation and acceptance status.
- Ensure relative links resolve and the complete Markdown changes pass `git diff --check`; document that OCR may skip Markdown if the tool does not support the extension.

## Task DAG

```text
Task 1: archive child-comment capture + source reconciliation
```

### Task 1: Archive complete child-comment refresh

**Dependencies:** The live-tree refresh has authenticated all child issue bodies, statuses, labels, and timelines. The current comments refresh separately fetched all child comment endpoints and pagination at `2026-09-29T18:57:20Z`.

**Owner role:** `mechanical_worker` for a bounded documentation-only archive.

**Validator role:** `reviewer` for independent API coverage, claim provenance, and current-state reconciliation.

**Owned files:**

- Create `docs/plans/issue-72-child-comments-refresh-2026-09-30.md`.
- Append one execution record to `docs/plans/issue-72-execution-ledger.md`.
- Do not modify the inventory, Issue DAG, ADR, Spec, product code, or any other plan.

**Consumes:**

- Current GitHub tree report: `/tmp/issue72-tree-refresh-current-20260930.md` (34 unique nodes, 33 native containment edges, no grandchildren, current children #73–#105).
- Paginated comment responses: `/tmp/issue72-refresh-20260930/comments-current/73.json` through `105.json`.
- Manifest SHA-256: `1010e2d6490d406e71e86fbc95f85ad62cbb29aea2498beeccb49e52eb64ee21`, computed in ascending numeric issue order #73–#105 from lines `<filename> SHA256(file bytes)` joined with `\n` and no trailing newline.
- Current ruling sources: `docs/plans/issue-72-user-rulings.md` (R-2, R-3, R-4, R-6) and `docs/adr/0012-lago-as-commercial-billing-authority.md`.
- Current acceptance evidence: `docs/plans/issue-72-execution-ledger.md`, `docs/plans/issue-72-ledger-74.md`, and `docs/plans/issue-72-ledger-82.md`.

**Produces:**

- A complete coverage record for 33 child endpoints, all 33 valid JSON arrays, 8 total comments on #73–#80, and no comments on #81–#105.
- A dated source-linked summary of each non-empty comment; identify #74's Sep 20 blocked-env note as historical relative to later #74 evidence while preserving its remote OPEN status, and identify #75's Sep 20 wallet-limit/options note as superseded by the Sep 23 user-confirmed R-2/ADR-0012 option B while preserving its remote CLOSED status.
- A checked classification that no comment introduces a new containment edge or unrecorded dependency; ordinary issue cross-references remain references.
- No change to the #82/#86/#87 acceptance gates or current DAG readiness.

**Implementation steps:**

- [x] Validate all 33 response JSON files parse as arrays and recompute the manifest in ascending numeric issue order #73–#105 using lines `<filename> SHA256(file bytes)` joined with `\n` and no trailing newline; expect 33 files, no missing/invalid file, 8 total comment objects, and the manifest hash above. Independently reproduced after implementation and fix; lexical order was verified to yield a different digest.
- [x] Write the dated comment audit with the API source, capture time, endpoint coverage, comment IDs/URLs, concise claims, and evidence-backed historical/superseded classification.
- [x] Append a Ledger entry linking the audit and manifest, recording the two stale-comment reconciliations and unchanged readiness gates.
- [x] Run `git diff --check`; exit code 0.
- [x] Verify relative links and ensure only the two task-owned files were committed.
- [x] Commit only the two owned paths with subject `docs(issue-72): archive child issue comment refresh`; initial commit `5650f6b`, correction commit `2cf53af`.

**Verification:** A deterministic script reparses the 33 responses and recomputes the exact manifest; `git diff --check` passes; all relative links resolve; the task package and review cover both owned Markdown paths. No tests or services are needed for this read-only evidence archive.

**Acceptance mapping:** Recursive child comment completeness → 33/33 authenticated paginated responses; comment-derived requirement/dependency classification → eight source-linked comments reconciled against the current DAG and approved records; global gates → explicit unchanged-state entry.

**Failure handling:** If any response fails parsing, coverage, pagination, hash reproduction, or source reconciliation, do not commit the claim of completeness. Re-fetch only failed endpoints; preserve any unreadable endpoint as an explicit unresolved gap.

## Self-review

- **Spec coverage:** This plan closes the root task's child-comment and comment-only dependency inspection requirement; containment/status/timeline coverage remains in the linked live-tree refresh.
- **Step scan:** Every step has a checkable output; no product decision is introduced.
- **Type and interface consistency:** No code interfaces change.
- **Review Focus:** Response coverage, stale comment provenance, DAG dependency classification, remote/local state distinction, and tool review coverage have explicit checks.
- **Proportion:** One audit note and one Ledger pointer match the read-only scope.
