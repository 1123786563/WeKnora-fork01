# Craft #107 Issue 摄取摘要

- 来源：GitHub GraphQL API，仓库 `1123786563/WeKnora-fork01`，UTC `2026-09-23T09:00:47.695405+00:00`。
- 覆盖：#107、#119–#139（22/22 请求节点成功）；#107 正式 `subIssues` 为空；未发现分页错误或 GraphQL errors。
- 计划证据：`docs/plans/2026-09-23-craft-web-artifact-issues-execution-prompt.md` 明确 T00–T20 对应 #119–#139，并要求实时依赖优先。

## 原生 dependency edges (blocking → blocked)

- #119 → #120 (GitHub `blocking` on #119; reciprocal `blockedBy` observed on #120: False)
- #119 → #124 (GitHub `blocking` on #119; reciprocal `blockedBy` observed on #124: False)
- #119 → #126 (GitHub `blocking` on #119; reciprocal `blockedBy` observed on #126: False)
- #119 → #129 (GitHub `blocking` on #119; reciprocal `blockedBy` observed on #129: False)
- #119 → #138 (GitHub `blocking` on #119; reciprocal `blockedBy` observed on #138: False)
- #120 → #121 (GitHub `blocking` on #120; reciprocal `blockedBy` observed on #121: False)
- #120 → #122 (GitHub `blocking` on #120; reciprocal `blockedBy` observed on #122: False)
- #120 → #123 (GitHub `blocking` on #120; reciprocal `blockedBy` observed on #123: False)
- #121 → #139 (GitHub `blocking` on #121; reciprocal `blockedBy` observed on #139: False)
- #122 → #123 (GitHub `blocking` on #122; reciprocal `blockedBy` observed on #123: False)
- #122 → #139 (GitHub `blocking` on #122; reciprocal `blockedBy` observed on #139: False)
- #123 → #125 (GitHub `blocking` on #123; reciprocal `blockedBy` observed on #125: False)
- #123 → #130 (GitHub `blocking` on #123; reciprocal `blockedBy` observed on #130: False)
- #124 → #125 (GitHub `blocking` on #124; reciprocal `blockedBy` observed on #125: False)
- #124 → #127 (GitHub `blocking` on #124; reciprocal `blockedBy` observed on #127: False)
- #125 → #128 (GitHub `blocking` on #125; reciprocal `blockedBy` observed on #128: False)
- #125 → #131 (GitHub `blocking` on #125; reciprocal `blockedBy` observed on #131: False)
- #126 → #127 (GitHub `blocking` on #126; reciprocal `blockedBy` observed on #127: False)
- #126 → #128 (GitHub `blocking` on #126; reciprocal `blockedBy` observed on #128: False)
- #126 → #135 (GitHub `blocking` on #126; reciprocal `blockedBy` observed on #135: False)
- #127 → #132 (GitHub `blocking` on #127; reciprocal `blockedBy` observed on #132: False)
- #128 → #133 (GitHub `blocking` on #128; reciprocal `blockedBy` observed on #133: False)
- #129 → #130 (GitHub `blocking` on #129; reciprocal `blockedBy` observed on #130: False)
- #130 → #131 (GitHub `blocking` on #130; reciprocal `blockedBy` observed on #131: False)
- #130 → #134 (GitHub `blocking` on #130; reciprocal `blockedBy` observed on #134: False)
- #131 → #132 (GitHub `blocking` on #131; reciprocal `blockedBy` observed on #132: False)
- #131 → #139 (GitHub `blocking` on #131; reciprocal `blockedBy` observed on #139: False)
- #132 → #133 (GitHub `blocking` on #132; reciprocal `blockedBy` observed on #133: False)
- #133 → #139 (GitHub `blocking` on #133; reciprocal `blockedBy` observed on #139: False)
- #134 → #135 (GitHub `blocking` on #134; reciprocal `blockedBy` observed on #135: False)
- #134 → #136 (GitHub `blocking` on #134; reciprocal `blockedBy` observed on #136: False)
- #134 → #137 (GitHub `blocking` on #134; reciprocal `blockedBy` observed on #137: False)
- #135 → #139 (GitHub `blocking` on #135; reciprocal `blockedBy` observed on #139: False)
- #136 → #139 (GitHub `blocking` on #136; reciprocal `blockedBy` observed on #139: False)
- #137 → #139 (GitHub `blocking` on #137; reciprocal `blockedBy` observed on #139: False)
- #138 → #139 (GitHub `blocking` on #138; reciprocal `blockedBy` observed on #139: False)

## 节点覆盖

- #107 `OPEN` [ready-for-agent] — Spec: Craft 网页作品首版完整闭环
- #119 `OPEN` [ready-for-agent] — [Craft T00] Freeze web-artifact contracts and parallel extension seams
  - 正文 Blocked by 段：- None (can start immediately after scheduling).
- #120 `OPEN` [ready-for-agent] — [Craft T01] Accept arbitrary extensions as opaque read-only inputs
  - 正文 Blocked by 段：- #119 — T00
- #121 `OPEN` [ready-for-agent] — [Craft T02] Extract archive inputs within hard resource limits
  - 正文 Blocked by 段：- #120 — T01
- #122 `OPEN` [ready-for-agent] — [Craft T03] Keep uploaded code as data and prevent direct execution
  - 正文 Blocked by 段：- #120 — T01
- #123 `OPEN` [ready-for-agent] — [Craft T04] Build web artifacts from a fixed offline runtime
  - 正文 Blocked by 段：- #120 — T01 / - #122 — T03
- #124 `OPEN` [ready-for-agent] — [Craft T05] Restrict retrieval to selected knowledge and record actual sources
  - 正文 Blocked by 段：- #119 — T00
- #125 `OPEN` [ready-for-agent] — [Craft T06] Render evidence-backed facts and distinguish model inference
  - 正文 Blocked by 段：- #123 — T04 / - #124 — T05
- #126 `OPEN` [ready-for-agent] — [Craft T08] Add Task Owner, Collaborator, and Viewer access
  - 正文 Blocked by 段：- #119 — T00
- #127 `OPEN` [ready-for-agent] — [Craft T10] Reauthorize every source open for the current viewer
  - 正文 Blocked by 段：- #124 — T05 / - #126 — T08
- #128 `OPEN` [ready-for-agent] — [Craft T11] Require owner consent before sharing restricted-source results
  - 正文 Blocked by 段：- #125 — T06 / - #126 — T08
- #129 `OPEN` [ready-for-agent] — [Craft T14] Serve isolated previews with enforced no-egress policy
  - 正文 Blocked by 段：- #119 — T00
- #130 `OPEN` [ready-for-agent] — [Craft T15] Promote web versions only after four independent checks
  - 正文 Blocked by 段：- #123 — T04 / - #129 — T14
- #131 `OPEN` [ready-for-agent] — [Craft T07] Pin historical artifact versions to their original evidence
  - 正文 Blocked by 段：- #125 — T06 / - #130 — T15
- #132 `OPEN` [ready-for-agent] — [Craft T12] Download a fixed source bundle with citation manifest
  - 正文 Blocked by 段：- #131 — T07 / - #127 — T10
- #133 `OPEN` [ready-for-agent] — [Craft T13] Require owner consent to export restricted derived data
  - 正文 Blocked by 段：- #128 — T11 / - #132 — T12
- #134 `OPEN` [ready-for-agent] — [Craft T16] Serialize Workspace-writing Runs with a durable lease
  - 正文 Blocked by 段：- #130 — T15
- #135 `OPEN` [ready-for-agent] — [Craft T09] Let Collaborators request serialized edits with their own grants
  - 正文 Blocked by 段：- #126 — T08 / - #134 — T16
- #136 `OPEN` [ready-for-agent] — [Craft T17] Persist stop intent and retain repairable drafts
  - 正文 Blocked by 段：- #134 — T16
- #137 `OPEN` [ready-for-agent] — [Craft T18] Reconnect to the authoritative Run without resubmission
  - 正文 Blocked by 段：- #134 — T16
- #138 `OPEN` [ready-for-agent] — [Craft T19] Pause Runs durably when Task Budget is exhausted
  - 正文 Blocked by 段：- #119 — T00
- #139 `OPEN` [ready-for-agent] — [Craft T20] Integrate and verify the complete web-artifact journey
  - 正文 Blocked by 段：- #121 — T02 / - #122 — T03 / - #131 — T07 / - #135 — T09 / - #133 — T13 / - #136 — T17 / - #137 — T18 / - #138 — T19

## 不完整项 / 推断

- #107 的 API `subIssues.total=0` 且 GraphQL `subIssues.nodes=[]`；因此 #119–#139 是执行提示词的编号映射，不能据此推断为正式父子树。
- 依赖边以上仅采用 GitHub 原生 `blocking`/`blockedBy` 字段；正文中的 “Blocked by” 另作候选证据，未擅自提升为正式依赖。
- 需要主控继续核实：任务正文中的文本依赖与原生边是否一致、是否存在正文引用但不在 #119–#139 的外部节点。
