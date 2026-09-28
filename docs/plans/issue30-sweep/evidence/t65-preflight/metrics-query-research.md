# T65 Task 3 metrics query research

Research target: integrated T65 base `93706830b78205de0c7d433097e89f33d9726513`; the Adoption/Upgrade sources were inspected in the available T65 custody checkout at `/Users/wuyongjun/.codex/worktrees/issue30-t65-custody/WeKnora-fork01` (same T65 dependency line). No production source was modified.

## Verified facts

- `agent_adoptions` is tenant scoped (`id, tenant_id`), unique per `(tenant_id, listing_id)`, and stores the currently accepted release in `accepted_release_id`, lifecycle `state`, `created_at`, `updated_at`, and (after T33 migration 000203/SQLite 000124) terminal `ended_at`/`ended_by`. Service code defines `active` as the only live Adoption state; `EndAdoption` moves it to a terminal state. See `internal/types/agent_adoption_persistence.go`, `internal/application/service/agent_adoption.go`, and migration 000203/000124.
- `agent_upgrade_proposals` is tenant scoped and stores `adoption_id`, `listing_id`, `from_release_id`, `to_release_id`, `state`, `created_at`, and `updated_at`. The unique identity is `(tenant_id, adoption_id, to_release_id)`. States are `open`, `accepted`, and `dismissed`; accepted and dismissed are terminal. `to_release_id` intentionally has no FK to `agent_releases` because it may reference an adopter-local introduced release. See `internal/types/agent_upgrade_persistence.go`, `internal/application/service/agent_upgrade.go`, and migrations 000200/SQLite 000120.
- Cross-tenant public releases are copied into `tenant_introduced_releases`, whose `(tenant_id, id)` is the adopter-local ID and whose immutable `public_release_id` points to `public_agent_releases(id)`. It also stores `public_listing_id`, digest, manifest, lock, bytes, `introduced_at`; uniqueness is `(tenant_id, public_release_id)`. See migration 000193/SQLite 000114.
- Public listing/release IDs are independent of tenant listing/release IDs. Therefore a metrics query must resolve `tenant_introduced_releases.public_release_id` to the requested immutable public release; matching only `listing_id`, `accepted_release_id`, or `to_release_id` against a public ID is incorrect.
- Existing indexes: `agent_adoptions` has the unique `(tenant_id, listing_id)` index; upgrade has `(tenant_id, adoption_id, created_at)` and unique `(tenant_id, adoption_id, to_release_id)`; introduction has unique `(tenant_id, public_release_id)`. There is no purpose-built index for release-attribution joins plus date.

## Recommended aggregate seam

Keep raw counts private and return only bucketed values. Bind every value (`public_release_id`, `since`, `until`, and status literals) as parameters. Use a half-open UTC window `[since, until)`.

Adoption cohort (current active Adoption attributable to a public Release):

```sql
SELECT COUNT(DISTINCT a.tenant_id)
FROM agent_adoptions AS a
JOIN tenant_introduced_releases AS tir
  ON tir.tenant_id = a.tenant_id AND tir.id = a.accepted_release_id
JOIN public_agent_releases AS pr ON pr.id = tir.public_release_id
WHERE pr.id = ?
  AND a.state = ?                 -- "active"
  AND a.created_at >= ? AND a.created_at < ?;
```

Upgrade proposal cohort (proposal creation attributable to a public target Release):

```sql
SELECT COUNT(DISTINCT p.tenant_id)
FROM agent_upgrade_proposals AS p
JOIN tenant_introduced_releases AS tir
  ON tir.tenant_id = p.tenant_id AND tir.id = p.to_release_id
JOIN public_agent_releases AS pr ON pr.id = tir.public_release_id
WHERE pr.id = ?
  AND p.created_at >= ? AND p.created_at < ?;
```

`JOIN public_agent_releases` is an existence/lineage guard; the `tir.public_release_id = ?` predicate is the actual immutable attribution. A proposal may remain `open`, become `accepted`, or become `dismissed`; count all proposals created in the window for a “proposal volume” metric, or add `p.state IN (?, ?)` only if the product explicitly defines “resolved upgrades.” Do not silently count only `accepted`: that measures accepted upgrades, not proposals. For a terminal-only measure, use `p.state IN ('accepted','dismissed')`; terminality is state based, not `updated_at` based.

## Date and semantic cautions

- Adoption attribution by `a.created_at` measures new Adoption rows in the window. Because re-adoption updates the same row’s `accepted_release_id` and `updated_at`, a historical initial-public-release cohort can move to another release. If the requirement is “ever introduced/adopted during the window,” the current schema has no immutable Adoption event timestamp keyed to each release; `introduced_at` is only the copy/import timestamp. Do not substitute `updated_at` without an explicit product ruling.
- Upgrade attribution by `p.created_at` is stable proposal creation time. `updated_at` reflects CAS resolution and is appropriate only for a separately named “resolved in window” metric.
- Suppress internal counts `<5`; expose only `5–9`, `10–19`, `20+`. Never return the `COUNT` or tenant IDs. `not_collected` remains the correct error-category value because no immutable Run→Adoption→Release source exists.

## Index recommendation

Add, if query plans show need (both dialects):

```sql
CREATE INDEX idx_agent_adoptions_release_window
  ON agent_adoptions (tenant_id, accepted_release_id, created_at);
CREATE INDEX idx_agent_upgrade_proposals_target_window
  ON agent_upgrade_proposals (tenant_id, to_release_id, created_at);
```

These support the tenant-local introduction join and date restriction. The existing introduction unique index already supports `(tenant_id, public_release_id)` lookup. If the optimizer starts from the public release and then joins introductions, a covering order `(public_release_id, tenant_id, id)` is a possible follow-up, but should be justified by `EXPLAIN`; do not add speculative indexes. SQLite and PostgreSQL accept the same parameterized SQL and index column order.

## Inference / recommendation boundary

- The exact “active Adoption count” and “proposal volume count” definitions above are recommendations derived from the current state machines and T65 fixed-window privacy ruling. The schema itself does not state whether ended Adoptions or dismissed proposals should be excluded from each publisher-facing metric.
- Safe implementation should expose separate internal measure names if needed (`active_adoptions_created`, `upgrade_proposals_created`, optionally `upgrade_proposals_resolved`) and map each to a documented state/date predicate. Never merge them into one ambiguous count.
