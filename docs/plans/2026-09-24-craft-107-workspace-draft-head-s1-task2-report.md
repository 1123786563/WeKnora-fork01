# Craft Workspace Draft Head S1 — Task 2 Report

## Scope and behavior

Implemented `DraftHeadStore.Advance` in the owned repository adapter with transaction-scoped head CAS and an insert-only revision/file manifest. It validates and sorts the submitted manifest, derives `ManifestDigest`, checks byte/path/hash/ref invariants, returns a defensive copy reconstructed from the inserted database rows, and rejects reuse of a source Run for another revision. Before advancing it validates the current selected revision and digest, locks/authorizes the Workspace and source Run, requires a durable terminal Run (`succeeded`, `failed`, or `canceled`), requires an empty session active Run slot, and rejects unfinished agent tool calls or Craft delegations. Any error rolls back the head, revision, and file writes. No Task 1 SQL migration was changed, and no `craft_versions` row is written.

`Advance` has an explicit caller precondition: the trusted caller has already verified the RunView is stopped and idle while holding the Workspace writer fence. The repository checks durable identity/terminal and writer journal facts but cannot prove filesystem quiescence by itself. S3 must supply that capture/fence authority before production use.

## RED → GREEN and verification

- RED: `go test ./internal/application/repository -run 'DraftHead' -count=1` failed on the existing `ErrUnsupported` stub for first revision, CAS, invalid manifest, and Run authorization expectations.
- GREEN SQLite: `go test ./internal/modules/craft ./internal/application/repository -run 'DraftHead' -count=1` passed.
- GREEN PostgreSQL: using a disposable `paradedb/paradedb:v0.22.2-pg17` container, `TRPC_TEST_POSTGRES_DSN='postgres://craft:craft@localhost:55439/craft?sslmode=disable' go test ./internal/modules/craft ./internal/application/repository -run 'DraftHead' -count=1` passed. This includes parallel same-revision CAS, sequential revisions, source/run checks and restart reads.
- Race: `go test ./internal/application/repository -run 'DraftHead' -race -count=1` passed.
- SQLite transaction rollback test injects a file insert failure after the head and revision writes, then verifies all three remain unchanged.
- Other assertions cover first and second selected revisions, CAS loser has no extra rows, source Run cannot be reused, unauthorized Workspace/foreign session Run, failed terminal capture fixture, running/waiting/unknown/reconciling-stop-requested rejection, active session writer slot, prepared/unknown delegation, unknown tool call, empty/duplicate/traversal/credential/malformed hash/negative/oversized/missing-ref manifests, digest tampering, defensive input/output slices, restart read and no `craft_versions` mutation.
- `git diff --check` passed. `gofmt -d` on changed Go files returned no output. The temporary PostgreSQL container was removed.

## Checkpoint

- Task 2 preimage is the reviewed S1 Task 1 fix1 checkpoint. Pre/post file hashes and both checkpoint patch hashes are in `2026-09-24-craft-107-workspace-draft-head-s1-task2-checkpoint.json`.
- Task-local incremental patch: `2026-09-24-craft-107-workspace-draft-head-s1-task2-checkpoint.patch`.
- No index changes or commits were made; unrelated shared files were not changed.

## Assumptions and remaining risks

- The approved material has no explicit Workspace draft byte quota. Validation caps total selected manifest bytes at 8 GiB, matching the repository's existing `MaxArtifactVersionBytes` ceiling. Review should confirm this is an acceptable S1 bound.
- A durable terminal Run plus clear DB writer signals still does not prove a sandbox process is idle; only the documented trusted RunView caller contract can provide that proof. No production caller exists in this task.
