# T08 B2: generic direct read of a Craft Task Session

> **For Codex:** Execute this scoped SDD task with RED→GREEN→REFACTOR, exact uncommitted checkpoint and independent Spec/quality review. It is one T08 acceptance slice; B3/B5 remain separate.

**Sources:** approved Craft Spec #107/#126, T08 gap map `2026-09-24-craft-107-t08-remaining-gap-map.md`, `t08-central-b-design.md`, T08 actor Task3 fix3 scoped PASS. Integration original BASE `4bcad69baf033a1310b4dce1372c8153e66adc81`; record current HEAD and pre-task owned-file hashes/content. No commits.

## Global Constraints

`GET /api/v1/sessions/:id` for a Craft-registered Task requires current explicit TaskRead for the authenticated tenant/user. It may return parent Session metadata to a granted Viewer/Collaborator, using the persisted owner only as a storage key. An ungranted tenant Admin or same-tenant nonmember gets a non-leaking denial. Generic non-Craft owner and channel Admin semantics, and `source=all` audit/list behavior, remain unchanged. No generic chat messages/writes/attachments are newly authorized by a Craft grant. Classification errors or missing/deleted registration fail closed.

## Review Focus

Craft vs ordinary Session classification, authenticated principal source, tenant boundary, owner/storage versus actor, Admin channel fallback, revoked Viewer, stale membership, no metadata/bytes on denial, no broadened generic route.

## Task 1 — current Craft TaskRead at generic direct route

**Depends on:** T08 reviewed access service/B1 interface and actor Task3 scoped PASS. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/application/service/session.go`, a new focused `internal/application/service/session_craft_read_test.go`, optionally `internal/handler/session/handler_test.go` or new focused handler test file only after exact request; no router/container changes anticipated. **Consumes:** injected `craft.TaskRunAccess` and existing `SessionRepository.GetByID`, authenticated `types` context. **Produces:** Craft-aware direct metadata read branch in `GetSession` only.

1. RED service/HTTP tests: Owner and granted Viewer read Craft metadata; ungranted tenant Admin/nonmember, cross-tenant user, revoked Viewer, stale membership and deleted registration cannot. Non-Craft owner and channel Admin retain existing behavior; Craft grant does not authorize generic write/messages. Assert denial returns no Session title/ID metadata.
2. Classify via `IsCraftTask` before owner-scoped generic lookup. For Craft, check current TaskRead using authenticated caller scope, then load the same tenant/session by ID and verify it remains Craft/active before returning metadata; map denial to the established non-leaking not-found response. For active non-Craft, call existing `loadSessionForRead` unchanged. On classification error, fail closed.
3. Run focused service/handler tests, affected `SessionShare|Craft.*Access` selection, `git diff --check`; save exact task-local patch/hashes/report for independent review.

**Acceptance:** generic direct Session metadata cannot bypass Craft grants and grants do not widen ordinary Session policy. **Failure handling:** if repository GetByID cannot safely return tenant-scoped owner row, stop and request the narrow repository seam; do not use an unscoped query.
