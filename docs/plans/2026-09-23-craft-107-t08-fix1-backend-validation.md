# T08 Fix 1 backend validation — membership incarnation

Status: **DONE_WITH_CONCERNS** for the assigned service checkpoint. The focused service behavior and migration-backed grant binding pass. This report does not certify central HTTP enforcement for listing, direct reads, preview, or download.

## Target and integrity

- Source worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t08/WeKnora-fork01`
- Source revision: `dad8236768b1de88764c8780e017cc3615db1aa2`
- Checkpoint manifest: `.superpowers/sdd/2026-09-23-craft-107-implementation/t08-fix1-checkpoint-01/manifest.json`
- Manifest SHA-256: `8944d1ecb0e979b46445c4b432789164db375d469764b4fa8755b3c3fe50e899` (matches assigned digest)
- Checkpoint source file digests: service `ab524813c5a71cba105372f1391e5e75f9296022d6c6e0dc79c9d483e7714721`; test `40b5e0f3bbdde648832a232027f7c6b0cb5b24ee56c4998769b042cc790ddb71`.
- Report worktree revision: `929fb287e5e0ed29c6637b118fbdf0d009232728`.

## Commands and results

Run from `/Users/wuyongjun/.codex/worktrees/craft-107-t08/WeKnora-fork01`:

- `sha256sum .superpowers/sdd/2026-09-23-craft-107-implementation/t08-fix1-checkpoint-01/manifest.json` — exit 0; digest matched above.
- `go test ./internal/application/service -run 'TestCraftT08Journey|TestCraftAccessMigrationJourney' -count=1` — exit 0; `ok github.com/Tencent/WeKnora/internal/application/service 3.522s`.
- `go test ./internal/handler/session -run 'CraftAccess' -count=1` — exit 0; `ok github.com/Tencent/WeKnora/internal/handler/session 1.282s`.

The service tests exercise owner grant/revoke, cross-tenant refusal, same-tenant nonmember/admin denial, Viewer preview/read and write/share denial, Collaborator write/share boundary, audit add/revoke entries, task listing, member listing, revocation denial, tenant removal/rejoin invalidation, and regrant. The migration journey uses `openCraftSessionDB`, asserts the grant's persisted `membership_id` equals the replacement membership row, and verifies stale grant denial and regrant behavior against the migrated schema.

Migration schema inspection confirmed `membership_id` is `INTEGER NOT NULL` in `migrations/sqlite/000110_craft_web_artifact_frontier.up.sql` and `BIGINT NOT NULL` in `migrations/versioned/000189_craft_web_artifact_frontier.up.sql`, matching their respective database integer types and the grant binding used by the service.

The handler test uses a fake access service: it verifies authenticated tenant/user/session scope derivation and rejects request body tenant spoofing, then checks the grant/list route response shape. It does not prove service authorization through a live database or router-wide guarded route wiring.

## Acceptance assessment

- **Covered and passing for this checkpoint:** active membership incarnation is bound into each grant; removal/rejoin invalidates an old grant; stale grants disappear from task/member lists; only fresh Owner regrant restores access; role decisions and add/revoke audit events are exercised.
- **Not established by this service checkpoint:** authenticated central HTTP list/direct/preview/download enforcement and audit behavior across those paths. These require integrated route checks and are outside the assigned service-fix boundary. The fix report also identifies this as a remaining integration gate.
- **Cancellation:** context is passed to GORM queries and transactions by the service; no explicit cancellation behavior test was found or run. Cancellation-specific behavior is not a stated criterion for this fix.
- **Error handling:** focused tests assert forbidden/not-found behavior for relevant authorization states. Database-error propagation and transaction rollback on audit failure were not specifically fault-injected.

## Risks and runtime disclosure

The migration-backed rejoin journey drops SQLite's unconditional tenant-membership unique index in its test DB to represent a replacement membership row; the test comments identify this as SQLite/PostgreSQL index parity outside this fix. PostgreSQL runtime migration execution was not run here. The migration DDL was inspected, while the focused SQLite migration journey passed.

Actual validator runtime role/model/reasoning effort were not exposed and are not asserted.
