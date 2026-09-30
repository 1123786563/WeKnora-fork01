# Fix1 Plan — Canonical version replay ordering

**Review finding:** R3-VM-01, Medium, from `docs/plans/2026-09-29-craft-107-ocr-r3-version-manifest-review.md`.

**Goal:** Preserve idempotent replay when `craft.ManifestDigest` accepts the same file manifest independent of input ordering.

**Owned source files:**
- `internal/application/repository/craft_version.go`
- `internal/application/repository/craft_version_test.go`

**Change:** Copy `Version.Files` and sort by path in `prepareCraftVersion` before deriving identity and publishing. That gives `sameCraftVersion` the same canonical order returned by `loadCraftVersion`, while leaving the caller's backing slice untouched.

**Acceptance and verification:**
- A reversed-order first `PublishWithDraftHead` succeeds against the canonical selected-head digest.
- Retrying that identical reversed-order request succeeds with the original Version ID and canonical returned files.
- Existing mismatch/stale-head/idempotent/different-content tests remain green.
- `git diff --check` passes.

**Constraints:** No changes outside the two source files for implementation; preserve original R2/R3 uncommitted changes; no staging, commit, or integration. PostgreSQL verification is out of scope for this fix because `TRPC_TEST_POSTGRES_DSN` is unset.
