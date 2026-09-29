# Craft #107 live issue refresh

Read-only refresh timestamp: 2026-09-29 (Asia/Shanghai). Source: GitHub REST API, repository `1123786563/WeKnora-fork01`.

## Coverage and access evidence

Requested nodes: #107 and #119–#139 (22 issues). Anonymous REST reads succeeded for #107 and #119–#129, including issue JSON, comments (`per_page=100`), and formal sub-issues (`per_page=100`). Each successful issue reported `comments=0`; each formal sub-issue response was an empty array. No pagination continuation was present in the successful responses.

The first `gh api` attempt failed with `TLS handshake timeout`; `gh auth status` also reported `Timeout trying to log in ... (keyring)`. Direct unauthenticated `curl` then succeeded until the GitHub API returned HTTP 403 for #130–#139. Exact failing operation was `curl -fsSL --max-time 12 https://api.github.com/repos/1123786563/WeKnora-fork01/issues/<n>` (and associated comments/sub_issues requests), with `curl: (56) The requested URL returned error: 403`. Therefore #130–#139 titles, state, comments, formal sub-issues, and native dependency edges remain unknown. The dependency endpoint attempted for available nodes, `/issues/<n>/dependencies`, returned HTTP 404; no native blocked_by/blocking graph could be read from that endpoint.

## Successful issue snapshot

All successful nodes were `open`, with zero comments, and no formal sub-issues:

| Issue | Title | Updated |
|---|---|---|
| #107 | Spec: Craft 网页作品首版完整闭环 | 2026-09-23T05:17:25Z |
| #119 | [Craft T00] Freeze web-artifact contracts and parallel extension seams | 2026-09-23T08:30:03Z |
| #120 | [Craft T01] Accept arbitrary extensions as opaque read-only inputs | 2026-09-23T08:30:08Z |
| #121 | [Craft T02] Extract archive inputs within hard resource limits | 2026-09-23T08:30:12Z |
| #122 | [Craft T03] Keep uploaded code as data and prevent direct execution | 2026-09-23T08:30:16Z |
| #123 | [Craft T04] Build web artifacts from a fixed offline runtime | 2026-09-23T08:30:21Z |
| #124 | [Craft T05] Restrict retrieval to selected knowledge and record actual sources | 2026-09-23T08:30:24Z |
| #125 | [Craft T06] Render evidence-backed facts and distinguish model inference | 2026-09-23T08:30:31Z |
| #126 | [Craft T08] Add Task Owner, Collaborator, and Viewer access | 2026-09-23T08:31:03Z |
| #127 | [Craft T10] Reauthorize every source open for the current viewer | 2026-09-23T08:31:09Z |
| #128 | [Craft T11] Require owner consent before sharing restricted-source results | 2026-09-23T08:31:40Z |
| #129 | [Craft T14] Serve isolated previews with enforced no-egress policy | 2026-09-23T08:32:06Z |

## Containment and dependency evidence

The bodies of #119–#129 each explicitly identify `Parent #107`. This supports containment edges `107 contains {119..129}` for the successfully read subset. Body-declared dependency edges (`blocked_by`) are:

```text
120 -> 119
121 -> 120
122 -> 120
123 -> 120,122
124 -> 119
125 -> 123,124
126 -> 119
127 -> 124,126
128 -> 125,126
129 -> 119
```

Here `A -> B` means A is blocked by B. These are source facts from the issue bodies, not inferred edges. #121 and #122 are siblings after #120; #124, #126, and #129 are parallel after #119; #125 follows #123/#124; #127 follows #124/#126; #128 follows #125/#126.

## DAG candidate (inference; parent agent must decide)

Containment: `107 -> 119..139` only where formal/parent evidence is obtained; presently verified only for #119–#129 via issue bodies. Dependency orientation for execution:

```text
119 -> 120
120 -> 121,122,123
119 -> 124,126,129
123,124 -> 125
124,126 -> 127
125,126 -> 128
```

This candidate is acyclic over the verified subset. No cycle or missing reference was found in the verified body-declared edges. References to #130–#139, any additional dependencies, and whether those nodes belong to #107 remain unresolved because of the 403 rate/access block. The formal sub-issue API returned no children for every successfully read node, so the parent links above are body-level containment evidence rather than formal sub-issue edges.

## Unresolved references

- #130–#139: issue records, comments, formal sub-issues, and native blocked_by/blocking relationships unavailable after HTTP 403.
- Native dependency graph for #107 and #119–#129: `/dependencies` returned 404; only body-declared `Blocked by` facts are available.
- The requested recursive graph is incomplete until authenticated/API access is restored and all 22 nodes plus pagination are reread.
