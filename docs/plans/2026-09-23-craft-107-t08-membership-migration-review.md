# T08 membership migration incremental independent review

**Scope:** Four Git-ignored Craft frontier SQL files in the integration Worktree, checkpoint-02 manifest SHA-256 `31e00b723903131f8c434e91859fdc8accd4b681a5bf6770b331b634bf404727`, compared with checkpoint-01 manifest SHA-256 `ad0256ef592bc729de9e3307f07609faed1d078a8015558aa47843954c8cb589`. Reviewed the full checkpoint copies and matching live files, not Git diff alone.

**Authority:** Approved `docs/specs/2026-09-23-craft-web-artifact-spec.md` (stories 28–31, explicit owner grant and all-operation enforcement); `CONTEXT.md` Task Grant/member permission language; ADR-0004 and ADR-0009; T08 membership migration brief and T08 rejoin analysis. The non-revival-on-rejoin rule is an inference from private-by-default and explicit grants, as the rejoin analysis states; it is not quoted as a separate Spec acceptance criterion.

## Verdict

- **Spec compliance: PASS for this incremental migration.** PostgreSQL `craft_task_grants.membership_id` is `BIGINT NOT NULL` and SQLite is `INTEGER NOT NULL`. Both types match the respective `tenant_members.id` storage types. The new table has not shipped, so no backfill or forward alteration is needed. These columns support binding a grant to one membership incarnation; runtime authorization enforcement remains T08's separate responsibility.
- **Code quality: PASS for this incremental migration.** The only content changes from checkpoint-01 are the two requested column declarations, immediately after `tenant_id`. Primary key `(tenant_id, session_id, user_id)`, lookup index `(tenant_id, user_id)`, all other columns and tables, and both rollback files are unchanged. No cascading foreign key was added.

## Findings

No critical, high, medium, or low findings in the reviewed four-file amendment.

## Evidence and limits

- Verified both manifest digests, every checkpoint-01 and checkpoint-02 file digest, and byte equality between checkpoint-02 copies and live ignored files. `.gitignore` ignores `migrations/`; the controller must explicitly carry these full-content files into the final delivery and review scope.
- Independent in-memory SQLite application of both checkpoint versions confirmed the pre-change grant table lacks `membership_id`; checkpoint-02 has `INTEGER NOT NULL`, accepts a grant with ID `42`, and rejects a grant omitting the ID. The existing recognition fields stayed nullable and returned `NULL` for an old input row. The grant index remained present. Both versions rolled back the new tables and recognition fields while preserving the old input row.
- PostgreSQL declaration and rollback were inspected for type and syntax consistency with the existing `tenant_members.id BIGSERIAL` definition. No live PostgreSQL server was available for an execution check.
- Existing SQLite `tenant_members` has a plain unique `(user_id, tenant_id)` index, unlike PostgreSQL's partial non-deleted unique index. This predates the amendment and does not change its schema verdict. It may affect how the separate T08 rejoin regression is exercised with SQLite.

**Reviewed checkpoint-02 SHA-256:**

```text
9a5b429f34e9e746573defc63ff5fd1302850c83e08fbe0ae416ab17532ba04c  migrations/versioned/000189_craft_web_artifact_frontier.up.sql
968a65b22b5f5e7a4d1ef370a7d198a6b7f1774aadac8d75bb861228b3010f0e  migrations/versioned/000189_craft_web_artifact_frontier.down.sql
b58a351d5b69eaad4f192b9ddaf533fdd5ac79825e71c30fff6c80263359e6f8  migrations/sqlite/000110_craft_web_artifact_frontier.up.sql
968a65b22b5f5e7a4d1ef370a7d198a6b7f1774aadac8d75bb861228b3010f0e  migrations/sqlite/000110_craft_web_artifact_frontier.down.sql
```
