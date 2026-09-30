# T08/#126 remaining acceptance gap map

Date: 2026-09-24. Read-only research in `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`; no source or test edits. Evidence was read from the live issue snapshot/DAG/implementation records, approved Craft Spec, T08 reports/reviews, and current integration source. This map does not claim a fresh full test run.

## Status at a glance

| Acceptance area | Evidence status | Remaining conclusion |
| --- | --- | --- |
| B1 private default, grant policy, list, direct Craft/version/file reads | Service/list implementation exists and was independently reviewed; exact checkpoint service rerun was blocked by concurrent compilation | **Implemented, not independently passing on the current integrated state.** Re-run exact ACL/list tests and add joined HTTP proof. |
| Membership add/revoke, cross-tenant, stale/rejoin | Grant/revoke, cross-tenant, nonmember/admin denial, membership incarnation and rejoin invalidation are covered by focused service/migration evidence | **Service seam passing/reviewed.** HTTP enforcement and denial-audit coverage remain open. |
| B2 generic direct session boundary | No reviewed implementation or passing evidence found | **Open.** Decide/implement Craft-aware `GET /sessions/:id` behavior while preserving non-Craft/Admin semantics. |
| B3 preview issue, redemption and file reads | Preview seam is owned by T14; central-B design explicitly leaves it open | **Open.** Current TaskPreview grant must be checked at issue and every capability read, including after revoke. |
| B5 joined list → direct → versions → download → preview HTTP journey | No complete authenticated real-service journey found; Task 4 explicitly says B2/B3/B5 remain outside scope | **Open final integration gate.** |
| Frontend access panel | Product mount review: scoped B4 PASS; API/route tests pass; live transport/browser not proven | **Independently reviewed for composition, not authorization.** Browser/accessibility and joined API evidence remain. |
| Audit | Grant/revoke success audit persists in real SQLite focused tests | **Partial.** No end-to-end audit assertion for denied content/list/preview/download requests. |

## Verified facts

1. `CraftAccessService` enforces current active membership incarnation plus explicit Task grant; Owner/Collaborator/Viewer action mapping is present. Focused validation covers owner grant/revoke, cross-tenant refusal, same-tenant nonmember/admin denial, Viewer/Collaborator boundaries, list/member listing, revocation, and add/revoke audit (`t08-fix1-backend-validation.md`, `t08-fix1-review.md`). The membership migration stores `membership_id`; the migration journey proves removal/rejoin does not restore the stale grant until a fresh grant.
2. B1 wires `TaskAccessChecker` into Craft service reads/writes and adds a bounded grant-aware list query. Its review confirms checks before storage resolution and zero-reader denial for downloads, and tests cover owner/grant ordering, cursor behavior, revoke, stale membership, cross-tenant and admin-without-grant (`t08-content-acl-review.md`). However, the B1 report says the final exact checkpoint service tests did not compile because concurrent source was incomplete. Earlier green results bind an earlier state only.
3. Central A is independently reviewed as assembly-owned and mounted through the guarded session route seam. Its remaining explicit obligation is central-B enforcement of existing list, direct, preview and download paths (`t08-central-a-fix1-review.md`).
4. The frontend access panel is mounted in the workbench and its API/route tests pass; the product-mount review gives scoped B4 a PASS. The same review states that mocked transport cannot prove backend authorization, revocation of content, or browser/screen-reader behavior (`t08-product-mount-review.md`).
5. The approved Spec requires private-by-default Tasks, explicit Owner/Collaborator/Viewer access, and application of roles to all Craft operations. Source links must use the viewer's own authority; sharing does not grant original-source access (`issue-snapshot.json`, approved Spec text).

## Remaining gaps by acceptance

### B1: private default and content/list reads

**Evidence:** The B1 service/list code is present and scoped review is favorable on policy, but the exact final checkpoint lacked a green service run. The current worktree contains the B1 files and broad concurrent changes; no fresh claim is made from that dirty state.

**Gap:** Re-run the focused B1 ACL/list, Craft service, handler and container tests against the exact integrated source. Then prove through authenticated HTTP that an owner/granted Viewer/Collaborator can list and read permitted Craft content, while an ungranted tenant admin, nonmember, cross-tenant user, revoked member and stale/rejoined member cannot. Assert denied download opens no reader/bytes and no metadata leak.

**Minimal ownership:** Backend implementer/validator: `internal/application/service/craft_session.go`, `craft_task_list_query.go`, focused service tests, and the existing Craft handler seam only as needed for B1. Integrator owns a new joined test file, not production policy code.

**RED test:** Seed owner Task + Viewer grant + ungranted Admin + cross-tenant user; call the real authenticated list, workspace, version metadata and file endpoints, revoke, then repeat. Expect grant success and 403/404 plus zero bytes after denial. Add a rejoin row with a new membership ID and expect denial until regrant.

### Membership add/revoke and cross-tenant

**Evidence:** The service and migration journey are already independently reviewed/passing for current membership incarnation, cross-tenant refusal, grant/revoke, and audit success rows.

**Gap:** This is no longer a service-policy gap. The missing proof is that the same current check is reached through production authenticated content routes and that revocation immediately affects existing capabilities/reads. Also retain the SQLite index-parity limitation recorded in `t08-fix1-review.md`; it is not evidence of runtime failure.

**Minimal ownership:** Integrator/backend validator owns the real-route journey and audit assertions; do not duplicate or rewrite the reviewed service tests.

**RED test:** Owner grants Viewer; Viewer succeeds on list/read/preview/download; owner revokes; each same request family fails on the next call, including an already issued preview capability. Assert grant/revoke audit rows and a defined denial audit contract if required by the approved API policy.

### B2: generic direct-session read

**Evidence:** Central-B seam map identifies `GET /api/v1/sessions/:id` as a separate generic path whose Admin fallback is channel-specific and whose Craft-grant behavior is undecided. No B2 implementation review or passing test is recorded.

**Gap:** Decide and enforce whether a Craft-registered session's generic direct metadata read requires `TaskRead`; ensure ordinary non-Craft owner reads, channel Admin fallback and `source=all` audit semantics remain unchanged.

**Minimal ownership:** Backend implementer owns a narrow Craft-aware guard/adapter and handler/router tests; leave generic session behavior unchanged except the explicit Craft branch. Integrator owns the cross-route assertion.

**RED test:** Create a Craft session and a non-Craft session. Un granted tenant Admin requests both generic direct reads; Craft must be denied/hidden, while the non-Craft result preserves its existing policy. Grant Viewer, then assert Craft metadata read succeeds; revoke and assert it fails.

### B3: preview issue, redemption and download-like file reads

**Evidence:** The seam map records no `TaskAccessChecker` in the current preview config and states capability redemption/read rechecks expiry/manifest but not current membership. T14 owns preview implementation/configuration; no reviewed T08 B3 result exists.

**Gap:** Require `TaskPreview` at authenticated ticket issuance, capability redemption, and every capability file read. Preserve isolated preview origin and do not add main-site cookies/tokens. Revoke/member removal between each stage must deny.

**Minimal ownership:** T14 preview owner implements the checker seam; central container owner wires the reviewed interface; backend validator supplies focused tests. Do not edit preview source from the T08 B1 lane.

**RED test:** Issue ticket as Viewer, revoke before redemption and expect denial; separately redeem once, revoke before second file read and expect denial/no bytes; repeat with membership removal/rejoin and cross-tenant identity. Assert capability cannot outlive current TaskPreview authority.

### B5: joined HTTP/audit gate

**Evidence:** `t08-joined-validation.md` explicitly limits itself to actor Task 4 and says B2/B3/B5 remain. Central-A and product-mount reviews both state that a real authenticated end-to-end journey is outstanding.

**Gap:** One integrated test must traverse the production route/auth seam with the real access service and migrations: list → direct Craft read → versions → download bytes → preview issue/redeem/read, then revoke and repeat. Include ordinary admin without explicit grant, anonymous/API-key guards, cross-tenant and stale membership. Assert audit persistence for grant/revoke and the approved denial audit behavior.

**Minimal ownership:** Integrator owns only new integration test fixtures/reporting; backend/frontend validators review behavior. No production source changes should be inferred from a failing joined test until the responsible seam is identified.

**RED test:** The sequence above must fail before any missing B2/B3/B5 guard is added, then pass with role-specific responses and zero denied bytes. Keep the test bound to the same source/migration revision as the final report.

## Recommendations (inference)

- Treat B1 and membership policy as **reviewed implementation with an outstanding exact-state rerun**, not as missing functionality and not as fully verified.
- Sequence work as B2 decision/guard, T14-owned B3, then one B5 joined test. B4 frontend can be accepted independently as composition, but must not be used as evidence for server authorization.
- Keep audit scope explicit: existing evidence proves successful grant/revoke audit only. If the approved acceptance requires denied-request audit, add that assertion at the joined HTTP seam rather than inferring it from service errors.
- Preserve the actor identity boundary already reviewed by Task 1–4: collaborator TaskWrite/Run admission and Craft content ACL are related but separate checks; do not replace Task owner storage identity with the requesting actor.

## Source pointers

- `docs/plans/2026-09-23-craft-107-issue-snapshot.json`
- `docs/plans/2026-09-23-craft-107-dag.md`
- `docs/plans/2026-09-23-craft-107-t08-review.md`
- `docs/plans/2026-09-23-craft-107-t08-content-acl-report.md`
- `docs/plans/t08-content-acl-review.md`
- `docs/plans/2026-09-23-craft-107-t08-fix1-backend-validation.md`
- `docs/plans/2026-09-23-craft-107-t08-fix1-review.md`
- `docs/plans/2026-09-23-craft-107-t08-central-a-fix1-review.md`
- `docs/plans/2026-09-23-craft-107-t08-central-b-design.md`
- `docs/plans/2026-09-23-craft-107-t08-product-mount-review.md`
- `docs/plans/2026-09-24-craft-107-t08-joined-validation.md`

