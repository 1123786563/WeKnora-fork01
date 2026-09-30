# Craft #107 live Issue-page supplement

- Read at: 2026-09-29 06:00 UTC (web page tool; page metadata says crawled today).
- Repository: `1123786563/WeKnora-fork01`.
- Sources: public GitHub issue pages #107 and #130–#139. GitHub REST API returned 403 for #130–#139; this supplement records accessible HTML issue pages, not API metadata.
- #107 remains Open and labeled `ready-for-agent`; page has no activity comments displayed. Its metadata says Relationships: None yet. It embeds the approved 36-story Spec.
- Every page #130–#139 was readable; each is Open and labeled `ready-for-agent`, with Parent #107 / matching Ticket key. Activity sections show no posted comments before the sign-in prompt. Formal sub-issue endpoint status for these ten remains unknown (REST API 403); browser Relationships section says `None yet` on individual issues, which does not establish absence of formal children.

## Page titles and body-declared dependencies

The GitHub issue body section `Blocked by` gives these edges (dependent → prerequisite); these are body declarations, not verified native dependency API relations:

| Issue | Ticket | Title | Blocked by |
|---|---|---|---|
| #130 | T15 | Promote web versions only after four independent checks | #123 (T04), #129 (T14) |
| #131 | T07 | Pin historical artifact versions to their original evidence | #125 (T06), #130 (T15) |
| #132 | T12 | Download a fixed source bundle with citation manifest | #131 (T07), #127 (T10) |
| #133 | T13 | Require owner consent to export restricted derived data | #128 (T11), #132 (T12) |
| #134 | T16 | Serialize Workspace-writing Runs with a durable lease | #130 (T15) |
| #135 | T09 | Let Collaborators request serialized edits with their own grants | #126 (T08), #134 (T16) |
| #136 | T17 | Persist stop intent and retain repairable drafts | #134 (T16) |
| #137 | T18 | Reconnect to the authoritative Run without resubmission | #134 (T16) |
| #138 | T19 | Pause Runs durably when Task Budget is exhausted | #119 (T00) |
| #139 | T20 | Integrate and verify the complete web-artifact journey | #121 (T02), #122 (T03), #131 (T07), #135 (T09), #133 (T13), #136 (T17), #137 (T18), #138 (T19) |

All ten pages report no comments in the visible Activity section and no assignees. Each page's Relationships panel reports `None yet`; formal children are still not established from the HTML. The page-declared edges are acyclic when combined with the already captured #119–#129 body edges. The candidate DAG is still incomplete as a full live graph because native `blocked_by`/`blocking` API relations and formal sub-issue pagination for #130–#139 were inaccessible.

## Acceptance/verification anchors

- #130 T15 requires independent build, entry, preview reachability, and browser load evidence; any failed/not-run check preserves the prior default version.
- #131 T07 binds immutable version evidence while new Runs reauthorize current sources.
- #132 T12 requires version-bound source/build/citation bundle and excludes restricted originals by default.
- #133 T13 binds owner consent to Version + manifest digest and returns no restricted bytes without valid consent.
- #134 T16 requires DB-backed writer lease, one winner under concurrent writes, and unknown outcomes retaining the fence.
- #135 T09 limits collaborator Runs to caller grant and requires a writer lease.
- #136 T17 distinguishes accepted/stopping from confirmed cancellation and retains drafts/prior version.
- #137 T18 reconnects to the same Run without resubmission and preserves unknown state.
- #138 T19 requires pre-activity budget admission, durable pause, and authorized safe resume.
- #139 T20 requires full integrated journey, all 36 stories mapped, full validation suite, and no mocked missing lanes.

## Web captures

- #107: https://github.com/1123786563/WeKnora-fork01/issues/107
- #130–#139: `https://github.com/1123786563/WeKnora-fork01/issues/<number>` (each opened/read in this refresh).
