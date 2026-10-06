# Craft #107 Live DAG and Execution Ledger — 2026-10-06

- Repository: `1123786563/WeKnora-fork01`.
- Retrieval time: `2026-10-06T10:52:15Z` UTC, authenticated GitHub REST API.
- Issue snapshot: `docs/plans/2026-10-06-craft-107-live-issue-refresh.md` (SHA-256 `cf7335866be6c5c8647d07f6b1beda1a0cf6746d58712aed195d7c082a2d37b3`).
- Approved requirements: `docs/specs/2026-09-23-craft-web-artifact-spec.md`; `docs/adr/0004-task-is-session.md`; `CONTEXT.md`; execution scope: `docs/plans/2026-09-23-craft-web-artifact-issues-execution-prompt.md`.
- Integration Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-execution/WeKnora-fork01`, detached at `main` HEAD `6e1b2072a13a798e7ec25be44784e5242bd2f178`.
- Commit policy: no commits, staging, push, merge, deploy, or Issue mutation authorized. Use workspace checkpoint and preserve unrelated source checkout changes.
- Live tree: #107 is closed; formal sub-issues endpoint is empty. #119–#139 are 21 execution-ticket mappings/body references, all closed with one comment each. Native `blocked_by`/`blocking` endpoints all succeeded; 36 native directed edges, reciprocal, no missing nodes/self-edge/cycle. Containment is not dependency.
- Prior documented inference edges remain separate: T08/#126 → T05/#124 and T08/#126 → T14/#129, justified by production TaskAccessChecker injection and live preview Task access requirements. They are verification prerequisites, not native GitHub edges.
- The live refresh report's inherited “T14 running / only T14 frontier” status is stale: it cites the 2026-09-29 DAG state and predates the 2026-10-02 T14 live evidence and 2026-10-05 F08 production integration in current `main`. Status below uses later main-branch Ledger entries and reachable commit evidence. Issue closed state is not acceptance proof.

## Dependency graph

Native edges (A blocks B):

```text
119 -> 120,124,126,129,138
120 -> 121,122,123
121 -> 139
122 -> 123,139
123 -> 125,130
124 -> 125,127
125 -> 128,131
126 -> 127,128,135
127 -> 132
128 -> 133
129 -> 130
130 -> 131,134
131 -> 132,139
132 -> 133
133 -> 139
134 -> 135,136,137
135 -> 139
136 -> 139
137 -> 139
138 -> 139
```

Stable topological order with the two inferred verification edges: T00 → T01 → T08 → T05 → T14 → T19 → T02 → T03 → T10 → T04 → T06 → T15 → T11 → T07 → T16 → T12 → T09 → T17 → T18 → T13 → T20. The existing full DAG `docs/plans/2026-09-23-craft-107-dag.md` retains per-ticket acceptance, role, interface, ownership, and verification definitions; all 21 IDs map one-to-one to #119–#139. Its early status snapshot is superseded by this ledger.

## Current ticket status and evidence

For each row, acceptance/owned files/interfaces/verification are those in the named Issue body and the per-node section of `2026-09-23-craft-107-dag.md`; execution evidence is in `2026-09-23-craft-107-ledger.md` unless noted.

| ID / Issue | Status | Evidence / checkpoint | Remaining gate |
|---|---|---|---|
| T00 / #119 | verified | Contract checkpoint in original DAG, T00 report, acceptance work carried into all later tickets | none |
| T01 / #120 | verified | T01 ledger section; unknown-input decision integrated into T20 journey | none |
| T02 / #121 | verified | T20 archive expansion panel and HTTP journey integration, `080913c66` | none |
| T03 / #122 | verified | OCR wrapper/class-path defenses in current `internal/modules/craft/input_code.go`; F08 production dispatcher uses T03-gated normal face (`main` ledger 2026-10-05); T03 feature and journey tests recorded | final review will inspect current complete code; do not reopen stale 2026-09-29 Fix1 checkpoint findings without matching them to current source |
| T04 / #123 | verified | offline build acceptance `docs/testing/craft/t04/2026-10-04-offline-build/`; F08 production receipt dispatch `93aa6bad7`; build gate tests | T20 live post-F08 run also validates integration |
| T05 / #124 | verified | T05 and T20 knowledge/CSV evidence mapping in Ledger; Task access edge integrated | none |
| T06 / #125 | verified | T06 six-criterion acceptance in Ledger, commit `db38fa26b` | none |
| T07 / #130? no: #131 | verified | T07/T10 dual-ticket evidence section and version-evidence binding | none |
| T08 / #126 | verified | access model and TaskAccessChecker production wiring; T20 integration | none |
| T09 / #135 | verified | collaborator-run and serial edit evidence-only acceptance in Ledger | none |
| T10 / #127 | verified | per-viewer source reauthorization evidence in T07/T10 Ledger section | none |
| T11 / #128 | verified | share-consent flow and workbench composition; evidence-only acceptance | none |
| T12 / #132 | verified | source download/export manifest acceptance; evidence-only section | none |
| T13 / #133 | verified | export consent flow and workbench composition; evidence-only section | none |
| T14 / #129 | blocked; external image pin and browser probe evidence incomplete | current main contains restoration `deb2f66d6` + live seven-criterion policy run and 140/140 evidence manifest in `docs/testing/craft/t14/2026-10-02-live-acceptance-rerun/`; live Issue #129 additionally requires a real browser to load valid local HTML/CSS/JS/assets and denied-egress browser evidence; final T20 plan expects `WebPageLoadProbe`, but current main only has an unregistered interface slot | Requires an authorized immutable RunView image digest/runtime pin set and T14 real-browser page-load/no-egress integration evidence. Do not infer production digest from local Docker image ID/RepoDigest. |
| T15 / #130 | verified | six-criterion promotion acceptance in Ledger; T14 is a verified predecessor | none |
| T16 / #134 | verified | six-criterion evidence in Ledger and following T17/T18/T20 acceptance | none |
| T17 / #136 | verified | stop-intent acceptance and OCR repairs in Ledger | none |
| T18 / #137 | verified | reconnect/replay acceptance and OCR repairs in Ledger | none |
| T19 / #138 | verified | six-criterion persisted budget pause acceptance, `91f395d60`; F08 uses budget-backed coordinator | none |
| T20 / #139 | blocked on verified T14/RunView deployment prerequisites; final OCR in progress | nine acceptance bullets mapped in Ledger; integration `080913c66`; F08 post-closure integration `93aa6bad7`; SDD boot fix and POST-only anonymous HMAC route exemption independently validated/reviewed; official harness starts/tears down cleanly but run evidence `docs/testing/craft/t20/2026-10-06-post-f08/` proves the local fixture hits a fail-closed missing RunView resolver, leaves delegation `prepared`, and produces no version | Requires deployment-owned RunView pin set and registered T14 browser-load/no-egress probe; then rerun the official e2e. Full-scope OCR independently continues over committed Craft delivery plus this workspace. |

Graph validation: 21 unique nodes; every edge endpoint resolves; no self-loop or cycle; 36 native edges and 2 separately identified inferred verification edges; all 36 parent stories map to at least one ticket and T20.

## Execution state

- The previously planned root-ticket fan-out has already occurred in the prior execution history; all ticket implementation and targeted review records are in the persistent Ledger and current `main` history. No duplicate implementation tickets are dispatched from this refreshed frontier.
- Current execution blockers are T14's required browser-load/no-egress implementation and the deployment-owned immutable RunView pin set. The previous T20 provider-registration and anonymous-HMAC-auth defects are repaired, independently validated, and independently reviewed. The official stack run produced a reproducible fail-closed result and was precisely torn down. Final OCR continues independently; it cannot substitute for the missing acceptance environment or promote T14/T20.
- Review state: task-level Reviews and multiple prior OCR repair rounds are recorded. Earlier `craft-107-ocr-final-3.md` is explicitly partial (`13 findings`, `7 of 27 selected items failed`) and is not a final clean report. A new full-scope OCR pass is mandatory after the live integration rerun. Any valid critical/high/medium finding becomes a scoped repair plan and returns through SDD, validation, independent review, and OCR re-review.
- Environment note: current source checkout was on `fix/migrations-dedup-preexisting` with 23 unrelated untracked docs and a divergent history (193 commits ahead / 71 behind `main`). It remains untouched. All execution documents and further work are isolated from it in this Worktree.
