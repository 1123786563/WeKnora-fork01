# T05 Atomic Material Publisher Plan

> **For Codex:** Execute with Superpowers SDD, RED → GREEN → REFACTOR, exact checkpoint, independent Review and backend validation. Do not commit.

**Goal:** Implement a durable filesystem `CraftKnowledgePackagePublisher`/`CraftKnowledgePackageResumer` for T05/#124: privately prepare a digest-verified complete candidate, atomically expose only the accepted package, resume it after process restart, and discard only the matching private candidate. Production RunView binding and dispatch-time permission recheck remain separate gates; do not wire a shared persistent Workdir as a released delegate view.

**Sources:** Approved Spec #107/#124; independently reviewed T05 fix4 service ports now copied to integration (`t05-interim-integrated-02` manifest); T05 material-boundary design and per-Run isolation design. The service interface is in `internal/application/service/craft_knowledge.go`. This task runs in integration Worktree while RunView binding work owns separate new repository/migration files.

**Global Constraints:** backend_implementer exclusively owns new `internal/container/craft_knowledge_publisher.go` and `craft_knowledge_publisher_test.go` only. No `container.go` provider wiring yet, no Craft service/repository/runtime/OpenCode/migrations/UI edits, no commits or subagents. Others share the Worktree; preserve their changes. The publisher constructor accepts a server-owned absolute root representing one isolated RunView; callers cannot provide arbitrary root/path through model or HTTP data.

**Review Focus:** Root and every target path are canonical, beneath the root, and not traversed through symlinks. Candidate directory is private (mode 0700), file content/digest/manifest is checked before exposure, files are not writable by delegate after Publish, Publish uses same-filesystem atomic rename plus durability sync, and existing Run ID with different digest conflicts. Resume after process restart returns exactly accepted bytes/digest from private or published candidate without live retrieval. A crash/failed write never exposes a partial directory. Discard cannot remove a published or different-digest package. Bounded file count/bytes; no secrets or host paths in error responses.

## Task 1 — durable atomic Publisher/Resumer

**Depends on:** reviewed T05 fix4 service contract. **Consumes:** server-owned isolated root, `CraftKnowledgeMaterialPackage` with Run ID/digest/path map. **Produces:** filesystem adapter implementing both interfaces.

1. RED: Add tests for private Prepare visibility, Publish atomic complete view, same-package replay, changed-digest conflict, failed write/rename cleanup, restart Resume before and after Publish, absent candidate, symlink/traversal escape, and hard file/byte caps. Test that previous Run directory is not implicitly copied into a new root.
2. GREEN: Implement a minimal adapter with a private candidate per Run/digest, content hashes and metadata seal, atomic rename to final directory, durable fsync of files/directories as supported. Validate every path against the package Directory and root; reject symlink components. Resume verifies seal and every byte before returning a package. Publication must not overwrite an existing different digest or expose a half-built tree.
3. REFACTOR: Run focused container adapter tests, `go test ./internal/container -run CraftKnowledgePublisher -count=1`, `git diff --check`; record RED/GREEN, full content/hash checkpoint, actual HEAD/status and all unimplemented RunView/dispatch/assembly gates.

**Failure handling:** If the filesystem cannot provide an atomic rename within the chosen root, fail closed and report the capability; do not copy into a visible final directory. No caller may bypass current T05 service ACL by invoking this adapter directly in a released path.
