# Craft container webhook provider repair — Task 1 report

## Scope and checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-webhook-provider/WeKnora-fork01`
- Baseline and final HEAD: `6e1b2072a13a798e7ec25be44784e5242bd2f178` (no commit)
- Task: add a real `BuildContainer` router resolution regression assertion, then register the missing commercial webhook handler provider.
- Owned source files only: `internal/container/bootsmoke/boot_smoke_test.go`, `internal/container/container.go`.
- Initial tracked diff: empty; initial scoped diff SHA-256 (empty input): `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.
- Initial untracked files preserved with hashes:
  - `docs/plans/2026-10-06-craft-107-execution-dag.md` — `fd3615c1b889e5e9cf772cfbc60c2e42ede46ccf6b8757216f1c5966db2a1216`
  - `docs/plans/2026-10-06-craft-107-live-issue-refresh.md` — `cf7335866be6c5c8647d07f6b1beda1a0cf6746d58712aed195d7c082a2d37b3`
  - `docs/superpowers/plans/2026-10-06-craft-container-webhook-provider-fix.md` — `52bdd8f3df7bd75d256d7ca663e1caf8e633009974a14ce42d95f30c96abbca3`

## Changes

- The boot smoke test now asks the real `BuildContainer` DI graph for `*gin.Engine` and requires a non-nil result.
- `BuildContainer` now provides `handler.NewCommercialWebhookHandler` immediately after providing `newCommercialWebhookService`. No service behavior or route behavior changed.

## TDD and verification evidence

1. RED: edited only the boot smoke test, then ran:
   ```text
   go test ./internal/container/bootsmoke -run TestBuildContainerBootsLite -count=1
   ```
   Result: failed at the new `c.Invoke` assertion as expected. Dig reported `failed to build *gin.Engine` from `router.NewRouter` and the precise missing dependency `*handler.CommercialWebhookHandler` (router.go:177). Exit status 1.
2. GREEN: registered `handler.NewCommercialWebhookHandler`, then reran the same command. Result: `ok github.com/Tencent/WeKnora/internal/container/bootsmoke 2.401s` (exit status 0).
3. Handler regression tests:
   ```text
   go test ./internal/handler -run 'TestCommercialWebhook' -count=1
   ```
   Result: `ok github.com/Tencent/WeKnora/internal/handler 1.114s` (exit status 0).
4. Router commercial tests: `rg -n '^func Test.*Commercial' internal/router --glob '*_test.go'` found matching tests; then ran:
   ```text
   go test ./internal/router -run 'TestCommercial' -count=1
   ```
   Result: `ok github.com/Tencent/WeKnora/internal/router 1.711s` (exit status 0).
5. Scoped whitespace check:
   ```text
   git diff --check -- internal/container/bootsmoke/boot_smoke_test.go internal/container/container.go
   ```
   Result: clean (exit status 0).

## Final diff and hashes

`git diff --binary -- internal/container/bootsmoke/boot_smoke_test.go internal/container/container.go` SHA-256:

```text
47793c658a15852ca42e5cd577347c607ed859df236973b9d52cc96f1e009ca4
```

Final source file SHA-256:

```text
e56723e5f15c7953d39fa86c8ae799e8c73c461098017c779c15e7b78f00d23a  internal/container/bootsmoke/boot_smoke_test.go
51e083922d82feafe00237e68a8436ce27fdac49c2812af35d81b581848b7e21  internal/container/container.go
```

Scoped patch:

```diff
--- a/internal/container/bootsmoke/boot_smoke_test.go
+++ b/internal/container/bootsmoke/boot_smoke_test.go
@@
 	"github.com/Tencent/WeKnora/internal/handler/session"
 	"github.com/Tencent/WeKnora/internal/types/interfaces"
+	"github.com/gin-gonic/gin"
@@
 	require.False(t, claims.IsNil(), "wireAgentSecuritySessionClaims must observe a live session handler")
+
+	var router *gin.Engine
+	require.NoError(t, c.Invoke(func(engine *gin.Engine) { router = engine }))
+	require.NotNil(t, router)
 
 	var cleaner interfaces.ResourceCleaner
--- a/internal/container/container.go
+++ b/internal/container/container.go
@@
 	must(container.Provide(newCommercialWebhookService))
+	must(container.Provide(handler.NewCommercialWebhookHandler))
 	must(container.Provide(newCommercialReconciliationService))
```

No staging or commit was performed. The three initial untracked input documents remain unchanged. The task report is the only additional untracked file.
