# Fix1 Report — Canonical version replay ordering

Status: implemented and verified, uncommitted; no staging or commit.

## Finding and fix

Independent review finding R3-VM-01 identified that manifest digest is order-independent, while replay equality compared the in-memory input slice with files reloaded in path order. A valid identical replay using unsorted input could therefore conflict.

`prepareCraftVersion` now copies `in.Files` and sorts the copy by path before digest derivation and persistence. This ensures the first returned version, persisted file rows, and replay comparison share canonical ordering without mutating the caller's slice. The ordering regression now publishes reversed-order files, retries the same request, and asserts the same Version ID and canonical returned files.

## Before and after hashes

Pre-fix hashes recorded by the R3 implementation checkpoint and first review:
- `internal/application/repository/craft_version.go`: `67b7a02951a094620b41659d6b4f566e84a619578bc5783f91c484d6548df2a5`
- `internal/application/repository/craft_version_test.go`: `a4620d95259c3d9afeef31c65e97509f6e9d6107269b0bfbe5492efb55db70c9`

Final hashes, also independently reviewed for Fix1:
- `internal/application/repository/craft_version.go`: `679aed9a9511a5c13161cb3a6da6b6c97e80ac598eebf3f6b8e6417a86e17204`
- `internal/application/repository/craft_version_test.go`: `e252c7ad7416736432e63c7babf961a9adc7ba714fc21793ce41384bff2d715d`

HEAD remained `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`. The checkpoint package contains exact final source copies with an SHA-256 manifest. It preserves the preexisting R2 and R3 changes in the same owned files; the recorded pre-fix hashes bind the start of this review-fix delta.

## TDD and verification

RED:
`go test ./internal/application/repository -run '^TestCraftVersionPublishWithDraftHeadAcceptsCanonicalManifestOrdering$' -count=1` failed on retry with `craft conflict: version ... already published with different content`.

GREEN:
`go test ./internal/application/repository -run '^TestCraftVersion(PublishWithDraftHead|PublishIdempotentByIdentity|DifferentContentIsNewVersion)' -count=1` passed: `ok github.com/Tencent/WeKnora/internal/application/repository 7.122s`.

`git diff --check` passed (exit 0). PostgreSQL DSN unset; this runtime test used SQLite.

## Checkpoint

Exact frozen source copies, worktree status, and SHA-256 manifest are in `.superpowers/sdd/2026-09-29-craft-107-ocr-r3-version-manifest/fix1-checkpoint/`.
