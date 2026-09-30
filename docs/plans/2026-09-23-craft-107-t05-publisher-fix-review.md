# T05 Publisher post-rename recovery fix independent review

Date: 2026-09-23. Read-only scoped re-review in integration Worktree at HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Sources: approved Craft Spec/#124, material-boundary design, Publisher plan/report, first `t05-publisher-review.md`, fix plan and implementation report. No source/test edits, staging, commit or OCR.

## Checkpoint and verdict

Both untracked owned files match the fix report's full-content SHA-256: `internal/container/craft_knowledge_publisher.go` `ecc1c5f8d85abf5ef495fdac3c9c9c4335b084c3d8a5e038a1d10f9514c5b453`; `craft_knowledge_publisher_test.go` `962412211c6bf2bddcc7c4603d515c2ecf78edb4941eacaad70780e3bca2253f`. The report records the previous hashes, RED failures, and no commit. Other shared files were outside this scope.

- **Scoped Spec compliance: PASS for the prior High crash window.** `Prepare`, `Publish` and `Resume` now call `recoverPublished` (`craft_knowledge_publisher.go:116, 207, 306`). A visible 0700 directory is repaired only with a matching private candidate seal; the code checks final path, candidate mode/symlinks, sole seal remainder, complete final bytes, digest, per-file SHA/size and manifest/workspace scope before chmod and cleanup (`:327-415, 597-627`). Corrupt bytes, symlinks or wrong scope fail closed with evidence retained. A 0555 final after chmod is revalidated, synced and has its matching private remainder removed (`:346-369`).
- **Scoped code quality: PASS with the Low durability follow-up below.** New tests simulate interruption immediately after successful rename and after chmod, verify exact restart Resume/idempotence, corruption/symlink/wrong-scope refusal, and assert sync of the immediate per-Run candidate parent after Publish/Discard/recovery (`craft_knowledge_publisher_test.go:289-428`). The prior stranded-accepted-package and wrong-candidate-parent-sync findings are closed for the tested paths.
- **Full T05/#124 acceptance: NOT VERIFIED.** The adapter is not production-wired to a per-Run RunView or a read-only sandbox mount; dispatch authorization and inside-delegate Run A→B exclusion are still separate gates. The complete directory remains temporarily 0700 after rename and before chmod; central integration must keep the delegate stopped or root unmounted until Publish succeeds. Unix mode alone is not a delegate isolation proof.

## Low — recovery omits one directory sync used by normal Publish

**Evidence / affected symbol:** Normal `Publish` syncs the final directory, its immediate parent (`knowledge/runs`) and its grandparent (`knowledge`) after rename (`craft_knowledge_publisher.go:248-260`). Both 0700 and 0555 recovery branches sync only the final directory and immediate parent (`:358-363, 400-408`). `ensureRunParent` can create `knowledge` and `knowledge/runs` immediately before publication (`:225-227, 688-729`). If the process stops before normal Publish reaches its grandparent sync, recovery does not flush the newly created `runs` directory entry in `knowledge`.

**Impact:** On a filesystem/power-loss sequence involving newly created parent directories, a recovery that returns success may not provide the same parent-entry durability as normal Publish. This does not reintroduce partial visible bytes or the unrecoverable 0700 retry; it is a residual durability asymmetry.

**Smallest defensible correction:** In both successful `recoverPublished` branches, sync the same grandparent as normal `Publish` before returning success/removing the candidate. Add a sync-spy assertion for that path. If the deployment creates and durably syncs `knowledge/runs` before any Publisher runs, document that invariant instead.

## Verification and limits

I independently ran `go test internal/container/craft_knowledge_publisher.go internal/container/craft_knowledge_publisher_test.go -run CraftKnowledgePublisher -count=1`: PASS (`ok`, 1.532 s). The implementation report also records all ten focused tests and `go test ./internal/container -run CraftKnowledgePublisher -count=1` passing (2.139 s), plus clean formatting/whitespace checks. I did not rerun the shared container package while concurrent integration edits were active. The tests model process interruption and directory sync calls; they do not simulate a real power failure or enforce a live delegate's filesystem permissions.
