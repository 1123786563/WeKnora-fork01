# Issue #72 live tree and gate refresh — 2026-09-30

## Scope and sources

Repository: `1123786563/WeKnora-fork01`. This is a read-only evidence synthesis and local documentation refresh. No GitHub state was changed, no product code or tests were run, and no service or shared database was started.

- Full authenticated recursive traversal: `/tmp/issue72-live-issue-tree-audit.md`, captured 2026-09-29 23:49:35 CST.
- Root, #74, and child-count spot checks: 2026-09-30 00:00 CST. These are later spot checks, not a second full traversal.
- #82 frontier and worktree audit: `/tmp/issue72-82-frontier-audit-20260930.md`.
- #86 checkpoint acceptance audit: `/tmp/issue72-86-checkpoint-validator.md`.
- #87 R16 scoped independent plan review: execution Ledger records PASS at plan SHA-256 `7f28e040637048a2e6944935d61aa23d738a49e864db56e6b6c86998a20ce968`, integration HEAD `8329b85d4dfff301d03f94406dfc829d87cb5b26`; this is a plan review, not implementation approval.
- Approved rulings: [`issue-72-user-rulings.md`](issue-72-user-rulings.md), R-3 and R-6.
- Local documents validated: [`issue-72-issues-inventory.md`](issue-72-issues-inventory.md), [`issue-72-dag.md`](issue-72-dag.md), and [`issue-72-execution-ledger.md`](issue-72-execution-ledger.md).

## Traversal result

The full traversal fetched `gh api repos/1123786563/WeKnora-fork01/issues/72`, paginated comments, timeline, and native `sub_issues`; for each discovered child N it fetched `gh api .../issues/N`, paginated `.../issues/N/sub_issues`, and paginated `.../issues/N/timeline`. The root's native child list contained 33 unique Issues, #73–#105. Each child's native child endpoint returned an empty array: 34 deduplicated nodes total, 33 containment edges (`72 -> N`), and no grandchildren. The audit reports no missing child, duplicate, failed endpoint, or unresolved pagination after retries.

At the full traversal time the root #72 was open. Among its children, 7 were closed (#73 and #75–#80) and 26 were open (#74 and #81–#105). Thus the combined count is 7 closed and 27 open including #72. The 2026-09-30 00:00 checks were root/#74/child-count spot checks only; they do not establish that every Issue's current status was refetched at that later time. The historical inventory's earlier status snapshot is preserved as history and is not rewritten to imply a GitHub transition.

## Containment, references, and dependencies

Native `sub_issues` proves parent-child containment only. The root timeline contains cross-reference records for the 33 children; descendant timelines also contain cross-references. The audit found no PR source URL/number and no timeline record that establishes those references as execution dependencies. Accordingly, the apparent `73 -> 77 -> 73` loop in candidate timeline-reference edges is a reference cycle, not an accepted dependency cycle: #77's references to #73/#78 and #73's reference to #77 have no body/API dependency evidence in the supplied audit. Keep dependency edges sourced from the Issue dependency/body evidence and the validated local DAG; do not add these timeline references as DAG edges or silently delete an actual dependency.

## Rulings and acceptance gates

R-6 is approved and settled: each published Plan Version's explicit Billable Metric-to-dimension mapping selects its unique owning subscription/version; ambiguity blocks only the affected dimension; paid purchases with Usage Charges remain excluded. This settles the #87 business decision, not implementation acceptance.

R-3 is also settled as the approved #81 deviation: pre-payment validation uses authoritative subscription Plan Version/currency/amount and a no-charges single subscription line; complete finalized-Invoice line-item review occurs at payment time for #81/#82/#84. R-6 continues to exclude charge-bearing purchases absent a separately approved R-3-compatible contract. See the ruling text for its scope and error cost.

- **#82:** GitHub Issue is OPEN. AC1/AC2 and AC3 have existing integrated four-leg evidence with qualification; the dedicated tagged T9 live test was not successfully rerun on the integrated revision, and AC4 real Alipay sandbox linkage remains unavailable. Overall acceptance remains incomplete. The reviewed T9 test-harness repair branch at `d8d21cd967c94c09d32c34a2981904758f442330` is clean but not an ancestor of integration HEAD `8329b85d4dfff301d03f94406dfc829d87cb5b26`; it has scoped review and compile/test-harness validation only. Do not integrate it here or treat it as live acceptance.
- **#86:** GitHub Issue is OPEN and the checkpoint audit verdict is FAIL/not accepted. The 110 offline helper tests and compilation passed, but do not prove the missing current-hash Lago behavior: cross-month expiry, current-script-bound rank/amount, race/expiry behavior, and exact-hash reconciliation. Historical runtime artifacts are partial and not bound to current script hashes; #85 paid-top-up fulfillment and #75 expiry evidence also remain prerequisites.
- **#87:** R16 scoped plan review PASS at the recorded plan hash. #87 implementation remains gated on verified #86 integration/acceptance and Lago Task 0 live contract evidence; R-6 and R-3 no longer await user decisions. No downstream issue status or acceptance is promoted by this documentation refresh.

## Local document validation and limits

The current inventory and DAG paths exist and were read; the full-traversal source report's statement that they were absent describes its separate checked-out workspace and is not a current-worktree fact. The 00:00 checks are represented only as reported in the Task 17 source record; raw command output was not supplied separately. The full traversal report is a saved audit summary; raw API JSON is referenced there under `/tmp/issue72-audit/` and is not copied into this repository. These limits do not alter the reported 33-child traversal result but bound independent reproduction from this worktree.

The #82 audit could not freshly confirm #74/#81 remote state because those API calls reset; their completion remains local DAG/ledger evidence. The #86 audit records its worktree was already dirty and did not modify it. At the audit checkpoint, `.worktrees-issue72/lago-int` was dirty under its owner; this task did not inspect or modify its working files. No shared resource ownership was inferred or changed.

The local DAG structural recount found 33 unique Mermaid nodes (#73–#105), 81 edges (75 business and 6 scheduling), and zero duplicate edges; no graph edges were changed. The existing DAG records its historical Kahn validation at 33 nodes/81 edges with no cycle and zero topological violations; the Task 17 recount confirms graph cardinality, while the timeline candidate loop above is not imported into the dependency graph. Validated document paths are the inventory, DAG, and execution Ledger named above; the 2026-09-29 historical traversal note that could not find them is retained as a source-workspace discrepancy, not treated as evidence of current absence.
