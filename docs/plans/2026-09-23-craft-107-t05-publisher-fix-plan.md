# T05 Publisher Post-Rename Recovery Fix Plan

> **For Codex:** Use SDD RED → GREEN → REFACTOR, exact checkpoint, independent Review. No commit.

**Goal:** Resolve High finding in `t05-publisher-review.md`: a crash between atomic payload rename and directory chmod leaves a complete but 0700 directory that all public publisher methods reject, stranding the accepted package. Close the Low candidate-parent fsync finding in the same narrow ownership.

**Sources:** Approved Spec #107/#124; T05 Publisher plan/report/review; `internal/container/craft_knowledge_publisher.go/test.go`; T05 service's prepared-record retry semantics. Integration HEAD `a5e9195...` with concurrent edits in other files.

**Global Constraints:** backend_implementer owns only `internal/container/craft_knowledge_publisher.go`, its focused `_test.go`, and report. Other agents own `container.go`, `craft_session.go`, T19 budget, T14 separate Worktree. Preserve all changes, do not spawn agents or commit. Do not permit delegate access to the RunView root until Publish returns and read-only mount is established. A recovery path must validate exact scope/digest/manifest and file contents before sealing; a mere directory existence or chmod is insufficient.

**Review Focus:** crash after successful rename before chmod; crash after chmod before fsync/candidate cleanup; exact same-package idempotent retry; changed bytes or symlink fails closed; parent directory fsync after candidate removal; no second publish/overwrite. All files 0444 before rename; only a private server-owned directory is temporarily 0700. Production wiring and OS read-only mount remain separate gates.

## Task 1 — recover exact published tree

1. RED: inject rename-success/next-step failure or construct a restarted publisher with exact final 0700 tree and sealed candidate. `Publish` or `Resume` must return the exact accepted package after validating all bytes, complete chmod/sync/cleanup, and be idempotent. Add corrupt-final and symlink variants that must refuse and preserve evidence. Add parent fsync assertion for candidate RemoveAll.
2. GREEN: introduce a narrow recovery function called before normal `readPublished` rejection in `Prepare`, `Publish` and `Resume` as appropriate. It recognizes only exact 0700 post-rename state with expected Run path/manifest/files and candidate seal, verifies size/SHA/contents and no symlinks, then chmods final to 0555, syncs final and immediate parent, removes candidate and syncs its immediate parent. Mismatch fails closed without removing either tree. Consider a failure after chmod but before cleanup as an idempotent published exact replay.
3. Run focused six plus new tests, `go test ./internal/container -run CraftKnowledgePublisher -count=1` once concurrent container source is stable, scoped diff check and exact full-content/hash checkpoint. Report if shared package test is temporarily blocked by unrelated in-progress compile. Do not claim T05 accepted until RunView binding/read mount, production wiring and dispatch ACL checks pass.
