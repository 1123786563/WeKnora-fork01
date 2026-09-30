# T05 durable knowledge snapshot Task 2: independent scoped review

Date: 2026-09-24. Scope: Task 2 and its 2026-09-24 query clarification in `2026-09-23-craft-107-t05-knowledge-snapshot-plan.md`, the Task 2 report and its four file hashes, approved Craft Spec #107 user story 2, `CONTEXT.md`, ADR-0004/0009, and the T05 worker seam map. Read-only source review; no OCR or delegation. This review covers the Task 2 changes in `craft_session.go`, `craft_session_test.go`, `agent_run_graph.go`, and `agent_run_snapshot_test.go`. It does not approve Task 3 worker handoff, Publisher, RunView isolation, or full T05.

## Checkpoint and verification

The four current files matched the Task 2 report's after SHA-256 hashes: `f23ab927` (session source), `f5c74739` (session tests), `5f1fbd79` (graph source), and `89108479` (snapshot tests). The working tree also has unrelated concurrent changes, so the whole `git diff` is not attributed to Task 2. The Task 2 report supplies before hashes and the narrow scope; there is no separate Task 2 patch file at review time.

I reran the report's focused snapshot, admission, and T05 ACL/replay command with `-count=1`; it passed (`ok github.com/Tencent/WeKnora/internal/application/service 2.198s`, exit 0). The report's attempted full service suite was interrupted and is not counted as passing.

## Verdict

- **Scoped Spec compliance: PASS.** `craftRunSnapshot` constructs the typed selection from `req.KnowledgeScope` and sets its retrieval query to the authenticated `req.Prompt` (`craft_session.go:966-1000`). The top-level snapshot query remains the server-composed model query with input and base-version guidance. The snapshot builder/parser accept those distinct values while bounding the retrieval query and rejecting null, malformed, noncanonical, or multi-KB selections (`agent_run_graph.go:161-243`). The snapshot bytes enter the admission digest, and same-key changed selection conflicts; cross-actor replay is rejected (`craft_session.go:811-850`, `craft_session_test.go:444-508`). Empty scope is an explicit empty list, not an all-KB fallback.
- **Scoped code quality: PASS.** The narrow code path preserves the T01 input manifest, T08 actor identity, and TaskWrite admission check. The selected KB ID is stored as metadata and is not appended to the model message. The focused tests passed against the exact reported file hashes. I found no actionable critical, high, or medium defect in this Task 2 delta.

## Evidence and limits

`StartRun` binds `scope.UserID` to the authenticated caller, obtains the current Craft Task write path through `writeSession`, retains the owner as the durable storage identity, and records the actor separately. The HTTP request contract exposes only the single `knowledge_scope` string (`internal/handler/session/craft.go:285,494`); the client cannot provide `CraftKnowledgeSelectionSnapshot` or overwrite the server-composed query. `durableUserMessage` still uses the top-level model query, and the snapshot test verifies the selected KB ID is absent from that message.

The focused tests prove snapshot parsing, selected/empty admission, stable snapshot bytes, changed-selection conflict, actor-bound replay, and existing T05 service ACL behavior. They do not prove the eventual worker consumes this new selection, checks current KB/document grants at dispatch, or confines published material to the RunView. Those are explicit Task 3 and integration gates, so full T05 remains unverified.

No separate Task 2 finding is raised. A successful collaborator admission carrying a selected KB would add direct test coverage for the owner/actor distinction; existing tests cover the collaborator's attempted same-key takeover and T08's worker actor restoration. This is a nonblocking coverage suggestion, not evidence of a behavior defect.
