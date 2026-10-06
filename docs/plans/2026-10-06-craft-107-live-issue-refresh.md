# Craft #107 live issue graph refresh

- Retrieved: `2026-10-06T10:52:15Z` (UTC), authenticated `gh` REST API, repository `1123786563/WeKnora-fork01`.
- Scope: root `#107`, requested range `#119`–`#139`; issue records, paginated comments, formal sub-issues, and native dependency endpoints.
- Source endpoints: [`#107`](https://api.github.com/repos/1123786563/WeKnora-fork01/issues/107), [`#107/sub_issues`](https://api.github.com/repos/1123786563/WeKnora-fork01/issues/107/sub_issues), and for each node `issues/{n}`, `issues/{n}/comments`, `issues/{n}/dependencies/blocked_by`, `issues/{n}/dependencies/blocking`.
- Local comparison: `docs/plans/2026-09-23-craft-107-dag.md`; no newer `craft-107` Ledger file was present under `docs/` (the DAG's embedded refresh/Ledger evidence is the latest located source).

## Retrieval coverage

- All 21 requested child records (`#119`–`#139`) were retrieved and deduplicated by repository + issue number. Root `#107` was also retrieved.
- `#107` formal sub-issues endpoint returned an empty array. The 21 tickets are therefore execution mappings/body references, not formal containment edges.
- Every ticket currently has state `closed`, label `ready-for-agent`, and one comment according to the issue record's comment count. Comments were fetched through `--paginate`; no pagination or API error was observed.
- Every ticket body contains the Craft execution mapping to parent `#107` (containment/reference evidence). This does not create dependency edges.
- Native dependency calls succeeded for every node. Reciprocal audit found zero violations. Deduplicated native graph has 36 directed edges and no self-edge or cycle.

## Current node snapshot

All nodes `#119`–`#139`: `closed`; label `ready-for-agent`; comments `1`. `updated_at` values, in issue-number order:

`119 2026-10-01T10:13:12Z`, `120 10:13:16Z`, `121 10:13:19Z`, `122 10:13:22Z`, `123 2026-10-03T23:15:03Z`, `124 10:13:25Z`, `125 2026-10-04T00:06:46Z`, `126 10:13:29Z`, `127 10:13:32Z`, `128 2026-10-04T00:37:55Z`, `129 2026-10-02T02:44:09Z`, `130 2026-10-03T23:23:01Z`, `131 2026-10-04T00:19:31Z`, `132 2026-10-04T00:48:22Z`, `133 2026-10-04T00:58:07Z`, `134 2026-10-03T23:36:52Z`, `135 2026-10-04T00:28:16Z`, `136 2026-10-03T23:43:17Z`, `137 2026-10-03T23:51:11Z`, `138 2026-10-04T00:01:27Z`, `139 2026-10-04T02:29:16Z` (all timestamps UTC; omitted date on same-day entries is `2026-10-01`).

Root `#107` is `closed`, label `ready-for-agent`, `comments=1`, `updated_at=2026-10-04T22:27:43Z`.

## Native dependency DAG candidate

Notation `A -> B` means A blocks B. This is the actual dependency graph, separate from containment/body references:

```
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

The graph matches the native edge audit embedded in `2026-09-23-craft-107-dag.md`: no node/edge additions or removals were found. The prior document's two inferred verification edges (`#126 -> #124` and `#126 -> #129`, corresponding T08 -> T05/T14) remain inference only and are not native GitHub dependencies.

## Verified/ready frontier from persisted implementation evidence

Issue labels and closed state are not treated as implementation evidence. The latest located DAG/Ledger evidence records:

- `verified`: T00/#119, T01/#120, T02/#121, T03/#122, T05/#124, T08/#126, T10/#127.
- `running`: T14/#129, with the last recorded blocker being the required current-source 37-attempt Chromium matrix / exact renderer image proof.
- `pending` behind T14 or other unmet predecessors: T04/#123, T06/#125, T07/#131, T09/#135, T11/#128, T12/#132, T13/#133, T15/#130, T16/#134, T17/#136, T18/#137, T19/#138, T20/#139.
- Candidate code frontier from the persisted evidence: T14/#129 only. T04/#123 and T19/#138 remain interface/resource blocked by the recorded F08/F22/F07 evidence; no newer integration, review, or test evidence was found in the repository during this refresh.

This frontier is a report of existing evidence, not a new completion decision. A later parent-agent review should re-check the referenced checkpoints and hashes before promoting any node.

## Unresolved references / gaps

- No missing node references or reciprocal-edge mismatches were found in the requested range.
- Formal containment is absent: `#107/sub_issues` is empty. Body references to `#107` establish mapping/context only.
- The API responses were complete for the requested endpoints. Raw transient capture was held in `/tmp/craft107-live.jsonl` during analysis and is not a repository source of truth; this report records the durable summary and endpoint evidence.
- This refresh does not prove acceptance, code integration, or review completion for closed Issues. Those remain governed by the persisted implementation Ledger/checkpoints and must not be inferred from GitHub state.
